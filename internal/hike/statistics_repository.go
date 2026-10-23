package hike

import (
	"context"
	"database/sql"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
)

type hikeStatisticsRow struct {
	TotalHikes      int     `db:"total_hikes"`
	VisitedHikes    int     `db:"visited_hikes"`
	PlannedHikes    int     `db:"planned_hikes"`
	TotalDistanceKM float64 `db:"total_distance_km"`
}

type visitsStatisticsRow struct {
	TotalVisits   int             `db:"total_visits"`
	AverageRating sql.NullFloat64 `db:"average_rating"`
}

// DashboardStats calculates aggregate statisitcs for hikes, visits, and tags.
//
// Statistics are derived from the current database contents an are note
// persisted separately.
func (r *Repository) DashboardStats(
	ctx context.Context,
) (DashboardStats, error) {
	ctx, span := r.tracer.Start(
		ctx,
		"hike.repository.dashboard_stats",
	)
	defer span.End()

	hikes, err := r.loadHikeStatistics(
		ctx,
	)
	if err != nil {
		markSpanError(
			span,
			"load hike statistics failed",
		)

		return DashboardStats{}, err
	}

	visits, err := r.loadVisitsStatistics(
		ctx,
	)
	if err != nil {
		markSpanError(
			span,
			"load visits statistics failed",
		)

		return DashboardStats{}, err
	}

	topTags, err := r.loadTopTags(
		ctx,
	)
	if err != nil {
		markSpanError(
			span,
			"load tag statistics failed",
		)

		return DashboardStats{}, err
	}

	mostVisited, err := r.loadMostVisitedHikes(
		ctx,
	)
	if err != nil {
		markSpanError(
			span,
			"load hike visits statistics failed",
		)

		return DashboardStats{}, err
	}

	monthlVisits, err := r.loadMonthlyVisits(
		ctx,
	)
	if err != nil {
		markSpanError(
			span,
			"load monthly statistics failed",
		)

		return DashboardStats{}, err
	}

	var averagetRating *float64

	if visits.AverageRating.Valid {
		value := visits.AverageRating.Float64

		averagetRating = &value
	}

	span.SetAttributes(
		attribute.Int(
			"traillog.stats.total_hikes",
			hikes.TotalHikes,
		),

		attribute.Int(
			"traillog.stats.total_vistis",
			visits.TotalVisits,
		),
	)

	return DashboardStats{
		TotalHikes:       hikes.TotalHikes,
		VisitedHikes:     hikes.VisitedHikes,
		PlannedHikes:     hikes.PlannedHikes,
		TotalDistanceKM:  hikes.TotalDistanceKM,
		TotalVisits:      visits.TotalVisits,
		AverageRating:    averagetRating,
		TopTags:          topTags,
		MostVisitedHikes: mostVisited,
		MonthlyVisits:    monthlVisits,
	}, nil
}

func (r *Repository) loadHikeStatistics(
	ctx context.Context,
) (hikeStatisticsRow, error) {
	ctx, span := r.startDatabaseSpan(
		ctx,
		"SELECT",
		"hikes",
		"AGGREGATE hikes",
	)
	defer span.End()

	var result hikeStatisticsRow

	err := r.db.GetContext(
		ctx,
		&result,
		`
		SELECT
			COUNT(*) AS total_hikes,

			COALESCE(
				SUM(
					CASE
						WHEN status = 'visited'
						THEN 1
						ELSE 0
					END
				),
				0
			) AS visited_hikes,

			COALESCE(
				SUM(
					CASE
					  WHEN status = 'planned'
						THEN 1
						ELSE 0
					END
				),
				0
			) AS planned_hikes,

			COALESCE(
				SUM(distance_km),
				0.0
			) AS total_distance_km
		FROM hikes
		`,
	)
	if err != nil {
		recordSpanError(
			span,
			err,
			"SQLite aggreagate query failed",
		)

		return hikeStatisticsRow{},
			fmt.Errorf("aggregate hike statistics: %w", err)
	}

	return result, nil
}

func (r *Repository) loadVisitsStatistics(
	ctx context.Context,
) (visitsStatisticsRow, error) {
	ctx, span := r.startDatabaseSpan(
		ctx,
		"SELECT",
		"vistis",
		"AGGREGATE visits",
	)
	defer span.End()

	var result visitsStatisticsRow

	err := r.db.GetContext(
		ctx,
		&result,
		`SELECT
			COUNT(*) AS total_visits,
			AVG(rating) AS average_rating
		FROM visits
		`,
	)
	if err != nil {
		recordSpanError(
			span,
			err,
			"SQLite aggregate query failed",
		)

		return visitsStatisticsRow{}, fmt.Errorf("aggregate visit statistics: %w", err)
	}

	return result, nil
}

func (r *Repository) loadTopTags(
	ctx context.Context,
) ([]TagUsage, error) {
	ctx, span := r.startDatabaseSpan(
		ctx,
		"SELECT",
		"tags",
		"AGGREGATE tag usage",
	)
	defer span.End()

	results := make([]TagUsage, 0)

	err := r.db.SelectContext(
		ctx,
		&results,
		`
		SELECT
			t.name AS name,
			COUNT(ht.hike_id) AS hike_count

		FROM tags AS t

		INNER JOIN hike_tags AS ht
			ON ht.tag_id = t.id

		GROUP BY
			t.id,
			t.name

		ORDER BY
			hike_count DESC,
			t.name ASC

		LIMIT 8
		`,
	)
	if err != nil {
		recordSpanError(
			span,
			err,
			"SQLite tag aggregate query failed",
		)

		return nil, fmt.Errorf("aggreagate tag usage: %w", err)
	}

	span.SetAttributes(
		attribute.Int64(
			"db.response.returned_rows",
			int64(len(results)),
		),
	)

	return results, nil
}

func (r *Repository) loadMostVisitedHikes(
	ctx context.Context,
) ([]HikeVisitCount, error) {
	ctx, span := r.startDatabaseSpan(
		ctx,
		"SELECT",
		"hikes",
		"AGGREGATE hike visits",
	)
	defer span.End()

	results := make([]HikeVisitCount, 0, 5)

	err := r.db.SelectContext(
		ctx,
		&results,
		`
		SELECT
			h.id AS hike_id,
			h.name AS name,
			COUNT(v.id) as visit_count

		FROM hikes AS h

		LEFT JOIN visits AS v
			ON v.hike_id = h.id

		GROUP BY
			h.id,
			h.name

		ORDER BY
			visit_count DESC,
			h.name ASC

		LIMIT 5
		`,
	)
	if err != nil {
		recordSpanError(
			span,
			err,
			"SQLite hike visit aggregate failed",
		)

		return nil, fmt.Errorf("aggregate hike visits: %w", err)
	}

	return results, nil
}

func (r *Repository) loadMonthlyVisits(
	ctx context.Context,
) ([]MonthlyVisitCount, error) {
	ctx, span := r.startDatabaseSpan(
		ctx,
		"SELECT",
		"visits",
		"AGGREGATE monthly visits",
	)
	defer span.End()

	results := make([]MonthlyVisitCount, 0, 12)

	err := r.db.SelectContext(
		ctx,
		&results,
		`
		SELECT
			month,
			visit_count
		FROM(
			SELECT
				strftime(
					'%Y-%m',
					visited_at
				) AS month,

				COUNT(*) AS visit_count

				FROM visits

				GROUP BY month

				ORDER BY month DESC

				LIMIT 12
		)

		ORDER BY month ASC
		`,
	)
	if err != nil {
		recordSpanError(
			span,
			err,
			"SQLite monthly aggregate failed",
		)

		return nil, fmt.Errorf("aggregate monthly visits: %w", err)
	}

	return results, nil
}
