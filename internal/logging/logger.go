// Package logging configures TrailLog's structured logger.
package logging

import (
	"io"
	"log/slog"

	"github.com/maciejlistos/traillog/internal/config"
)

// New creates a structured logger that writes to output according to cfg.
func New(cfg config.LogConfig, output io.Writer) *slog.Logger {
	options := &slog.HandlerOptions{
		Level: cfg.Level,
	}

	var handler slog.Handler

	switch cfg.Format {
	case config.LogFormatJSON:
		handler = slog.NewJSONHandler(output, options)
	default:
		handler = slog.NewTextHandler(output, options)
	}

	return slog.New(handler)
}
