package observability

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type staticDatabaseStats struct {
	stats sql.DBStats
}

func (s staticDatabaseStats) Stats() sql.DBStats {
	return s.stats
}

func TestDatabaseMetricsAreExported(
	t *testing.T,
) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			reader,
		),
	)

	t.Cleanup(
		func() {
			if err := provider.Shutdown(
				context.Background(),
			); err != nil {
				t.Errorf(
					"shutdown meter provider: %v",
					err,
				)
			}
		},
	)

	database := staticDatabaseStats{
		stats: sql.DBStats{
			MaxOpenConnections: 5,
			OpenConnections:    3,
			InUse:              2,
			Idle:               1,
			WaitCount:          7,
			WaitDuration:       150 * time.Millisecond,
		},
	}

	registration, err := RegisterDatabaseMetrics(
		provider.Meter(
			"traillog-test-database",
		),
		database,
	)
	if err != nil {
		t.Fatalf(
			"register database metrics: %v",
			err,
		)
	}

	t.Cleanup(
		func() {
			if err := registration.Unregister(); err != nil {
				t.Errorf(
					"unregister metrics: %v",
					err,
				)
			}
		},
	)

	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(
		context.Background(),
		&metrics,
	); err != nil {
		t.Fatalf(
			"collect database metrics: %v",
			err,
		)
	}

	got := make(
		map[string]float64,
	)

	for _, scope := range metrics.ScopeMetrics {
		for _, metric := range scope.Metrics {
			switch data := metric.Data.(type) {
			case metricdata.Gauge[int64]:
				got[metric.Name] = float64(
					data.DataPoints[0].Value,
				)
			case metricdata.Sum[int64]:
				got[metric.Name] = float64(
					data.DataPoints[0].Value,
				)
			case metricdata.Sum[float64]:
				got[metric.Name] = data.DataPoints[0].Value
			}
		}
	}

	want := map[string]float64{
		"traillog.db.pool.max_open_connections": 5,
		"traillog.db.pool.open_connections":     3,
		"traillog.db.pool.in_use":               2,
		"traillog.db.pool.idle":                 1,
		"traillog.db.pool.wait_count":           7,
		"traillog.db.pool.wait_duration":        0.15,
	}

	for name, wantValue := range want {
		gotValue, ok := got[name]
		if !ok {
			t.Errorf(
				"collected metrics do not contain %q",
				name,
			)

			continue
		}

		if gotValue != wantValue {
			t.Errorf(
				"metric %q = %v, want %v",
				name,
				gotValue,
				wantValue,
			)
		}
	}
}
