package server

import (
	"net/http"
	"testing"
	"time"

	"github.com/maciejlistos/traillog/internal/config"
)

func TestNewHTTPServer(t *testing.T) {
	handler := http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		response.WriteHeader(http.StatusNoContent)
	})

	cfg := config.HTTPConfig{
		Address:           ":9090",
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       3 * time.Second,
		WriteTimeout:      4 * time.Second,
		IdleTimeout:       5 * time.Second,
	}

	httpServer := NewHTTPServer(cfg, handler)

	if got, want := httpServer.Addr, ":9090"; got != want {
		t.Errorf("address = %q, want %q", got, want)
	}

	if httpServer.Handler == nil {
		t.Fatal("handler is nil")
	}

	if got, want := httpServer.ReadHeaderTimeout, 2*time.Second; got != want {
		t.Errorf("read header timeout = %s, want %s", got, want)
	}

	if got, want := httpServer.ReadTimeout, 3*time.Second; got != want {
		t.Errorf("read timeout = %s, want %s", got, want)
	}

	if got, want := httpServer.WriteTimeout, 4*time.Second; got != want {
		t.Errorf("write timeout = %s, want %s", got, want)
	}

	if got, want := httpServer.IdleTimeout, 5*time.Second; got != want {
		t.Errorf("idle timeout = %s, want %s", got, want)
	}
}
