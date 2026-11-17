package backup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// CreateOptions configures one logical SQLite backup.
type CreateOptions struct {
	SourcePath     string
	DestinationDir string
	Retain         int

	// Now exists primarily to make backup naming deterministric in tests.
	// When nil, time.Now is used.
	Now func() time.Time
}

// Result describes one successfully created backup.
type Result struct {
	Path         string
	ChecksumPath string
	SHA256       string
	CreatedAd    time.Time
	SizeBytes    int64
}

func openSQLite(
	path string,
	mode string,
) (*sql.DB, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}

	location := url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(absolutePath),
	}

	query := location.Query()
	query.Set("mode", mode)
	query.Add("_pragma", "busy_timeout(5000)")

	location.RawQuery = query.Encode()

	db, err := sql.Open("sqlite", location.String())
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}

	// The backup helper needs one physical connection.
	// This also makes connection-level PRAGMA behavio predictable.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	return db, nil
}

// Create creates and verifies one logical backup of a live SQLite database.
//
// The source database remains available while SQLite creates the destination
// database. A backup is not published under its final filename until its
// integrity, foreign keys, and checksum have been verified.
func Create(
	ctx context.Context,
	options CreateOptions,
) (Result, error) {
	if strings.TrimSpace(options.SourcePath) == "" {
		return Result{}, fmt.Errorf("source database path is required")
	}

	if strings.TrimSpace(options.DestinationDir) == "" {
		return Result{}, fmt.Errorf("backup destination directory is required")
	}

	if options.Retain < 1 {
		return Result{}, fmt.Errorf("backup retention must be at least 1")
	}

	now := options.Now

	if now == nil {
		now = time.Now
	}

	createdAt := now().UTC()

	sourceInfo, err := os.Stat(options.SourcePath)
	if err != nil {
		return Result{}, fmt.Errorf("stat source database: %w", err)
	}

	if !sourceInfo.Mode().IsRegular() {
		return Result{}, fmt.Errorf("source database is not a regular file")
	}

	if err := os.MkdirAll(options.DestinationDir, 0o750); err != nil {
		return Result{}, fmt.Errorf("create backup directory: %w", err)
	}

	baseName := fmt.Sprintf("traillog-%s.db", createdAt.Format("20060102T150405.000000000Z"))

	finalPath := filepath.Join(options.DestinationDir, baseName)
	temporaryPath := finalPath + ".tmp"

	if _, err := os.Stat(finalPath); err == nil {
		return Result{}, fmt.Errorf("backup already exists: %s", finalPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, fmt.Errorf("check backup destination: %w", err)
	}

	_ = os.Remove(temporaryPath)
	defer func() {
		_ = os.Remove(temporaryPath)
	}()

	source, err := openSQLite(options.SourcePath, "rw")
	if err != nil {
		return Result{}, err
	}
	defer source.Close()

	if err := source.PingContext(ctx); err != nil {
		return Result{}, fmt.Errorf("ping source database: %w", err)
	}

	// SQLite accepts the destination as a scalar expression, so a boudn
	// parameter keeps the filename out of SQL string construction.
	_, err = source.ExecContext(
		ctx,
		"VACUUM INTO ?",
		temporaryPath,
	)
	if err != nil {
		return Result{}, fmt.Errorf("VACUUM INTO backup: %w", err)
	}

	if err := CheckDatabase(
		ctx,
		temporaryPath,
	); err != nil {
		return Result{}, fmt.Errorf("verify new backup: %w", err)
	}

	if err := syncFile(temporaryPath); err != nil {
		return Result{}, err
	}

	checksum, err := checksumFile(temporaryPath)
	if err != nil {
		return Result{}, err
	}

	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return Result{}, fmt.Errorf("publish backup: %w", err)
	}

	checksumPath := finalPath + ".sha256"

	if err := writeChecksum(
		checksumPath,
		checksum,
		filepath.Base(finalPath),
	); err != nil {
		_ = os.Remove(finalPath)

		return Result{}, err
	}

	if err := syncDirectory(options.DestinationDir); err != nil {
		return Result{}, err
	}

	if err := Prune(options.DestinationDir, options.Retain); err != nil {
		return Result{}, fmt.Errorf("prune old backups: %w", err)
	}

	info, err := os.Stat(finalPath)
	if err != nil {
		return Result{}, fmt.Errorf("stat completed backup: %w", err)
	}

	return Result{
		Path:         finalPath,
		ChecksumPath: checksumPath,
		SHA256:       checksum,
		CreatedAd:    createdAt,
		SizeBytes:    info.Size(),
	}, nil
}

func checksumFile(
	path string,
) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open backup for checksum: %w", err)
	}

	defer file.Close()

	hash := sha256.New()

	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("calculate backup checksum: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeChecksum(
	path string,
	checksum string,
	backupName string,
) error {
	temporary := path + ".tmp"

	content := fmt.Sprintf("%s %s\n", checksum, backupName)

	if err := os.WriteFile(temporary, []byte(content), 0o640); err != nil {
		return fmt.Errorf("write checksum: %w", err)
	}

	if err := syncFile(temporary); err != nil {
		_ = os.Remove(temporary)

		return err
	}

	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("publish checksum: %w", err)
	}

	return nil
}

func syncFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open %s for sync: %w", path, err)
	}

	defer file.Close()

	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", path, err)
	}

	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open backup directory for sync: %w", err)
	}
	defer directory.Close()

	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync backup directory: %w", err)
	}

	return nil
}

// Prune retains the newest retain completed TrailLog backups and removes older
// database/checksum paris.
func Prune(
	directory string,
	retain int,
) error {
	if retain < 1 {
		return fmt.Errorf("backup retention must be at least 1")
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read backup directory: %w", err)
	}

	var backups []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if !strings.HasPrefix(name, "traillog-") {
			continue
		}

		if !strings.HasSuffix(name, ".db") {
			continue
		}

		backups = append(backups, name)
	}

	sort.Strings(backups)

	excess := len(backups) - retain

	if excess <= 0 {
		return nil
	}

	for _, name := range backups[:excess] {
		databasePath := filepath.Join(directory, name)
		checksumPath := databasePath + ".sha256"

		if err := os.Remove(databasePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove old backup %s: %w", name, err)
		}

		if err := os.Remove(checksumPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove old checksum %s: %w", name, err)
		}
	}
	return syncDirectory(directory)
}

// Latest returns the newest completed TrailLog backup in directory.
func Latest(
	directory string,
) (string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", fmt.Errorf("read backup directory: %w", err)
	}

	var backups []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if strings.HasPrefix(name, "traillog-") && strings.HasSuffix(name, ".db") {
			backups = append(backups, name)
		}
	}

	if len(backups) == 0 {
		return "", fmt.Errorf("no TrailLog backups found")
	}

	sort.Strings(backups)

	return filepath.Join(directory, backups[len(backups)-1]), nil
}
