package hike

import "time"

// Visit records one completed visit to a hike.
type Visit struct {
	ID        int64     `db:"id"`
	HikeID    int64     `db:"hike_id"`
	VisitedAt time.Time `db:"visited_at"`
	Weather   string    `db:"weather"`
	Notes     string    `db:"notes"`
	Rating    *int      `db:"rating"`
	CreatedAt time.Time `db:"created_at"`
}

// CreateVisitParams contains the values needed to record a visit.
type CreateVisitParams struct {
	VisitedAt time.Time `db:"visited_at"`
	Weather   string    `db:"weather"`
	Notes     string    `db:"notes"`
	Rating    *int      `db:"rating"`
}
