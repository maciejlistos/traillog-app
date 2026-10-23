package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/maciejlistos/traillog/internal/buildinfo"
	"github.com/maciejlistos/traillog/internal/config"
	"github.com/maciejlistos/traillog/internal/database"
	"github.com/maciejlistos/traillog/internal/hike"
	"github.com/maciejlistos/traillog/internal/logging"
	"github.com/maciejlistos/traillog/internal/observability"
	"github.com/maciejlistos/traillog/internal/server"
)

const (
	serviceName = "traillog"
)

func main() {
	if err := run(); err != nil {
		log.Printf("application stopped with an error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	// -- Config ---
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	// -- Logger ---
	logger := logging.New(cfg.Log, os.Stdout)
	slog.SetDefault(logger)

	// -- BuildInfo ---
	build := buildinfo.Current()

	// -- Telemetry ---
	telemetry, err := observability.New(
		context.Background(),
		serviceName,
		build.Version,
		cfg.Environment,
	)
	if err != nil {
		return fmt.Errorf("initialize observability: %w", err)
	}
	defer func() {
		shutdownContext, cancel := context.WithTimeout(
			context.Background(),
			cfg.HTTP.ShutdownTimeout,
		)
		defer cancel()

		if err := telemetry.Shutdown(shutdownContext); err != nil {
			logger.Error("shut down telemetry", "error", err)
		}
	}()

	// --- Database ---
	db, err := database.Open(context.Background(), cfg.Database)
	if err != nil {
		return fmt.Errorf("initialize database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("close database", "error", err)
		}
	}()

	// --- Migrations ---
	migrationCount, err := database.Migrate(context.Background(), db)
	if err != nil {
		return fmt.Errorf("migrate database: %v", err)
	}
	logger.Info("database ready", "path", cfg.Database.Path, "migration_applied", migrationCount)

	// --- DB Telemetry ---
	databaseMetricsRegistration, err := observability.RegisterDatabaseMetrics(
		telemetry.Meter("github.com/maciejlistos/traillog/internal/database"),
		db,
	)
	if err != nil {
		return fmt.Errorf("register database metrics: %w", err)
	}
	defer func() {
		if err := databaseMetricsRegistration.Unregister(); err != nil {
			logger.Error("unregister database metrics", "error", err)
		}
	}()

	// --- Hikes ---
	hikeRepository := hike.NewRepository(
		db,
		telemetry.Tracer("traillog/hike/repository"),
	)
	hikeService := hike.NewService(
		hikeRepository,
		telemetry.Tracer("traillog/hike/service"),
	)
	hikeAPIHandler := hike.NewHandler(hikeService)
	hikeWebHandler := hike.NewWebHandler(
		hikeService,
		hike.WebHandlerOptions{
			GPX: hike.GPXUploadLimits{
				MaxUploadBytes:    cfg.GPX.MaxUploadBytes,
				MaxTrackPoints:    cfg.GPX.MaxTrackPoints,
				MaxTextFieldBytes: cfg.GPX.MaxTextFieldsBytes,
			},
			Map: hike.MapOptions{
				TileURL:     cfg.Map.TileURL,
				Attribution: cfg.Map.Attribution,
			},
		},
	)
	gpxExportHandler := hike.NewGPXExportHandler(hikeService)

	// --- Statistics ---
	statsHandler := hike.NewStatsHandler(hikeService)

	// --- Router ---
	router := server.NewRouter(
		logger,
		db,
		hikeAPIHandler,
		hikeWebHandler,
		statsHandler,
		gpxExportHandler,
	)

	logger.Info(
		"starting TrailLog",
		"version",
		build.Version,
		"revision",
		build.Revision,
		"build_time",
		build.BuildTime,
		"environment",
		cfg.Environment,
	)
	httpServer := server.NewHTTPServer(cfg.HTTP, router)

	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)

	// --- Run http server ---
	go func() {
		logger.Info("TrailLog started", "address", cfg.HTTP.Address)

		serverErrors <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}

		return nil

	case <-shutdownContext.Done():
		log.Print("shutdown signal received")
	}

	timeoutContext, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(timeoutContext); err != nil {
		return fmt.Errorf("gracefully shut down HTTP server: %w", err)
	}

	err = <-serverErrors
	if !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("wait for HTTP server shutdown: %w", err)
	}

	log.Printf("TrailLog stopped")
	return nil
}
