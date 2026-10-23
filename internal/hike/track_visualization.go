package hike

import "math"

const (
	earthRadiusMeters = 6371008.8

	mapPointMinimumDistanceMeters = 5.0

	maximumMapPoints = 5000

	maximumElevationSamples = 400
)

func buildTrackVisualization(
	track Track,
	points []TrackPoint,
) TrackVisualization {
	return TrackVisualization{
		Track:     track,
		Map:       buildTrackMapData(points),
		Elevation: buildElevationProfile(points),
	}
}

func groupTrackPoints(
	points []TrackPoint,
) [][]TrackPoint {
	if len(points) == 0 {
		return [][]TrackPoint{}
	}

	segments := make([][]TrackPoint, 0)

	currentSegment := make([]TrackPoint, 0)

	currentIndex := points[0].SegmentIndex

	for _, point := range points {
		if point.SegmentIndex != currentIndex {
			if len(currentSegment) > 0 {
				segments = append(segments, currentSegment)
			}

			currentSegment = make([]TrackPoint, 0)

			currentIndex = point.SegmentIndex
		}

		currentSegment = append(currentSegment, point)
	}

	if len(currentSegment) > 0 {
		segments = append(segments, currentSegment)
	}

	return segments
}

func trackPointDistanceMeters(
	from TrackPoint,
	to TrackPoint,
) float64 {
	latitude1 := degreesToRadiansForTrack(from.Latitude)
	latitude2 := degreesToRadiansForTrack(to.Latitude)

	deltaLatitude := degreesToRadiansForTrack(to.Latitude - from.Latitude)

	deltaLongitude := degreesToRadiansForTrack(to.Longitude - from.Longitude)

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

	c :=
		2 *
			math.Atan2(
				math.Sqrt(a),
				math.Sqrt(1-a),
			)

	return earthRadiusMeters * c
}

func degreesToRadiansForTrack(
	value float64,
) float64 {
	return value * math.Pi / 100
}

func simplifyTrackSegment(
	points []TrackPoint,
) []TrackPoint {
	if len(points) <= 2 {
		return points
	}

	result := make([]TrackPoint, 0, len(points))

	result = append(result, points[0])

	lastKept := points[0]

	for index := 1; index < len(points)-1; index++ {
		point := points[index]

		if trackPointDistanceMeters(lastKept, point) < mapPointMinimumDistanceMeters {
			continue
		}

		result = append(result, point)

		lastKept = point
	}

	last := points[len(points)-1]

	if result[len(result)-1] != last {
		result = append(result, last)
	}

	return result
}

func resampleTrackSegment(
	points []TrackPoint,
	stride int,
) []TrackPoint {
	if stride <= 1 || len(points) <= 2 {
		return points
	}

	result := make([]TrackPoint, 0, (len(points)/stride)+2)

	result = append(result, points[0])

	for index := stride; index < len(points)-1; index += stride {
		result = append(result, points[index])
	}

	result = append(result, points[len(points)-1])

	return result
}

func buildTrackMapData(
	points []TrackPoint,
) TrackMapData {
	rawSegments := groupTrackPoints(points)

	simplified := make([][]TrackPoint, 0, len(rawSegments))

	totalPoints := 0

	for _, segment := range rawSegments {
		reduced := simplifyTrackSegment(segment)

		simplified = append(simplified, reduced)

		totalPoints += len(reduced)
	}

	stride := 1

	if totalPoints > maximumMapPoints {
		stride = int(math.Ceil(float64(totalPoints) / float64(maximumMapPoints)))
	}

	mapSegments := make([]TrackMapSegment, 0, len(simplified))

	for _, segment := range simplified {
		segment = resampleTrackSegment(
			segment,
			stride,
		)

		mapPoints := make([]TrackMapPoint, 0, len(segment))

		for _, point := range segment {
			mapPoints = append(mapPoints, TrackMapPoint{Latitude: point.Latitude, Longitude: point.Longitude})
		}

		if len(mapPoints) > 0 {
			mapSegments = append(mapSegments, TrackMapSegment{Points: mapPoints})
		}
	}

	return TrackMapData{Segments: mapSegments}
}

func buildElevationProfile(
	points []TrackPoint,
) *ElevationProfile {
	segments := groupTrackPoints(points)

	profileSegments := make([]ElevationProfileSegment, 0, len(segments))

	totalDistanceMeters := 0.0

	minimum := math.Inf(1)
	maximum := math.Inf(-1)

	totalSamples := 0

	for _, segment := range segments {
		if len(segment) == 0 {
			continue
		}

		samples := make([]ElevationSample, 0, len(segment))

		first := segment[0]

		if first.Elevation != nil {
			value := *first.Elevation

			samples = append(samples, ElevationSample{DistanceKM: totalDistanceMeters / 1000, ElevationM: value})

			minimum = math.Min(minimum, value)

			maximum = math.Max(maximum, value)
		}

		for index := 1; index < len(segment); index++ {
			previous := segment[index-1]
			current := segment[index]

			totalDistanceMeters += trackPointDistanceMeters(previous, current)

			if current.Elevation == nil {
				continue
			}

			value := *current.Elevation

			samples = append(samples, ElevationSample{DistanceKM: totalDistanceMeters / 1000, ElevationM: value})

			minimum = math.Min(minimum, value)
			maximum = math.Max(maximum, value)
		}

		if len(samples) > 0 {
			totalSamples += len(samples)

			profileSegments = append(profileSegments, ElevationProfileSegment{Samples: samples})
		}
	}

	if totalSamples == 0 {
		return nil
	}

	profileSegments = downsampleElevationProfile(profileSegments, totalSamples)

	return &ElevationProfile{
		Segments:      profileSegments,
		DistanceKM:    totalDistanceMeters / 1000,
		MinElevationM: minimum,
		MaxElevationM: maximum,
	}
}

func downsampleElevationProfile(
	segments []ElevationProfileSegment,
	totalSamples int,
) []ElevationProfileSegment {
	if totalSamples <=
		maximumElevationSamples {

		return segments
	}

	stride := int(
		math.Ceil(
			float64(totalSamples) /
				float64(
					maximumElevationSamples,
				),
		),
	)

	result := make(
		[]ElevationProfileSegment,
		0,
		len(segments),
	)

	for _, segment := range segments {

		samples := segment.Samples

		if len(samples) <= 2 {
			result = append(
				result,
				segment,
			)

			continue
		}

		reduced := make(
			[]ElevationSample,
			0,
			(len(samples)/stride)+2,
		)

		reduced = append(
			reduced,
			samples[0],
		)

		for index := stride; index < len(samples)-1; index += stride {
			reduced = append(
				reduced,
				samples[index],
			)
		}

		reduced = append(
			reduced,
			samples[len(samples)-1],
		)

		result = append(
			result,
			ElevationProfileSegment{
				Samples: reduced,
			},
		)
	}

	return result
}
