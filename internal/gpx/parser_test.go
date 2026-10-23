package gpx

import (
	"errors"
	"strings"
	"testing"
)

func TestParseTrack(
	t *testing.T,
) {
	input := `
<?xml version="1.0" encoding="UTF-8"?>
<gpx
	xmlns="http://www.topografix.com/GPX/1/1"
	version="1.1"
	creator="TrailLog test"
>
	<trk>
		<name>Forest Loop</name>
		<desc>Quiet forest route</desc>

		<trkseg>
			<trkpt lat="51.1000" lon="22.1000">
				<ele>200</ele>
				<time>2026-08-17T08:00:00Z</time>
			</trkpt>

			<trkpt lat="51.1010" lon="22.1020">
				<ele>205</ele>
				<time>2026-08-17T08:10:00Z</time>
			</trkpt>
		</trkseg>

		<trkseg>
			<trkpt lat="51.2000" lon="22.2000">
				<ele>220</ele>
			</trkpt>
		</trkseg>
	</trk>
</gpx>
`

	track, err := Parse(
		strings.NewReader(input),
		100,
	)
	if err != nil {
		t.Fatalf(
			"Parse() returned error: %v",
			err,
		)
	}

	if got, want :=
		track.Name,
		"Forest Loop"; got != want {
		t.Errorf(
			"name = %q, want %q",
			got,
			want,
		)
	}

	if got, want :=
		len(track.Segments),
		2; got != want {
		t.Fatalf(
			"segments = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		len(track.Segments[0].Points),
		2; got != want {
		t.Errorf(
			"segment points = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		*track.Segments[0].
			Points[1].
			Elevation,
		205.0; got != want {
		t.Errorf(
			"elevation = %.1f, want %.1f",
			got,
			want,
		)
	}
}

func TestParseRejectsMultipleTracks(
	t *testing.T,
) {
	input := `
<gpx
	xmlns="http://www.topografix.com/GPX/1/1"
	version="1.1"
	creator="test"
>
	<trk>
		<trkseg>
			<trkpt lat="51" lon="22"/>
		</trkseg>
	</trk>

	<trk>
		<trkseg>
			<trkpt lat="52" lon="23"/>
		</trkseg>
	</trk>
</gpx>
`

	_, err := Parse(
		strings.NewReader(input),
		100,
	)

	if !errors.Is(
		err,
		ErrMultipleTracks,
	) {
		t.Fatalf(
			"error = %v, want ErrMultipleTracks",
			err,
		)
	}
}
