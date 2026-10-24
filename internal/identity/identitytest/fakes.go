// Package identitytest holds in-memory fakes of the identity ports plus a
// ready-wired Service, for use in tests of identity and of adapters.
package identitytest

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"babylon/internal/identity"
)

const SessionTTL = 7 * 24 * time.Hour

// Env is a Service wired to fakes. Mutate Clock.T to move time; add entries to
// Provider.Codes to invent GitHub users.
type Env struct {
	Service  *identity.Service
	Clock    *Clock
	Sessions *Sessions
	Tokens   *Tokens
	Provider *Provider
}

func NewEnv(admins ...string) *Env {
	clock := &Clock{T: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	sessions := NewSessions()
	tokens := &Tokens{}
	provider := &Provider{Codes: map[string]identity.Profile{
		"code-alice": {GitHubID: 1001, Login: "alice"},
		"code-bob":   {GitHubID: 1002, Login: "bob"},
		"code-root":  {GitHubID: 1, Login: "Root"},
	}}
	return &Env{
		Service: &identity.Service{
			Users:      NewUsers(),
			Sessions:   sessions,
			Provider:   provider,
			Tokens:     tokens,
			Clock:      clock,
			Admins:     admins,
			SessionTTL: SessionTTL,
		},
		Clock:    clock,
		Sessions: sessions,
		Tokens:   tokens,
		Provider: provider,
	}
}

type Users struct {
	next int64
	byID map[int64]identity.User
}

func NewUsers() *Users { return &Users{byID: map[int64]identity.User{}} }

func (u *Users) UpsertByGitHubID(_ context.Context, in identity.User) (identity.User, error) {
	for id, ex := range u.byID {
		if ex.GitHubID == in.GitHubID {
			ex.Login, ex.Role = in.Login, in.Role
			u.byID[id] = ex
			return ex, nil
		}
	}
	u.next++
	in.ID = u.next
	u.byID[in.ID] = in
	return in, nil
}

func (u *Users) ByID(_ context.Context, id int64) (identity.User, error) {
	user, ok := u.byID[id]
	if !ok {
		return identity.User{}, identity.ErrNotFound
	}
	return user, nil
}

type Sessions struct{ m map[string]identity.Session }

func NewSessions() *Sessions { return &Sessions{m: map[string]identity.Session{}} }

func (s *Sessions) Create(_ context.Context, sess identity.Session) error {
	s.m[sess.TokenHash] = sess
	return nil
}

func (s *Sessions) ByTokenHash(_ context.Context, hash string) (identity.Session, error) {
	sess, ok := s.m[hash]
	if !ok {
		return identity.Session{}, identity.ErrNotFound
	}
	return sess, nil
}

func (s *Sessions) Delete(_ context.Context, hash string) error {
	delete(s.m, hash)
	return nil
}

func (s *Sessions) Len() int { return len(s.m) }

type Provider struct{ Codes map[string]identity.Profile }

func (p *Provider) AuthURL(state string) string {
	return "https://github.test/authorize?state=" + url.QueryEscape(state)
}

func (p *Provider) Exchange(_ context.Context, code string) (identity.Profile, error) {
	pr, ok := p.Codes[code]
	if !ok {
		return identity.Profile{}, errors.New("identitytest: unknown code")
	}
	return pr, nil
}

// Tokens yields tok-1, tok-2, ... so tests are deterministic.
type Tokens struct{ n int }

func (t *Tokens) New() (string, error) {
	t.n++
	return fmt.Sprintf("tok-%d", t.n), nil
}

type Clock struct{ T time.Time }

func (c *Clock) Now() time.Time { return c.T }
