# Identity and Authorization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add GitHub SSO login with self-signup, Postgres-backed users and sessions, and a Casbin ABAC authorizer (owner or admin), structured as DDD + hexagonal.

**Architecture:** Domain and application code (`internal/identity`, `internal/authz`) import no HTTP, SQL, OAuth or Casbin packages. Adapters under `internal/adapters/` (postgres, github, casbin, web, system) implement the ports. `main.go` is the composition root and mounts the `web` adapter's routes onto the existing mux. Spec: `docs/superpowers/specs/2026-10-24-identity-authz-design.md`.

**Tech Stack:** Go 1.25.3, `github.com/jackc/pgx/v5` v5.11.0, `golang.org/x/oauth2` v0.36.0, `github.com/casbin/casbin/v2` v2.135.0, Postgres 17.

## Global Constraints

- Module path is `babylon`; imports look like `babylon/internal/identity`.
- `go.mod` keeps `go 1.25.3`. Prefix every `go get` / `go mod` command with `GOTOOLCHAIN=local`, and pin exact versions (`oauth2@v0.37.0` and later need Go 1.26 and must not be used). After any `go.mod` change, `head -3 go.mod` must still show `go 1.25.3`.
- Domain/application packages (`internal/identity`, `internal/authz`) import only the standard library (and `babylon/internal/...` siblings where stated).
- Roles are exactly `user` and `admin`. Actions are exactly `read`, `update`, `delete`.
- Session cookie: name `session`, `HttpOnly`, `SameSite=Lax`, `Secure` unless `BASE_URL` starts with `http://localhost` or `http://127.0.0.1`, lifetime 7 days. Only the SHA-256 hex of the token is stored in the DB.
- Env vars: `DATABASE_URL`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `BASE_URL` (all required), `ADMIN_LOGINS` (optional, comma-separated), `ADDR` (optional, default `:8080`).
- Existing routes (`/`, `/greet`, `/time`, `/hello/{name}`, `/healthz`) stay public and unchanged.
- Use `html/template` only. `render` helpers execute into a buffer first so a template error is a clean 500.
- Every commit message ends with the trailer `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>` (pass it as a second `-m`).
- The Claude sandbox blocks connections to localhost and container IPs. Anything needing a running Postgres or a browser is run by the user with the `!` prefix; steps say so explicitly.
- `gofmt` is run automatically by a hook on `.go` edits; `go vet ./...` must pass at the end of every task.
- Work happens on branch `feat/identity-authz`. Uncommitted pre-existing changes (`.claude/settings.json`, `.claude/skills/project-conventions/SKILL.md`, untracked `CLAUDE.md`) belong to Task 8; never `git add -A` or `git add .` before then. Always add explicit paths.

## File Structure

```
internal/identity/user.go                 User, Role, RoleFor, sentinel errors
internal/identity/user_test.go
internal/identity/session.go              Session, Expired, HashToken
internal/identity/session_test.go
internal/identity/ports.go                Profile + 5 ports
internal/identity/service.go              Service: LoginURL, LoginWithGitHub, Authenticate, Logout
internal/identity/service_test.go
internal/identity/identitytest/fakes.go   in-memory fakes + NewEnv (test support, not a _test file)
internal/authz/authz.go                   Subject, Resource, Action, Authorizer, RoleAdmin
internal/adapters/casbin/casbin.go        Authorizer implementation
internal/adapters/casbin/casbin_test.go
internal/adapters/github/github.go       IdentityProvider via oauth2
internal/adapters/github/github_test.go
internal/adapters/postgres/schema.sql
internal/adapters/postgres/postgres.go    Migrate
internal/adapters/postgres/users.go       UserRepo
internal/adapters/postgres/sessions.go    SessionRepo
internal/adapters/postgres/postgres_test.go
internal/adapters/system/system.go        Clock, Tokens
internal/adapters/system/system_test.go
internal/adapters/web/web.go              Handler, Mount, middleware
internal/adapters/web/templates/me.html
internal/adapters/web/web_test.go
config.go, config_test.go                 env parsing (package main)
main.go                                   modify: wiring, routes(mounts...)
main_test.go                              modify: mount test
Dockerfile                                modify: copy go.sum
CLAUDE.md, .claude/skills/project-conventions/SKILL.md, spec  docs (Task 8)
```

---

### Task 1: Identity domain (User, Role, Session)

**Files:**
- Create: `internal/identity/user.go`, `internal/identity/session.go`
- Test: `internal/identity/user_test.go`, `internal/identity/session_test.go`

**Interfaces:**
- Produces: `identity.Role` (`RoleUser`, `RoleAdmin`), `identity.User{ID, GitHubID int64; Login string; Role Role; CreatedAt time.Time}`, `identity.RoleFor(login string, admins []string) Role`, `identity.ErrNotFound`, `identity.ErrUnauthenticated`, `identity.Session{TokenHash string; UserID int64; ExpiresAt time.Time}`, `Session.Expired(now time.Time) bool`, `identity.HashToken(token string) string`.

- [ ] **Step 1: Confirm the branch**

Run: `cd /Users/binhct/workspace/babylon && git branch --show-current`
Expected: `feat/identity-authz`. If not, `git switch feat/identity-authz`.

- [ ] **Step 2: Write the failing tests**

`internal/identity/user_test.go`:

```go
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
```

`internal/identity/session_test.go`:

```go
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
	if a == "tok-1" {
		t.Fatal("token stored in the clear")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/identity/ -v`
Expected: FAIL to compile: `undefined: identity.RoleFor`, `undefined: identity.Session`, `undefined: identity.HashToken` (and the package has no non-test files yet).

- [ ] **Step 4: Write the implementation**

`internal/identity/user.go`:

```go
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
```

`internal/identity/session.go`:

```go
package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type Session struct {
	TokenHash string
	UserID    int64
	ExpiresAt time.Time
}

func (s Session) Expired(now time.Time) bool {
	return !now.Before(s.ExpiresAt)
}

// HashToken returns the hex SHA-256 of a session token. Only the hash is ever
// stored, so a database leak does not leak live sessions.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/identity/ -v && go vet ./internal/identity/`
Expected: `PASS` for `TestRoleFor` (6 subtests), `TestSessionExpired`, `TestHashToken`; vet silent.

- [ ] **Step 6: Commit**

```bash
git add internal/identity/user.go internal/identity/session.go internal/identity/user_test.go internal/identity/session_test.go
git commit -m "feat(identity): add User, Role and Session domain" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Identity ports, service and in-memory fakes

**Files:**
- Create: `internal/identity/ports.go`, `internal/identity/service.go`, `internal/identity/identitytest/fakes.go`
- Test: `internal/identity/service_test.go`

**Interfaces:**
- Consumes: Task 1 types.
- Produces:
  - `identity.Profile{GitHubID int64; Login string}`
  - ports: `IdentityProvider{AuthURL(state string) string; Exchange(ctx, code string) (Profile, error)}`, `UserRepo{UpsertByGitHubID(ctx, User) (User, error); ByID(ctx, int64) (User, error)}`, `SessionRepo{Create(ctx, Session) error; ByTokenHash(ctx, string) (Session, error); Delete(ctx, string) error}`, `TokenGenerator{New() (string, error)}`, `Clock{Now() time.Time}`
  - `identity.Service{Users, Sessions, Provider, Tokens, Clock, Admins []string, SessionTTL time.Duration}` with `LoginURL(state string) string`, `LoginWithGitHub(ctx, code string) (LoginResult, error)`, `Authenticate(ctx, token string) (User, error)`, `Logout(ctx, token string) error`
  - `identity.LoginResult{Token string; ExpiresAt time.Time; User User}`
  - repos must return `identity.ErrNotFound` for a missing row; `SessionRepo.Delete` of a missing row returns nil.
  - `identitytest.NewEnv(admins ...string) *Env` with fields `Service *identity.Service`, `Clock *Clock` (field `T time.Time`), `Sessions *Sessions` (method `Len() int`), `Tokens *Tokens` (yields `tok-1`, `tok-2`, ...), `Provider *Provider` (field `Codes map[string]identity.Profile`). Built-in codes: `code-alice` → {1001, "alice"}, `code-bob` → {1002, "bob"}, `code-root` → {1, "Root"}. `identitytest.SessionTTL` = 7 days. Fake `Provider.AuthURL(state)` returns `https://github.test/authorize?state=<escaped state>`.

- [ ] **Step 1: Write the ports**

`internal/identity/ports.go`:

```go
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
```

- [ ] **Step 2: Write the fakes**

`internal/identity/identitytest/fakes.go`:

```go
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
```

- [ ] **Step 3: Write the failing service tests**

`internal/identity/service_test.go`:

```go
package identity_test

import (
	"context"
	"errors"
	"testing"

	"babylon/internal/identity"
	"babylon/internal/identity/identitytest"
)

var ctx = context.Background()

func TestLoginCreatesUserAndSession(t *testing.T) {
	env := identitytest.NewEnv()
	res, err := env.Service.LoginWithGitHub(ctx, "code-alice")
	if err != nil {
		t.Fatal(err)
	}
	if res.User.ID == 0 || res.User.Login != "alice" || res.User.Role != identity.RoleUser {
		t.Fatalf("user = %+v", res.User)
	}
	if want := env.Clock.T.Add(identitytest.SessionTTL); !res.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", res.ExpiresAt, want)
	}
	got, err := env.Service.Authenticate(ctx, res.Token)
	if err != nil || got.ID != res.User.ID {
		t.Fatalf("Authenticate = %+v, %v", got, err)
	}
	// Only the hash is stored: the raw token must not be a valid key.
	if _, err := env.Sessions.ByTokenHash(ctx, res.Token); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("raw token found in store: %v", err)
	}
	if _, err := env.Sessions.ByTokenHash(ctx, identity.HashToken(res.Token)); err != nil {
		t.Fatalf("hashed token missing: %v", err)
	}
}

func TestAdminRoleFromAllowlist(t *testing.T) {
	env := identitytest.NewEnv("root") // profile login is "Root": match is case-insensitive
	res, err := env.Service.LoginWithGitHub(ctx, "code-root")
	if err != nil {
		t.Fatal(err)
	}
	if res.User.Role != identity.RoleAdmin {
		t.Fatalf("role = %q, want admin", res.User.Role)
	}
}

func TestRoleResyncedOnEveryLogin(t *testing.T) {
	env := identitytest.NewEnv("root")
	first, err := env.Service.LoginWithGitHub(ctx, "code-root")
	if err != nil || first.User.Role != identity.RoleAdmin {
		t.Fatalf("first login: %+v, %v", first.User, err)
	}
	env.Service.Admins = nil // removed from ADMIN_LOGINS
	second, err := env.Service.LoginWithGitHub(ctx, "code-root")
	if err != nil {
		t.Fatal(err)
	}
	if second.User.Role != identity.RoleUser {
		t.Fatalf("role = %q, want user after allowlist removal", second.User.Role)
	}
	if second.User.ID != first.User.ID {
		t.Fatalf("user re-created: id %d then %d", first.User.ID, second.User.ID)
	}
}

func TestTwoUsersGetDistinctSessions(t *testing.T) {
	env := identitytest.NewEnv()
	a, err := env.Service.LoginWithGitHub(ctx, "code-alice")
	if err != nil {
		t.Fatal(err)
	}
	b, err := env.Service.LoginWithGitHub(ctx, "code-bob")
	if err != nil {
		t.Fatal(err)
	}
	if a.Token == b.Token {
		t.Fatal("two users share a token")
	}
	for _, tc := range []struct {
		token, login string
	}{{a.Token, "alice"}, {b.Token, "bob"}} {
		u, err := env.Service.Authenticate(ctx, tc.token)
		if err != nil || u.Login != tc.login {
			t.Fatalf("Authenticate(%s) = %+v, %v; want %s", tc.token, u, err, tc.login)
		}
	}
}

func TestAuthenticateRejectsUnknownTokens(t *testing.T) {
	env := identitytest.NewEnv()
	for _, token := range []string{"", "nope"} {
		if _, err := env.Service.Authenticate(ctx, token); !errors.Is(err, identity.ErrUnauthenticated) {
			t.Fatalf("Authenticate(%q) err = %v, want ErrUnauthenticated", token, err)
		}
	}
}

func TestAuthenticateExpiredSessionIsRejectedAndDeleted(t *testing.T) {
	env := identitytest.NewEnv()
	res, err := env.Service.LoginWithGitHub(ctx, "code-alice")
	if err != nil {
		t.Fatal(err)
	}
	env.Clock.T = env.Clock.T.Add(identitytest.SessionTTL)
	if _, err := env.Service.Authenticate(ctx, res.Token); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
	if env.Sessions.Len() != 0 {
		t.Fatalf("expired session not cleaned up: %d left", env.Sessions.Len())
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	env := identitytest.NewEnv()
	res, err := env.Service.LoginWithGitHub(ctx, "code-alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.Service.Logout(ctx, res.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Service.Authenticate(ctx, res.Token); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated after logout", err)
	}
	if err := env.Service.Logout(ctx, res.Token); err != nil {
		t.Fatalf("second logout should be a no-op, got %v", err)
	}
}

func TestLoginProviderFailureCreatesNothing(t *testing.T) {
	env := identitytest.NewEnv()
	if _, err := env.Service.LoginWithGitHub(ctx, "bogus"); err == nil {
		t.Fatal("expected error for unknown code")
	}
	if env.Sessions.Len() != 0 {
		t.Fatalf("session created despite failed login: %d", env.Sessions.Len())
	}
}

func TestLoginURLDelegatesToProvider(t *testing.T) {
	env := identitytest.NewEnv()
	if got, want := env.Service.LoginURL("s1"), env.Provider.AuthURL("s1"); got != want {
		t.Fatalf("LoginURL = %q, want %q", got, want)
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./internal/identity/... 2>&1 | head -20`
Expected: FAIL to compile: `undefined: identity.Service` (in `identitytest` and the test file).

- [ ] **Step 5: Write the service**

`internal/identity/service.go`:

```go
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
	Admins     []string
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
		Role:      RoleFor(profile.Login, s.Admins),
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
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/identity/... -v 2>&1 | tail -30 && go vet ./internal/identity/...`
Expected: all `PASS` (Task 1 tests plus 8 new tests); vet silent.

- [ ] **Step 7: Commit**

```bash
git add internal/identity
git commit -m "feat(identity): add ports, service and in-memory fakes" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Authorization kernel and Casbin adapter

**Files:**
- Create: `internal/authz/authz.go`, `internal/adapters/casbin/casbin.go`
- Test: `internal/adapters/casbin/casbin_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: `identity.RoleAdmin` (only in a test guard).
- Produces: `authz.Action` (`authz.Read`, `authz.Update`, `authz.Delete`), `authz.Subject{ID, Role string}`, `authz.Resource{Type, OwnerID string}`, `authz.RoleAdmin = "admin"`, `authz.Authorizer{Can(ctx, Subject, Action, Resource) (bool, error)}`; `casbin.New() (*casbin.Authorizer, error)` implementing it.

- [ ] **Step 1: Add the dependency**

Run: `GOTOOLCHAIN=local go get github.com/casbin/casbin/v2@v2.135.0 && head -3 go.mod`
Expected: `go 1.25.3` still on line 3.

- [ ] **Step 2: Write the kernel**

`internal/authz/authz.go`:

```go
// Package authz is the shared kernel for authorization: the vocabulary that
// bounded contexts use to ask "may this subject do this to that resource?".
// It has no engine; adapters implement Authorizer.
package authz

import "context"

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

type Authorizer interface {
	Can(ctx context.Context, sub Subject, act Action, res Resource) (bool, error)
}
```

- [ ] **Step 3: Write the failing tests**

`internal/adapters/casbin/casbin_test.go`:

```go
package casbin_test

import (
	"context"
	"testing"

	"babylon/internal/adapters/casbin"
	"babylon/internal/authz"
	"babylon/internal/identity"
)

var (
	alice = authz.Subject{ID: "1001", Role: "user"}
	bob   = authz.Subject{ID: "1002", Role: "user"}
	root  = authz.Subject{ID: "1", Role: "admin"}
	anon  = authz.Subject{}

	doc = authz.Resource{Type: "doc", OwnerID: "1001"} // owned by alice
)

func TestCan(t *testing.T) {
	a, err := casbin.New()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		sub  authz.Subject
		act  authz.Action
		res  authz.Resource
		want bool
	}{
		{"owner reads", alice, authz.Read, doc, true},
		{"owner updates", alice, authz.Update, doc, true},
		{"owner deletes", alice, authz.Delete, doc, true},
		{"other user reads", bob, authz.Read, doc, false},
		{"other user updates", bob, authz.Update, doc, false},
		{"other user deletes", bob, authz.Delete, doc, false},
		{"admin reads", root, authz.Read, doc, true},
		{"admin updates", root, authz.Update, doc, true},
		{"admin deletes", root, authz.Delete, doc, true},
		{"anonymous reads", anon, authz.Read, doc, false},
		{"anonymous reads ownerless resource", anon, authz.Read, authz.Resource{Type: "doc"}, false},
		{"unknown action denied even for admin", root, authz.Action("create"), doc, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := a.Can(context.Background(), tc.sub, tc.act, tc.res)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("Can(%+v, %q, %+v) = %v, want %v", tc.sub, tc.act, tc.res, got, tc.want)
			}
		})
	}
}

// The identity context and the authz kernel each spell the admin role; they
// must agree or admins silently lose access.
func TestRoleNamesAgree(t *testing.T) {
	if string(identity.RoleAdmin) != authz.RoleAdmin {
		t.Fatalf("identity.RoleAdmin = %q, authz.RoleAdmin = %q", identity.RoleAdmin, authz.RoleAdmin)
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./internal/adapters/casbin/ 2>&1 | head`
Expected: FAIL to compile: `no non-test Go files` / `undefined: casbin.New`.

- [ ] **Step 5: Write the adapter**

`internal/adapters/casbin/casbin.go`:

```go
// Package casbin implements authz.Authorizer with a Casbin ABAC model:
// a subject may act on a resource if they own it or are an admin.
package casbin

import (
	"context"
	"fmt"

	casbinlib "github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"

	"babylon/internal/authz"
)

var modelText = fmt.Sprintf(`
[request_definition]
r = sub, act, obj

[policy_definition]
p = act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.act == p.act && r.sub.ID != "" && (r.sub.Role == %q || r.sub.ID == r.obj.OwnerID)
`, authz.RoleAdmin)

var _ authz.Authorizer = (*Authorizer)(nil)

type Authorizer struct {
	e *casbinlib.Enforcer
}

func New() (*Authorizer, error) {
	m, err := model.NewModelFromString(modelText)
	if err != nil {
		return nil, fmt.Errorf("casbin model: %w", err)
	}
	e, err := casbinlib.NewEnforcer(m)
	if err != nil {
		return nil, fmt.Errorf("casbin enforcer: %w", err)
	}
	for _, act := range []authz.Action{authz.Read, authz.Update, authz.Delete} {
		if _, err := e.AddPolicy(string(act)); err != nil {
			return nil, fmt.Errorf("casbin policy %q: %w", act, err)
		}
	}
	return &Authorizer{e: e}, nil
}

func (a *Authorizer) Can(_ context.Context, sub authz.Subject, act authz.Action, res authz.Resource) (bool, error) {
	// act must be passed as a plain string: Casbin's expression engine compares
	// with reflect.DeepEqual, so authz.Action("read") never equals the policy's
	// "read" and every check would silently come back false.
	return a.e.Enforce(sub, string(act), res)
}
```

- [ ] **Step 6: Tidy and run tests**

Run: `GOTOOLCHAIN=local go mod tidy && head -3 go.mod && go test ./internal/adapters/casbin/ -v 2>&1 | tail -20 && go vet ./...`
Expected: `go 1.25.3`; `TestCan` with 12 passing subtests, `TestRoleNamesAgree` PASS; vet silent.

- [ ] **Step 7: Commit**

```bash
git add internal/authz internal/adapters/casbin go.mod go.sum
git commit -m "feat(authz): add authz kernel and Casbin ABAC adapter" -m "Casbin chosen for policy-as-data ABAC with struct attributes; owner-or-admin rule." -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 4: GitHub identity provider adapter

**Files:**
- Create: `internal/adapters/github/github.go`
- Test: `internal/adapters/github/github_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: `identity.Profile`, `identity.IdentityProvider`.
- Produces: `github.Config{ClientID, ClientSecret, RedirectURL, AuthURL, TokenURL, APIURL string}` (last three optional, default to github.com / api.github.com), `github.New(Config) *github.Provider` implementing `identity.IdentityProvider`.

- [ ] **Step 1: Add the dependency**

Run: `GOTOOLCHAIN=local go get golang.org/x/oauth2@v0.36.0 && head -3 go.mod`
Expected: `go 1.25.3` still on line 3.

- [ ] **Step 2: Write the failing tests**

`internal/adapters/github/github_test.go`:

```go
package github_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"babylon/internal/adapters/github"
)

// fakeGitHub serves the three endpoints the adapter talks to. userStatus is the
// status returned by GET /user.
func fakeGitHub(t *testing.T, userStatus int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.PostForm.Get("code") != "good-code" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":"bad_verification_code"}`)
			return
		}
		_, _ = io.WriteString(w, `{"access_token":"gh-token","token_type":"bearer"}`)
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer gh-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if userStatus != http.StatusOK {
			w.WriteHeader(userStatus)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":4242,"login":"alice"}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newProvider(srv *httptest.Server) *github.Provider {
	return github.New(github.Config{
		ClientID:     "cid",
		ClientSecret: "secret",
		RedirectURL:  "http://localhost:8080/auth/callback",
		AuthURL:      srv.URL + "/login/oauth/authorize",
		TokenURL:     srv.URL + "/login/oauth/access_token",
		APIURL:       srv.URL,
	})
}

func TestAuthURL(t *testing.T) {
	p := newProvider(fakeGitHub(t, http.StatusOK))
	u, err := url.Parse(p.AuthURL("st8"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("state") != "st8" || q.Get("client_id") != "cid" ||
		q.Get("redirect_uri") != "http://localhost:8080/auth/callback" {
		t.Fatalf("query = %v", q)
	}
}

func TestExchangeReturnsProfile(t *testing.T) {
	p := newProvider(fakeGitHub(t, http.StatusOK))
	got, err := p.Exchange(t.Context(), "good-code")
	if err != nil {
		t.Fatal(err)
	}
	if got.GitHubID != 4242 || got.Login != "alice" {
		t.Fatalf("profile = %+v", got)
	}
}

func TestExchangeRejectsBadCode(t *testing.T) {
	p := newProvider(fakeGitHub(t, http.StatusOK))
	if _, err := p.Exchange(t.Context(), "bad-code"); err == nil {
		t.Fatal("expected error for bad code")
	}
}

func TestExchangeFailsWhenUserAPIFails(t *testing.T) {
	p := newProvider(fakeGitHub(t, http.StatusInternalServerError))
	if _, err := p.Exchange(t.Context(), "good-code"); err == nil {
		t.Fatal("expected error when GET /user returns 500")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/adapters/github/ 2>&1 | head`
Expected: FAIL to compile: `no non-test Go files` / `undefined: github.New`.

- [ ] **Step 4: Write the adapter**

`internal/adapters/github/github.go`:

```go
// Package github implements identity.IdentityProvider with GitHub OAuth.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	oauth2gh "golang.org/x/oauth2/github"

	"babylon/internal/identity"
)

// Config holds the OAuth app credentials. AuthURL, TokenURL and APIURL default
// to GitHub's; tests point them at a fake server.
type Config struct {
	ClientID, ClientSecret, RedirectURL string
	AuthURL, TokenURL, APIURL           string
}

var _ identity.IdentityProvider = (*Provider)(nil)

type Provider struct {
	oauth  *oauth2.Config
	apiURL string
	client *http.Client
}

func New(c Config) *Provider {
	ep := oauth2gh.Endpoint
	if c.AuthURL != "" {
		ep.AuthURL = c.AuthURL
	}
	if c.TokenURL != "" {
		ep.TokenURL = c.TokenURL
	}
	api := c.APIURL
	if api == "" {
		api = "https://api.github.com"
	}
	return &Provider{
		oauth: &oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			RedirectURL:  c.RedirectURL,
			Endpoint:     ep,
		},
		apiURL: api,
		// Below the server's 10s WriteTimeout so a slow GitHub yields our error,
		// not a dropped connection.
		client: &http.Client{Timeout: 8 * time.Second},
	}
}

func (p *Provider) AuthURL(state string) string {
	return p.oauth.AuthCodeURL(state)
}

func (p *Provider) Exchange(ctx context.Context, code string) (identity.Profile, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.client)
	tok, err := p.oauth.Exchange(ctx, code)
	if err != nil {
		return identity.Profile{}, fmt.Errorf("github: exchange code: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiURL+"/user", nil)
	if err != nil {
		return identity.Profile{}, fmt.Errorf("github: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := p.oauth.Client(ctx, tok).Do(req)
	if err != nil {
		return identity.Profile{}, fmt.Errorf("github: get user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return identity.Profile{}, fmt.Errorf("github: get user: status %d", resp.StatusCode)
	}

	var u struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return identity.Profile{}, fmt.Errorf("github: decode user: %w", err)
	}
	if u.ID == 0 || u.Login == "" {
		return identity.Profile{}, fmt.Errorf("github: user response missing id or login")
	}
	return identity.Profile{GitHubID: u.ID, Login: u.Login}, nil
}
```

- [ ] **Step 5: Tidy and run tests**

Run: `GOTOOLCHAIN=local go mod tidy && head -3 go.mod && go test ./internal/adapters/github/ -v 2>&1 | tail -15 && go vet ./...`
Expected: `go 1.25.3`; 4 tests PASS; vet silent.

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/github go.mod go.sum
git commit -m "feat(github): add GitHub OAuth identity provider" -m "x/oauth2 handles the code exchange; GET /user supplies id and login." -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Postgres adapter

**Files:**
- Create: `internal/adapters/postgres/schema.sql`, `postgres.go`, `users.go`, `sessions.go`
- Test: `internal/adapters/postgres/postgres_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: `identity.User`, `identity.Session`, `identity.ErrNotFound`.
- Produces: `postgres.Migrate(ctx, *pgxpool.Pool) error` (idempotent), `postgres.NewUserRepo(*pgxpool.Pool) *UserRepo` implementing `identity.UserRepo`, `postgres.NewSessionRepo(*pgxpool.Pool) *SessionRepo` implementing `identity.SessionRepo`.

The sandbox cannot reach a database. Tests skip unless `TEST_DATABASE_URL` is set; the last step has the user run them against a real Postgres.

- [ ] **Step 1: Add the dependency**

Run: `GOTOOLCHAIN=local go get github.com/jackc/pgx/v5@v5.11.0 && head -3 go.mod`
Expected: `go 1.25.3` still on line 3.

- [ ] **Step 2: Write the failing tests**

`internal/adapters/postgres/postgres_test.go`:

```go
package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"babylon/internal/adapters/postgres"
	"babylon/internal/identity"
)

// testPool connects to TEST_DATABASE_URL, migrates, and empties the tables.
// Without the variable the test is skipped so `go test ./...` needs no database.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE sessions, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestMigrateIsIdempotent(t *testing.T) {
	pool := testPool(t)
	if err := postgres.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
}

func TestUserUpsertInsertsThenUpdates(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewUserRepo(testPool(t))
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	first, err := repo.UpsertByGitHubID(ctx, identity.User{GitHubID: 1001, Login: "alice", Role: identity.RoleUser, CreatedAt: created})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == 0 || first.Login != "alice" || first.Role != identity.RoleUser || !first.CreatedAt.Equal(created) {
		t.Fatalf("first = %+v", first)
	}

	second, err := repo.UpsertByGitHubID(ctx, identity.User{GitHubID: 1001, Login: "alice2", Role: identity.RoleAdmin, CreatedAt: created.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("id changed: %d -> %d", first.ID, second.ID)
	}
	if second.Login != "alice2" || second.Role != identity.RoleAdmin {
		t.Fatalf("second = %+v", second)
	}
	if !second.CreatedAt.Equal(created) {
		t.Fatalf("CreatedAt overwritten: %v", second.CreatedAt)
	}
}

func TestUserByID(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewUserRepo(testPool(t))
	u, err := repo.UpsertByGitHubID(ctx, identity.User{GitHubID: 1, Login: "root", Role: identity.RoleAdmin, CreatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.ByID(ctx, u.ID)
	if err != nil || got.Login != "root" || got.Role != identity.RoleAdmin {
		t.Fatalf("ByID = %+v, %v", got, err)
	}
	if _, err := repo.ByID(ctx, 99999); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("missing id err = %v, want ErrNotFound", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	users, sessions := postgres.NewUserRepo(pool), postgres.NewSessionRepo(pool)
	u, err := users.UpsertByGitHubID(ctx, identity.User{GitHubID: 1001, Login: "alice", Role: identity.RoleUser, CreatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}

	exp := time.Now().Add(time.Hour).Truncate(time.Microsecond)
	if err := sessions.Create(ctx, identity.Session{TokenHash: "h1", UserID: u.ID, ExpiresAt: exp}); err != nil {
		t.Fatal(err)
	}
	got, err := sessions.ByTokenHash(ctx, "h1")
	if err != nil || got.UserID != u.ID || !got.ExpiresAt.Equal(exp) {
		t.Fatalf("ByTokenHash = %+v, %v", got, err)
	}

	if err := sessions.Delete(ctx, "h1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.ByTokenHash(ctx, "h1"); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("after delete err = %v, want ErrNotFound", err)
	}
	if err := sessions.Delete(ctx, "h1"); err != nil {
		t.Fatalf("deleting a missing session should be a no-op, got %v", err)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/adapters/postgres/ 2>&1 | head`
Expected: FAIL to compile: `no non-test Go files` / `undefined: postgres.Migrate`.

- [ ] **Step 4: Write schema and Migrate**

`internal/adapters/postgres/schema.sql`:

```sql
CREATE TABLE IF NOT EXISTS users (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    github_id  BIGINT      NOT NULL UNIQUE,
    login      TEXT        NOT NULL,
    role       TEXT        NOT NULL CHECK (role IN ('user', 'admin')),
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT        PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON sessions (user_id);
```

`internal/adapters/postgres/postgres.go`:

```go
// Package postgres implements the identity repositories on Postgres via pgx.
package postgres

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

// Migrate applies the schema. Every statement is idempotent, so it is safe to
// run on every boot. With no arguments pgx uses the simple protocol, which
// allows several statements in one Exec.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("postgres: migrate: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Write the repositories**

`internal/adapters/postgres/users.go`:

```go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"babylon/internal/identity"
)

var _ identity.UserRepo = (*UserRepo)(nil)

type UserRepo struct{ pool *pgxpool.Pool }

func NewUserRepo(pool *pgxpool.Pool) *UserRepo { return &UserRepo{pool: pool} }

func (r *UserRepo) UpsertByGitHubID(ctx context.Context, u identity.User) (identity.User, error) {
	var out identity.User
	var role string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (github_id, login, role, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (github_id) DO UPDATE SET login = EXCLUDED.login, role = EXCLUDED.role
		RETURNING id, github_id, login, role, created_at`,
		u.GitHubID, u.Login, string(u.Role), u.CreatedAt,
	).Scan(&out.ID, &out.GitHubID, &out.Login, &role, &out.CreatedAt)
	if err != nil {
		return identity.User{}, fmt.Errorf("postgres: upsert user: %w", err)
	}
	out.Role = identity.Role(role)
	return out, nil
}

func (r *UserRepo) ByID(ctx context.Context, id int64) (identity.User, error) {
	var out identity.User
	var role string
	err := r.pool.QueryRow(ctx, `
		SELECT id, github_id, login, role, created_at FROM users WHERE id = $1`, id,
	).Scan(&out.ID, &out.GitHubID, &out.Login, &role, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.User{}, identity.ErrNotFound
	}
	if err != nil {
		return identity.User{}, fmt.Errorf("postgres: user by id: %w", err)
	}
	out.Role = identity.Role(role)
	return out, nil
}
```

`internal/adapters/postgres/sessions.go`:

```go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"babylon/internal/identity"
)

var _ identity.SessionRepo = (*SessionRepo)(nil)

type SessionRepo struct{ pool *pgxpool.Pool }

func NewSessionRepo(pool *pgxpool.Pool) *SessionRepo { return &SessionRepo{pool: pool} }

func (r *SessionRepo) Create(ctx context.Context, s identity.Session) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		s.TokenHash, s.UserID, s.ExpiresAt)
	if err != nil {
		return fmt.Errorf("postgres: create session: %w", err)
	}
	return nil
}

func (r *SessionRepo) ByTokenHash(ctx context.Context, hash string) (identity.Session, error) {
	var s identity.Session
	err := r.pool.QueryRow(ctx,
		`SELECT token_hash, user_id, expires_at FROM sessions WHERE token_hash = $1`, hash,
	).Scan(&s.TokenHash, &s.UserID, &s.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Session{}, identity.ErrNotFound
	}
	if err != nil {
		return identity.Session{}, fmt.Errorf("postgres: session by hash: %w", err)
	}
	return s, nil
}

func (r *SessionRepo) Delete(ctx context.Context, hash string) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hash); err != nil {
		return fmt.Errorf("postgres: delete session: %w", err)
	}
	return nil
}
```

- [ ] **Step 6: Tidy, then verify compile and skip behaviour**

Run: `GOTOOLCHAIN=local go mod tidy && head -3 go.mod && go test ./internal/adapters/postgres/ -v 2>&1 | tail -15 && go vet ./...`
Expected: `go 1.25.3`; the 4 tests report `--- SKIP ... TEST_DATABASE_URL not set`; package `ok`; vet silent.

- [ ] **Step 7: User-run verification against a real Postgres**

Ask the user to run these (sandbox cannot reach localhost); each line with the `!` prefix:

```
! container system start
! container run -d --name pg -e POSTGRES_PASSWORD=pg -e POSTGRES_DB=babylon_test -p 5432:5432 postgres:17
! TEST_DATABASE_URL=postgres://postgres:pg@localhost:5432/babylon_test go test ./internal/adapters/postgres/ -v
```
Expected: 4 tests `PASS`, none skipped. If any fail, fix `users.go` / `sessions.go` / `schema.sql` and re-run before committing. Leave the `pg` container running; later tasks and the final manual check reuse it.

- [ ] **Step 8: Commit**

```bash
git add internal/adapters/postgres go.mod go.sum
git commit -m "feat(postgres): add user and session repositories" -m "pgx pool; idempotent schema applied at boot; integration tests gated on TEST_DATABASE_URL." -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 6: System adapter and web adapter

**Files:**
- Create: `internal/adapters/system/system.go`, `internal/adapters/web/web.go`, `internal/adapters/web/templates/me.html`
- Test: `internal/adapters/system/system_test.go`, `internal/adapters/web/web_test.go`

**Interfaces:**
- Consumes: `identity.Service` (`LoginURL`, `LoginWithGitHub`, `Authenticate`, `Logout`), `identity.TokenGenerator`, `identity.ErrUnauthenticated`, and `identitytest.NewEnv` for tests.
- Produces:
  - `system.Clock{}` (`Now() time.Time`) and `system.Tokens{}` (`New() (string, error)`: 32 random bytes, base64url, 43 chars).
  - `web.Handler{Auth *identity.Service; Tokens identity.TokenGenerator; Log *slog.Logger; SecureCookies bool}`; `(*Handler).Mount(mux *http.ServeMux)` registers `GET /auth/login`, `GET /auth/callback`, `POST /auth/logout`, `GET /me`; `web.UserFrom(ctx) (identity.User, bool)`.

- [ ] **Step 1: Write the failing system tests**

`internal/adapters/system/system_test.go`:

```go
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
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/adapters/system/ 2>&1 | head -5`
Expected: FAIL to compile: `no non-test Go files` / `undefined: system.Tokens`.

- [ ] **Step 3: Write the system adapter**

`internal/adapters/system/system.go`:

```go
// Package system provides the real clock and token generator for the identity ports.
package system

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

type Clock struct{}

func (Clock) Now() time.Time { return time.Now() }

// Tokens generates 256-bit random URL-safe tokens.
type Tokens struct{}

func (Tokens) New() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
```

- [ ] **Step 4: Verify system tests pass**

Run: `go test ./internal/adapters/system/ -v 2>&1 | tail -6`
Expected: `TestTokens`, `TestClock` PASS.

- [ ] **Step 5: Write the failing web tests**

`internal/adapters/web/web_test.go`:

```go
package web_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"babylon/internal/adapters/web"
	"babylon/internal/identity"
	"babylon/internal/identity/identitytest"
)

func newServer(t *testing.T, secure bool, admins ...string) (http.Handler, *identitytest.Env) {
	t.Helper()
	env := identitytest.NewEnv(admins...)
	h := &web.Handler{
		Auth:          env.Service,
		Tokens:        env.Tokens,
		Log:           slog.New(slog.DiscardHandler),
		SecureCookies: secure,
	}
	mux := http.NewServeMux()
	h.Mount(mux)
	return mux, env
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %q cookie in %v", name, cookies)
	return nil
}

func do(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// startLogin runs GET /auth/login and returns the state and the state cookie.
func startLogin(t *testing.T, h http.Handler) (string, *http.Cookie) {
	t.Helper()
	rec := do(h, httptest.NewRequest("GET", "/auth/login", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("login status = %d, want 302", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return loc.Query().Get("state"), findCookie(t, rec.Result().Cookies(), "oauth_state")
}

// login drives the whole OAuth flow through HTTP and returns the session cookie.
func login(t *testing.T, h http.Handler, code string) *http.Cookie {
	t.Helper()
	state, stateCookie := startLogin(t, h)
	req := httptest.NewRequest("GET", "/auth/callback?code="+code+"&state="+url.QueryEscape(state), nil)
	req.AddCookie(stateCookie)
	rec := do(h, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
		t.Fatalf("callback = %d -> %q, want 302 -> /", rec.Code, rec.Header().Get("Location"))
	}
	return findCookie(t, rec.Result().Cookies(), "session")
}

func getMe(h http.Handler, session *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/me", nil)
	if session != nil {
		req.AddCookie(session)
	}
	return do(h, req)
}

func TestLoginRedirectsToProviderWithStateCookie(t *testing.T) {
	h, _ := newServer(t, false)
	rec := do(h, httptest.NewRequest("GET", "/auth/login", nil))
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if loc.Host != "github.test" || loc.Query().Get("state") == "" {
		t.Fatalf("Location = %q", rec.Header().Get("Location"))
	}
	c := findCookie(t, rec.Result().Cookies(), "oauth_state")
	if c.Value != loc.Query().Get("state") || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Secure {
		t.Fatalf("state cookie = %+v", c)
	}
}

func TestCookiesAreSecureWhenConfigured(t *testing.T) {
	h, _ := newServer(t, true)
	rec := do(h, httptest.NewRequest("GET", "/auth/login", nil))
	if c := findCookie(t, rec.Result().Cookies(), "oauth_state"); !c.Secure {
		t.Fatal("state cookie not Secure")
	}
	session := login(t, h, "code-alice")
	if !session.Secure || !session.HttpOnly || session.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie = %+v", session)
	}
}

// The core multi-user scenario: each person sees only their own identity, and
// the admin allowlist is honoured.
func TestMeShowsEachUsersOwnIdentity(t *testing.T) {
	h, _ := newServer(t, false, "root")
	alice := login(t, h, "code-alice")
	bob := login(t, h, "code-bob")
	root := login(t, h, "code-root")

	tests := []struct {
		name    string
		session *http.Cookie
		want    []string
		notWant []string
	}{
		{"alice", alice, []string{"alice", "(user)"}, []string{"bob", "Root", "(admin)"}},
		{"bob", bob, []string{"bob", "(user)"}, []string{"alice", "Root", "(admin)"}},
		{"root", root, []string{"Root", "(admin)"}, []string{"alice", "bob"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := getMe(h, tc.session)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			body := rec.Body.String()
			for _, s := range tc.want {
				if !strings.Contains(body, s) {
					t.Errorf("body missing %q: %s", s, body)
				}
			}
			for _, s := range tc.notWant {
				if strings.Contains(body, s) {
					t.Errorf("body leaks %q: %s", s, body)
				}
			}
		})
	}
}

func TestMeRedirectsAnonymousToLogin(t *testing.T) {
	h, _ := newServer(t, false)
	rec := getMe(h, nil)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/auth/login" {
		t.Fatalf("got %d -> %q, want 302 -> /auth/login", rec.Code, rec.Header().Get("Location"))
	}
}

func TestMeUsesHXRedirectForHtmx(t *testing.T) {
	h, _ := newServer(t, false)
	req := httptest.NewRequest("GET", "/me", nil)
	req.Header.Set("HX-Request", "true")
	rec := do(h, req)
	if rec.Code != http.StatusOK || rec.Header().Get("HX-Redirect") != "/auth/login" {
		t.Fatalf("got %d, HX-Redirect=%q", rec.Code, rec.Header().Get("HX-Redirect"))
	}
}

func TestMeRejectsBogusAndExpiredSessions(t *testing.T) {
	h, env := newServer(t, false)
	if rec := getMe(h, &http.Cookie{Name: "session", Value: "forged"}); rec.Code != http.StatusFound {
		t.Fatalf("forged cookie status = %d, want 302", rec.Code)
	}
	session := login(t, h, "code-alice")
	env.Clock.T = env.Clock.T.Add(identitytest.SessionTTL)
	if rec := getMe(h, session); rec.Code != http.StatusFound {
		t.Fatalf("expired session status = %d, want 302", rec.Code)
	}
}

type brokenSessions struct{}

func (brokenSessions) Create(context.Context, identity.Session) error { return errors.New("db down") }
func (brokenSessions) ByTokenHash(context.Context, string) (identity.Session, error) {
	return identity.Session{}, errors.New("db down")
}
func (brokenSessions) Delete(context.Context, string) error { return errors.New("db down") }

// A database outage must not look like "logged out" (which would bounce users
// into a login loop); it is a 500.
func TestMeBackendFailureIs500(t *testing.T) {
	h, env := newServer(t, false)
	env.Service.Sessions = brokenSessions{}
	rec := getMe(h, &http.Cookie{Name: "session", Value: "anything"})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestCallbackRejectsBadState(t *testing.T) {
	h, _ := newServer(t, false)
	state, stateCookie := startLogin(t, h)

	tests := []struct {
		name  string
		query string
		add   *http.Cookie
	}{
		{"wrong state", "code=code-alice&state=forged", stateCookie},
		{"missing state param", "code=code-alice", stateCookie},
		{"missing state cookie", "code=code-alice&state=" + url.QueryEscape(state), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/auth/callback?"+tc.query, nil)
			if tc.add != nil {
				req.AddCookie(tc.add)
			}
			rec := do(h, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			for _, c := range rec.Result().Cookies() {
				if c.Name == "session" && c.MaxAge >= 0 {
					t.Fatalf("session cookie set on rejected callback: %+v", c)
				}
			}
		})
	}
}

func TestCallbackProviderFailureIs502(t *testing.T) {
	h, env := newServer(t, false)
	state, stateCookie := startLogin(t, h)
	req := httptest.NewRequest("GET", "/auth/callback?code=nope&state="+url.QueryEscape(state), nil)
	req.AddCookie(stateCookie)
	rec := do(h, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	if env.Sessions.Len() != 0 {
		t.Fatal("session created on failed login")
	}
}

func TestCallbackMissingCodeIs400(t *testing.T) {
	h, _ := newServer(t, false)
	state, stateCookie := startLogin(t, h)
	req := httptest.NewRequest("GET", "/auth/callback?state="+url.QueryEscape(state), nil)
	req.AddCookie(stateCookie)
	if rec := do(h, req); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestLogout(t *testing.T) {
	h, _ := newServer(t, false)
	session := login(t, h, "code-alice")

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	req.AddCookie(session)
	rec := do(h, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("logout = %d -> %q, want 303 -> /", rec.Code, rec.Header().Get("Location"))
	}
	if c := findCookie(t, rec.Result().Cookies(), "session"); c.MaxAge >= 0 {
		t.Fatalf("session cookie not cleared: %+v", c)
	}
	// The old cookie is dead server-side too, not just cleared in the browser.
	if rec := getMe(h, session); rec.Code != http.StatusFound {
		t.Fatalf("old session still works after logout: %d", rec.Code)
	}
}

func TestLogoutRequiresPost(t *testing.T) {
	h, _ := newServer(t, false)
	if rec := do(h, httptest.NewRequest("GET", "/auth/logout", nil)); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /auth/logout = %d, want 405", rec.Code)
	}
}

func TestMeEscapesLogin(t *testing.T) {
	h, env := newServer(t, false)
	env.Provider.Codes["code-xss"] = identity.Profile{GitHubID: 9, Login: "<script>alert(1)</script>"}
	rec := getMe(h, login(t, h, "code-xss"))
	if strings.Contains(rec.Body.String(), "<script>") {
		t.Fatalf("login not escaped: %s", rec.Body.String())
	}
}
```

- [ ] **Step 6: Run to verify failure**

Run: `go test ./internal/adapters/web/ 2>&1 | head -5`
Expected: FAIL to compile: `no non-test Go files` / `undefined: web.Handler`.

- [ ] **Step 7: Write the template**

`internal/adapters/web/templates/me.html`:

```html
{{define "me"}}<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>me - babylon</title>
</head>
<body>
<p>Signed in as <strong>{{.Login}}</strong> ({{.Role}})</p>
<form method="post" action="/auth/logout"><button>Log out</button></form>
</body>
</html>{{end}}
```

- [ ] **Step 8: Write the handler**

`internal/adapters/web/web.go`:

```go
// Package web is the HTTP adapter for the identity context: GitHub login,
// logout, session middleware and the /me page.
package web

import (
	"bytes"
	"context"
	"crypto/subtle"
	"embed"
	"errors"
	"html/template"
	"log/slog"
	"net/http"

	"babylon/internal/identity"
)

//go:embed templates/*.html
var assets embed.FS

var tmpl = template.Must(template.ParseFS(assets, "templates/*.html"))

const (
	sessionCookie = "session"
	stateCookie   = "oauth_state"
)

type Handler struct {
	Auth          *identity.Service
	Tokens        identity.TokenGenerator // used for the OAuth state value
	Log           *slog.Logger
	SecureCookies bool
}

// Mount registers the identity routes on mux.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/login", h.login)
	mux.HandleFunc("GET /auth/callback", h.callback)
	mux.HandleFunc("POST /auth/logout", h.logout)
	mux.HandleFunc("GET /me", h.requireUser(h.me))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	state, err := h.Tokens.New()
	if err != nil {
		h.Log.Error("new state", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.setCookie(w, &http.Cookie{Name: stateCookie, Value: state, Path: "/auth", MaxAge: 600})
	http.Redirect(w, r, h.Auth.LoginURL(state), http.StatusFound)
}

func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	got := r.URL.Query().Get("state")
	c, err := r.Cookie(stateCookie)
	if err != nil || got == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(got)) != 1 {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	h.setCookie(w, &http.Cookie{Name: stateCookie, Path: "/auth", MaxAge: -1})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	res, err := h.Auth.LoginWithGitHub(r.Context(), code)
	if err != nil {
		h.Log.Error("login failed", "err", err)
		http.Error(w, "login failed", http.StatusBadGateway)
		return
	}
	h.setCookie(w, &http.Cookie{Name: sessionCookie, Value: res.Token, Path: "/", Expires: res.ExpiresAt})
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := h.Auth.Logout(r.Context(), c.Value); err != nil {
			h.Log.Error("logout", "err", err)
		}
	}
	h.setCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFrom(r.Context())
	render(w, "me", u)
}

type userKey struct{}

// UserFrom returns the signed-in user placed in the context by requireUser.
func UserFrom(ctx context.Context) (identity.User, bool) {
	u, ok := ctx.Value(userKey{}).(identity.User)
	return u, ok
}

// requireUser resolves the session cookie and calls next with the user in the
// context. Not-logged-in sends the browser to /auth/login (via HX-Redirect for
// htmx requests, so a login page is never swapped into a fragment); an
// infrastructure error is a 500, not a login bounce.
func (h *Handler) requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := h.currentUser(r)
		switch {
		case errors.Is(err, identity.ErrUnauthenticated):
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/auth/login")
				w.WriteHeader(http.StatusOK)
				return
			}
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		case err != nil:
			h.Log.Error("authenticate", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
	}
}

func (h *Handler) currentUser(r *http.Request) (identity.User, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return identity.User{}, identity.ErrUnauthenticated
	}
	return h.Auth.Authenticate(r.Context(), c.Value)
}

// setCookie applies the attributes every cookie here shares.
func (h *Handler) setCookie(w http.ResponseWriter, c *http.Cookie) {
	c.HttpOnly = true
	c.Secure = h.SecureCookies
	c.SameSite = http.SameSiteLaxMode
	http.SetCookie(w, c)
}

// render executes into a buffer first so a template error is a clean 500.
func render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}
```

- [ ] **Step 9: Run tests to verify they pass**

Run: `go test ./internal/adapters/... -v 2>&1 | grep -E "^(--- |=== RUN|ok|FAIL|PASS)" | grep -vE "=== RUN" | head -60 && go vet ./...`
Expected: every web test `PASS` (15 including subtests: login redirect, secure cookies, 3 `/me` users, anonymous, htmx, bogus/expired, backend failure, 3 bad-state subtests, provider failure, missing code, logout, logout-needs-POST, escaping); vet silent. Any `FAIL` must be fixed in `web.go` before moving on.

- [ ] **Step 10: Commit**

```bash
git add internal/adapters/system internal/adapters/web
git commit -m "feat(web): add GitHub login routes, session middleware and /me" -m "Also adds the system clock/token adapter. Backend errors in auth are 500s, not login bounces." -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Config, composition root and Dockerfile

**Files:**
- Create: `config.go`
- Test: `config_test.go`, modify `main_test.go`
- Modify: `main.go`, `Dockerfile`

**Interfaces:**
- Consumes: `web.Handler{...}.Mount`, `postgres.Migrate/NewUserRepo/NewSessionRepo`, `github.New(github.Config)`, `system.Clock{}/Tokens{}`, `identity.Service`.
- Produces: `loadConfig(getenv func(string) string) (config, error)`, `config.secureCookies() bool`, `routes(mounts ...func(*http.ServeMux)) http.Handler` (variadic, so existing `routes()` calls and tests keep compiling).

- [ ] **Step 1: Write the failing config tests**

`config_test.go`:

```go
package main

import (
	"reflect"
	"strings"
	"testing"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func fullEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":         "postgres://u:p@h:5432/db",
		"GITHUB_CLIENT_ID":     "cid",
		"GITHUB_CLIENT_SECRET": "sec",
		"BASE_URL":             "http://localhost:8080/",
	}
}

func TestLoadConfig(t *testing.T) {
	env := fullEnv()
	env["ADMIN_LOGINS"] = " root, ,Alice ,"
	env["ADDR"] = ":9000"
	c, err := loadConfig(envFrom(env))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":9000" || c.DatabaseURL != env["DATABASE_URL"] || c.GitHubClientID != "cid" || c.GitHubClientSecret != "sec" {
		t.Fatalf("config = %+v", c)
	}
	if c.BaseURL != "http://localhost:8080" {
		t.Fatalf("BaseURL = %q, want trailing slash trimmed", c.BaseURL)
	}
	if want := []string{"root", "Alice"}; !reflect.DeepEqual(c.AdminLogins, want) {
		t.Fatalf("AdminLogins = %q, want %q", c.AdminLogins, want)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	c, err := loadConfig(envFrom(fullEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8080" || len(c.AdminLogins) != 0 {
		t.Fatalf("config = %+v", c)
	}
}

func TestLoadConfigNamesEveryMissingVar(t *testing.T) {
	_, err := loadConfig(envFrom(map[string]string{"GITHUB_CLIENT_ID": "cid"}))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, name := range []string{"DATABASE_URL", "GITHUB_CLIENT_SECRET", "BASE_URL"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not mention %s", err, name)
		}
	}
	if strings.Contains(err.Error(), "GITHUB_CLIENT_ID") {
		t.Errorf("error %q mentions a variable that was set", err)
	}
}

func TestSecureCookies(t *testing.T) {
	tests := map[string]bool{
		"http://localhost:8080": false,
		"http://127.0.0.1:8080": false,
		"https://example.com":   true,
		"http://example.com":    true,
	}
	for base, want := range tests {
		if got := (config{BaseURL: base}).secureCookies(); got != want {
			t.Errorf("secureCookies(%q) = %v, want %v", base, got, want)
		}
	}
}
```

And append to `main_test.go`:

```go
func TestRoutesMountsExtraRoutes(t *testing.T) {
	h := routes(func(mux *http.ServeMux) {
		mux.HandleFunc("GET /extra", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("mounted"))
		})
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/extra", nil))
	if rec.Code != 200 || rec.Body.String() != "mounted" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	// Existing routes still work alongside mounts.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 200 {
		t.Fatalf("/healthz = %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test . 2>&1 | head -8`
Expected: FAIL to compile: `undefined: loadConfig`, `undefined: config`, and `too many arguments in call to routes`.

- [ ] **Step 3: Write config**

`config.go`:

```go
package main

import (
	"fmt"
	"strings"
)

type config struct {
	Addr               string
	DatabaseURL        string
	GitHubClientID     string
	GitHubClientSecret string
	BaseURL            string
	AdminLogins        []string
}

// loadConfig reads the environment through getenv (os.Getenv in main) so tests
// can supply their own. Every missing required variable is named in one error.
func loadConfig(getenv func(string) string) (config, error) {
	c := config{
		Addr:               ":8080",
		DatabaseURL:        getenv("DATABASE_URL"),
		GitHubClientID:     getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: getenv("GITHUB_CLIENT_SECRET"),
		BaseURL:            strings.TrimRight(getenv("BASE_URL"), "/"),
		AdminLogins:        splitLogins(getenv("ADMIN_LOGINS")),
	}
	if v := getenv("ADDR"); v != "" {
		c.Addr = v
	}

	var missing []string
	for _, kv := range []struct{ name, val string }{
		{"DATABASE_URL", c.DatabaseURL},
		{"GITHUB_CLIENT_ID", c.GitHubClientID},
		{"GITHUB_CLIENT_SECRET", c.GitHubClientSecret},
		{"BASE_URL", c.BaseURL},
	} {
		if kv.val == "" {
			missing = append(missing, kv.name)
		}
	}
	if len(missing) > 0 {
		return config{}, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}
	return c, nil
}

func splitLogins(csv string) []string {
	var out []string
	for _, s := range strings.Split(csv, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// secureCookies is false only for plain-http local development, where browsers
// would drop Secure cookies.
func (c config) secureCookies() bool {
	return !(strings.HasPrefix(c.BaseURL, "http://localhost") ||
		strings.HasPrefix(c.BaseURL, "http://127.0.0.1"))
}
```

- [ ] **Step 4: Rewire main.go**

In `main.go`, replace the import block with:

```go
import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"babylon/internal/adapters/github"
	"babylon/internal/adapters/postgres"
	"babylon/internal/adapters/system"
	"babylon/internal/adapters/web"
	"babylon/internal/identity"
)
```

Replace the whole `func main() { ... }` with:

```go
func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := postgres.Migrate(ctx, pool); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}

	auth := &identity.Service{
		Users:    postgres.NewUserRepo(pool),
		Sessions: postgres.NewSessionRepo(pool),
		Provider: github.New(github.Config{
			ClientID:     cfg.GitHubClientID,
			ClientSecret: cfg.GitHubClientSecret,
			RedirectURL:  cfg.BaseURL + "/auth/callback",
		}),
		Tokens:     system.Tokens{},
		Clock:      system.Clock{},
		Admins:     cfg.AdminLogins,
		SessionTTL: 7 * 24 * time.Hour,
	}
	ui := &web.Handler{
		Auth:          auth,
		Tokens:        system.Tokens{},
		Log:           log,
		SecureCookies: cfg.secureCookies(),
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           logging(log, routes(ui.Mount)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info("listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown failed", "err", err)
		os.Exit(1)
	}
}
```

Replace `routes()`:

```go
// routes builds the mux. Mounts let adapters register their own routes; with
// none given it serves only the demo routes, which is what most tests want.
func routes(mounts ...func(*http.ServeMux)) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleIndex)
	mux.HandleFunc("POST /greet", handleGreet)
	mux.HandleFunc("GET /time", handleTime)
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /hello/{name}", handleHello)
	for _, mount := range mounts {
		mount(mux)
	}
	return mux
}
```

Note: the casbin adapter is intentionally not wired. No resource consumes `authz.Authorizer` yet; the first resource context will take it as a port.

- [ ] **Step 5: Update the Dockerfile**

In `Dockerfile` change `COPY go.mod ./` to `COPY go.mod go.sum ./` (the module now has dependencies, so `go.sum` must be present for `go mod download`).

- [ ] **Step 6: Tidy, run the full suite**

Run: `GOTOOLCHAIN=local go mod tidy && head -3 go.mod && gofmt -l . && go vet ./... && go test ./... 2>&1 | tail -15`
Expected: `go 1.25.3`; `gofmt -l` prints nothing; vet silent; every package `ok` (postgres tests skipped). `go.mod` should list pgx, oauth2 and casbin as direct requirements (no `// indirect` on them).

- [ ] **Step 7: Verify the binary builds and fails fast without config**

Run: `go build -o /tmp/claude-501/-Users-binhct-workspace-babylon/0ab86ea0-0ce2-4788-8467-16c5fe84248c/scratchpad/babylon-bin . && env -i /tmp/claude-501/-Users-binhct-workspace-babylon/0ab86ea0-0ce2-4788-8467-16c5fe84248c/scratchpad/babylon-bin; echo "exit=$?"`
Expected: a log line `level=ERROR msg=config err="missing required env: DATABASE_URL, GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET, BASE_URL"` and `exit=1`.

- [ ] **Step 8: Commit**

```bash
git add config.go config_test.go main.go main_test.go Dockerfile go.mod go.sum
git commit -m "feat: wire identity into main with env config" -m "routes() takes optional mounts so adapters register on the shared mux. Dockerfile copies go.sum now that the module has dependencies." -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Documentation, security review and end-to-end check

**Files:**
- Modify: `CLAUDE.md`, `.claude/skills/project-conventions/SKILL.md`, `docs/superpowers/specs/2026-10-24-identity-authz-design.md`, `.claude/settings.json` (already edited, commit as-is)

**Interfaces:** none.

- [ ] **Step 1: Update `CLAUDE.md`**

(a) In the Commands code block, replace the line `go run .                              # serve on :8080 (override with ADDR=:9000)` with:

```
go run .                              # serve on :8080 (override with ADDR=:9000); needs the env vars below
```

and add this section directly after the Commands code block that ends with `container stop babylon` and the distroless sentence:

````markdown
## Configuration

The app refuses to boot without these env vars:

```
DATABASE_URL=postgres://postgres:pg@localhost:5432/babylon   # Postgres
GITHUB_CLIENT_ID=...  GITHUB_CLIENT_SECRET=...                # GitHub OAuth app
BASE_URL=http://localhost:8080                                # callback is BASE_URL/auth/callback
ADMIN_LOGINS=alice,bob                                        # optional; these GitHub logins become admins
```

Local Postgres: `container run -d --name pg -e POSTGRES_PASSWORD=pg -e POSTGRES_DB=babylon -p 5432:5432 postgres:17`. Postgres integration tests run only when `TEST_DATABASE_URL` is set; otherwise they skip.
````

(b) Replace the sentence beginning `Single-package app (` through the end of that paragraph's first line: change

`Single-package app (`main.go`, no subpackages). Two response styles share one `ServeMux` built by `routes()`:`

to

`The demo app lives in `main.go` (package main). New code follows DDD + hexagonal: `internal/<context>` holds domain and application code and its ports (no HTTP/SQL/OAuth/Casbin imports); `internal/adapters/{postgres,github,casbin,web,system}` implement the ports; `main.go` is the composition root. `internal/identity` is the login/session context; `internal/authz` is the shared kernel with the `Authorizer` port (Casbin ABAC: owner or admin). Two response styles share one `ServeMux` built by `routes()`:`

(c) Replace `` `routes()` is the test seam: `main_test.go` drives it with `httptest`, no real port. `` with `` `routes(mounts...)` is the test seam: `main_test.go` drives it with `httptest`, no real port; adapters register their routes through the mounts. Auth tests use `identity/identitytest` fakes instead of GitHub or Postgres. ``

(d) In "Environment gotchas" add a bullet: `- Postgres and the real GitHub login can't be reached from Claude's sandbox. Have the user run DB-backed tests (`TEST_DATABASE_URL=... go test ./internal/adapters/postgres/`) and the browser login flow with `!`.`

- [ ] **Step 2: Update the conventions skill**

In `.claude/skills/project-conventions/SKILL.md`: under `## Go` add the bullet

`- **Hexagonal layout.** Domain and application code in `internal/<context>` imports no HTTP, SQL, OAuth or Casbin packages; it declares ports in `ports.go`. Adapters live in `internal/adapters/<name>` and implement them. Wire everything in `main.go`.`

and under `## Tests` add

`- Services are tested with the in-memory fakes in `internal/identity/identitytest`; adapters get their own tests (Postgres gated on `TEST_DATABASE_URL`, GitHub against an `httptest` fake). Multi-user access tests log in as several users through the real HTTP flow with different session cookies.`

- [ ] **Step 3: Amend the spec to match what was built**

In `docs/superpowers/specs/2026-10-24-identity-authz-design.md`:
- In "Architecture", replace the sentence ``registers its routes on the same mux; `routes(app)` remains the test seam.`` with ``registers its routes on the same mux via `routes(mounts ...func(*http.ServeMux))`, which remains the test seam.``
- In "Middleware", replace the two bullets about `loadUser` and `requireUser` with one bullet: ``- `requireUser` wraps protected routes: it reads the cookie, looks up a non-expired session and puts the user in the request context. Not-logged-in requests redirect to `/auth/login` (htmx requests get `HX-Redirect` so a login page is never swapped into a fragment). An infrastructure error is a 500, not a login redirect. A separate `loadUser` for all routes was dropped: nothing but `requireUser` consumes the user yet.``

- [ ] **Step 4: Run the security reviewer**

Dispatch the `security-reviewer` agent (read-only) on `internal/adapters/web/web.go`, `internal/adapters/github/github.go`, `internal/adapters/postgres/*.go`, `internal/identity/service.go`, `main.go` and `Dockerfile`. Fix any real finding (with a test) in a separate commit before continuing; report anything you decline to fix.

- [ ] **Step 5: Final automated verification**

Run: `gofmt -l . && go vet ./... && go test ./... && head -3 go.mod`
Expected: no gofmt output, vet silent, all packages `ok`, `go 1.25.3`.

- [ ] **Step 6: Commit the docs and the earlier rule-drop edits**

```bash
git add CLAUDE.md .claude/settings.json .claude/skills/project-conventions/SKILL.md docs/superpowers/specs/2026-10-24-identity-authz-design.md
git commit -m "docs: document hexagonal layout, config and auth testing; drop std-lib-only rule" -m "The go.mod/go.sum edit-block hook is removed: std lib is preferred, not required." -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 7: User-run end-to-end check (needs the user)**

Tell the user to:

1. Register a GitHub OAuth app (GitHub Settings → Developer settings → OAuth Apps) with callback `http://localhost:8080/auth/callback`; note the client ID and secret. Create a second GitHub account (or ask a friend) for the second user.
2. Ensure Postgres is up (`! container ls` shows `pg`; if not, run the `container run` from Task 5 with `POSTGRES_DB=babylon`).
3. Run the app:
   `! DATABASE_URL=postgres://postgres:pg@localhost:5432/babylon GITHUB_CLIENT_ID=... GITHUB_CLIENT_SECRET=... BASE_URL=http://localhost:8080 ADMIN_LOGINS=<account-1-login> go run .`
4. Browser profile A: open `http://localhost:8080/me` → expect redirect to GitHub → approve → expect "Signed in as <account-1> (admin)".
5. Browser profile B (or incognito): same with account 2 → expect "(user)", and no sign of account 1.
6. In each profile click "Log out", then reload `/me` → expect redirect to GitHub again.
7. Stop the app, restart without `ADMIN_LOGINS`, log in as account 1 again → expect "(user)" (role re-synced).
8. Confirm `http://localhost:8080/`, `/time` and `/healthz` still work without logging in.

Report any deviation as a bug before treating the branch as done.

---

## Self-review (spec coverage)

| Spec section | Task |
|---|---|
| Architecture layout, dependency rule | 1-7 (file structure) |
| Schema `users`, `sessions` | 5 |
| Sessions: hashed token, cookie attrs, 7 days | 1 (`HashToken`), 2 (TTL), 6 (cookies), 7 (Secure rule) |
| Routes `/auth/login`, `/auth/callback`, `/auth/logout`, `/me` | 6 |
| Role from `ADMIN_LOGINS`, re-synced each login | 1, 2 |
| `requireUser`, `HX-Redirect` | 6 (spec amended in 8: `loadUser` folded into it) |
| Casbin owner-or-admin model, string IDs, anonymous denied, create not an authz decision | 3 |
| Config env + fail at boot | 7 |
| Dependencies (pgx, oauth2, casbin) | 3, 4, 5 (pinned for Go 1.25.3) |
| Tests: domain, service fakes, casbin table, GitHub fake, web multi-user, postgres gated | 1-6 |
| Manual E2E with two accounts, no dev-login backdoor | 8 |
| Deployment: Postgres container, no volume; Dockerfile | 7 (go.sum), 8 (docs) |
| Docs to update | 8 |

Type names used across tasks were checked for consistency: `identity.Service` fields (`Users, Sessions, Provider, Tokens, Clock, Admins, SessionTTL`), `identitytest.Env` fields (`Service, Clock, Sessions, Tokens, Provider`), `web.Handler` fields (`Auth, Tokens, Log, SecureCookies`), `postgres.NewUserRepo/NewSessionRepo/Migrate`, `github.New(github.Config)`, `casbin.New()`, `system.Clock{}/Tokens{}`.
