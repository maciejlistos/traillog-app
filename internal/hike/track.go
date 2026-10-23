package hike

import (
	"errors"
	"time"
)

// ErrTrackNotFound indicates that a Hike has no imported GPX Track.
var ErrTrackNotFound = errors.New("track not found")

// Track represents GPX-derived route metadata belonging to a Hike.
type Track struct {
	ID             int64
	HikeID         int64
	SourceFilename string

	PointCount     int
	DistanceKM     float64
	ElevationGainM float64
	Duration       *time.Duration

	StartLat float64
	StartLon float64
	EndLat   float64
	EndLon   float64

	CreatedAt time.Time
}

// TrackPoint represents one persisted point in an imported GPS track.
type TrackPoint struct {
	SegmentIndex int `db:"segment_index"`

	Latitude  float64 `db:"latitude"`
	Longitude float64 `db:"longitude"`

	Elevation  *float64   `db:"elevation_m"`
	RecordedAt *time.Time `db:"recorded_at"`
}

// ImportTrackParams contains a Hike and its parsed GPX route data.
//
// ImportTrack creates the Hike, Track and TrackPoint records atomically.
type ImportTrackParams struct {
	Hike CreateParams

	SourceFilename string

	Points []TrackPoint

	DistanceKM     float64
	ElevationGainM float64
	Duration       *time.Duration

	StartLat float64
	StartLon float64
	EndLat   float64
	EndLon   float64
}

// TrackMapPoint represents one coordinate sent to the browser map.
type TrackMapPoint struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}

// TrackMapSegment represents one indepedent GPX segment on the route map.
//
// Segment boundaries are retained so the browser does not draw artificial
// lines between unrelated GPX track segments.
type TrackMapSegment struct {
	Points []TrackMapPoint `json:"points"`
}

// TrackMapData contains simplified route geometry for map rendering.
type TrackMapData struct {
	Segments []TrackMapSegment `json:"segments"`
}

// ElevationSample represents elevation at a cumulative route distance.
type ElevationSample struct {
	DistanceKM float64
	ElevationM float64
}

// ElevationProfileSegment contains samples belonging to on GPX segment.
type ElevationProfileSegment struct {
	Samples []ElevationSample
}

// ElevationProfile contains elevation samples and chart bounds.
//
// Segment boundaries are retained so discontinuous GPX segments are not
// connected by artificial elevation lines.
type ElevationProfile struct {
	Segments []ElevationProfileSegment

	DistanceKM float64

	MinElevationM float64
	MaxElevationM float64
}

// TrackVisualization contains the route data needed by TrailLog's map and
// elevation-profile views.
type TrackVisualization struct {
	Track Track
	Map   TrackMapData

	Elevation *ElevationProfile
}

// TrackExport contains the normalized TrailLog data required to renegerate
// an import route as GPX.
//
// Points contains the complete stored route geometry, not the simplified
// browser representation used by TrackVisualization.
type TrackExport struct {
	Hike   Hike
	Track  Track
	Points []TrackPoint
}

// SetTrackParams contains GPX-derived route data used to attach or replace
// the Track belonging to an existing Hike.
//
// SetTrackParams does not contain Hike metadata such as name of location.
// Replacing a Track updates the Hike distance from DinstanceKM but otherwise
// leaves the Hike unchanged.
type SetTrackParams struct {
	SourceFilename string

	Points []TrackPoint

	DistanceKM     float64
	ElevationGainM float64
	Duration       *time.Duration

	StartLat float64
	StartLon float64

	EndLat float64
	EndLon float64
}

func importTrackParamsFromSet(
	params SetTrackParams,
) ImportTrackParams {
	return ImportTrackParams{
		SourceFilename: params.SourceFilename,

		Points: params.Points,

		DistanceKM: params.DistanceKM,

		ElevationGainM: params.ElevationGainM,

		Duration: params.Duration,

		StartLat: params.StartLat,

		StartLon: params.StartLon,

		EndLat: params.EndLat,

		EndLon: params.EndLon,
	}
}
