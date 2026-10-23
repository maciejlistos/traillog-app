package backup

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"strings"
)

// CheckDatabase performs SQLite structural and foreign-key integrity checks on
// databasePath.
//
// The function does not require a checksum sidecar and is therefore useful for
// checking restored databases as well as backup files.
func CheckDatabase(
	ctx context.Context,
	databasePath string,
) error {
	db, err := openSQLite(databasePath, "ro")
	if err != nil {
		return err
	}

	defer db.Close()

	rows, err := db.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return fmt.Errorf("run SQLite integrity_check: %w", err)
	}

	var integrityErrors []string

	for rows.Next() {
		var message string

		if err := rows.Scan(&message); err != nil {
			rows.Close()

			return fmt.Errorf("read integrity_check result: %w", err)
		}

		if message != "ok" {
			integrityErrors = append(integrityErrors, message)
		}
	}

	if err := rows.Err(); err != nil {
		rows.Close()

		return fmt.Errorf("iterate integrity_check result: %w", err)
	}

	rows.Close()

	if len(integrityErrors) > 0 {
		return fmt.Errorf("SQLite integrity check failed: %s", strings.Join(integrityErrors, "; "))
	}

	foreignKeys, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("run SQLite foreign_key_check: %w", err)
	}

	defer foreignKeys.Close()

	if foreignKeys.Next() {
		var table string
		var rowID sql.NullInt64
		var parent string
		var foreignKeyID int

		if err := foreignKeys.Scan(
			&table,
			&rowID,
			&parent,
			&foreignKeyID,
		); err != nil {
			return fmt.Errorf("read foreign_key_check result: %w", err)
		}

		return fmt.Errorf("foreign-key violation: table=%s parent=%s foreign_key_id=%d", table, parent, foreignKeyID)
	}

	if err := foreignKeys.Err(); err != nil {
		return fmt.Errorf("iterate foreign_key_check result: %w", err)
	}

	return nil
}

// Verify checks a backup's SHA-256 sidecar and SQLite integrity.
func Verify(
	ctx context.Context,
	backupPath string,
) error {
	checksumPath := backupPath + ".sha256"

	expected, err := readChecksum(checksumPath)
	if err != nil {
		return err
	}

	actual, err := checksumFile(backupPath)
	if err != nil {
		return err
	}

	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("backup checksum mismatch: expected=%s actual=%s", expected, actual)
	}

	if err := CheckDatabase(ctx, backupPath); err != nil {
		return fmt.Errorf("backup database verification failed: %w", err)
	}

	return nil
}

func readChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open checksum file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("read checksum file: %w", err)
		}

		return "", fmt.Errorf("checksum file is empty")
	}

	fields := strings.Fields(scanner.Text())

	if len(fields) == 0 {
		return "", fmt.Errorf("checksum file has no checksum")
	}

	if len(fields[0]) != sha256.Size*2 {
		return "", fmt.Errorf("checksum has invalid length")
	}

	return fields[0], nil
}
