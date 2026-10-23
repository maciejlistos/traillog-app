package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

type healthCheckerFunc func(
	context.Context,
) error

func (f healthCheckerFunc) PingContext(ctx context.Context) error {
	return f(ctx)
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestHomeEndpoint(t *testing.T) {
	router := newTestRouter()

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusOK; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}

	var body struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	if got, want := body.Name, "TrailLog"; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}

	if body.Message == "" {
		t.Error("message is empty")
	}
}

func TestLivenessEndpoint(t *testing.T) {
	router := newTestRouter()

	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusOK; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}

	assertJSONStatus(t, response, "ok")
}

func TestReadinessEndpoint(t *testing.T) {
	router := newTestRouter()

	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusOK; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}

	assertJSONStatus(t, response, "ok")
}

func TestReadinessEndpointReturnsUnavailable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	checker := healthCheckerFunc(
		func(context.Context) error {
			return errors.New("database unavailable")
		},
	)

	router := NewRouter(logger, checker)

	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)

	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusServiceUnavailable; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}

	assertJSONStatus(t, response, "not_ready")
}

func TestVersionEndpoint(t *testing.T) {
	router := newTestRouter()
	request := httptest.NewRequest(http.MethodGet, "/version", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusOK; got != want {
		t.Fatalf("statuc code = %d, want %d", got, want)
	}

	contentType := response.Header().Get("Content-Type")

	if got, want := contentType, "application/json; charset=utf-8"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
}

func TestUnknownRouteRetrunsNotFound(t *testing.T) {
	router := newTestRouter()

	request := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusNotFound; got != want {
		t.Errorf("status code = %d, want %d", got, want)
	}
}

// --- Helpers ---

func assertJSONStatus(
	t *testing.T,
	response *httptest.ResponseRecorder,
	want string,
) {
	t.Helper()

	var body struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	if body.Status != want {
		t.Errorf("status = %q, want %q", body.Status, want)
	}
}

func newTestRouter() http.Handler {
	logger := slog.New(
		slog.NewTextHandler(
			io.Discard,
			nil,
		),
	)

	checker := healthCheckerFunc(
		func(context.Context) error {
			return nil
		},
	)

	return NewRouter(logger, checker)
}
