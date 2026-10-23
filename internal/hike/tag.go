package hike

import "time"

// Tag represents a resuable label assigned to one or more hikes.
//
// Tag names are stored in nomalized lowercase form by the service layer.
// Multiple hikes may reference the same Tag.
type Tag struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}
