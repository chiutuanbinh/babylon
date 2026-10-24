// Package authz is the shared kernel for authorization: the vocabulary that
// bounded contexts use to ask "may this subject do this to that resource?".
// It has no engine; adapters implement Authorizer.
package authz

import "context"

// Action is something a subject can do to a resource.
type Action string

const (
	Read   Action = "read"
	Update Action = "update"
	Delete Action = "delete"
)

// RoleAdmin must equal string(identity.RoleAdmin); the casbin adapter's tests
// guard that.
const RoleAdmin = "admin"

// Subject is the actor. An empty ID means anonymous.
type Subject struct {
	ID   string
	Role string
}

// Resource is the thing acted on. OwnerID is the Subject.ID of its owner.
type Resource struct {
	Type    string
	OwnerID string
}

// Authorizer answers whether sub may perform act on res. A non-nil error means
// the check itself failed, which callers must treat as a denial.
type Authorizer interface {
	Can(ctx context.Context, sub Subject, act Action, res Resource) (bool, error)
}
