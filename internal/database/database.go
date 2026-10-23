// Package database opens and migrates TrailLog's SQLite database.
package database

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/maciejlistos/traillog/internal/config"
)

const driverName = "sqlite"

// Open opens and verifies a configured SQLite database connection pool.
func Open(
	ctx context.Context,
	cfg config.DatabaseConfig,
) (*sqlx.DB, error) {
	if err := ensureDatabaseDirectory(cfg.Path); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	dsn, err := buildDSN(cfg)
	if err != nil {
		return nil, fmt.Errorf("building database DSN: %w", err)
	}

	db, err := sqlx.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConnections)

	pingContext, cancel := context.WithTimeout(
		ctx,
		cfg.ConnectTimeout,
	)
	defer cancel()

	if err := db.PingContext(pingContext); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("ping SQLite database: %w", err)
	}

	return db, nil
}

// --- Helpers ---

func buildDSN(cfg config.DatabaseConfig) (string, error) {
	absolutePath, err := filepath.Abs(cfg.Path)
	if err != nil {
		return "", fmt.Errorf("resolving absolute database path: %w", err)
	}

	databaseURL := &url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(absolutePath),
	}

	query := databaseURL.Query()

	query.Set("_foreign_keys", "on")
	query.Set("_busy_timeout", strconv.FormatInt(cfg.BusyTimeout.Milliseconds(), 10))
	query.Set("_journal_mode", "WAL")

	query.Set("_time_format", "sqlite")
	query.Set("_timezone", "UTC")
	query.Set("_texttotime", "1")

	databaseURL.RawQuery = query.Encode()

	return databaseURL.String(), nil
}

func ensureDatabaseDirectory(path string) error {
	directory := filepath.Dir(path)

	if err := os.MkdirAll(directory, 0o750); err != nil {
		return fmt.Errorf("create %q: %w", directory, err)
	}

	return nil
}
