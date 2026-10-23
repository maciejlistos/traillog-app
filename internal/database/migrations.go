package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate applies all pending embedded migrations and returns the number applied.
func Migrate(ctx context.Context, db *sqlx.DB) (int, error) {
	migrations, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return 0, fmt.Errorf("open embeded migrations: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectSQLite3,
		db.DB,
		migrations,
	)
	if err != nil {
		return 0, fmt.Errorf("create migration provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return 0, fmt.Errorf("apply database migrations: %w", err)
	}

	return len(results), nil
}
