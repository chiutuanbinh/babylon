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
