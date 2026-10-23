// Package server configures TrailLog's HTTP server and routes.
package server

import (
	"net/http"

	"github.com/maciejlistos/traillog/internal/config"
)

// NewHTTPServer creates a configured HTTP server.
func NewHTTPServer(
	cfg config.HTTPConfig,
	handler http.Handler,
) *http.Server {
	return &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}
