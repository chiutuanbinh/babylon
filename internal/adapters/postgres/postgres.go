// Package postgres implements the identity repositories on Postgres via pgx.
package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate applies pending goose migrations. All SQL schema changes must be a
// new numbered file in migrations/; never edit an applied one. Safe to run on
// every boot: goose records applied versions in goose_db_version, and
// concurrent instances serialize on a Postgres advisory lock.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	fsys, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("postgres: migrate: %w", err)
	}
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("postgres: migrate: %w", err)
	}
	p, err := goose.NewProvider(goose.DialectPostgres, db, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("postgres: migrate: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("postgres: migrate: %w", err)
	}
	return nil
}
