package hike

import "time"

// Status describes whether a hike is planned or has been visited.
type Status string

const (
	// StatusPlanned identifies a hike that has not yet been visited.
	StatusPlanned Status = "planned"
	// StatusVisited identifies a hike that has been visited.
	StatusVisited Status = "visited"
)

// Sort defines the ordering applied to a hike search.
type Sort string

const (
	// SortNewest orders hikes from newest to oldest.
	SortNewest Sort = "newest"
	// SortName orders hikes alphabetically by name.
	SortName Sort = "name"
	// SortDistance orders hikes by distance.
	SortDistance Sort = "distance"
)

// Valid reports whether s is a supported sort order.
func (s Sort) Valid() bool {
	switch s {
	case SortNewest, SortName, SortDistance:
		return true
	default:
		return false
	}
}

// String returns the string representation of s.
func (s Sort) String() string {
	return string(s)
}

// ListParams contains filtering, ordering, and pagination options for a hike search.
type ListParams struct {
	Query    string
	Status   *Status
	Tags     []string
	Sort     Sort
	Page     int
	PageSize int
}

// ListResult contains one page of hikes and its pagination metadata.
type ListResult struct {
	Hikes      []Hike
	Total      int
	Page       int
	PageSize   int
	TotalPages int
}

// Hike is a trail or route recorded in TrailLog.
type Hike struct {
	ID          int64     `db:"id"`
	Name        string    `db:"name"`
	Location    string    `db:"location"`
	Description string    `db:"description"`
	DistanceKM  *float64  `db:"distance_km"`
	Status      Status    `db:"status"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	Tags        []Tag     `db:"-"`
}

// CreateParams contains the values needed to create a hike.
type CreateParams struct {
	Name        string   `db:"name"`
	Location    string   `db:"location"`
	Description string   `db:"description"`
	DistanceKM  *float64 `db:"distance_km"`
	Status      Status   `db:"status"`
}

// UpdateParams contains the replacement values for an existing hike.
type UpdateParams struct {
	Name        string   `db:"name"`
	Location    string   `db:"location"`
	Description string   `db:"description"`
	DistanceKM  *float64 `db:"distance_km"`
	Status      Status   `db:"status"`
}

// String returns the string representation of s.
func (s Status) String() string {
	return string(s)
}

// Valid reports whether s is a supported hike status.
func (s Status) Valid() bool {
	switch s {
	case StatusPlanned, StatusVisited:
		return true
	default:
		return false
	}
}
