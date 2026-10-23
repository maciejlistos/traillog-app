package hike

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const selectVisitColumns = `
	SELECT
		id,
		hike_id,
		visited_at,
		weather,
		notes,
		rating,
		created_at
	FROM visits
`

type transactionStore struct {
	tx     *sqlx.Tx
	now    func() time.Time
	tracer trace.Tracer
}

// WithinTransaction runs fn in a database transaction and commits only when fn succeeds.
func (r *Repository) WithinTransaction(
	ctx context.Context,
	fn func(hikeTransaction) error,
) error {
	ctx, span := r.tracer.Start(
		ctx,
		"hike.repository.transaction",
	)
	defer span.End()

	tx, err := r.db.BeginTxx(
		ctx,
		nil,
	)
	if err != nil {
		recordSpanError(
			span,
			err,
			"begin transaction failed",
		)

		return fmt.Errorf("begin transaction: %w", err)
	}

	// Safety net for every path, including a future
	// early return we might accidentally introduce.
	defer func() {
		_ = tx.Rollback()
	}()

	store := &transactionStore{
		tx:     tx,
		now:    r.now,
		tracer: r.tracer,
	}

	if err := fn(store); err != nil {
		rollbackErr := tx.Rollback()

		span.AddEvent("transaction rolled back")

		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			recordSpanError(
				span,
				rollbackErr,
				"transaction rollback failed",
			)

			return errors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr))
		}

		if !errors.Is(err, ErrNotFound) {
			markSpanError(
				span,
				"transaction operation failed",
			)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(
			span,
			err,
			"commit transaction failed",
		)

		return fmt.Errorf("commit transaction: %w", err)
	}

	span.AddEvent("transaction commited")

	return nil
}

// ListVisits returns the visits for a hike in reverse chronological order.
func (r *Repository) ListVisits(
	ctx context.Context,
	hikeID int64,
) ([]Visit, error) {
	ctx, span := r.tracer.Start(
		ctx,
		"hike.repository.list_visits",
	)
	defer span.End()

	results := make(
		[]Visit,
		0,
	)

	dbContext, dbSpan := r.startDatabaseSpan(
		ctx,
		"SELECT",
		"visits",
		"SELECT visits",
	)

	err := r.db.SelectContext(
		dbContext,
		&results,
		selectVisitColumns+`
		WHERE hike_id = ?
		ORDER BY
			visited_at DESC,
			id DESC
		`,
		hikeID,
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
			"list visited failed",
		)

		return nil, fmt.Errorf("select visit: %w", err)
	}

	dbSpan.SetAttributes(
		attribute.Int64(
			"db.response.returned_rows",
			int64(len(results)),
		),
	)

	dbSpan.End()

	return results, nil
}

// --- Transaction ---

func (s *transactionStore) MarkHikeVisited(
	ctx context.Context,
	hikeID int64,
) error {
	ctx, span := s.tracer.Start(
		ctx,
		"hike.repository.mark_visited",
	)
	defer span.End()

	dbContext, dbSpan := s.startDatabaseSpan(
		ctx,
		"UPDATE",
		"hikes",
		"UPDATE hikes",
	)

	result, err := s.tx.ExecContext(
		dbContext,
		`
		UPDATE hikes
		SET
			status = ?,
			updated_at = ?
		WHERE id = ?
		`,
		StatusVisited.String(),
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
			"database UPDATE failed",
		)

		return fmt.Errorf("mark hike visited: %w", err)
	}

	dbSpan.End()

	affected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(
			span,
			err,
			"read affected rows failed",
		)

		return fmt.Errorf("read updataed row count: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("%w: id=%d", ErrNotFound, hikeID)
	}

	return nil
}

func (s *transactionStore) CreateVisit(
	ctx context.Context,
	hikeID int64,
	params CreateVisitParams,
) (Visit, error) {
	ctx, span := s.tracer.Start(ctx, "hike.repository.create_visit")
	defer span.End()

	createdAt := s.now().UTC()

	values := struct {
		HikeID int64 `db:"hike_id"`

		CreateVisitParams

		CreatedAt time.Time `db:"created_at"`
	}{
		HikeID:            hikeID,
		CreateVisitParams: params,
		CreatedAt:         createdAt,
	}

	dbContext, dbSpan := s.startDatabaseSpan(
		ctx,
		"INSERT",
		"visits",
		"INSERT visits",
	)

	result, err := s.tx.NamedExecContext(
		dbContext,
		`
		INSERT INTO visits (
			hike_id,
			visited_at,
			weather,
			notes,
			rating,
			created_at
		)
		VALUES (
			:hike_id,
			:visited_at,
			:weather,
			:notes,
			:rating,
			:created_at
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

		return Visit{}, fmt.Errorf("insert visit: %w", err)
	}

	dbSpan.End()

	id, err := result.LastInsertId()
	if err != nil {
		recordSpanError(
			span,
			err,
			"read inserted visit ID failed",
		)

		return Visit{}, fmt.Errorf("get inserted visit ID: %w", err)
	}

	var created Visit

	selectContext, selectSpan := s.startDatabaseSpan(
		ctx,
		"SELECT",
		"visits",
		"SELECT visits",
	)

	err = s.tx.GetContext(
		selectContext,
		&created,
		selectVisitColumns+`
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

		markSpanError(
			span,
			"load created visit failed",
		)

		return Visit{}, fmt.Errorf("load created visit: %w", err)
	}

	selectSpan.SetAttributes(
		attribute.Int64(
			"db.response.returned_rows",
			1,
		),
	)

	selectSpan.End()

	return created, nil
}

// --- Helpers ---
func (s *transactionStore) startDatabaseSpan(
	ctx context.Context,
	operation string,
	collection string,
	summary string,
) (
	context.Context,
	trace.Span,
) {
	return s.tracer.Start(
		ctx,
		summary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(
				"db.system.name",
				"sqlite",
			),
			attribute.String(
				"db.collection",
				collection,
			),
			attribute.String(
				"db.operation.name",
				operation,
			),
			attribute.String(
				"db.query.summary",
				summary,
			),
		),
	)
}
