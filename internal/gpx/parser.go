package gpx

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const gpx11Namespace = "http://www.topografix.com/GPX/1/1"

// ErrNoTrack indicates that a GPX document contains no track.
var ErrNoTrack = errors.New("GPX document contains no track")

// ErrMultipleTracks indicates that the document contrains more than one track.
//
// TrailLog currently imports one hike per GPX upload, so multi-track files
// must be split before importing.
var ErrMultipleTracks = errors.New("GPX document contains multiple tracks")

// ErrTooManyPoints indicates that a track exceeded the configured point limit.
var ErrTooManyPoints = errors.New("GPX track contains too many points")

// Parse reads one GPX 1.1 track from reader.
//
// The parser supports one track containing any number of track segments.
// Unknown GPX elements and extensions are ignored. maxPoints limits the
// number of imported track points and must be greater than zero.
func Parse(
	reader io.Reader,
	maxPoints int,
) (Track, error) {
	if maxPoints <= 0 {
		return Track{}, fmt.Errorf("maxPoints must be positive")
	}

	decoder := xml.NewDecoder(reader)

	var track Track

	trackCount := 0
	pointCount := 0

	rootSeen := false
	inTrack := false
	currentSegment := -1

	for {
		token, err := decoder.Token()

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return Track{}, fmt.Errorf("decode GPX XML: %w", err)
		}

		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {

			case "gpx":
				if rootSeen {
					continue
				}

				rootSeen = true
				if err := validateRoot(value); err != nil {
					return Track{}, err
				}

			case "trk":
				trackCount++

				if trackCount > 1 {
					return Track{}, ErrMultipleTracks
				}

				inTrack = true

			case "trkseg":
				if !inTrack {
					continue
				}

				track.Segments = append(track.Segments, Segment{Points: make([]Point, 0)})

				currentSegment = len(track.Segments) - 1

			case "name":
				if !inTrack || currentSegment >= 0 {
					continue
				}

				var name string

				if err := decoder.DecodeElement(&name, &value); err != nil {
					return Track{}, fmt.Errorf("decode track name: %w", err)
				}

				track.Name = strings.TrimSpace(name)

			case "desc":
				if !inTrack || currentSegment >= 0 {
					continue
				}

				var description string

				if err := decoder.DecodeElement(&description, &value); err != nil {
					return Track{}, fmt.Errorf("decode description: %w", err)
				}

				track.Description = strings.TrimSpace(description)

			case "trkpt":
				if !inTrack || currentSegment < 0 {
					continue
				}

				point, err := decodeTrackPoint(decoder, value)
				if err != nil {
					return Track{}, err
				}

				pointCount++

				if pointCount > maxPoints {
					return Track{}, ErrTooManyPoints
				}

				track.Segments[currentSegment].Points = append(
					track.Segments[currentSegment].Points,
					point,
				)
			}

		case xml.EndElement:
			switch value.Name.Local {

			case "trkseg":
				if inTrack {
					currentSegment = -1
				}

			case "trk":
				inTrack = false
				currentSegment = -1
			}
		}
	}

	if !rootSeen {
		return Track{}, fmt.Errorf("document has no GPX root element")
	}

	if trackCount == 0 {
		return Track{}, ErrNoTrack
	}

	return track, nil
}

// ---- Helpers ----

type trackPointXML struct {
	Elevation *float64 `xml:"ele"`
	Time      string   `xml:"time"`
}

func validateRoot(
	start xml.StartElement,
) error {
	if start.Name.Space != gpx11Namespace {
		return fmt.Errorf("unsupported GPX namespace %q", start.Name.Space)
	}

	version := ""

	for _, attr := range start.Attr {
		if attr.Name.Local == "version" {
			version = strings.TrimSpace(attr.Value)

			break
		}
	}

	if version != "1.1" {
		return fmt.Errorf("unsupported GPX version %v", version)
	}

	return nil
}

func decodeTrackPoint(
	decoder *xml.Decoder,
	start xml.StartElement,
) (Point, error) {
	latitudeValue := ""
	longitudeValue := ""

	for _, attr := range start.Attr {
		switch attr.Name.Local {

		case "lat":
			latitudeValue = attr.Value

		case "lon":
			longitudeValue = attr.Value
		}
	}

	if latitudeValue == "" {
		return Point{},
			fmt.Errorf(
				"track point is missing latitude",
			)
	}

	if longitudeValue == "" {
		return Point{},
			fmt.Errorf(
				"track point is missing longitude",
			)
	}

	latitude, err := strconv.ParseFloat(
		latitudeValue,
		64,
	)
	if err != nil {
		return Point{},
			fmt.Errorf(
				"parse latitude %q: %w",
				latitudeValue,
				err,
			)
	}

	longitude, err := strconv.ParseFloat(
		longitudeValue,
		64,
	)
	if err != nil {
		return Point{},
			fmt.Errorf(
				"parse longitude %q: %w",
				longitudeValue,
				err,
			)
	}

	if latitude < -90 ||
		latitude > 90 {

		return Point{},
			fmt.Errorf(
				"latitude %.6f is outside [-90, 90]",
				latitude,
			)
	}

	if longitude < -180 ||
		longitude > 180 {

		return Point{},
			fmt.Errorf(
				"longitude %.6f is outside [-180, 180]",
				longitude,
			)
	}

	var content trackPointXML

	if err := decoder.DecodeElement(
		&content,
		&start,
	); err != nil {
		return Point{},
			fmt.Errorf(
				"decode track point: %w",
				err,
			)
	}

	recordedAt, err := parseOptionalTime(
		content.Time,
	)
	if err != nil {
		return Point{},
			err
	}

	return Point{
		Latitude: latitude,

		Longitude: longitude,

		Elevation: content.Elevation,

		Time: recordedAt,
	}, nil
}

func parseOptionalTime(
	value string,
) (*time.Time, error) {
	value = strings.TrimSpace(value)

	if value == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, fmt.Errorf("parse GPX timestamp %q: %w", value, err)
	}

	parsed = parsed.UTC()

	return &parsed, nil
}
