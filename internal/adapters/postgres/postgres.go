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
