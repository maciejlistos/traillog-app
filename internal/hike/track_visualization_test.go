package hike

import (
	"testing"
)

func TestTrackVisualizationPreservesSegments(
	t *testing.T,
) {
	elevation100 := 100.0
	elevation110 := 110.0
	elevation200 := 200.0
	elevation210 := 210.0

	points := []TrackPoint{
		{
			SegmentIndex: 0,
			Latitude:     51.0000,
			Longitude:    22.0000,
			Elevation:    &elevation100,
		},
		{
			SegmentIndex: 0,
			Latitude:     51.0010,
			Longitude:    22.0010,
			Elevation:    &elevation110,
		},

		// Deliberately far away.
		{
			SegmentIndex: 1,
			Latitude:     52.0000,
			Longitude:    23.0000,
			Elevation:    &elevation200,
		},
		{
			SegmentIndex: 1,
			Latitude:     52.0010,
			Longitude:    23.0010,
			Elevation:    &elevation210,
		},
	}

	visualization := buildTrackVisualization(
		Track{
			PointCount: len(points),
		},
		points,
	)

	if got, want :=
		len(
			visualization.Map.Segments,
		),
		2; got != want {
		t.Fatalf(
			"map segments = %d, want %d",
			got,
			want,
		)
	}

	if visualization.Elevation == nil {
		t.Fatal(
			"elevation profile is nil",
		)
	}

	if got, want :=
		len(
			visualization.
				Elevation.
				Segments,
		),
		2; got != want {
		t.Fatalf(
			"elevation segments = %d, want %d",
			got,
			want,
		)
	}

	// Each individual segment is only about 130 m.
	// If TrailLog connected segment 0 to segment 1,
	// the result would exceed 100 km.
	if got := visualization.
		Elevation.
		DistanceKM; got > 1 {
		t.Errorf(
			"distance = %.2f km; segment boundary was probably bridged",
			got,
		)
	}
}

func TestTrackMapSimplifiesDensePoints(
	t *testing.T,
) {
	points := make(
		[]TrackPoint,
		0,
		1000,
	)

	for index := range 1000 {
		points = append(
			points,
			TrackPoint{
				SegmentIndex: 0,

				Latitude: 51 +
					float64(index)*
						0.000001,

				Longitude: 22,
			},
		)
	}

	data := buildTrackMapData(
		points,
	)

	if got, want :=
		len(data.Segments),
		1; got != want {
		t.Fatalf(
			"segments = %d, want %d",
			got,
			want,
		)
	}

	got := len(
		data.Segments[0].Points,
	)

	if got >= len(points) {
		t.Errorf(
			"map retained %d points, expected fewer than %d",
			got,
			len(points),
		)
	}

	if got < 2 {
		t.Errorf(
			"map retained only %d points, want at least 2",
			got,
		)
	}
}

func TestElevationProfileCreatesOneProfilePerTrackSegment(
	t *testing.T,
) {
	elevations := []float64{100, 110, 105, 120}

	points := make([]TrackPoint, 0, len(elevations))

	for index := range elevations {
		points = append(
			points,
			TrackPoint{
				SegmentIndex: 0,
				Latitude:     51 + float64(index)*0.001,
				Longitude:    22 + float64(index)*0.001,
				Elevation:    &elevations[index],
			},
		)
	}

	profile := buildElevationProfile(points)
	if profile == nil {
		t.Fatal("buildElevationProfile() returned nil")
	}

	if got, want := len(profile.Segments), 1; got != want {
		t.Fatalf("profile segments = %d, want %d", got, want)
	}

	if got, want := len(profile.Segments[0].Samples), len(points); got != want {
		t.Errorf("profile samples = %d, want %d", got, want)
	}
}
