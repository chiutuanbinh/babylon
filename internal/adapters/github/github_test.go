package github_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"babylon/internal/adapters/github"
)

// fakeGitHub serves the three endpoints the adapter talks to. userStatus is the
// status returned by GET /user.
func fakeGitHub(t *testing.T, userStatus int) *httptest.Server {
	t.Helper()
	return fakeGitHubWith(t, userStatus, `{"id":4242,"login":"alice"}`, false)
}

// fakeGitHubWith also controls the /user body and, with bad200, mimics GitHub's
// real bad-code response: HTTP 200 carrying {"error": ...}.
func fakeGitHubWith(t *testing.T, userStatus int, userBody string, bad200 bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.PostForm.Get("code") != "good-code" {
			if !bad200 {
				w.WriteHeader(http.StatusBadRequest)
			}
			_, _ = io.WriteString(w, `{"error":"bad_verification_code"}`)
			return
		}
		_, _ = io.WriteString(w, `{"access_token":"gh-token","token_type":"bearer"}`)
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer gh-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if userStatus != http.StatusOK {
			w.WriteHeader(userStatus)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, userBody)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newProvider(srv *httptest.Server) *github.Provider {
	return github.New(github.Config{
		ClientID:     "cid",
		ClientSecret: "secret",
		RedirectURL:  "http://localhost:8080/auth/callback",
		AuthURL:      srv.URL + "/login/oauth/authorize",
		TokenURL:     srv.URL + "/login/oauth/access_token",
		APIURL:       srv.URL,
	})
}

func TestAuthURL(t *testing.T) {
	p := newProvider(fakeGitHub(t, http.StatusOK))
	u, err := url.Parse(p.AuthURL("st8"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("state") != "st8" || q.Get("client_id") != "cid" ||
		q.Get("redirect_uri") != "http://localhost:8080/auth/callback" {
		t.Fatalf("query = %v", q)
	}
}

func TestExchangeReturnsProfile(t *testing.T) {
	p := newProvider(fakeGitHub(t, http.StatusOK))
	got, err := p.Exchange(t.Context(), "good-code")
	if err != nil {
		t.Fatal(err)
	}
	if got.GitHubID != 4242 || got.Login != "alice" {
		t.Fatalf("profile = %+v", got)
	}
}

func TestExchangeRejectsBadCode(t *testing.T) {
	p := newProvider(fakeGitHub(t, http.StatusOK))
	if _, err := p.Exchange(t.Context(), "bad-code"); err == nil {
		t.Fatal("expected error for bad code")
	}
}

func TestExchangeFailsWhenUserAPIFails(t *testing.T) {
	p := newProvider(fakeGitHub(t, http.StatusInternalServerError))
	if _, err := p.Exchange(t.Context(), "good-code"); err == nil {
		t.Fatal("expected error when GET /user returns 500")
	}
}

func TestExchangeRejectsBadCodeReturnedAs200(t *testing.T) {
	p := newProvider(fakeGitHubWith(t, http.StatusOK, `{"id":4242,"login":"alice"}`, true))
	if _, err := p.Exchange(t.Context(), "bad-code"); err == nil {
		t.Fatal("expected error for GitHub's 200 + error body")
	}
}

func TestExchangeRejectsBadUserBodies(t *testing.T) {
	for name, body := range map[string]string{
		"empty object":  `{}`,
		"not json":      `<html>oops</html>`,
		"missing id":    `{"login":"alice"}`,
		"missing login": `{"id":4242}`,
	} {
		t.Run(name, func(t *testing.T) {
			p := newProvider(fakeGitHubWith(t, http.StatusOK, body, false))
			if _, err := p.Exchange(t.Context(), "good-code"); err == nil {
				t.Fatalf("expected error for /user body %q", body)
			}
		})
	}
}
