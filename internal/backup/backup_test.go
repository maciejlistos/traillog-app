package backup

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateBacksUpLiveWALDatabase(
	t *testing.T,
) {
	root := t.TempDir()

	sourcePath := filepath.Join(
		root,
		"live.db",
	)

	backupDirectory := filepath.Join(
		root,
		"backups",
	)

	source, err := sql.Open(
		"sqlite",
		sourcePath,
	)
	if err != nil {
		t.Fatalf(
			"open source: %v",
			err,
		)
	}

	defer source.Close()

	if _, err := source.Exec(
		"PRAGMA journal_mode=WAL",
	); err != nil {
		t.Fatalf(
			"enable WAL: %v",
			err,
		)
	}

	if _, err := source.Exec(`
			CREATE TABLE hikes (
				id INTEGER PRIMARY KEY,
				name TEXT NOT NULL
			)
		`); err != nil {
		t.Fatalf(
			"create test table: %v",
			err,
		)
	}

	if _, err := source.Exec(
		"INSERT INTO hikes(name) VALUES (?)",
		"Forest Loop",
	); err != nil {
		t.Fatalf(
			"insert test row: %v",
			err,
		)
	}

	fixedTime := time.Date(
		2026,
		time.October,
		14,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	result, err := Create(
		context.Background(),
		CreateOptions{
			SourcePath: sourcePath,

			DestinationDir: backupDirectory,

			Retain: 5,

			Now: func() time.Time {
				return fixedTime
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"Create() returned error: %v",
			err,
		)
	}

	if err := Verify(
		context.Background(),
		result.Path,
	); err != nil {
		t.Fatalf(
			"Verify() returned error: %v",
			err,
		)
	}

	restored, err := sql.Open(
		"sqlite",
		result.Path,
	)
	if err != nil {
		t.Fatalf(
			"open backup: %v",
			err,
		)
	}

	defer restored.Close()

	var name string

	if err := restored.QueryRow(
		"SELECT name FROM hikes WHERE id = 1",
	).Scan(
		&name,
	); err != nil {
		t.Fatalf(
			"read backed-up row: %v",
			err,
		)
	}

	if got, want :=
		name,
		"Forest Loop"; got != want {
		t.Errorf(
			"name = %q, want %q",
			got,
			want,
		)
	}

	if _, err := os.Stat(
		result.ChecksumPath,
	); err != nil {
		t.Errorf(
			"checksum file missing: %v",
			err,
		)
	}
}

func TestPruneKeepsNewestBackups(
	t *testing.T,
) {
	directory := t.TempDir()

	names := []string{
		"traillog-20261010T120000.000000000Z.db",
		"traillog-20261011T120000.000000000Z.db",
		"traillog-20261012T120000.000000000Z.db",
	}

	for _, name := range names {

		path := filepath.Join(
			directory,
			name,
		)

		if err := os.WriteFile(
			path,
			[]byte("test"),
			0o640,
		); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(
			path+".sha256",
			[]byte("test"),
			0o640,
		); err != nil {
			t.Fatal(err)
		}
	}

	if err := Prune(
		directory,
		2,
	); err != nil {
		t.Fatalf(
			"Prune() returned error: %v",
			err,
		)
	}

	if _, err := os.Stat(
		filepath.Join(
			directory,
			names[0],
		),
	); !os.IsNotExist(err) {
		t.Errorf(
			"oldest backup was not pruned",
		)
	}

	for _, name := range names[1:] {
		if _, err := os.Stat(
			filepath.Join(
				directory,
				name,
			),
		); err != nil {
			t.Errorf(
				"retained backup %s missing: %v",
				name,
				err,
			)
		}
	}

	for i, name := range names {
		_, err := os.Stat(filepath.Join(directory, name) + ".sha256")
		if i == 0 {
			if !os.IsNotExist(err) {
				t.Errorf("oldest checksum was not pruned: %v", err)
			}
		} else if err != nil {
			t.Errorf("retained checksum %s missing: %v", name, err)
		}
	}
}

func TestRestoreReplacesDatabaseAndPreservesPreviousCopy(
	t *testing.T,
) {
	root := t.TempDir()

	sourcePath := filepath.Join(
		root,
		"source.db",
	)

	backupDirectory := filepath.Join(
		root,
		"backups",
	)

	targetPath := filepath.Join(
		root,
		"target",
		"traillog.db",
	)

	if err := os.MkdirAll(
		filepath.Dir(
			targetPath,
		),
		0o750,
	); err != nil {
		t.Fatal(err)
	}

	createDatabaseWithName := func(
		path string,
		name string,
	) {
		t.Helper()

		db, err := sql.Open(
			"sqlite",
			path,
		)
		if err != nil {
			t.Fatal(err)
		}

		defer db.Close()

		if _, err := db.Exec(`
					CREATE TABLE hikes (
						id INTEGER PRIMARY KEY,
						name TEXT NOT NULL
					)
				`); err != nil {
			t.Fatal(err)
		}

		if _, err := db.Exec(
			"INSERT INTO hikes(name) VALUES (?)",
			name,
		); err != nil {
			t.Fatal(err)
		}
	}

	createDatabaseWithName(
		sourcePath,
		"Backup Route",
	)

	result, err := Create(
		context.Background(),
		CreateOptions{
			SourcePath: sourcePath,

			DestinationDir: backupDirectory,

			Retain: 5,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	createDatabaseWithName(
		targetPath,
		"Current Route",
	)

	restoreResult, err := Restore(
		context.Background(),
		result.Path,
		targetPath,
	)
	if err != nil {
		t.Fatalf(
			"Restore() returned error: %v",
			err,
		)
	}

	if restoreResult.PreviousDatabase == "" {
		t.Fatal(
			"previous database was not preserved",
		)
	}

	previous, err := sql.Open("sqlite", restoreResult.PreviousDatabase)
	if err != nil {
		t.Fatal(err)
	}
	defer previous.Close()
	var previousName string
	if err := previous.QueryRow("SELECT name FROM hikes WHERE id = 1").Scan(&previousName); err != nil {
		t.Fatal(err)
	}
	if previousName != "Current Route" {
		t.Errorf("previous name = %q, want %q", previousName, "Current Route")
	}

	db, err := sql.Open(
		"sqlite",
		targetPath,
	)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	var name string

	if err := db.QueryRow(
		"SELECT name FROM hikes WHERE id = 1",
	).Scan(
		&name,
	); err != nil {
		t.Fatal(err)
	}

	if got, want :=
		name,
		"Backup Route"; got != want {
		t.Errorf(
			"restored name = %q, want %q",
			got,
			want,
		)
	}

	t.Run("missing target", func(t *testing.T) {
		newTarget := filepath.Join(root, "new-target", "traillog.db")
		result, err := Restore(context.Background(), result.Path, newTarget)
		if err != nil {
			t.Fatalf("Restore() returned error: %v", err)
		}
		if result.PreviousDatabase != "" {
			t.Errorf("unexpected previous database: %s", result.PreviousDatabase)
		}
		restored, err := sql.Open("sqlite", newTarget)
		if err != nil {
			t.Fatal(err)
		}
		defer restored.Close()
		var name string
		if err := restored.QueryRow("SELECT name FROM hikes WHERE id = 1").Scan(&name); err != nil {
			t.Fatal(err)
		}
		if name != "Backup Route" {
			t.Errorf("restored name = %q, want %q", name, "Backup Route")
		}
	})
}
