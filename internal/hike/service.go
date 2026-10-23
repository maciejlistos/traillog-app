package hike

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type hikeRepository interface {
	Create(
		context.Context,
		CreateParams,
	) (Hike, error)

	FindByID(
		context.Context,
		int64,
	) (Hike, error)

	List(
		context.Context,
	) ([]Hike, error)

	Search(
		context.Context,
		ListParams,
	) (ListResult, error)

	Update(
		context.Context,
		int64,
		UpdateParams,
	) (Hike, error)

	Delete(
		context.Context,
		int64,
	) error

	WithinTransaction(
		context.Context,
		func(hikeTransaction) error,
	) error

	ListVisits(
		context.Context,
		int64,
	) ([]Visit, error)

	ReplaceHikeTags(
		context.Context,
		int64,
		[]string,
	) ([]Tag, error)

	ListTags(
		context.Context,
	) ([]Tag, error)

	ListHikeTags(
		context.Context,
		int64,
	) ([]Tag, error)

	DashboardStats(
		context.Context,
	) (DashboardStats, error)

	FindTrackByHikeID(
		context.Context,
		int64,
	) (Track, error)

	ListTrackPoints(
		context.Context,
		int64,
	) ([]TrackPoint, error)
}

type hikeTransaction interface {
	CreateHike(
		context.Context,
		CreateParams,
	) (Hike, error)

	CreateTrack(
		context.Context,
		int64,
		ImportTrackParams,
	) (Track, error)

	MarkHikeVisited(
		context.Context,
		int64,
	) error

	CreateVisit(
		context.Context,
		int64,
		CreateVisitParams,
	) (Visit, error)

	UpdateHikeDistance(
		context.Context,
		int64,
		*float64,
	) error

	DeleteTrackByHikeID(
		context.Context,
		int64,
	) (bool, error)
}

// Does *Repository implements hikeRepository
var _ hikeRepository = (*Repository)(nil)

// Service implements TrailLog's hike and visit business operations.
type Service struct {
	repository hikeRepository
	tracer     trace.Tracer
}

// NewService creates a Service using repository for persistence.
func NewService(repository hikeRepository, tracer trace.Tracer) *Service {
	return &Service{
		repository: repository,
		tracer:     tracer,
	}
}

// Create validates and creates a hike.
func (s *Service) Create(
	ctx context.Context,
	params CreateParams,
) (Hike, error) {
	params = normalizeCreateParams(
		params,
	)

	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.create",
		trace.WithAttributes(
			attribute.String(
				"traillog.hike.status",
				params.Status.String(),
			),
			attribute.Bool(
				"traillog.hike.distance_provided",
				params.DistanceKM != nil,
			),
		),
	)
	defer span.End()

	if err := validateHikeParams(
		params.Name,
		params.Location,
		params.DistanceKM,
		params.Status,
	); err != nil {

		if validationError, ok := errors.AsType[*ValidationError](err); ok {
			span.AddEvent(
				"validation failed",
				trace.WithAttributes(
					attribute.Int(
						"traillog.validation.error_count",
						len(validationError.Fields),
					),
				),
			)
		}

		return Hike{}, err
	}

	created, err := s.repository.Create(
		ctx,
		params,
	)
	if err != nil {
		markSpanError(
			span,
			"repository create failed",
		)

		return Hike{}, fmt.Errorf(
			"create hike: %w",
			err,
		)
	}

	return created, nil
}

// FindByID returns the hike with id.
func (s *Service) FindByID(ctx context.Context, id int64) (Hike, error) {
	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.find_by_id",
	)
	defer span.End()

	result, err := s.repository.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			span.SetAttributes(
				attribute.Bool("traillog.hike.found", false),
			)
			return Hike{}, fmt.Errorf("find hike: %w", err)
		}

		markSpanError(span, "repository lookup failed")

		return Hike{}, fmt.Errorf("find hike: %w", err)
	}

	span.SetAttributes(attribute.Bool("traillog.hike.found", true))

	return result, nil
}

// List returns all hikes.
func (s *Service) List(ctx context.Context) ([]Hike, error) {
	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.list",
	)
	defer span.End()

	results, err := s.repository.List(ctx)
	if err != nil {
		markSpanError(
			span,
			"repository list failed",
		)

		return nil, fmt.Errorf("list hike: %w", err)
	}

	span.SetAttributes(
		attribute.Int("traillog.hike.result_count", len(results)),
	)

	return results, nil
}

// Search validates params and returns matching hikes.
func (s *Service) Search(
	ctx context.Context,
	params ListParams,
) (ListResult, error) {
	params = normalizeListParams(
		params,
	)

	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.search",
		trace.WithAttributes(
			attribute.Bool(
				"traillog.hike.search_query_present",
				params.Query != "",
			),

			attribute.String(
				"traillog.hike.sort",
				params.Sort.String(),
			),

			attribute.Int(
				"traillog.pagination.page",
				params.Page,
			),

			attribute.Int(
				"traillog.pagination.page_size",
				params.PageSize,
			),
		),
	)
	defer span.End()

	if params.Status != nil {
		span.SetAttributes(
			attribute.String(
				"traillog.hike.status_filter",
				params.Status.String(),
			),
		)
	}

	if err := validateListParams(
		params,
	); err != nil {
		return ListResult{}, err
	}

	result, err := s.repository.Search(
		ctx,
		params,
	)
	if err != nil {
		markSpanError(
			span,
			"repository search failed",
		)

		return ListResult{}, fmt.Errorf(
			"search hikes: %w",
			err,
		)
	}

	result.Page = params.Page
	result.PageSize = params.PageSize

	if result.Total > 0 {
		result.TotalPages =
			(result.Total + params.PageSize - 1) / params.PageSize
	}

	span.SetAttributes(
		attribute.Int(
			"traillog.hike.result_count",
			len(result.Hikes),
		),

		attribute.Int(
			"traillog.hike.total_count",
			result.Total,
		),
	)

	return result, nil
}

// Update validates and updates the hike with id.
func (s *Service) Update(ctx context.Context, id int64, params UpdateParams) (Hike, error) {
	params = normalizeUpdateParams(params)

	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.update",
		trace.WithAttributes(
			attribute.String("traillog.hike.status", params.Status.String()),
			attribute.Bool("traillog.hike.distance_provided", params.DistanceKM != nil),
		),
	)
	defer span.End()

	if err := validateHikeParams(
		params.Name,
		params.Location,
		params.DistanceKM,
		params.Status,
	); err != nil {

		if validationError, ok := errors.AsType[*ValidationError](err); ok {
			span.AddEvent(
				"validation failed",
				trace.WithAttributes(
					attribute.Int(
						"traillog.validation.error_count",
						len(validationError.Fields),
					),
				),
			)
		}

		return Hike{}, err
	}

	updated, err := s.repository.Update(ctx, id, params)
	if err != nil {
		if errors.Is(
			err,
			ErrNotFound,
		) {
			span.SetAttributes(
				attribute.Bool(
					"traillog.hike.found",
					false,
				),
			)

			return Hike{}, fmt.Errorf("update hike: %w", err)
		}

		markSpanError(
			span,
			"repository update failed",
		)

		return Hike{}, fmt.Errorf("update hike: %w", err)
	}

	span.SetAttributes(
		attribute.Bool(
			"traillog.hike.found",
			true,
		),
	)

	return updated, nil
}

// Delete removes the hike with id.
func (s *Service) Delete(ctx context.Context, id int64) error {
	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.delete",
	)
	defer span.End()

	err := s.repository.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			span.SetAttributes(
				attribute.Bool("traillog.hike.found", false),
			)

			return fmt.Errorf("delete hike: %w", err)
		}

		markSpanError(
			span,
			"repository delete failed",
		)

		return fmt.Errorf("delete hike: %w", err)
	}

	span.SetAttributes(
		attribute.Bool(
			"traillog.hike.found",
			true,
		),
	)

	return nil
}

// AddVisit records a visit and marks the hike as visited atomically.
func (s *Service) AddVisit(
	ctx context.Context,
	hikeID int64,
	params CreateVisitParams,
) (Visit, error) {
	params = normalizeCreateVisitParams(params)

	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.add_visit",
		trace.WithAttributes(
			attribute.Bool(
				"traillog.visit.rating_provided",
				params.Rating != nil,
			),
		),
	)
	defer span.End()

	if err := validateCreateVisitParams(params); err != nil {

		if validationError, ok := errors.AsType[*ValidationError](err); ok {
			span.AddEvent(
				"validation failed",
				trace.WithAttributes(
					attribute.Int(
						"traillog.validation.error_count",
						len(validationError.Fields),
					),
				),
			)
		}

		return Visit{}, err
	}

	var created Visit

	err := s.repository.WithinTransaction(
		ctx,
		func(tx hikeTransaction) error {
			if err := tx.MarkHikeVisited(
				ctx,
				hikeID,
			); err != nil {
				return fmt.Errorf("mark hike visited: %w", err)
			}
			visit, err := tx.CreateVisit(
				ctx,
				hikeID,
				params,
			)
			if err != nil {
				return fmt.Errorf("create visit: %w", err)
			}
			created = visit

			return nil
		},
	)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			markSpanError(
				span,
				"add visit transaction failed",
			)
		}
		return Visit{}, fmt.Errorf("add visit: %w", err)
	}

	return created, nil
}

// ListVisits returns the recorded visits for a hike.
func (s *Service) ListVisits(
	ctx context.Context,
	hikeID int64,
) ([]Visit, error) {
	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.list_visits",
	)
	defer span.End()

	results, err := s.repository.ListVisits(
		ctx,
		hikeID,
	)
	if err != nil {
		markSpanError(
			span,
			"repository list visits failed",
		)

		return nil, fmt.Errorf("list visits: %w", err)
	}

	span.SetAttributes(
		attribute.Int(
			"traillog.visit.result_count",
			len(results),
		),
	)

	return results, nil
}

// SetTags replaces all tags assigned to a hike.
//
// Tag names are trimmed, converted to lowercase, deduplicated, and sorted
// before persistance. assing an empty slice removes all tag assignments.
func (s *Service) SetTags(
	ctx context.Context,
	hikeID int64,
	names []string,
) ([]Tag, error) {
	names = normalizeTagNames(names)

	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.set_tags",
		trace.WithAttributes(
			attribute.Int(
				"traillog.tag.count",
				len(names),
			),
		),
	)
	defer span.End()

	if err := validateTagNames(names); err != nil {
		return nil, err
	}

	tags, err := s.repository.ReplaceHikeTags(ctx, hikeID, names)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			markSpanError(
				span,
				"replace hike tags failed",
			)
		}

		return nil, fmt.Errorf("set hike tags: %w", err)
	}

	return tags, nil
}

// ListTags returns all known tags ordered by name.
func (s *Service) ListTags(ctx context.Context) ([]Tag, error) {
	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.list_tags",
	)
	defer span.End()

	tags, err := s.repository.ListTags(ctx)
	if err != nil {
		markSpanError(
			span,
			"list tags failed",
		)

		return nil, fmt.Errorf("list tags: %w", err)
	}

	return tags, nil
}

// ListHikeTags returns the tags assigned to a specifc hike.
func (s *Service) ListHikeTags(
	ctx context.Context,
	hikeID int64,
) ([]Tag, error) {
	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.list_hike_tags",
	)
	defer span.End()

	tags, err := s.repository.ListHikeTags(
		ctx,
		hikeID,
	)
	if err != nil {
		markSpanError(
			span,
			"list hike tags failed",
		)

		return nil, fmt.Errorf("list hike tags: %w", err)
	}

	return tags, nil
}

// DashboardStats returns aggregate statistics for the TrailLog dashboard.
func (s *Service) DashboardStats(
	ctx context.Context,
) (DashboardStats, error) {
	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.dashboard_stats",
	)
	defer span.End()

	stats, err := s.repository.DashboardStats(ctx)
	if err != nil {
		markSpanError(
			span,
			"load dashboard statisitcs failed",
		)

		return DashboardStats{}, fmt.Errorf("load dashboard statistics: %w", err)
	}

	span.SetAttributes(
		attribute.Int(
			"traillog.stats.total_hike",
			stats.TotalHikes,
		),

		attribute.Int(
			"traillog.stats.total_visits",
			stats.TotalVisits,
		),
	)

	return stats, nil
}

// ImportTrack atomically creates a Hike and its GPX-derived Track.
//
// The Hike and all TrackPoint records are committed together. If any part of
// the import fails, the transaction is rolled back.
func (s *Service) ImportTrack(
	ctx context.Context,
	params ImportTrackParams,
) (Hike, Track, error) {
	params.Hike = normalizeCreateParams(params.Hike)

	params.SourceFilename = strings.TrimSpace(params.SourceFilename)

	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.import_track",

		trace.WithAttributes(
			attribute.Int(
				"traillog.track.point_count",
				len(params.Points),
			),

			attribute.Bool(
				"traillog.track.duration_available",
				params.Duration != nil,
			),
		),
	)
	defer span.End()

	if err := validateHikeParams(
		params.Hike.Name,
		params.Hike.Location,
		params.Hike.DistanceKM,
		params.Hike.Status,
	); err != nil {
		return Hike{}, Track{}, err
	}

	if err := validateImportTrackParams(params); err != nil {
		return Hike{}, Track{}, err
	}

	var createdHike Hike
	var createdTrack Track

	err := s.repository.WithinTransaction(
		ctx,
		func(
			tx hikeTransaction,
		) error {
			hike, err := tx.CreateHike(
				ctx,
				params.Hike,
			)
			if err != nil {
				return fmt.Errorf("create imported hike: %w", err)
			}

			createdHike = hike

			track, err := tx.CreateTrack(
				ctx,
				hike.ID,
				params,
			)
			if err != nil {
				return fmt.Errorf("create imported track %w", err)
			}

			createdTrack = track

			return nil
		},
	)
	if err != nil {
		markSpanError(
			span,
			"GPX import transaction failed",
		)

		return Hike{}, Track{}, fmt.Errorf("import GPX track: %w", err)
	}

	return createdHike, createdTrack, nil
}

// TrackVisualization returns map and elevation data for hikeID.
//
// A nil result with a nil error means that the Hike has no imported GPX
// Track. The returned map geometry is simplified for browser rendering while
// elevation calculations use the full stored point sequence.
func (s *Service) TrackVisualization(
	ctx context.Context,
	hikeID int64,
) (*TrackVisualization, error) {
	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.track_visualization",
	)
	defer span.End()

	track, err := s.repository.FindTrackByHikeID(
		ctx,
		hikeID,
	)

	if errors.Is(err, ErrTrackNotFound) {
		span.SetAttributes(
			attribute.Bool(
				"traillog.track.present",
				false,
			),
		)

		return nil, nil
	}

	if err != nil {
		markSpanError(
			span,
			"load track failed",
		)

		return nil, fmt.Errorf("load track: %w", err)
	}

	points, err := s.repository.ListTrackPoints(ctx, track.ID)
	if err != nil {
		markSpanError(
			span,
			"load track points failed",
		)

		return nil, fmt.Errorf("load track points: %w", err)
	}

	if len(points) < 2 {
		markSpanError(
			span,
			"stored track contains insufficient points",
		)

		return nil, fmt.Errorf("track contains %d points, want at least 2", len(points))
	}

	visualization := buildTrackVisualization(track, points)

	mapPointCount := 0

	for _, segment := range visualization.Map.Segments {
		mapPointCount += len(segment.Points)
	}

	span.SetAttributes(
		attribute.Bool(
			"traillog.track.present",
			true,
		),

		attribute.Int(
			"traillog.track.stored_points",
			len(points),
		),

		attribute.Int(
			"traillog.track.map_points",
			mapPointCount,
		),
	)

	return &visualization, nil
}

// ExportTrack returns the nomalized route data required to generate gpx.
//
// ExportTrack preserves all persisted TrackPoint and their segment indexes;
// it does not use the simplified geometry produced for browser maps.
func (s *Service) ExportTrack(
	ctx context.Context,
	hikeID int64,
) (TrackExport, error) {
	ctx, span := s.tracer.Start(ctx, "hike.service.export_track")
	defer span.End()

	found, err := s.repository.FindByID(ctx, hikeID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return TrackExport{}, err
		}

		markSpanError(
			span,
			"load  hike failed",
		)
		return TrackExport{}, fmt.Errorf("load hike for GPX export: %w", err)
	}

	track, err := s.repository.FindTrackByHikeID(
		ctx,
		hikeID,
	)
	if err != nil {
		if errors.Is(
			err,
			ErrTrackNotFound,
		) {
			span.SetAttributes(
				attribute.Bool(
					"traillog.track.present",
					false,
				),
			)

			return TrackExport{}, err
		}

		markSpanError(
			span,
			"load track failed",
		)

		return TrackExport{}, fmt.Errorf("load track from GPX export: %w", err)
	}

	points, err := s.repository.ListTrackPoints(
		ctx,
		track.ID,
	)
	if err != nil {
		markSpanError(
			span,
			"load track points fetch",
		)

		return TrackExport{}, fmt.Errorf("load GPX export points: %w", err)
	}

	if len(points) < 2 {
		markSpanError(
			span,
			"track contains insufficient points",
		)
		return TrackExport{}, fmt.Errorf("track contains %d points, want at least 2", len(points))

	}

	span.SetAttributes(
		attribute.Bool(
			"traillog.track.present",
			true,
		),

		attribute.Int(
			"traillog.track.point_count",
			len(points),
		),
	)

	return TrackExport{
		Hike:   found,
		Track:  track,
		Points: points,
	}, nil
}

// SetTrack atomically attaches or replaces the GPX-derived Track belonging to
// hikeID.
//
// When replacing an existing Track, the previous Track and TrackPoints remain
// intact if creation of the replacement fails. The Hike distance is updated
// from the new Track only if the complete transaction commits.
func (s *Service) SetTrack(
	ctx context.Context,
	hikeID int64,
	params SetTrackParams,
) (Track, error) {
	params.SourceFilename = strings.TrimSpace(params.SourceFilename)

	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.set_track",
		trace.WithAttributes(
			attribute.Int(
				"traillog.track.point_count",
				len(params.Points),
			),
			attribute.Bool(
				"traillog.track.duration_available",
				params.Duration != nil,
			),
		),
	)
	defer span.End()

	if err := validateSetTrackParams(
		params,
	); err != nil {
		return Track{}, err
	}

	var created Track

	err := s.repository.WithinTransaction(
		ctx, func(tx hikeTransaction) error {
			distance := params.DistanceKM

			if err := tx.UpdateHikeDistance(
				ctx,
				hikeID,
				&distance,
			); err != nil {
				return fmt.Errorf(
					"update hike distance: %w",
					err,
				)
			}

			_, err := tx.DeleteTrackByHikeID(
				ctx,
				hikeID,
			)
			if err != nil {
				return fmt.Errorf(
					"delete previous track: %w",
					err,
				)
			}

			track, err := tx.CreateTrack(
				ctx,
				hikeID,
				importTrackParamsFromSet(params),
			)
			if err != nil {
				return fmt.Errorf("create replacement track: %w", err)
			}

			created = track

			return nil
		},
	)
	if err != nil {
		if !errors.Is(
			err,
			ErrNotFound,
		) {
			markSpanError(
				span,
				"set track transaction failed",
			)
		}

		return Track{}, fmt.Errorf("set hike track: %w", err)
	}

	return created, nil
}

// RemoveTrack removes the GPX-derived Track belonging to hikeID.
//
// Removing a Track also removes its TrackPoints through the SQLite foregin-key
// cascade. Hike metadata, including DistanceKM, is preserved.
func (s *Service) RemoveTrack(
	ctx context.Context,
	hikeID int64,
) error {
	ctx, span := s.tracer.Start(
		ctx,
		"hike.service.remove_track",
	)
	defer span.End()

	err := s.repository.WithinTransaction(
		ctx, func(tx hikeTransaction) error {
			deleted, err := tx.DeleteTrackByHikeID(
				ctx,
				hikeID,
			)
			if err != nil {
				return err
			}

			if !deleted {
				return ErrTrackNotFound
			}

			return nil
		},
	)
	if err != nil {
		if !errors.Is(
			err,
			ErrTrackNotFound,
		) {
			markSpanError(
				span,
				"remove track failed",
			)
		}

		return fmt.Errorf("remove hike track: %w", err)
	}

	return nil
}
