// Package web is the HTTP adapter for the identity context: GitHub login,
// logout, session middleware and the /me page.
package web

import (
	"bytes"
	"context"
	"crypto/subtle"
	"embed"
	"errors"
	"html/template"
	"log/slog"
	"net/http"

	"babylon/internal/identity"
)

//go:embed templates/*.html
var assets embed.FS

var tmpl = template.Must(template.ParseFS(assets, "templates/*.html"))

const (
	sessionCookie = "session"
	stateCookie   = "oauth_state"
)

type Handler struct {
	Auth          *identity.Service
	Tokens        identity.TokenGenerator // used for the OAuth state value
	Log           *slog.Logger
	SecureCookies bool
}

// Mount registers the identity routes on mux.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/login", h.login)
	mux.HandleFunc("GET /auth/callback", h.callback)
	mux.HandleFunc("POST /auth/logout", h.logout)
	mux.HandleFunc("GET /me", h.requireUser(h.me))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	state, err := h.Tokens.New()
	if err != nil {
		h.Log.Error("new state", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.setCookie(w, &http.Cookie{Name: stateCookie, Value: state, Path: "/auth", MaxAge: 600})
	http.Redirect(w, r, h.Auth.LoginURL(state), http.StatusFound)
}

func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	got := r.URL.Query().Get("state")
	c, err := r.Cookie(stateCookie)
	if err != nil || got == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(got)) != 1 {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	h.setCookie(w, &http.Cookie{Name: stateCookie, Path: "/auth", MaxAge: -1})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	res, err := h.Auth.LoginWithGitHub(r.Context(), code)
	if err != nil {
		h.Log.Error("login failed", "err", err)
		http.Error(w, "login failed", http.StatusBadGateway)
		return
	}
	h.setCookie(w, &http.Cookie{Name: sessionCookie, Value: res.Token, Path: "/", Expires: res.ExpiresAt})
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := h.Auth.Logout(r.Context(), c.Value); err != nil {
			h.Log.Error("logout", "err", err)
		}
	}
	h.setCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFrom(r.Context())
	render(w, "me", u)
}

type userKey struct{}

// UserFrom returns the signed-in user placed in the context by requireUser.
func UserFrom(ctx context.Context) (identity.User, bool) {
	u, ok := ctx.Value(userKey{}).(identity.User)
	return u, ok
}

// requireUser resolves the session cookie and calls next with the user in the
// context. Not-logged-in sends the browser to /auth/login (via HX-Redirect for
// htmx requests, so a login page is never swapped into a fragment); an
// infrastructure error is a 500, not a login bounce.
func (h *Handler) requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := h.currentUser(r)
		switch {
		case errors.Is(err, identity.ErrUnauthenticated):
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/auth/login")
				w.WriteHeader(http.StatusOK)
				return
			}
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		case err != nil:
			h.Log.Error("authenticate", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
	}
}

func (h *Handler) currentUser(r *http.Request) (identity.User, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return identity.User{}, identity.ErrUnauthenticated
	}
	return h.Auth.Authenticate(r.Context(), c.Value)
}

// setCookie applies the attributes every cookie here shares.
func (h *Handler) setCookie(w http.ResponseWriter, c *http.Cookie) {
	c.HttpOnly = true
	c.Secure = h.SecureCookies
	c.SameSite = http.SameSiteLaxMode
	http.SetCookie(w, c)
}

// render executes into a buffer first so a template error is a clean 500.
func render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}
