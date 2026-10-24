package sim

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newMux(t *testing.T) http.Handler {
	t.Helper()
	h, err := New()
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h.Mount(mux)
	return mux
}

func TestRoutes(t *testing.T) {
	mux := newMux(t)
	tests := []struct {
		method, path string
		status       int
		typ          string // substring of Content-Type, checked when non-empty
	}{
		{"GET", "/sim", 200, "text/html"},
		{"GET", "/sim/algorithms/quick-sort", 200, "text/html"},
		{"GET", "/static/sim/vendor/htmx/htmx.min.js", 200, "javascript"},
		{"GET", "/static/sim/vendor/three/three.module.js", 200, "javascript"},
		{"GET", "/static/sim/vendor/three/three.core.js", 200, "javascript"},
		{"GET", "/static/sim/vendor/", 404, ""}, // no directory listing
		{"GET", "/static/sim/", 404, ""},        // no directory listing
		{"GET", "/static/sim/nope.js", 404, ""}, // not in the allowlist
		{"GET", "/sim/Algorithms/quick-sort", 404, ""},
		{"GET", "/sim/algorithms/a_b", 404, ""},
		{"GET", "/sim/algorithms/quick-sort/", 404, ""},
		{"POST", "/sim", 405, ""},
		{"POST", "/static/sim/vendor/htmx/htmx.min.js", 405, ""},
	}
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if got := rec.Header().Get("Content-Type"); tc.typ != "" && !strings.Contains(got, tc.typ) {
				t.Fatalf("Content-Type = %q, want contain %q", got, tc.typ)
			}
		})
	}
}

func TestAssetCaching(t *testing.T) {
	mux := newMux(t)
	const path = "/static/sim/vendor/htmx/htmx.min.js"

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}

	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("conditional GET status = %d, want 304", rec.Code)
	}
}
