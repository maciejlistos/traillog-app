package hike

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (s *transactionStore) CreateHike(
	ctx context.Context,
	params CreateParams,
) (Hike, error) {
	ctx, span := s.tracer.Start(
		ctx,
		"hike.repository.create_hike",
	)
	defer span.End()

	now := s.now().UTC()

	values := struct {
		CreateParams

		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}{
		CreateParams: params,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	dbContext, dbSpan := s.startDatabaseSpan(
		ctx,
		"INSERT",
		"hikes",
		"INSERT hikes",
	)

	result, err := s.tx.NamedExecContext(
		dbContext,
		`
		INSERT INTO hikes (
				name,
				location,
				description,
				distance_km,
				status,
				created_at,
				updated_at
			)
			VALUES (
				:name,
				:location,
				:description,
				:distance_km,
				:status,
				:created_at,
				:updated_at
			)
		`,
		values,
	)
	if err != nil {
		recordSpanError(
			dbSpan,
			err,
			"SQLite INSERT failed",
		)

		dbSpan.End()

		markSpanError(
			span,
			"create hike failed",
		)

		return Hike{}, fmt.Errorf("insert hike: %w", err)
	}

	dbSpan.End()

	id, err := result.LastInsertId()
	if err != nil {
		recordSpanError(
			span,
			err,
			"read hike ID failed",
		)

		return Hike{}, fmt.Errorf("get inserted hike ID: %w", err)
	}

	var created Hike

	selectContext, selectSpan := s.startDatabaseSpan(
		ctx,
		"SELECT",
		"hikes",
		"SELECT hikes",
	)
	err = s.tx.GetContext(
		selectContext,
		&created,
		selectHikeColumns+`
	WHERE id = ?
	`,
		id,
	)
	if err != nil {
		recordSpanError(
			selectSpan,
			err,
			"SQLite SELECT failed",
		)

		selectSpan.End()

		return Hike{}, fmt.Errorf("load created hike: %w", err)
	}

	selectSpan.End()

	return created, nil
}

func (s *transactionStore) CreateTrack(
	ctx context.Context,
	hikeID int64,
	params ImportTrackParams,
) (Track, error) {
	ctx, span := s.tracer.Start(
		ctx,
		"hike.repository.create_track",
	)

	trace.WithAttributes(
		attribute.Int(
			"traillog.track.point_count",
			len(params.Points),
		),
	)
	defer span.End()

	var durationSeconds *int64

	if params.Duration != nil {
		value := int64(params.Duration.Seconds())

		durationSeconds = &value
	}

	createdAt := s.now().UTC()

	dbContext, dbSpan := s.startDatabaseSpan(
		ctx,
		"INSERT",
		"tracks",
		"INSERT tracks",
	)

	result, err := s.tx.ExecContext(
		dbContext,
		`
			INSERT INTO tracks (
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
			)
			VALUES (
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?
			)
			`,
		hikeID,
		params.SourceFilename,
		len(params.Points),
		params.DistanceKM,
		params.ElevationGainM,
		durationSeconds,
		params.StartLat,
		params.StartLon,
		params.EndLat,
		params.EndLon,
		createdAt,
	)
	if err != nil {
		recordSpanError(
			dbSpan,
			err,
			"SQLite INSERT failed",
		)
		dbSpan.End()

		markSpanError(
			span,
			"create track failed",
		)

		return Track{}, fmt.Errorf("insert track: %w", err)
	}

	dbSpan.End()

	trackID, err := result.LastInsertId()
	if err != nil {
		recordSpanError(
			span,
			err,
			"read track ID failed",
		)

		return Track{}, fmt.Errorf("get inserted track ID: %w", err)
	}

	if err := s.insertTrackPoints(ctx, trackID, params.Points); err != nil {
		markSpanError(
			span,
			"store track points failed",
		)

		return Track{}, err
	}

	return Track{
		ID: trackID,

		HikeID: hikeID,

		SourceFilename: params.SourceFilename,

		PointCount: len(params.Points),

		DistanceKM: params.DistanceKM,

		ElevationGainM: params.ElevationGainM,

		Duration: params.Duration,

		StartLat: params.StartLat,

		StartLon: params.StartLon,

		EndLat: params.EndLat,

		EndLon: params.EndLon,

		CreatedAt: createdAt,
	}, nil
}

func (s *transactionStore) insertTrackPoints(
	ctx context.Context,
	trackID int64,
	points []TrackPoint,
) error {
	ctx, span := s.startDatabaseSpan(
		ctx,
		"INSERT",
		"track_points",
		"INSERT track points",
	)
	defer span.End()

	statement, err := s.tx.PrepareContext(
		ctx,
		`
		INSERT INTO track_points (
				track_id,
				sequence,
				segment_index,
				latitude,
				longitude,
				elevation_m,
				recorded_at
			)
			VALUES (
				?,
				?,
				?,
				?,
				?,
				?,
				?
			)
		`,
	)
	if err != nil {
		recordSpanError(
			span,
			err,
			"prepare point INSERT failed",
		)

		return fmt.Errorf("prepare track point insert: %w", err)
	}

	defer statement.Close()

	for sequence, point := range points {
		_, err := statement.ExecContext(
			ctx,
			trackID,
			sequence,
			point.SegmentIndex,
			point.Latitude,
			point.Longitude,
			point.Elevation,
			point.RecordedAt,
		)
		if err != nil {
			recordSpanError(
				span,
				err,
				"insert track point failed",
			)

			return fmt.Errorf("insert track point %d: %w", sequence, err)
		}
	}

	span.SetAttributes(
		attribute.Int(
			"traillog.track.point_count",
			len(points),
		),
	)

	return nil
}

func (s *transactionStore) UpdateHikeDistance(
	ctx context.Context,
	hikeID int64,
	distanceKM *float64,
) error {
	ctx, span := s.tracer.Start(
		ctx,
		"hike.repository.update_hike_distance",
	)
	defer span.End()

	dbContext, dbSpan := s.startDatabaseSpan(
		ctx,
		"UPDATE",
		"hikes",
		"UPDATE hike distance",
	)

	result, err := s.tx.ExecContext(
		dbContext,
		`
		UPDATE hikes
		SET
			distance_km = ?,
			updated_at = ?
		WHERE id = ?
		`,
		distanceKM,
		s.now().UTC(),
		hikeID,
	)
	if err != nil {
		recordSpanError(
			dbSpan,
			err,
			"SQLite UPDATE failed",
		)
		dbSpan.End()

		markSpanError(
			span,
			"update hike distance failed",
		)

		return fmt.Errorf("update hike distance: %w", err)
	}

	dbSpan.End()

	affected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(
			span,
			err,
			"read affected rows failed",
		)

		return fmt.Errorf("read updated hike fow count: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("%w: id=%d", ErrNotFound, hikeID)
	}

	return nil
}

func (s *transactionStore) DeleteTrackByHikeID(
	ctx context.Context,
	hikeID int64,
) (bool, error) {
	ctx, span := s.tracer.Start(
		ctx,
		"hike.repository.delete_track",
	)
	defer span.End()

	dbContext, dbSpan := s.startDatabaseSpan(
		ctx,
		"DELETE",
		"tracks",
		"DELETE track",
	)

	result, err := s.tx.ExecContext(
		dbContext,
		`
		DELETE FROM tracks
		WHERE hike_id = ?
		`,
		hikeID,
	)
	if err != nil {
		recordSpanError(
			dbSpan,
			err,
			"SQLite DELETE failed",
		)

		dbSpan.End()

		markSpanError(
			span,
			"delete track failed",
		)

		return false, fmt.Errorf("delete track: %w", err)
	}

	dbSpan.End()

	affected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(
			span,
			err,
			"read deleted row count failed",
		)

		return false, fmt.Errorf("read deleted track count: %w", err)
	}

	return affected > 0, nil
}
