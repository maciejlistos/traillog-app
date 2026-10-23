package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// RestoreResult describes a completed database restore.
type RestoreResult struct {
	TargetPath       string
	PreviousDatabase string
}

// Restore replaces targetPath with backupPath after validating the backup.
//
// TrailLog must not be running against targetPath while Restore executes.
// The previous database and any WAL/SHM files are preserver under a timestamped pre-restore basename.
func Restore(
	ctx context.Context,
	backupPath string,
	targetPath string,
) (RestoreResult, error) {
	if err := Verify(
		ctx, backupPath,
	); err != nil {
		return RestoreResult{}, fmt.Errorf("verify restore source: %w", err)
	}

	targetDirectory := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDirectory, 0o750); err != nil {
		return RestoreResult{}, fmt.Errorf("create target directory: %w", err)
	}

	temporaryPath := targetPath + ".restore.tmp"

	_ = os.Remove(temporaryPath)

	defer func() {
		_ = os.Remove(temporaryPath)
	}()

	if err := copyFile(backupPath, temporaryPath); err != nil {
		return RestoreResult{}, err
	}

	if err := CheckDatabase(ctx, temporaryPath); err != nil {
		return RestoreResult{}, fmt.Errorf("verify restored temporary database: %w", err)
	}

	// Preserver the stopped database's WAL/SHM too: deleting a non-empty WAL.
	// would make the previous main file an incomplete rollback artifact.
	previousPath := ""

	if _, err := os.Stat(targetPath); err == nil {
		previousPath = fmt.Sprintf("%s.pre-restore-%s", targetPath, time.Now().UTC().Format("20060102T150405.000000000Z"))

		if err := os.Rename(targetPath, previousPath); err != nil {
			return RestoreResult{}, fmt.Errorf("preserve previous database: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return RestoreResult{}, fmt.Errorf("stat target database: %w", err)
	}

	type movedFile struct{ from, to string }
	var moved []movedFile
	if previousPath != "" {
		moved = append(moved, movedFile{targetPath, previousPath})
	}
	rollback := func(cause error) error {
		for i := len(moved) - 1; i >= 0; i-- {
			if err := os.Rename(moved[i].to, moved[i].from); err != nil {
				cause = errors.Join(cause, fmt.Errorf("rollback %s: %w", moved[i].from, err))
			}
		}
		return cause
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		from := targetPath + suffix
		if _, err := os.Stat(from); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return RestoreResult{}, rollback(err)
		}
		if previousPath == "" {
			return RestoreResult{}, rollback(fmt.Errorf("orphan SQLite sidecar: %s", from))
		}
		to := previousPath + suffix
		if err := os.Rename(from, to); err != nil {
			return RestoreResult{}, rollback(fmt.Errorf("preserve %s: %w", from, err))
		}
		moved = append(moved, movedFile{from, to})
	}

	if err := syncDirectory(targetDirectory); err != nil {
		return RestoreResult{}, rollback(err)
	}
	if err := os.Rename(temporaryPath, targetPath); err != nil {
		return RestoreResult{}, rollback(fmt.Errorf("activate restored database: %w", err))
	}

	if err := syncDirectory(targetDirectory); err != nil {
		return RestoreResult{}, err
	}

	return RestoreResult{
		TargetPath:       targetPath,
		PreviousDatabase: previousPath,
	}, nil
}

func copyFile(
	sourcePath string,
	destinationPath string,
) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open restore source: %w", err)
	}
	defer source.Close()

	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("create restore destination: %w", err)
	}

	success := false

	defer func() {
		_ = destination.Close()

		if !success {
			_ = os.Remove(destinationPath)
		}
	}()

	if _, err := io.Copy(destination, source); err != nil {
		return fmt.Errorf("copy restored database: %w", err)
	}

	if err := destination.Sync(); err != nil {
		return fmt.Errorf("sync restored database: %w", err)
	}

	if err := destination.Close(); err != nil {
		return fmt.Errorf("close restored database: %w", err)
	}

	success = true

	return nil
}
