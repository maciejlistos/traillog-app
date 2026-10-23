package gpx

import (
	"math"
	"testing"
)

func TestSummarizeDoesNotBridgeSegments(
	t *testing.T,
) {
	track := Track{
		Segments: []Segment{
			{
				Points: []Point{
					{
						Latitude:  0,
						Longitude: 0,
					},
					{
						Latitude:  0,
						Longitude: 0.01,
					},
				},
			},

			{
				Points: []Point{
					{
						Latitude:  50,
						Longitude: 20,
					},
					{
						Latitude:  50,
						Longitude: 20.01,
					},
				},
			},
		},
	}

	summary, err := Summarize(track)
	if err != nil {
		t.Fatalf(
			"Summarize() returned error: %v",
			err,
		)
	}

	// The total should be roughly two short segment distances,
	// not the enormous gap between the segments.
	if summary.DistanceKM > 3 {
		t.Errorf(
			"distance = %.2f km; segment boundary was probably bridged",
			summary.DistanceKM,
		)
	}

	if math.Abs(
		summary.DistanceKM-1.83,
	) > 0.2 {
		t.Errorf(
			"distance = %.2f km, want approximately 1.83 km",
			summary.DistanceKM,
		)
	}
}
