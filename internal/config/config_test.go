package config

import (
	"log/slog"
	"testing"
)

func TestLoadUsesDefaultAddress(t *testing.T) {
	clearConfigurationEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpeced error: %v", err)
	}

	if got, want := cfg.HTTP.Address, ":8080"; got != want {
		t.Errorf("HTTP address = %q, want %q", got, want)
	}

	if got, want := cfg.Log.Level, slog.LevelInfo; got != want {
		t.Errorf("log level = %q, want %q", got, want)
	}

	if got, want := cfg.Log.Format, LogFormatText; got != want {
		t.Errorf("log format = %q, want %q", got, want)
	}

	if got, want := cfg.Database.Path, "data/traillog.db"; got != want {
		t.Errorf("database path = %q, want %q", got, want)
	}
}

func TestLoadReadsAddressFromEnvironment(t *testing.T) {
	t.Setenv("TRAILLOG_ENVIRONMENT", "production")
	t.Setenv("TRAILLOG_HTTP_ADDRESS", ":9090")
	t.Setenv("TRAILLOG_LOG_LEVEL", "debug")
	t.Setenv("TRAILLOG_LOG_FORMAT", "json")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned and unexpeced error: %v", err)
	}

	if got, want := cfg.Environment, "production"; got != want {
		t.Errorf("environment = %q, want %q", got, want)
	}

	if got, want := cfg.HTTP.Address, ":9090"; got != want {
		t.Errorf("HTTP address = %q, want %q", got, want)
	}

	if got, want := cfg.Log.Level, slog.LevelDebug; got != want {
		t.Errorf("log level = %q, want %q", got, want)
	}

	if got, want := cfg.Log.Format, LogFormatJSON; got != want {
		t.Errorf("log format = %q, want %q", got, want)
	}
}

func TestLoadUsesJSONLoggingInProduction(t *testing.T) {
	t.Setenv("TRAILLOG_ENVIRONMENT", "production")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned and unexpected error: %v", err)
	}

	if got, want := cfg.Log.Format, LogFormatJSON; got != want {
		t.Errorf("log format = %q, want %q", got, want)
	}
}

func TestLoadRejectsInvalidAddress(t *testing.T) {
	t.Setenv("TRAILLOG_HTTP_ADDRESS", "localhost:9090")

	_, err := Load()
	if err == nil {
		t.Fatalf("Load() returned nil error, want an error")
	}
}

func TestLoadRejectsInvalidLogLevel(t *testing.T) {
	clearConfigurationEnvironment(t)

	t.Setenv("TRAILLOG_LOG_LEVEL", "verbose")

	_, err := Load()
	if err == nil {
		t.Fatalf("Load() return nil error, want an error")
	}
}

func TestLoadRejectsInvalidLogFormat(t *testing.T) {
	clearConfigurationEnvironment(t)

	t.Setenv("TRAILLOG_LOG_FORMAT", "yaml")

	_, err := Load()
	if err == nil {
		t.Fatalf("Load() return nil error, want an error")
	}
}

func TestLoadReadsDatabasePath(t *testing.T) {
	clearConfigurationEnvironment(t)

	t.Setenv("TRAILLOG_DATABASE_PATH", "/tmp/traillog-test.db")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error")
	}

	if got, want := cfg.Database.Path, "/tmp/traillog-test.db"; got != want {
		t.Errorf("database path = %q, want %q", got, want)
	}
}

// --- Helpers ----
func clearConfigurationEnvironment(t *testing.T) {
	t.Helper()

	t.Setenv("TRAILLOG_ENVIRONMENT", "")
	t.Setenv("TRAILLOG_HTTP_ADDRESS", "")
	t.Setenv("TRAILLOG_LOG_LEVEL", "")
	t.Setenv("TRAILLOG_LOG_FORMAT", "")
	t.Setenv("TRAILLOG_DATABASE_PATH", "")
}
