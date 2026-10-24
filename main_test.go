package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"babylon/internal/adapters/sim"
	"babylon/internal/adapters/web"
	"babylon/internal/identity/identitytest"
)

func TestRoutes(t *testing.T) {
	h := routes()
	tests := []struct {
		method, path string
		status       int
		body         string
	}{
		{"GET", "/", 200, `hx-post="/greet"`},
		{"GET", "/time", 200, `<time>`},
		{"GET", "/healthz", 200, `"status":"ok"`},
		{"GET", "/hello/bob", 200, `hello, bob`},
		{"POST", "/healthz", 405, ""},
		{"GET", "/nope", 404, ""},
	}
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if !strings.Contains(rec.Body.String(), tc.body) {
				t.Fatalf("body = %q, want contain %q", rec.Body.String(), tc.body)
			}
		})
	}
}

func TestGreet(t *testing.T) {
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/greet", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		routes().ServeHTTP(rec, req)
		return rec
	}

	if rec := post("name=Ada"); rec.Code != 200 || rec.Body.String() != "Hello, Ada!" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	if rec := post("name=+++"); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank name: status = %d, want 400", rec.Code)
	}
	rec := post("name=%3Cscript%3Ealert(1)%3C%2Fscript%3E")
	if strings.Contains(rec.Body.String(), "<script>") {
		t.Fatalf("name not escaped: %q", rec.Body.String())
	}
}

func TestRoutesMountsExtraRoutes(t *testing.T) {
	h := routes(func(mux *http.ServeMux) {
		mux.HandleFunc("GET /extra", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("mounted"))
		})
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/extra", nil))
	if rec.Code != 200 || rec.Body.String() != "mounted" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	// Existing routes still work alongside mounts.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 200 {
		t.Fatalf("/healthz = %d", rec.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := logging(slog.New(slog.NewTextHandler(io.Discard, nil)), securityHeaders(routes()))
	want := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "same-origin",
		"Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'",
	}
	for _, path := range []string{"/healthz", "/"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		for k, v := range want {
			if got := rec.Header().Get(k); got != v {
				t.Errorf("GET %s: %s = %q, want %q", path, k, got, v)
			}
		}
	}
}

func TestIndexUsesVendoredHTMX(t *testing.T) {
	rec := httptest.NewRecorder()
	routes().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	body := rec.Body.String()
	if !strings.Contains(body, `src="/static/sim/vendor/htmx/htmx.min.js"`) {
		t.Errorf("index does not load vendored htmx: %s", body)
	}
	if strings.Contains(body, "jsdelivr") {
		t.Error("index still references jsdelivr")
	}
}

func TestBuildHandlerFullChain(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	env := identitytest.NewEnv()
	ui := &web.Handler{Auth: env.Service, Tokens: env.Tokens, Log: log}
	simUI, err := sim.New()
	if err != nil {
		t.Fatal(err)
	}
	h := buildHandler(log, ui.Mount, simUI.Mount) // panics on a route clash between demo, auth and sim routes

	tests := []struct {
		path, location string
		status         int
	}{
		{"/healthz", "", 200},
		{"/me", "/auth/login", 302},
		{"/auth/login", "", 302},
		{"/sim", "", 200},
		{"/static/sim/vendor/htmx/htmx.min.js", "", 200},
	}
	for _, tc := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", tc.path, nil))
		if rec.Code != tc.status {
			t.Errorf("GET %s: status = %d, want %d", tc.path, rec.Code, tc.status)
		}
		if tc.location != "" && rec.Header().Get("Location") != tc.location {
			t.Errorf("GET %s: Location = %q, want %q", tc.path, rec.Header().Get("Location"), tc.location)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Content-Security-Policy") == "" {
			t.Errorf("GET %s: security headers missing: %v", tc.path, rec.Header())
		}
	}
}

// deadlineWriter records SetWriteDeadline calls, like the real connection
// writer that http.ResponseController reaches through Unwrap.
type deadlineWriter struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func (d *deadlineWriter) SetWriteDeadline(t time.Time) error {
	d.deadlines = append(d.deadlines, t)
	return nil
}

func TestAssetExtendsWriteDeadlineThroughChain(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	simUI, err := sim.New()
	if err != nil {
		t.Fatal(err)
	}
	h := buildHandler(log, simUI.Mount)
	w := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	h.ServeHTTP(w, httptest.NewRequest("GET", "/static/sim/vendor/three/three.core.js", nil))
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if len(w.deadlines) != 1 {
		t.Fatalf("SetWriteDeadline calls = %d, want 1 (not reachable through the middleware chain?)", len(w.deadlines))
	}
	if d := time.Until(w.deadlines[0]); d < 30*time.Second {
		t.Errorf("deadline in %v, want well beyond the server's 10s WriteTimeout", d)
	}
}
