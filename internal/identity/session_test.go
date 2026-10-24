package identity_test

import (
	"testing"
	"time"

	"babylon/internal/identity"
)

func TestSessionExpired(t *testing.T) {
	exp := time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC)
	s := identity.Session{ExpiresAt: exp}
	if s.Expired(exp.Add(-time.Second)) {
		t.Fatal("expired one second before ExpiresAt")
	}
	if !s.Expired(exp) {
		t.Fatal("not expired exactly at ExpiresAt")
	}
	if !s.Expired(exp.Add(time.Second)) {
		t.Fatal("not expired after ExpiresAt")
	}
}

func TestHashToken(t *testing.T) {
	a, b := identity.HashToken("tok-1"), identity.HashToken("tok-2")
	if a == b {
		t.Fatal("different tokens hashed equal")
	}
	if a != identity.HashToken("tok-1") {
		t.Fatal("hash not deterministic")
	}
	if len(a) != 64 {
		t.Fatalf("len = %d, want 64 hex chars", len(a))
	}
	// Known-answer vector: SHA-256("abc").
	if got, want := identity.HashToken("abc"), "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"; got != want {
		t.Fatalf("HashToken(abc) = %s, want %s", got, want)
	}
}
