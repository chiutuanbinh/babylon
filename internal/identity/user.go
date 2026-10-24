// Package identity is the login/session bounded context. It knows nothing
// about HTTP, SQL or OAuth; those live behind the ports in ports.go.
package identity

import (
	"errors"
	"time"
)

var (
	ErrNotFound        = errors.New("identity: not found")
	ErrUnauthenticated = errors.New("identity: unauthenticated")
	// ErrProvider marks a failure of the external identity provider (bad code,
	// network, GitHub down) as opposed to an internal failure.
	ErrProvider = errors.New("identity: provider failure")
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

// RoleFor maps an immutable numeric GitHub user ID to a role. Logins are
// mutable and can be re-registered by someone else, so they are never used.
// A zero or negative ID is never an admin, even if the allowlist contains it.
func RoleFor(githubID int64, admins []int64) Role {
	if githubID <= 0 {
		return RoleUser
	}
	for _, a := range admins {
		if a == githubID {
			return RoleAdmin
		}
	}
	return RoleUser
}
