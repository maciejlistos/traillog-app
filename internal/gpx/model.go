package gpx

import "time"

// Point represents on GPX track point.
type Point struct {
	Latitude  float64
	Longitude float64
	Elevation *float64
	Time      *time.Time
}

// Segment represents one GPX tract segment.
type Segment struct {
	Points []Point
}

// Track represents one GPX track and all of its segments.
type Track struct {
	Name        string
	Description string
	Segments    []Segment
}

// Summary contains values calculated from a Track.
type Summary struct {
	PointCount     int
	DistanceKM     float64
	ElevationGainM float64
	Duration       *time.Duration

	Start Point
	End   Point
}
