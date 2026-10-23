package observability

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otlpmetricgrpc "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otlptracegrpc "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Telemetry owns TrailLog's tracing and metrics providers.
type Telemetry struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
}

// New configures OTLP tracing and metrics for a service.
func New(
	ctx context.Context,
	serviceName string,
	serviceVersion string,
	environment string,
) (*Telemetry, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			attribute.String(
				"service.name",
				serviceName,
			),
			attribute.String(
				"service.version",
				serviceVersion,
			),
			attribute.String(
				"deployment.environment.name",
				environment,
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create OpenTelemetry resource: %w",
			err,
		)
	}

	tracerProvider, err := newTracerProvider(ctx, res)
	if err != nil {
		return nil, err
	}

	meterProvider, err := newMeterProvider(ctx, res)
	if err != nil {
		_ = tracerProvider.Shutdown(
			context.Background(),
		)

		return nil, err
	}

	otel.SetTracerProvider(
		tracerProvider,
	)

	otel.SetMeterProvider(
		meterProvider,
	)

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return &Telemetry{
		tracerProvider: tracerProvider,
		meterProvider:  meterProvider,
	}, nil
}

func newTracerProvider(
	ctx context.Context,
	res *resource.Resource,
) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"create OTLP trace exporter: %w",
			err,
		)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),

		sdktrace.WithSampler(
			sdktrace.ParentBased(
				sdktrace.AlwaysSample(),
			),
		),

		sdktrace.WithBatcher(
			exporter,
		),
	)

	return provider, nil
}

func newMeterProvider(
	ctx context.Context,
	res *resource.Resource,
) (*sdkmetric.MeterProvider, error) {
	exporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"create OTLP metric exporter: %w",
			err,
		)
	}

	reader := sdkmetric.NewPeriodicReader(
		exporter,
	)

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)

	return provider, nil
}

// Meter returns a meter from the configured provider.
func (t *Telemetry) Meter(
	name string,
) metric.Meter {
	return t.meterProvider.Meter(name)
}

// Tracer returns a tracer from the configured provider.
func (t *Telemetry) Tracer(name string) trace.Tracer {
	return t.tracerProvider.Tracer(name)
}

// Shutdown flushes pending telemetry and stops the providers.
func (t *Telemetry) Shutdown(
	ctx context.Context,
) error {
	return errors.Join(
		t.meterProvider.Shutdown(ctx),
		t.tracerProvider.Shutdown(ctx),
	)
}
