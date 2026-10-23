package hike

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const selectHikeColumns = `
	SELECT
		id,
		name,
		location,
		description,
		distance_km,
		status,
		created_at,
		updated_at
	FROM hikes
`

// ErrNotFound indicates that a requested hike does not exist.
var ErrNotFound = errors.New("hike not found")

// Repository persists hikes and visits in a SQL database.
type Repository struct {
	db     *sqlx.DB
	now    func() time.Time
	tracer trace.Tracer
}

// NewRepository creates a Repository backed by db.
func NewRepository(db *sqlx.DB, tracer trace.Tracer) *Repository {
	return newRepository(db, time.Now, tracer)
}

// Create stores a hike and returns the persisted record.
func (r *Repository) Create(
	ctx context.Context,
	params CreateParams,
) (Hike, error) {
	ctx, span := r.tracer.Start(
		ctx,
		"hike.repository.create",
	)
	defer span.End()

	now := r.now().UTC()

	values := struct {
		CreateParams

		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}{
		CreateParams: params,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	dbContext, dbSpan := r.startDatabaseSpan(
		ctx,
		"INSERT",
		"hikes",
		"INSERT hikes",
	)

	result, err := r.db.NamedExecContext(
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
			"database INSERT failed",
		)

		return Hike{}, fmt.Errorf(
			"insert hike: %w",
			err,
		)
	}

	dbSpan.End()

	id, err := result.LastInsertId()
	if err != nil {
		recordSpanError(
			span,
			err,
			"read inserted ID failed",
		)

		return Hike{}, fmt.Errorf(
			"get inserted hike ID: %w",
			err,
		)
	}

	created, err := r.FindByID(
		ctx,
		id,
	)
	if err != nil {
		markSpanError(
			span,
			"load created hike failed",
		)

		return Hike{}, fmt.Errorf(
			"load created hike: %w",
			err,
		)
	}

	return created, nil
}

// FindByID returns the hike with id, or ErrNotFound if it does not exist.
func (r *Repository) FindByID(ctx context.Context, id int64) (Hike, error) {
	ctx, span := r.tracer.Start(
		ctx,
		"hike.repository.find_by_id",
	)
	defer span.End()

	var result Hike

	dbContext, dbSpan := r.startDatabaseSpan(
		ctx,
		"SELECT",
		"hikes",
		"SELECT hikes",
	)

	err := r.db.GetContext(
		dbContext,
		&result,
		selectHikeColumns+`WHERE id = ?`,
		id,
	)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		dbSpan.SetAttributes(
			attribute.Int64("db.response.returned_rows", 0),
		)
		dbSpan.End()

		span.SetAttributes(
			attribute.Bool(
				"traillog.hike.found",
				false,
			),
		)

		return Hike{}, fmt.Errorf("%w: id=%d", ErrNotFound, id)

	case err != nil:
		recordSpanError(
			dbSpan,
			err,
			"SQLite SELECT failed",
		)
		dbSpan.End()

		markSpanError(
			span,
			"database SELECT failed",
		)

		return Hike{}, fmt.Errorf("select hike by ID: %w", err)
	}

	dbSpan.SetAttributes(
		attribute.Int64(
			"db.response.returned_rows",
			1,
		),
	)
	dbSpan.End()

	span.SetAttributes(
		attribute.Bool(
			"traillog.hike.found",
			true,
		),
	)

	return result, nil
}

// List returns all hikes in descending creation order.
func (r *Repository) List(ctx context.Context) ([]Hike, error) {
	ctx, span := r.tracer.Start(
		ctx,
		"hike.repository.list",
	)
	defer span.End()

	results := make([]Hike, 0)

	dbContext, dbSpan := r.startDatabaseSpan(
		ctx,
		"SELECT",
		"hikes",
		"SELECT hikes",
	)

	err := r.db.SelectContext(
		dbContext,
		&results,
		selectHikeColumns+`
		ORDER BY
			created_at DESC,
			id DESC
		`,
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
			"database SELECT failed",
		)

		return nil, fmt.Errorf("select hikes: %v", err)
	}

	dbSpan.SetAttributes(
		attribute.Int64(
			"db.response.returned_rows",
			int64(len(results)),
		),
	)
	dbSpan.End()

	span.SetAttributes(
		attribute.Int(
			"traillog.hike.result_count",
			len(results),
		),
	)

	return results, nil
}

// Search returns a filtered and paginated collection of hikes.
func (r *Repository) Search(
	ctx context.Context,
	params ListParams,
) (ListResult, error) {
	ctx, span := r.tracer.Start(
		ctx,
		"hike.repository.search",
	)
	defer span.End()

	whereClause, arguments := buildSearchWhere(params)

	var total int

	countContext, countSpan := r.startDatabaseSpan(
		ctx,
		"SELECT",
		"hikes",
		"COUNT hikes",
	)

	countQuery := `
		SELECT COUNT(*)
		FROM hikes
	` + whereClause

	countQuery, countArguments, err := expandInQuery(
		r.db,
		countQuery,
		arguments...,
	)
	if err != nil {
		return ListResult{}, fmt.Errorf("expand count query: %w", err)
	}

	err = r.db.GetContext(
		countContext,
		&total,
		countQuery,
		countArguments...,
	)
	if err != nil {
		recordSpanError(
			countSpan,
			err,
			"SQLite COUNT failed",
		)

		countSpan.End()

		markSpanError(
			span,
			"database COUNT failed",
		)

		return ListResult{},
			fmt.Errorf(
				"count hikes: %w",
				err,
			)
	}

	countSpan.End()

	offset :=
		(params.Page - 1) *
			params.PageSize

	results := make(
		[]Hike,
		0,
		params.PageSize,
	)

	queryContext, querySpan := r.startDatabaseSpan(
		ctx,
		"SELECT",
		"hikes",
		"SELECT hikes",
	)

	searchQuery := selectHikeColumns + whereClause + `
	ORDER BY
	` + searchOrderBy(params.Sort) + `
	LIMIT ?
	OFFSET ?
	`

	searchArguments := append([]any{}, arguments...)
	searchArguments = append(searchArguments, params.PageSize, offset)

	searchQuery, searchArguments, err = expandInQuery(r.db, searchQuery, searchArguments...)
	if err != nil {
		return ListResult{}, fmt.Errorf("expand search query: %w", err)
	}

	err = r.db.SelectContext(
		queryContext,
		&results,
		searchQuery,
		searchArguments...,
	)
	if err != nil {
		recordSpanError(
			querySpan,
			err,
			"SQLite SELECT failed",
		)

		querySpan.End()

		markSpanError(
			span,
			"database SELECT failed",
		)

		return ListResult{},
			fmt.Errorf(
				"search hikes: %w",
				err,
			)
	}

	querySpan.SetAttributes(
		attribute.Int64(
			"db.response.returned_rows",
			int64(len(results)),
		),
	)

	querySpan.End()

	if err := r.loadHikeTags(ctx, results); err != nil {
		markSpanError(
			span,
			"load search result tags failed",
		)

		return ListResult{}, fmt.Errorf(
			"load search result tags: %w",
			err,
		)
	}

	span.SetAttributes(
		attribute.Int(
			"traillog.hike.total_count",
			total,
		),
	)

	return ListResult{
		Hikes: results,
		Total: total,
	}, nil
}

// Update replaces the editable values of a hike and returns the updated record.
func (r *Repository) Update(
	ctx context.Context,
	id int64,
	params UpdateParams,
) (Hike, error) {
	ctx, span := r.tracer.Start(
		ctx,
		"hike.repository.update",
	)
	defer span.End()

	values := struct {
		UpdateParams

		ID        int64     `db:"id"`
		UpdatedAt time.Time `db:"updated_at"`
	}{
		UpdateParams: params,
		ID:           id,
		UpdatedAt:    r.now().UTC(),
	}

	dbContext, dbSpan := r.startDatabaseSpan(
		ctx,
		"UPDATE",
		"hikes",
		"UPDATE hikes",
	)

	result, err := r.db.NamedExecContext(
		dbContext,
		`
		UPDATE hikes
		SET
			name = :name,
			location = :location,
			description = :description,
			distance_km = :distance_km,
			status = :status,
			updated_at = :updated_at
		WHERE id = :id
		`,
		values,
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
			"database UPDATE failed",
		)

		return Hike{}, fmt.Errorf(
			"update hike: %w",
			err,
		)
	}

	dbSpan.End()

	affected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(
			span,
			err,
			"read affected row count failed",
		)

		return Hike{}, fmt.Errorf(
			"get updated row count: %w",
			err,
		)
	}

	if affected == 0 {
		span.SetAttributes(
			attribute.Bool(
				"traillog.hike.found",
				false,
			),
		)

		return Hike{}, fmt.Errorf(
			"%w: id=%d",
			ErrNotFound,
			id,
		)
	}

	span.SetAttributes(
		attribute.Bool(
			"traillog.hike.found",
			true,
		),
	)

	updated, err := r.FindByID(
		ctx,
		id,
	)
	if err != nil {
		markSpanError(
			span,
			"load updated hike failed",
		)

		return Hike{}, fmt.Errorf(
			"load updated hike: %w",
			err,
		)
	}

	return updated, nil
}

// Delete removes the hike with id, or returns ErrNotFound if it does not exist.
func (r *Repository) Delete(
	ctx context.Context,
	id int64,
) error {
	ctx, span := r.tracer.Start(
		ctx,
		"hike.repository.delete",
	)
	defer span.End()

	dbContext, dbSpan := r.startDatabaseSpan(
		ctx,
		"DELETE",
		"hikes",
		"DELETE hikes",
	)

	result, err := r.db.ExecContext(
		dbContext,
		`
		DELETE FROM hikes
		WHERE id = ?
		`,
		id,
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
			"database DELETE failed",
		)

		return fmt.Errorf(
			"delete hike: %w",
			err,
		)
	}

	dbSpan.End()

	affected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(
			span,
			err,
			"read deleted row count failed",
		)

		return fmt.Errorf(
			"get deleted row count: %w",
			err,
		)
	}

	if affected == 0 {
		span.SetAttributes(
			attribute.Bool(
				"traillog.hike.found",
				false,
			),
		)

		return fmt.Errorf(
			"%w: id=%d",
			ErrNotFound,
			id,
		)
	}

	span.SetAttributes(
		attribute.Bool(
			"traillog.hike.found",
			true,
		),
	)

	return nil
}

func (r *Repository) startDatabaseSpan(
	ctx context.Context,
	operation string,
	collection string,
	summary string,
) (
	context.Context,
	trace.Span,
) {
	return r.tracer.Start(
		ctx,
		summary,
		trace.WithSpanKind(
			trace.SpanKindClient,
		),
		trace.WithAttributes(
			attribute.String(
				"db.system.name",
				"sqlite",
			),
			attribute.String(
				"db.collection.name",
				"hikes",
			),
			attribute.String(
				"db.operation.name",
				operation,
			),
			attribute.String(
				"db.query.summary",
				summary,
			),
			attribute.String(
				"db.collectoin.name",
				collection,
			),
		),
	)
}

// --- Helpers ---

func newRepository(db *sqlx.DB, now func() time.Time, tracer trace.Tracer) *Repository {
	return &Repository{
		db:     db,
		now:    now,
		tracer: tracer,
	}
}

func escapeLikePattern(
	value string,
) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	)

	return "%" +
		replacer.Replace(value) +
		"%"
}

func buildSearchWhere(params ListParams) (string, []any) {
	clauses := make([]string, 0, 2)

	arguments := make([]any, 0, 4)

	if params.Query != "" {
		pattern := escapeLikePattern(params.Query)

		clauses = append(
			clauses,
			`(
				name LIKE ? ESCAPE '\'
				OR location LIKE ? ESCAPE '\'
				OR description LIKE ? ESCAPE '\'
			)`,
		)

		arguments = append(
			arguments,
			pattern,
			pattern,
			pattern,
		)
	}

	if params.Status != nil {
		clauses = append(clauses, "status = ?")
		arguments = append(arguments, params.Status.String())
	}

	if len(params.Tags) > 0 {
		clauses = append(
			clauses,
			`
			hikes.id IN (
				SELECT ht.hike_id
				FROM hike_tags AS ht

				INNER JOIN tags AS t
					ON t.id = ht.tag_id

				WHERE t.name IN (?)

				GROUP BY ht.hike_id

				HAVING COUNT (DISTINCT t.id) = ?
			)
			`,
		)

		arguments = append(arguments, params.Tags, len(params.Tags))
	}

	if len(clauses) == 0 {
		return "", arguments
	}

	return "\nWHERE " +
			strings.Join(
				clauses,
				"\nAND ",
			),
		arguments
}

func searchOrderBy(sort Sort) string {
	switch sort {
	case SortName:
		return `
			name COLLATE NOCASE ASC,
			id ASC
		`

	case SortDistance:
		return `
			distance_km IS NULL ASC,
			distance_km ASC,
			id ASC
		`

	case SortNewest:
		fallthrough

	default:
		return `
			created_at DESC,
			id DESC
		`
	}
}
