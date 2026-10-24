package identity_test

import (
	"testing"

	"babylon/internal/identity"
)

func TestRoleFor(t *testing.T) {
	tests := []struct {
		name   string
		id     int64
		admins []int64
		want   identity.Role
	}{
		{"listed", 42, []int64{7, 42}, identity.RoleAdmin},
		{"unlisted", 5, []int64{42}, identity.RoleUser},
		{"nil list", 42, nil, identity.RoleUser},
		{"zero never admin", 0, []int64{0}, identity.RoleUser},
		{"negative not admin", -1, []int64{-1}, identity.RoleUser},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := identity.RoleFor(tc.id, tc.admins); got != tc.want {
				t.Fatalf("RoleFor(%d, %v) = %q, want %q", tc.id, tc.admins, got, tc.want)
			}
		})
	}
}
