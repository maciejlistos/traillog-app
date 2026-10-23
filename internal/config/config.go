// Package config loads and validates TrailLog configuration from the environment.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

const (
	defaultHTTPAddress       = ":8080"
	defaultReadHeaderTimeout = 5 * time.Second
	defaultReadTimeout       = 10 * time.Second
	defaultWriteTimeout      = 15 * time.Second
	defaultIdleTimeout       = 60 * time.Second
	defaultShotdownTimeout   = 10 * time.Second

	defaultEnvironment = "development"
	defaultLogLevel    = slog.LevelInfo

	// LogFormatText selects human-readable structured logs.
	LogFormatText = "text"
	// LogFormatJSON selects JSON structured logs.
	LogFormatJSON = "json"

	defaultDatabasePath           = "data/traillog.db"
	defaultDatabaseBusyTimeout    = 5 * time.Second
	defaultDatabaseConnectTimeout = 5 * time.Second
	defaultDatabaseMaxOpenConns   = 5
	defaultDatabaseMaxIdleConns   = 5
)

// Config contains all configuration required by the application.
type Config struct {
	Environment string

	HTTP     HTTPConfig
	Log      LogConfig
	Database DatabaseConfig

	GPX GPXConfig
	Map MapConfig
}

// DatabaseConfig contains SQLite connection and pool settings.
type DatabaseConfig struct {
	Path               string
	BusyTimeout        time.Duration
	ConnectTimeout     time.Duration
	MaxOpenConnections int
	MaxIdleConnections int
}

// HTTPConfig contains HTTP server settings.
type HTTPConfig struct {
	Address           string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// LogConfig contains structured logging settings.
type LogConfig struct {
	Level  slog.Level
	Format string
}

// Load reads configuration from the environment.
func Load() (Config, error) {
	environment := readEnvironment(
		"TRAILLOG_ENVIRONMENT",
		defaultEnvironment,
	)

	address := readEnvironment(
		"TRAILLOG_HTTP_ADDRESS",
		defaultHTTPAddress,
	)

	if !strings.HasPrefix(address, ":") {
		return Config{}, fmt.Errorf(
			"TRAILLOG_HTTP_ADDRESS mus use the form :PORT, got %q",
			address,
		)
	}

	logLevel, err := loadLogLevel()
	if err != nil {
		return Config{}, err
	}

	logFormt, err := loadLogFormat(environment)
	if err != nil {
		return Config{}, err
	}

	gpxConfig, err := loadGPXConfig()
	if err != nil {
		return Config{}, fmt.Errorf("load GPX configuration: %w", err)
	}
	mapConfig := loadMapConfig()

	databasePath := readEnvironment("TRAILLOG_DATABASE_PATH", defaultDatabasePath)

	return Config{
		Environment: environment,
		HTTP: HTTPConfig{
			Address:           address,
			ReadHeaderTimeout: defaultReadHeaderTimeout,
			ReadTimeout:       defaultReadTimeout,
			WriteTimeout:      defaultWriteTimeout,
			IdleTimeout:       defaultIdleTimeout,
			ShutdownTimeout:   defaultShotdownTimeout,
		},
		Log: LogConfig{
			Level:  logLevel,
			Format: logFormt,
		},
		Database: DatabaseConfig{
			Path:               databasePath,
			BusyTimeout:        defaultDatabaseBusyTimeout,
			ConnectTimeout:     defaultDatabaseConnectTimeout,
			MaxOpenConnections: defaultDatabaseMaxOpenConns,
			MaxIdleConnections: defaultDatabaseMaxIdleConns,
		},
		GPX: gpxConfig,
		Map: mapConfig,
	}, nil
}

func loadLogLevel() (slog.Level, error) {
	value := readEnvironment("TRAILLOG_LOG_LEVEL", "info")

	var level slog.Level

	if err := level.UnmarshalText([]byte(value)); err != nil {
		return defaultLogLevel, fmt.Errorf(
			"parse TRAILLOG_LOG_LEVEL %q: %w",
			value,
			err,
		)
	}

	return level, nil
}

func loadLogFormat(environment string) (string, error) {
	defaultFormat := LogFormatText

	if environment == "production" {
		defaultFormat = LogFormatJSON
	}

	format := strings.ToLower(
		readEnvironment("TRAILLOG_LOG_FORMAT", defaultFormat),
	)

	switch format {
	case LogFormatText, LogFormatJSON:
		return format, nil
	default:
		return "", fmt.Errorf(
			"TRAILLOG_LOG_FORMAT must be %q or %q, got %q",
			LogFormatText,
			LogFormatJSON,
			format,
		)
	}
}

func readEnvironment(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))

	if value == "" {
		return fallback
	}

	return value
}

// GPXConfig contains operational limits applied to GPX uploads.
//
// These limits protect the HTTP and parsing layers from unbounded resource
// consumption. Domain-level validation may impose additional hard limits
type GPXConfig struct {
	MaxUploadBytes     int64
	MaxTrackPoints     int
	MaxTextFieldsBytes int64
}

// MapConfig configurews the browser map tile source.
//
// Attributions is trused operator-controlled HTML passed to Leaflet. It must
// satisfy the attribution requirements of the configured tile provider.
type MapConfig struct {
	TileURL     string
	Attribution string
}
