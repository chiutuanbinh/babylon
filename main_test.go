package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
