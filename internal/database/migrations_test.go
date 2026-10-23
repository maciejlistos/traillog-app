package database

import (
	"context"
	"testing"
)

func TestMigrateCreatesHikesTable(t *testing.T) {
	db := newTestDatabase(t)

	applied, err := Migrate(context.Background(), db)
	if err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	if applied == 0 {
		t.Error("no migrations were appilied to a fresh database")
	}

	var tableName string

	err = db.GetContext(
		context.Background(),
		&tableName,
		`
		SELECT name
		FROM sqlite_schema
		WHERE
			type = 'table'
			AND name = 'hikes'
		`,
	)
	if err != nil {
		t.Fatalf("find hikes table: %v", err)
	}

	if got, want := tableName, "hikes"; got != want {
		t.Errorf("table = %q, want %q", got, want)
	}
}

func TestMigrateCreatesVisitsTable(t *testing.T) {
	db := newTestDatabase(t)

	applied, err := Migrate(context.Background(), db)
	if err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	if applied == 0 {
		t.Error("no migrations were appilied to a fresh database")
	}

	var tableName string

	err = db.GetContext(
		context.Background(),
		&tableName,
		`
		SELECT name
		FROM sqlite_schema
		WHERE
			type = 'table'
			AND name = 'visits'
		`,
	)
	if err != nil {
		t.Fatalf("find visits table: %v", err)
	}

	if got, want := tableName, "visits"; got != want {
		t.Errorf("table = %q, want %q", got, want)
	}
}

func TestMigrateCanRunRepeatedly(t *testing.T) {
	db := newTestDatabase(t)

	ctx := context.Background()

	applied, err := Migrate(ctx, db)
	if err != nil {
		t.Fatalf("first migration run: %v", err)
	}

	if applied == 0 {
		t.Error("no migrations were appilied to a fresh database")
	}

	second, err := Migrate(ctx, db)
	if err != nil {
		t.Fatalf("second migration run: %v", err)
	}

	if got, want := second, 0; got != want {
		t.Errorf("second run applied %d migrations, want %d", got, want)
	}
}
