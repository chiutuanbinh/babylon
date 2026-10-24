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
