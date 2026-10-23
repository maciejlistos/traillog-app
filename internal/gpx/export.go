package gpx

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

const exportCreator = "TrailLog"

// Write writes track as a GPX 1.1 document.
//
// The document is written incrementally to writer rather than first being
// marshaled into a complete byte slice. Track segment boundaries, coordinates,
// optional elevations, and optional timestamps are preserved.
//
// Write validates all track points before writing any XML.
func Write(
	writer io.Writer,
	track Track,
) error {
	if err := validateExportTrack(
		track,
	); err != nil {
		return err
	}

	encoder := xml.NewEncoder(
		writer,
	)

	encoder.Indent(
		"",
		"  ",
	)

	if err := encoder.EncodeToken(
		xml.ProcInst{
			Target: "xml",

			Inst: []byte(
				`version="1.0" encoding="UTF-8"`,
			),
		},
	); err != nil {
		return fmt.Errorf(
			"write XML declaration: %w",
			err,
		)
	}

	root := xml.StartElement{
		Name: xml.Name{
			Local: "gpx",
		},

		Attr: []xml.Attr{
			{
				Name: xml.Name{
					Local: "xmlns",
				},

				Value: gpx11Namespace,
			},
			{
				Name: xml.Name{
					Local: "version",
				},

				Value: "1.1",
			},
			{
				Name: xml.Name{
					Local: "creator",
				},

				Value: exportCreator,
			},
		},
	}

	if err := encoder.EncodeToken(
		root,
	); err != nil {
		return fmt.Errorf(
			"write GPX root: %w",
			err,
		)
	}

	trackElement := xml.StartElement{
		Name: xml.Name{
			Local: "trk",
		},
	}

	if err := encoder.EncodeToken(
		trackElement,
	); err != nil {
		return fmt.Errorf(
			"write track start: %w",
			err,
		)
	}

	if name := strings.TrimSpace(
		track.Name,
	); name != "" {
		if err := writeTextElement(
			encoder,
			"name",
			name,
		); err != nil {
			return err
		}
	}

	if description := strings.TrimSpace(
		track.Description,
	); description != "" {
		if err := writeTextElement(
			encoder,
			"desc",
			description,
		); err != nil {
			return err
		}
	}

	for _, segment := range track.Segments {
		if err := writeSegment(
			encoder,
			segment,
		); err != nil {
			return err
		}
	}

	if err := encoder.EncodeToken(
		xml.EndElement{
			Name: trackElement.Name,
		},
	); err != nil {
		return fmt.Errorf(
			"write track end: %w",
			err,
		)
	}

	if err := encoder.EncodeToken(
		xml.EndElement{
			Name: root.Name,
		},
	); err != nil {
		return fmt.Errorf(
			"write GPX end: %w",
			err,
		)
	}

	if err := encoder.Flush(); err != nil {
		return fmt.Errorf(
			"flush GPX XML: %w",
			err,
		)
	}

	return nil
}

func writeTextElement(
	encoder *xml.Encoder,
	name string,
	value string,
) error {
	start := xml.StartElement{
		Name: xml.Name{
			Local: name,
		},
	}

	if err := encoder.EncodeToken(
		start,
	); err != nil {
		return fmt.Errorf(
			"write %s start: %w",
			name,
			err,
		)
	}

	if err := encoder.EncodeToken(
		xml.CharData(
			[]byte(value),
		),
	); err != nil {
		return fmt.Errorf(
			"write %s value: %w",
			name,
			err,
		)
	}

	if err := encoder.EncodeToken(
		xml.EndElement{
			Name: start.Name,
		},
	); err != nil {
		return fmt.Errorf(
			"write %s end: %w",
			name,
			err,
		)
	}

	return nil
}

func writeSegment(
	encoder *xml.Encoder,
	segment Segment,
) error {
	start := xml.StartElement{
		Name: xml.Name{
			Local: "trkseg",
		},
	}

	if err := encoder.EncodeToken(
		start,
	); err != nil {
		return fmt.Errorf(
			"write track segment start: %w",
			err,
		)
	}

	for _, point := range segment.Points {
		if err := writeTrackPoint(
			encoder,
			point,
		); err != nil {
			return err
		}
	}

	if err := encoder.EncodeToken(
		xml.EndElement{
			Name: start.Name,
		},
	); err != nil {
		return fmt.Errorf(
			"write track segment end: %w",
			err,
		)
	}

	return nil
}

func writeTrackPoint(
	encoder *xml.Encoder,
	point Point,
) error {
	start := xml.StartElement{
		Name: xml.Name{
			Local: "trkpt",
		},

		Attr: []xml.Attr{
			{
				Name: xml.Name{
					Local: "lat",
				},

				Value: formatDecimal(
					point.Latitude,
				),
			},
			{
				Name: xml.Name{
					Local: "lon",
				},

				Value: formatDecimal(
					point.Longitude,
				),
			},
		},
	}

	if err := encoder.EncodeToken(
		start,
	); err != nil {
		return fmt.Errorf(
			"write track point start: %w",
			err,
		)
	}

	if point.Elevation != nil {
		if err := writeTextElement(
			encoder,
			"ele",
			formatDecimal(
				*point.Elevation,
			),
		); err != nil {
			return err
		}
	}

	if point.Time != nil {
		if err := writeTextElement(
			encoder,
			"time",
			point.Time.
				UTC().
				Format(
					time.RFC3339Nano,
				),
		); err != nil {
			return err
		}
	}

	if err := encoder.EncodeToken(
		xml.EndElement{
			Name: start.Name,
		},
	); err != nil {
		return fmt.Errorf(
			"write track point end: %w",
			err,
		)
	}

	return nil
}

func formatDecimal(
	value float64,
) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func validateExportTrack(
	track Track,
) error {
	pointCount := 1
	for segmentIndex, segment := range track.Segments {
		for pointIndex, point := range segment.Points {
			pointCount++

			if !validLatitude(
				point.Latitude,
			) {
				return fmt.Errorf(
					"segment %d point %d has invalid latitude %.8f",
					segmentIndex,
					pointIndex,
					point.Latitude,
				)
			}

			if !validLongitude(
				point.Longitude,
			) {
				return fmt.Errorf(
					"segment %d point %d has invalid longitude %.8f",
					segmentIndex,
					pointIndex,
					point.Longitude,
				)
			}

			if point.Elevation != nil && !finiteFloat(*point.Elevation) {
				return fmt.Errorf(
					"segment %d point %d has invalid elevation",
					segmentIndex,
					pointIndex,
				)
			}
		}
	}

	if pointCount < 2 {
		return fmt.Errorf("track contains %d points, want at least 2", pointCount)
	}

	return nil
}

func validLatitude(
	value float64,
) bool {
	return finiteFloat(value) && value >= -90 && value <= 90
}

func validLongitude(
	value float64,
) bool {
	return finiteFloat(value) && value >= -180 && value <= 180
}

func finiteFloat(
	value float64,
) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
