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
