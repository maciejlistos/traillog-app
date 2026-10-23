package gpx

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestWriteRoundTrip(
	t *testing.T,
) {
	firstTime := time.Date(
		2026,
		time.August,
		17,
		8,
		0,
		0,
		123456000,
		time.UTC,
	)

	secondTime := time.Date(
		2026,
		time.August,
		17,
		8,
		15,
		0,
		0,
		time.UTC,
	)

	elevation100 := 100.5

	elevation120 := 120.75

	elevation200 := 200.0

	original := Track{
		Name: "Forest & River <Loop>",

		Description: "Imported by TrailLog",

		Segments: []Segment{
			{
				Points: []Point{
					{
						Latitude: 51.1234567,

						Longitude: 22.7654321,

						Elevation: &elevation100,

						Time: &firstTime,
					},
					{
						Latitude: 51.1240001,

						Longitude: 22.7660002,

						Elevation: &elevation120,

						Time: &secondTime,
					},
				},
			},

			{
				Points: []Point{
					{
						Latitude: 51.2,

						Longitude: 22.8,

						Elevation: &elevation200,
					},
				},
			},
		},
	}

	var output bytes.Buffer

	if err := Write(
		&output,
		original,
	); err != nil {
		t.Fatalf(
			"Write() returned error: %v",
			err,
		)
	}

	xmlDocument := output.String()

	if !strings.Contains(
		xmlDocument,
		`xmlns="http://www.topografix.com/GPX/1/1"`,
	) {
		t.Error(
			"export does not contain GPX 1.1 namespace",
		)
	}

	if !strings.Contains(
		xmlDocument,
		`version="1.1"`,
	) {
		t.Error(
			"export does not contain GPX version",
		)
	}

	if !strings.Contains(
		xmlDocument,
		`creator="TrailLog"`,
	) {
		t.Error(
			"export does not contain creator",
		)
	}

	parsed, err := Parse(
		bytes.NewReader(
			output.Bytes(),
		),
		100,
	)
	if err != nil {
		t.Fatalf(
			"parse exported GPX: %v",
			err,
		)
	}

	if got, want :=
		parsed.Name,
		original.Name; got != want {
		t.Errorf(
			"name = %q, want %q",
			got,
			want,
		)
	}

	if got, want :=
		parsed.Description,
		original.Description; got != want {
		t.Errorf(
			"description = %q, want %q",
			got,
			want,
		)
	}

	if got, want :=
		len(parsed.Segments),
		2; got != want {
		t.Fatalf(
			"segments = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		len(parsed.Segments[0].Points),
		2; got != want {
		t.Fatalf(
			"first segment points = %d, want %d",
			got,
			want,
		)
	}

	gotPoint := parsed.Segments[0].
		Points[0]

	wantPoint := original.Segments[0].
		Points[0]

	if gotPoint.Latitude !=
		wantPoint.Latitude {

		t.Errorf(
			"latitude = %.10f, want %.10f",
			gotPoint.Latitude,
			wantPoint.Latitude,
		)
	}

	if gotPoint.Longitude !=
		wantPoint.Longitude {

		t.Errorf(
			"longitude = %.10f, want %.10f",
			gotPoint.Longitude,
			wantPoint.Longitude,
		)
	}

	if gotPoint.Elevation == nil {
		t.Fatal(
			"round-trip elevation is nil",
		)
	}

	if *gotPoint.Elevation !=
		*wantPoint.Elevation {

		t.Errorf(
			"elevation = %f, want %f",
			*gotPoint.Elevation,
			*wantPoint.Elevation,
		)
	}

	if gotPoint.Time == nil {
		t.Fatal(
			"round-trip time is nil",
		)
	}

	if !gotPoint.Time.Equal(
		*wantPoint.Time,
	) {
		t.Errorf(
			"time = %s, want %s",
			gotPoint.Time,
			wantPoint.Time,
		)
	}
}
