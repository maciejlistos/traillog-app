package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RequestLogger logs one structured event for each completed HTTP request.
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()

		requestID := RequestIDFromGin(c)

		htmx := strings.EqualFold(c.GetHeader("HX-Request"), "true")

		span := trace.SpanFromContext(c.Request.Context())

		span.SetAttributes(
			attribute.String("traillog.request.id", requestID),
			attribute.Bool("traillog.request.htmx", htmx),
		)

		c.Next()

		status := c.Writer.Status()
		level := levelForStatus(status)

		attributes := []slog.Attr{
			slog.String("request_id", RequestIDFromGin(c)),
			slog.Bool("htmx", htmx),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("route", routeName(c)),
			slog.Int("status", status),
			slog.Int64("duration_ms", time.Since(startedAt).Microseconds()),
			slog.Int("response_bytes", c.Writer.Size()),
			slog.String("remote_address", remoteAddress(c.Request)),
		}

		spanContext := trace.SpanContextFromContext(c.Request.Context())

		if spanContext.IsValid() {
			attributes = append(
				attributes,
				slog.String("trace_id", spanContext.TraceID().String()),
				slog.String("span_id", spanContext.SpanID().String()),
			)
		}

		if len(c.Errors) > 0 {
			attributes = append(attributes, slog.String("errors", c.Errors.String()))
		}

		logger.LogAttrs(
			c.Request.Context(),
			level,
			"http request completed",
			attributes...,
		)
	}
}

func levelForStatus(status int) slog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

func remoteAddress(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}

	return request.RemoteAddr
}

func routeName(c *gin.Context) string {
	route := c.FullPath()

	if route == "" {
		return "unmatched"
	}

	return route
}
