package identity

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Service is the application layer of the identity context.
type Service struct {
	Users      UserRepo
	Sessions   SessionRepo
	Provider   IdentityProvider
	Tokens     TokenGenerator
	Clock      Clock
	Admins     []int64 // numeric GitHub user IDs
	SessionTTL time.Duration
}

type LoginResult struct {
	Token     string // raw token for the cookie; never stored
	ExpiresAt time.Time
	User      User
}

func (s *Service) LoginURL(state string) string {
	return s.Provider.AuthURL(state)
}

// LoginWithGitHub completes the OAuth code exchange, signs the person up if
// they are new, re-syncs their role from the admin allowlist, and opens a session.
func (s *Service) LoginWithGitHub(ctx context.Context, code string) (LoginResult, error) {
	profile, err := s.Provider.Exchange(ctx, code)
	if err != nil {
		return LoginResult{}, fmt.Errorf("exchange code: %w", err)
	}
	now := s.Clock.Now()
	user, err := s.Users.UpsertByGitHubID(ctx, User{
		GitHubID:  profile.GitHubID,
		Login:     profile.Login,
		Role:      RoleFor(profile.GitHubID, s.Admins),
		CreatedAt: now,
	})
	if err != nil {
		return LoginResult{}, fmt.Errorf("upsert user: %w", err)
	}
	token, err := s.Tokens.New()
	if err != nil {
		return LoginResult{}, fmt.Errorf("new token: %w", err)
	}
	expires := now.Add(s.SessionTTL)
	if err := s.Sessions.Create(ctx, Session{
		TokenHash: HashToken(token),
		UserID:    user.ID,
		ExpiresAt: expires,
	}); err != nil {
		return LoginResult{}, fmt.Errorf("create session: %w", err)
	}
	return LoginResult{Token: token, ExpiresAt: expires, User: user}, nil
}

// Authenticate resolves a raw session token to a user. Every "not logged in"
// outcome (empty, unknown, expired, orphaned) is ErrUnauthenticated; any other
// error is an infrastructure failure.
func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthenticated
	}
	hash := HashToken(token)
	sess, err := s.Sessions.ByTokenHash(ctx, hash)
	if errors.Is(err, ErrNotFound) {
		return User{}, ErrUnauthenticated
	}
	if err != nil {
		return User{}, fmt.Errorf("load session: %w", err)
	}
	if sess.Expired(s.Clock.Now()) {
		_ = s.Sessions.Delete(ctx, hash) // best-effort cleanup
		return User{}, ErrUnauthenticated
	}
	user, err := s.Users.ByID(ctx, sess.UserID)
	if errors.Is(err, ErrNotFound) {
		return User{}, ErrUnauthenticated
	}
	if err != nil {
		return User{}, fmt.Errorf("load user: %w", err)
	}
	return user, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := s.Sessions.Delete(ctx, HashToken(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
