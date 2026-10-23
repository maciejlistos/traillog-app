package hike

// DashboardStats conteains aggregate information displayed on TrailLog's
// statistics dashboard.
//
// The values are calculated from hikes, visits, and tags when requested.
type DashboardStats struct {
	TotalHikes      int
	VisitedHikes    int
	PlannedHikes    int
	TotalDistanceKM float64
	TotalVisits     int
	AverageRating   *float64

	TopTags          []TagUsage
	MostVisitedHikes []HikeVisitCount
	MonthlyVisits    []MonthlyVisitCount
}

// TagUsage descibes how many hikes currently use a tag.
type TagUsage struct {
	Name      string `db:"name"`
	HikeCount int    `db:"hike_count"`
}

// HikeVisitCount describes how many recorded vistis belong to a hike.
type HikeVisitCount struct {
	HikeID     int64  `db:"hike_id"`
	Name       string `db:"name"`
	VisitCount int    `db:"visit_count"`
}

// MonthlyVisitCount describes the number of recorded visits in one calendar
// month. Mont uses the YYYY-MM respresentation, for example "2026-08"
type MonthlyVisitCount struct {
	Month      string `db:"month"`
	VisitCount int    `db:"visit_count"`
}
