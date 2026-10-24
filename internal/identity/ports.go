package identity

import (
	"context"
	"time"
)

// Profile is what the identity provider tells us about a signed-in person.
type Profile struct {
	GitHubID int64
	Login    string
}

type IdentityProvider interface {
	AuthURL(state string) string
	Exchange(ctx context.Context, code string) (Profile, error)
}

type UserRepo interface {
	// UpsertByGitHubID inserts the user, or updates Login and Role of the
	// existing user with the same GitHubID. It returns the stored user
	// (ID and CreatedAt of an existing user are preserved).
	UpsertByGitHubID(ctx context.Context, u User) (User, error)
	// ByID returns ErrNotFound if there is no such user.
	ByID(ctx context.Context, id int64) (User, error)
}

type SessionRepo interface {
	Create(ctx context.Context, s Session) error
	// ByTokenHash returns ErrNotFound if there is no such session.
	ByTokenHash(ctx context.Context, hash string) (Session, error)
	// Delete removes a session; deleting a missing session is not an error.
	Delete(ctx context.Context, hash string) error
}

type TokenGenerator interface {
	New() (string, error)
}

type Clock interface {
	Now() time.Time
}
