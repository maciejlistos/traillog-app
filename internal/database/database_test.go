package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/maciejlistos/traillog/internal/config"
)

func TestOpensConfigureSQLite(t *testing.T) {
	db := newTestDatabase(t)

	ctx := context.Background()

	var foreignKeys int

	if err := db.GetContext(
		ctx,
		&foreignKeys,
		"PRAGMA foreign_keys",
	); err != nil {
		t.Fatalf("query foreign_keys pragma: %v", err)
	}

	if got, want := foreignKeys, 1; got != want {
		t.Errorf("foreign_keys = %d, want %d", got, want)
	}

	var journalMode string

	if err := db.GetContext(ctx, &journalMode, "PRAGMA journal_mode"); err != nil {
		t.Fatalf("query journal_mode pragma: %v", err)
	}

	if got, want := journalMode, "wal"; got != want {
		t.Errorf("journal_mode = %q, want %q", got, want)
	}

	var busyTimeout int

	if err := db.GetContext(ctx, &busyTimeout, "PRAGMA busy_timeout"); err != nil {
		t.Fatalf("query busy_timeout pragma: %v", err)
	}

	if got, want := busyTimeout, 5_000; got != want {
		t.Errorf("busy_timeout = %d, want %d", got, want)
	}

	stats := db.Stats()

	if got, want := stats.MaxOpenConnections, 5; got != want {
		t.Errorf(
			"max open connections = %d, want %d",
			got,
			want,
		)
	}
}

func TestHikesRejectNegativeDistance(t *testing.T) {
	db := newTestDatabase(t)

	if _, err := Migrate(
		context.Background(),
		db,
	); err != nil {
		t.Fatalf(
			"apply migrations: %v",
			err,
		)
	}

	_, err := db.ExecContext(
		context.Background(),
		`
		INSERT INTO hikes (
			name,
			location,
			distance_km,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?)
		`,
		"Test hike",
		"Lublin",
		-10,
		time.Now(),
		time.Now(),
	)

	if err == nil {
		t.Fatal(
			"negative distance was accepted",
		)
	}
}

// --- Helpers ----

func newTestDatabase(t *testing.T) *sqlx.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "traillog-test.db")

	cfg := config.DatabaseConfig{
		Path:               path,
		BusyTimeout:        5 * time.Second,
		ConnectTimeout:     5 * time.Second,
		MaxOpenConnections: 5,
		MaxIdleConnections: 5,
	}

	db, err := Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	return db
}
