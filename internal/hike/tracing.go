package hike

import (
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func recordSpanError(
	span trace.Span,
	err error,
	description string,
) {
	span.RecordError(err)

	span.SetStatus(codes.Error, description)
}

func markSpanError(
	span trace.Span,
	description string,
) {
	span.SetStatus(
		codes.Error,
		description,
	)
}
