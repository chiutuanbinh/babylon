package system_test

import (
	"regexp"
	"testing"
	"time"

	"babylon/internal/adapters/system"
)

func TestTokens(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		tok, err := system.Tokens{}.New()
		if err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`).MatchString(tok) {
			t.Fatalf("token %q is not 43 url-safe chars", tok)
		}
		if seen[tok] {
			t.Fatalf("duplicate token %q", tok)
		}
		seen[tok] = true
	}
}

func TestClock(t *testing.T) {
	if d := time.Since(system.Clock{}.Now()); d < 0 || d > time.Second {
		t.Fatalf("Clock.Now off by %v", d)
	}
}
