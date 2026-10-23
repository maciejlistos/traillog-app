package hike

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func newNoopTracer() trace.Tracer {
	return noop.NewTracerProvider().Tracer("traillog/test")
}

func newRecordingTracerProvider(t *testing.T) (
	*sdktrace.TracerProvider,
	*tracetest.SpanRecorder,
) {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()

	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

	t.Cleanup(
		func() {
			if err := provider.Shutdown(context.Background()); err != nil {
				t.Errorf("shutdown tracer provider: %v", err)
			}
		},
	)

	return provider, recorder
}

func findEndedSpan(
	t *testing.T,
	recorder *tracetest.SpanRecorder,
	name string,
) sdktrace.ReadOnlySpan {
	t.Helper()

	for _, span := range recorder.Ended() {
		if span.Name() == name {
			return span
		}
	}

	t.Fatalf(
		"span %q not found",
		name,
	)

	return nil
}

func stringSpanAttribute(
	span sdktrace.ReadOnlySpan,
	name string,
) string {
	for _, attribute := range span.Attributes() {
		if string(attribute.Key) == name {
			return attribute.Value.AsString()
		}
	}

	return ""
}
