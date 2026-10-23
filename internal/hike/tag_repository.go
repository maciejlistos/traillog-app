package hike

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel/attribute"
)

const selectTagColumns = `
	SELECT
		id,
		name,
		created_at
	FROM tags
`

// ListTags returns every know tag ordered alphabetically.
func (r *Repository) ListTags(
	ctx context.Context,
) ([]Tag, error) {
	ctx, span := r.tracer.Start(
		ctx,
		"hike.repository.list_tags",
	)
	defer span.End()

	tags := make([]Tag, 0)

	dbContext, dbSpan := r.startDatabaseSpan(
		ctx,
		"SELECT",
		"tags",
		"SELECT tags",
	)

	err := r.db.SelectContext(
		dbContext,
		&tags,
		selectTagColumns+`
		ORDER BY name ASC
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
			"list tags failed",
		)

		return nil, fmt.Errorf("select tags: %w", err)
	}

	dbSpan.SetAttributes(
		attribute.Int64(
			"db.response.returned_rows",
			int64(len(tags)),
		),
	)

	dbSpan.End()

	return tags, nil
}

// ListHikeTags returns tags assigned to hikeID ordered alphabetically.
func (r *Repository) ListHikeTags(
	ctx context.Context,
	hikeID int64,
) ([]Tag, error) {
	ctx, span := r.tracer.Start(
		ctx,
		"hike.repository.list_hike_tags",
	)
	defer span.End()

	tags := make([]Tag, 0)

	dbContext, dbSpan := r.startDatabaseSpan(
		ctx,
		"SELETCT",
		"hike_tags",
		"SELECT hike tags",
	)

	err := r.db.SelectContext(
		dbContext,
		&tags,
		`
		SELECT
			t.id,
			t.name,
			t.created_at
		FROM tags AS t

		INNER JOIN hike_tags AS ht
			ON ht.tag_id = t.id

		WHERE ht.hike_id = ?

		ORDER BY t.name ASC
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
			"list hike tags failed",
		)

		return nil, fmt.Errorf("select hike tags: %w", err)
	}

	dbSpan.SetAttributes(
		attribute.Int64(
			"db.response.returned_rows",
			int64(len(tags)),
		),
	)

	dbSpan.End()

	return tags, nil
}

// loadHikeTags adds all tag assignments to a page of hikes in one query.
func (r *Repository) loadHikeTags(
	ctx context.Context,
	hikes []Hike,
) error {
	if len(hikes) == 0 {
		return nil
	}

	hikeByID := make(map[int64]*Hike, len(hikes))
	ids := make([]int64, 0, len(hikes))

	for i := range hikes {
		hikes[i].Tags = make([]Tag, 0)
		hikeByID[hikes[i].ID] = &hikes[i]
		ids = append(ids, hikes[i].ID)
	}

	query, arguments, err := sqlx.In(`
		SELECT
			ht.hike_id,
			t.id,
			t.name,
			t.created_at
		FROM hike_tags AS ht

		INNER JOIN tags AS t
			ON t.id = ht.tag_id

		WHERE ht.hike_id IN (?)

		ORDER BY t.name ASC
	`, ids)
	if err != nil {
		return fmt.Errorf("expand hike tag query: %w", err)
	}

	rows, err := r.db.QueryxContext(
		ctx,
		r.db.Rebind(query),
		arguments...,
	)
	if err != nil {
		return fmt.Errorf("select tags for hikes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var hikeID int64
		var tag Tag

		if err := rows.Scan(
			&hikeID,
			&tag.ID,
			&tag.Name,
			&tag.CreatedAt,
		); err != nil {
			return fmt.Errorf("scan tag for hike: %w", err)
		}

		if hike := hikeByID[hikeID]; hike != nil {
			hike.Tags = append(hike.Tags, tag)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tags for hikes: %w", err)
	}

	return nil
}

// ReplaceHikeTags atomically replaces every tag assigned to hikeID.
//
// Missing tag records are created as part of the same transaction.
// Passing an empty names slice remove all tags assignments while retaining
// the reusable Tag record themselves.
func (r *Repository) ReplaceHikeTags(
	ctx context.Context,
	hikeID int64,
	names []string,
) ([]Tag, error) {
	ctx, span := r.tracer.Start(
		ctx,
		"hike.repository.replace_tags",
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
			"begin tag transaction failed",
		)

		return nil, fmt.Errorf("begin tag transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := ensureHikeExists(
		ctx,
		tx,
		hikeID,
	); err != nil {
		return nil, err
	}

	for _, name := range names {
		_, err := tx.ExecContext(
			ctx,
			`
			INSERT INTO tags (
				name,
				created_at
			)
			VALUES (?, ?)

			ON CONFLICT(name)
			DO NOTHING
			`,
			name,
			r.now().UTC(),
		)
		if err != nil {
			recordSpanError(
				span,
				err,
				"create tag failed",
			)

			return nil, fmt.Errorf("ensure tag %q: %w", name, err)
		}
	}

	tags, err := selectTagsByNames(
		ctx,
		r.db,
		tx,
		names,
	)
	if err != nil {
		markSpanError(
			span,
			"load tags failed",
		)

		return nil, err
	}

	if _, err := tx.ExecContext(
		ctx,
		`
		DELETE FROM hike_tags
		WHERE hike_id = ?
		`,
		hikeID,
	); err != nil {
		recordSpanError(
			span,
			err,
			"delete old hike tags failed",
		)

		return nil, fmt.Errorf("delete existing hike tags: %w", err)
	}

	for _, tag := range tags {
		_, err := tx.ExecContext(
			ctx,
			`
			INSERT INTO hike_tags (
				hike_id,
				tag_id
			)
			VALUES (?, ?)
			`,
			hikeID,
			tag.ID,
		)

		if err != nil {
			recordSpanError(
				span,
				err,
				"assign tag failed",
			)

			return nil, fmt.Errorf("assign tag %q: %w", tag.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(
			span,
			err,
			"commit tag transaction failed",
		)

		return nil, fmt.Errorf("commit tag transaction: %w", err)
	}

	return tags, nil
}

// --- Helpers ---

func expandInQuery(
	db *sqlx.DB,
	query string,
	args ...any,
) (
	string,
	[]any,
	error,
) {
	expandedQuery, expandedArgs, err := sqlx.In(query, args...)
	if err != nil {
		return "", nil, fmt.Errorf("expand SQL IN arguments: %w", err)
	}

	return db.Rebind(expandedQuery), expandedArgs, nil
}

func ensureHikeExists(
	ctx context.Context,
	tx *sqlx.Tx,
	hikeID int64,
) error {
	var exists int

	err := tx.GetContext(
		ctx,
		&exists,
		`
		SELECT 1
		FROM hikes
		WHERE id = ?
		`,
		hikeID,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: id=%d", ErrNotFound, hikeID)
	}

	if err != nil {
		return fmt.Errorf("check hike exists: %w", err)
	}

	return nil
}

func selectTagsByNames(
	ctx context.Context,
	db *sqlx.DB,
	tx *sqlx.Tx,
	names []string,
) ([]Tag, error) {
	if len(names) == 0 {
		return []Tag{}, nil
	}

	query, args, err := expandInQuery(
		db,
		`
		SELECT
			id,
			name,
			created_at
		FROM tags
		WHERE name IN (?)
		ORDER BY name ASC
		`,
		names,
	)

	if err != nil {
		return nil, err
	}

	tags := make([]Tag, 0, len(names))

	if err := tx.SelectContext(
		ctx,
		&tags,
		query,
		args...,
	); err != nil {
		return nil, fmt.Errorf("select tags by name: %w", err)
	}

	return tags, nil
}
