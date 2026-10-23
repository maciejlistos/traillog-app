// Package observability configures TrailLog telemetry and database metrics.
package observability

import (
	"context"
	"database/sql"
	"fmt"

	"go.opentelemetry.io/otel/metric"
)

// DatabaseStatsSource exposes database connection-pool statistics.
type DatabaseStatsSource interface {
	Stats() sql.DBStats
}

// RegisterDatabaseMetrics registers observable database pool metrics.
func RegisterDatabaseMetrics(
	meter metric.Meter,
	database DatabaseStatsSource,
) (metric.Registration, error) {
	maxOpen, err := meter.Int64ObservableGauge(
		"traillog.db.pool.max_open_connections",
		metric.WithDescription(
			"Maximum number of open database connections.",
		),
		metric.WithUnit(
			"{connection}",
		),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create max-open database metric: %w",
			err,
		)
	}

	open, err := meter.Int64ObservableGauge(
		"traillog.db.pool.open_connections",
		metric.WithDescription(
			"Number of established database connections.",
		),
		metric.WithUnit(
			"{connection}",
		),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create open-connections metric: %w",
			err,
		)
	}

	inUse, err := meter.Int64ObservableGauge(
		"traillog.db.pool.in_use",
		metric.WithDescription(
			"Number of database connections currently in use.",
		),
		metric.WithUnit(
			"{connection}",
		),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create in-use database metric: %w",
			err,
		)
	}

	idle, err := meter.Int64ObservableGauge(
		"traillog.db.pool.idle",
		metric.WithDescription(
			"Number of idle database connections.",
		),
		metric.WithUnit(
			"{connection}",
		),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create idle database metric: %w",
			err,
		)
	}

	waitCount, err := meter.Int64ObservableCounter(
		"traillog.db.pool.wait_count",
		metric.WithDescription(
			"Total number of waits for a database connection.",
		),
		metric.WithUnit(
			"{wait}",
		),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create database wait-count metric: %w",
			err,
		)
	}

	waitDuration, err := meter.Float64ObservableCounter(
		"traillog.db.pool.wait_duration",
		metric.WithDescription(
			"Total time spent waiting for database connections.",
		),
		metric.WithUnit(
			"s",
		),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create database wait-duration metric: %w",
			err,
		)
	}

	registration, err := meter.RegisterCallback(
		func(
			ctx context.Context,
			observer metric.Observer,
		) error {
			stats := database.Stats()

			observer.ObserveInt64(
				maxOpen,
				int64(
					stats.MaxOpenConnections,
				),
			)

			observer.ObserveInt64(
				open,
				int64(
					stats.OpenConnections,
				),
			)

			observer.ObserveInt64(
				inUse,
				int64(
					stats.InUse,
				),
			)

			observer.ObserveInt64(
				idle,
				int64(
					stats.Idle,
				),
			)

			observer.ObserveInt64(
				waitCount,
				stats.WaitCount,
			)

			observer.ObserveFloat64(
				waitDuration,
				stats.WaitDuration.Seconds(),
			)

			return nil
		},

		maxOpen,
		open,
		inUse,
		idle,
		waitCount,
		waitDuration,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"register database metrics callback: %w",
			err,
		)
	}

	return registration, nil
}
