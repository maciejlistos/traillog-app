package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

func TestRequestLoggerIncludesTraceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var output bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&output, nil))

	router := gin.New()

	router.Use(
		RequestID(
			func() string {
				return "request-123"
			},
		),
		RequestLogger(logger),
	)

	router.GET(
		"/example",
		func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		},
	)

	traceID, err := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("parse trace ID: %v", err)
	}

	spanID, err := trace.SpanIDFromHex("0123456789abcdef")
	if err != nil {
		t.Fatalf("parse span ID: %v", err)
	}

	spanContext := trace.NewSpanContext(
		trace.SpanContextConfig{
			TraceID:    traceID,
			SpanID:     spanID,
			TraceFlags: trace.FlagsSampled,
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/example", nil)

	request = request.WithContext(trace.ContextWithSpanContext(request.Context(), spanContext))

	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	var record map[string]any

	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode log: %v", err)
	}

	if got, want := record["trace_id"], "0123456789abcdef0123456789abcdef"; got != want {
		t.Errorf("trace_id = %v, want %v", got, want)
	}

	if got, want := record["span_id"], "0123456789abcdef"; got != want {
		t.Errorf("span_id = %v, want %v", got, want)
	}
}
