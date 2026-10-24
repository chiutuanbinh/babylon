package identity_test

import (
	"testing"

	"babylon/internal/identity"
)

func TestRoleFor(t *testing.T) {
	tests := []struct {
		name   string
		login  string
		admins []string
		want   identity.Role
	}{
		{"not listed", "alice", []string{"root"}, identity.RoleUser},
		{"listed", "root", []string{"root"}, identity.RoleAdmin},
		{"case-insensitive", "Root", []string{"rOOt"}, identity.RoleAdmin},
		{"padded entry", "root", []string{" root "}, identity.RoleAdmin},
		{"no admins", "root", nil, identity.RoleUser},
		{"empty login never admin", "", []string{""}, identity.RoleUser},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := identity.RoleFor(tc.login, tc.admins); got != tc.want {
				t.Fatalf("RoleFor(%q, %q) = %q, want %q", tc.login, tc.admins, got, tc.want)
			}
		})
	}
}
