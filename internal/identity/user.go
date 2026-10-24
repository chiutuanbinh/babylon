// Package identity is the login/session bounded context. It knows nothing
// about HTTP, SQL or OAuth; those live behind the ports in ports.go.
package identity

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound        = errors.New("identity: not found")
	ErrUnauthenticated = errors.New("identity: unauthenticated")
)

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type User struct {
	ID        int64
	GitHubID  int64
	Login     string
	Role      Role
	CreatedAt time.Time
}

// RoleFor maps a GitHub login to a role. GitHub logins are case-insensitive.
// An empty login is never an admin, even if the allowlist has an empty entry.
func RoleFor(login string, admins []string) Role {
	if login == "" {
		return RoleUser
	}
	for _, a := range admins {
		if strings.EqualFold(strings.TrimSpace(a), login) {
			return RoleAdmin
		}
	}
	return RoleUser
}
