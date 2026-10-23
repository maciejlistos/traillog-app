package hike

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

type storedTrackRow struct {
	ID     int64 `db:"id"`
	HikeID int64 `db:"hike_id"`

	SourceFilename string `db:"source_filename"`

	PointCount     int     `db:"point_count"`
	DistanceKM     float64 `db:"distance_km"`
	ElevationGainM float64 `db:"elevation_gain_m"`

	DurationSeconds sql.NullInt64 `db:"duration_seconds"`

	StartLat float64 `db:"start_lat"`
	StartLon float64 `db:"start_lon"`

	EndLat float64 `db:"end_lat"`
	EndLon float64 `db:"end_lon"`

	CreatedAt time.Time `db:"created_at"`
}

// FindTrackByHikeID returns GPX Track metadata belonging to hikeID.
func (r *Repository) FindTrackByHikeID(
	ctx context.Context,
	hikeID int64,
) (Track, error) {
	ctx, span := r.tracer.Start(
		ctx,
		"hike.repository.find_track",
	)
	defer span.End()

	dbContext, dbSpan := r.startDatabaseSpan(
		ctx,
		"SELECT",
		"tracks",
		"SELECT track",
	)

	var row storedTrackRow

	err := r.db.GetContext(
		dbContext,
		&row,
		`
		SELECT
			id,
			hike_id,
			source_filename,
			point_count,
			distance_km,
			elevation_gain_m,
			duration_seconds,
			start_lat,
			start_lon,
			end_lat,
			end_lon,
			created_at
		FROM tracks
		WHERE hike_id = ?
		`,
		hikeID,
	)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		dbSpan.SetAttributes(
			attribute.Int64(
				"db.response.returned_rows",
				0,
			),
		)

		dbSpan.End()

		return Track{},
			ErrTrackNotFound

	case err != nil:
		recordSpanError(
			dbSpan,
			err,
			"SQLite SELECT failed",
		)

		dbSpan.End()

		markSpanError(
			span,
			"load track failed",
		)

		return Track{},
			fmt.Errorf(
				"select track: %w",
				err,
			)
	}

	dbSpan.SetAttributes(
		attribute.Int64(
			"db.response.returned_rows",
			1,
		),
	)

	dbSpan.End()

	var duration *time.Duration

	if row.DurationSeconds.Valid {
		value :=
			time.Duration(
				row.DurationSeconds.Int64,
			) * time.Second

		duration = &value
	}

	return Track{
		ID: row.ID,

		HikeID: row.HikeID,

		SourceFilename: row.SourceFilename,

		PointCount: row.PointCount,

		DistanceKM: row.DistanceKM,

		ElevationGainM: row.ElevationGainM,

		Duration: duration,

		StartLat: row.StartLat,

		StartLon: row.StartLon,

		EndLat: row.EndLat,

		EndLon: row.EndLon,

		CreatedAt: row.CreatedAt,
	}, nil
}

// ListTrackPoints returns every point belonging to trackID in route order.
func (r *Repository) ListTrackPoints(
	ctx context.Context,
	trackID int64,
) ([]TrackPoint, error) {
	ctx, span := r.tracer.Start(
		ctx,
		"hike.repository.list_track_points",
	)
	defer span.End()

	points := make([]TrackPoint, 0)

	dbContext, dbSpan := r.startDatabaseSpan(
		ctx,
		"SELECT",
		"track_points",
		"SELECT track_points",
	)

	err := r.db.SelectContext(
		dbContext,
		&points,
		`
		SELECT
			segment_index,
			latitude,
			longitude,
			elevation_m,
			recorded_at
		FROM track_points
		WHERE track_id = ?
		ORDER BY sequence ASC
		`,
		trackID,
	)
	if err != nil {
		recordSpanError(
			dbSpan,
			err,
			"SQLite SELECT failed",
		)
		dbSpan.End()

		markSpanError(
			span,
			"load track point failed",
		)

		return nil, fmt.Errorf("select track points: %w", err)
	}

	dbSpan.SetAttributes(
		attribute.Int64(
			"db.response.returned_rows",
			int64(len(points)),
		),
	)

	dbSpan.End()

	return points, nil
}
