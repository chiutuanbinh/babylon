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
