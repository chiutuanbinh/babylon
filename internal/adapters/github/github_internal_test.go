package github

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExchangeHonoursTotalTimeout(t *testing.T) {
	old := exchangeTimeout
	exchangeTimeout = 50 * time.Millisecond
	t.Cleanup(func() { exchangeTimeout = old })

	const sleep = 500 * time.Millisecond
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"t","token_type":"bearer"}`))
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(sleep):
		case <-r.Context().Done():
		}
		_, _ = w.Write([]byte(`{"id":1,"login":"a"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	p := New(Config{ClientID: "cid", ClientSecret: "s", TokenURL: srv.URL + "/token", APIURL: srv.URL})
	start := time.Now()
	_, err := p.Exchange(t.Context(), "code")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if d := time.Since(start); d > 400*time.Millisecond {
		t.Fatalf("Exchange took %v; total timeout not applied", d)
	}
}
