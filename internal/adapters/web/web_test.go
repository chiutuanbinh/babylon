package web_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"babylon/internal/adapters/web"
	"babylon/internal/identity"
	"babylon/internal/identity/identitytest"
)

func newServer(t *testing.T, secure bool, admins ...int64) (http.Handler, *identitytest.Env) {
	t.Helper()
	env := identitytest.NewEnv(admins...)
	h := &web.Handler{
		Auth:          env.Service,
		Tokens:        env.Tokens,
		Log:           slog.New(slog.DiscardHandler),
		SecureCookies: secure,
	}
	mux := http.NewServeMux()
	h.Mount(mux)
	return mux, env
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %q cookie in %v", name, cookies)
	return nil
}

func do(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// startLogin runs GET /auth/login and returns the state and the state cookie.
func startLogin(t *testing.T, h http.Handler) (string, *http.Cookie) {
	t.Helper()
	rec := do(h, httptest.NewRequest("GET", "/auth/login", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("login status = %d, want 302", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return loc.Query().Get("state"), findCookie(t, rec.Result().Cookies(), "oauth_state")
}

// login drives the whole OAuth flow through HTTP and returns the session cookie.
func login(t *testing.T, h http.Handler, code string) *http.Cookie {
	t.Helper()
	state, stateCookie := startLogin(t, h)
	req := httptest.NewRequest("GET", "/auth/callback?code="+code+"&state="+url.QueryEscape(state), nil)
	req.AddCookie(stateCookie)
	rec := do(h, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
		t.Fatalf("callback = %d -> %q, want 302 -> /", rec.Code, rec.Header().Get("Location"))
	}
	return findCookie(t, rec.Result().Cookies(), "session")
}

func getMe(h http.Handler, session *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/me", nil)
	if session != nil {
		req.AddCookie(session)
	}
	return do(h, req)
}

func TestLoginRedirectsToProviderWithStateCookie(t *testing.T) {
	h, _ := newServer(t, false)
	rec := do(h, httptest.NewRequest("GET", "/auth/login", nil))
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if loc.Host != "github.test" || loc.Query().Get("state") == "" {
		t.Fatalf("Location = %q", rec.Header().Get("Location"))
	}
	c := findCookie(t, rec.Result().Cookies(), "oauth_state")
	if c.Value != loc.Query().Get("state") || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Secure {
		t.Fatalf("state cookie = %+v", c)
	}
}

func TestCookiesAreSecureWhenConfigured(t *testing.T) {
	h, _ := newServer(t, true)
	rec := do(h, httptest.NewRequest("GET", "/auth/login", nil))
	if c := findCookie(t, rec.Result().Cookies(), "oauth_state"); !c.Secure {
		t.Fatal("state cookie not Secure")
	}
	session := login(t, h, "code-alice")
	if !session.Secure || !session.HttpOnly || session.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie = %+v", session)
	}
}

// The core multi-user scenario: each person sees only their own identity, and
// the admin allowlist is honoured.
func TestMeShowsEachUsersOwnIdentity(t *testing.T) {
	h, _ := newServer(t, false, 1)
	alice := login(t, h, "code-alice")
	bob := login(t, h, "code-bob")
	root := login(t, h, "code-root")

	tests := []struct {
		name    string
		session *http.Cookie
		want    []string
		notWant []string
	}{
		{"alice", alice, []string{"alice", "(user)"}, []string{"bob", "Root", "(admin)"}},
		{"bob", bob, []string{"bob", "(user)"}, []string{"alice", "Root", "(admin)"}},
		{"root", root, []string{"Root", "(admin)"}, []string{"alice", "bob"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := getMe(h, tc.session)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			body := rec.Body.String()
			for _, s := range tc.want {
				if !strings.Contains(body, s) {
					t.Errorf("body missing %q: %s", s, body)
				}
			}
			for _, s := range tc.notWant {
				if strings.Contains(body, s) {
					t.Errorf("body leaks %q: %s", s, body)
				}
			}
		})
	}
}

func TestMeRedirectsAnonymousToLogin(t *testing.T) {
	h, _ := newServer(t, false)
	rec := getMe(h, nil)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/auth/login" {
		t.Fatalf("got %d -> %q, want 302 -> /auth/login", rec.Code, rec.Header().Get("Location"))
	}
}

func TestMeUsesHXRedirectForHtmx(t *testing.T) {
	h, _ := newServer(t, false)
	req := httptest.NewRequest("GET", "/me", nil)
	req.Header.Set("HX-Request", "true")
	rec := do(h, req)
	if rec.Code != http.StatusOK || rec.Header().Get("HX-Redirect") != "/auth/login" {
		t.Fatalf("got %d, HX-Redirect=%q", rec.Code, rec.Header().Get("HX-Redirect"))
	}
}

func TestMeRejectsBogusAndExpiredSessions(t *testing.T) {
	h, env := newServer(t, false)
	if rec := getMe(h, &http.Cookie{Name: "session", Value: "forged"}); rec.Code != http.StatusFound {
		t.Fatalf("forged cookie status = %d, want 302", rec.Code)
	}
	session := login(t, h, "code-alice")
	env.Clock.T = env.Clock.T.Add(identitytest.SessionTTL)
	if rec := getMe(h, session); rec.Code != http.StatusFound {
		t.Fatalf("expired session status = %d, want 302", rec.Code)
	}
}

type brokenSessions struct{}

func (brokenSessions) Create(context.Context, identity.Session) error { return errors.New("db down") }
func (brokenSessions) ByTokenHash(context.Context, string) (identity.Session, error) {
	return identity.Session{}, errors.New("db down")
}
func (brokenSessions) Delete(context.Context, string) error { return errors.New("db down") }

// A database outage must not look like "logged out" (which would bounce users
// into a login loop); it is a 500.
func TestMeBackendFailureIs500(t *testing.T) {
	h, env := newServer(t, false)
	env.Service.Sessions = brokenSessions{}
	rec := getMe(h, &http.Cookie{Name: "session", Value: "anything"})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestCallbackRejectsBadState(t *testing.T) {
	h, _ := newServer(t, false)
	state, stateCookie := startLogin(t, h)

	tests := []struct {
		name  string
		query string
		add   *http.Cookie
	}{
		{"wrong state", "code=code-alice&state=forged", stateCookie},
		{"missing state param", "code=code-alice", stateCookie},
		{"missing state cookie", "code=code-alice&state=" + url.QueryEscape(state), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/auth/callback?"+tc.query, nil)
			if tc.add != nil {
				req.AddCookie(tc.add)
			}
			rec := do(h, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			for _, c := range rec.Result().Cookies() {
				if c.Name == "session" && c.MaxAge >= 0 {
					t.Fatalf("session cookie set on rejected callback: %+v", c)
				}
			}
		})
	}
}

func TestCallbackProviderFailureIs502(t *testing.T) {
	h, env := newServer(t, false)
	state, stateCookie := startLogin(t, h)
	req := httptest.NewRequest("GET", "/auth/callback?code=nope&state="+url.QueryEscape(state), nil)
	req.AddCookie(stateCookie)
	rec := do(h, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	if env.Sessions.Len() != 0 {
		t.Fatal("session created on failed login")
	}
}

func TestCallbackMissingCodeIs400(t *testing.T) {
	h, _ := newServer(t, false)
	state, stateCookie := startLogin(t, h)
	req := httptest.NewRequest("GET", "/auth/callback?state="+url.QueryEscape(state), nil)
	req.AddCookie(stateCookie)
	if rec := do(h, req); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestLogout(t *testing.T) {
	h, _ := newServer(t, false)
	session := login(t, h, "code-alice")

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	req.AddCookie(session)
	rec := do(h, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("logout = %d -> %q, want 303 -> /", rec.Code, rec.Header().Get("Location"))
	}
	if c := findCookie(t, rec.Result().Cookies(), "session"); c.MaxAge >= 0 {
		t.Fatalf("session cookie not cleared: %+v", c)
	}
	// The old cookie is dead server-side too, not just cleared in the browser.
	if rec := getMe(h, session); rec.Code != http.StatusFound {
		t.Fatalf("old session still works after logout: %d", rec.Code)
	}
}

func TestLogoutRequiresPost(t *testing.T) {
	h, _ := newServer(t, false)
	if rec := do(h, httptest.NewRequest("GET", "/auth/logout", nil)); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /auth/logout = %d, want 405", rec.Code)
	}
}

func TestMeEscapesLogin(t *testing.T) {
	h, env := newServer(t, false)
	env.Provider.Codes["code-xss"] = identity.Profile{GitHubID: 9, Login: "<script>alert(1)</script>"}
	rec := getMe(h, login(t, h, "code-xss"))
	if strings.Contains(rec.Body.String(), "<script>") {
		t.Fatalf("login not escaped: %s", rec.Body.String())
	}
}
