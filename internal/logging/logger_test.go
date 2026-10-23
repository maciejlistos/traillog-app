package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/maciejlistos/traillog/internal/config"
)

func TestNewCreatesJSONLogger(t *testing.T) {
	var output bytes.Buffer

	logger := New(config.LogConfig{
		Level:  slog.LevelInfo,
		Format: config.LogFormatJSON,
	}, &output)

	logger.Info(
		"application started",
		"environment",
		"test",
	)

	var record map[string]any

	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode log record: %v", err)
	}

	if got, want := record["msg"], "application started"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}

	if got, want := record["environment"], "test"; got != want {
		t.Errorf("environment = %q, want %q", got, want)
	}
}

func TestNewFiltersRecordsBelowConfiguredLevel(t *testing.T) {
	var output bytes.Buffer

	logger := New(config.LogConfig{
		Level:  slog.LevelWarn,
		Format: config.LogFormatText,
	}, &output)

	logger.Info("invisible")
	logger.Warn("visible")

	logOutput := output.String()

	if strings.Contains(logOutput, "invisible") {
		t.Errorf("info record was emitted below configured level")
	}

	if !strings.Contains(logOutput, "visible") {
		t.Errorf("warn record was not emitted")
	}
}
