package gpx

import (
	"fmt"
	"math"
	"time"
)

const earthRadiusMeters = 6371008.0

// Summarize calculates route statistics from track.
//
// Distance and elevation gain are calculated only between consecutive points
// belonging to the same segment. Duration is returned only when each non-empty
// segment has timestamps on its first and last points.
func Summarize(
	track Track,
) (Summary, error) {
	var summary Summary

	var firstPoint *Point
	var lastPoint *Point

	durationKnown := true
	totalDuration := time.Duration(0)

	for _, segment := range track.Segments {
		if len(segment.Points) == 0 {
			continue
		}

		summary.PointCount += len(segment.Points)

		if firstPoint == nil {
			value := segment.Points[0]

			firstPoint = &value
		}

		value := segment.Points[len(segment.Points)-1]

		lastPoint = &value

		for index := 1; index < len(segment.Points); index++ {
			previous := segment.Points[index-1]
			current := segment.Points[index]

			summary.DistanceKM += distanceMeters(previous, current) / 1000

			if previous.Elevation != nil && current.Elevation != nil {
				gain := *current.Elevation - *previous.Elevation

				if gain > 0 {
					summary.ElevationGainM += gain
				}
			}
		}

		first := segment.Points[0]
		last := segment.Points[len(segment.Points)-1]

		if first.Time == nil || last.Time == nil {
			durationKnown = false

			continue
		}

		elapsed := last.Time.Sub(*first.Time)

		if elapsed < 0 {
			return Summary{},
				fmt.Errorf("track segment ends before it starts")
		}

		totalDuration += elapsed
	}

	if summary.PointCount < 2 || firstPoint == nil || lastPoint == nil {
		return Summary{}, fmt.Errorf("track must contain at least two points")
	}

	summary.Start = *firstPoint
	summary.End = *lastPoint

	if durationKnown {
		summary.Duration = &totalDuration
	}

	return summary, nil
}

func distanceMeters(
	from Point,
	to Point,
) float64 {
	latitude1 := degreesToRadians(from.Latitude)
	latitude2 := degreesToRadians(to.Latitude)

	deltaLatitude := degreesToRadians(to.Latitude - from.Latitude)

	deltaLongitude := degreesToRadians(to.Longitude - from.Longitude)

	a :=
		math.Sin(
			deltaLatitude/2,
		)*
			math.Sin(
				deltaLatitude/2,
			) +
			math.Cos(latitude1)*
				math.Cos(latitude2)*
				math.Sin(
					deltaLongitude/2,
				)*
				math.Sin(
					deltaLongitude/2,
				)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusMeters * c
}

func degreesToRadians(
	value float64,
) float64 {
	return value * math.Pi / 180
}
