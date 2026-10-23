package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/maciejlistos/traillog/internal/backup"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if len(os.Args) < 2 {
		printUsage()
		return 2
	}

	switch os.Args[1] {
	case "create":
		return runCreate(
			logger,
			os.Args[2:],
		)
	case "verify":
		return runVerify(
			logger,
			os.Args[2:],
		)
	case "check":
		return runCheck(
			logger,
			os.Args[2:],
		)
	case "restore":
		return runRestore(
			logger,
			os.Args[2:],
		)
	case "schedule":
		return runSchedule(
			logger,
			os.Args[2:],
		)
	default:
		printUsage()
		return 2
	}
}

func printUsage() {
	fmt.Fprintln(
		os.Stderr,
		`usage:
  traillog-backup create   --source DB --destination DIR --retain N
  traillog-backup verify   --backup FILE
  traillog-backup check    --database FILE
  traillog-backup restore  --backup FILE --target DB
  traillog-backup restore  --latest --backup-dir DIR --target DB
  traillog-backup schedule --source DB --destination DIR --retain N --interval DURATION --initial-delay DURATION`,
	)
}

func runCreate(
	logger *slog.Logger,
	args []string,
) int {
	flags := flag.NewFlagSet(
		"create",
		flag.ContinueOnError,
	)

	source := flags.String(
		"source",
		"",
		"source SQLite database",
	)

	destination := flags.String(
		"destination",
		"",
		"backup directory",
	)

	retain := flags.Int(
		"retain",
		14,
		"number of logical backups to retain",
	)

	if err := flags.Parse(args); err != nil {
		return 2
	}

	ctx := context.Background()

	result, err := backup.Create(
		ctx,
		backup.CreateOptions{
			SourcePath: *source,

			DestinationDir: *destination,

			Retain: *retain,
		},
	)
	if err != nil {
		logger.Error(
			"backup failed",
			"error",
			err,
		)

		return 1
	}

	logger.Info(
		"backup created",
		"path",
		result.Path,
		"sha256",
		result.SHA256,
		"size_bytes",
		result.SizeBytes,
	)

	return 0
}

func runVerify(
	logger *slog.Logger,
	args []string,
) int {
	flags := flag.NewFlagSet(
		"verify",
		flag.ContinueOnError,
	)

	path := flags.String(
		"backup",
		"",
		"backup database file",
	)

	if err := flags.Parse(args); err != nil {
		return 2
	}

	if err := backup.Verify(
		context.Background(),
		*path,
	); err != nil {

		logger.Error(
			"backup verification failed",
			"error",
			err,
		)

		return 1
	}

	logger.Info(
		"backup verified",
		"path",
		*path,
	)

	return 0
}

func runCheck(
	logger *slog.Logger,
	args []string,
) int {
	flags := flag.NewFlagSet(
		"check",
		flag.ContinueOnError,
	)

	path := flags.String(
		"database",
		"",
		"SQLite database file",
	)

	if err := flags.Parse(args); err != nil {
		return 2
	}

	if err := backup.CheckDatabase(
		context.Background(),
		*path,
	); err != nil {

		logger.Error(
			"database check failed",
			"error",
			err,
		)

		return 1
	}

	logger.Info(
		"database is healthy",
		"path",
		*path,
	)

	return 0
}

func runRestore(
	logger *slog.Logger,
	args []string,
) int {
	flags := flag.NewFlagSet(
		"restore",
		flag.ContinueOnError,
	)

	backupPath := flags.String(
		"backup",
		"",
		"backup database file",
	)

	backupDirectory := flags.String(
		"backup-dir",
		"",
		"directory containing backups",
	)

	latest := flags.Bool(
		"latest",
		false,
		"restore newest backup from --backup-dir",
	)

	target := flags.String(
		"target",
		"",
		"target SQLite database",
	)

	if err := flags.Parse(args); err != nil {
		return 2
	}

	selected := *backupPath

	if *latest {
		path, err := backup.Latest(
			*backupDirectory,
		)
		if err != nil {
			logger.Error(
				"select latest backup",
				"error",
				err,
			)

			return 1
		}

		selected = path
	}

	result, err := backup.Restore(
		context.Background(),
		selected,
		*target,
	)
	if err != nil {
		logger.Error(
			"restore failed",
			"error",
			err,
		)

		return 1
	}

	logger.Info(
		"restore completed",
		"backup",
		selected,
		"target",
		result.TargetPath,
		"previous_database",
		result.PreviousDatabase,
	)

	return 0
}

func runSchedule(
	logger *slog.Logger,
	args []string,
) int {
	flags := flag.NewFlagSet(
		"schedule",
		flag.ContinueOnError,
	)

	source := flags.String(
		"source",
		"",
		"source SQLite database",
	)

	destination := flags.String(
		"destination",
		"",
		"backup directory",
	)

	retain := flags.Int(
		"retain",
		14,
		"number of backups to retain",
	)

	interval := flags.Duration(
		"interval",
		24*time.Hour,
		"backup interval",
	)

	initialDelay := flags.Duration(
		"initial-delay",
		2*time.Minute,
		"delay before first backup",
	)

	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *interval <= 0 {
		logger.Error(
			"interval must be positive",
		)

		return 2
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	defer stop()

	logger.Info(
		"backup scheduler started",
		"interval",
		*interval,
		"initial_delay",
		*initialDelay,
		"retain",
		*retain,
	)

	timer := time.NewTimer(
		*initialDelay,
	)

	defer timer.Stop()

	select {
	case <-ctx.Done():
		return 0

	case <-timer.C:
	}

	create := func() {
		backupContext, cancel := context.WithTimeout(
			ctx,
			30*time.Minute,
		)

		defer cancel()

		result, err := backup.Create(
			backupContext,
			backup.CreateOptions{
				SourcePath: *source,

				DestinationDir: *destination,

				Retain: *retain,
			},
		)
		if err != nil {
			logger.Error(
				"scheduled backup failed",
				"error",
				err,
			)

			return
		}

		logger.Info(
			"scheduled backup created",
			"path",
			result.Path,
			"sha256",
			result.SHA256,
			"size_bytes",
			result.SizeBytes,
		)
	}

	create()

	ticker := time.NewTicker(
		*interval,
	)

	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return 0

		case <-ticker.C:
			create()
		}
	}
}
