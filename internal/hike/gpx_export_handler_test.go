package hike

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maciejlistos/traillog/internal/gpx"
)

type fakeGPXExportService struct {
	exportTrackFunc func(
		context.Context,
		int64,
	) (TrackExport, error)
}

func (f *fakeGPXExportService) ExportTrack(
	ctx context.Context,
	hikeID int64,
) (TrackExport, error) {
	if f.exportTrackFunc == nil {
		panic(
			"unexpected ExportTrack call",
		)
	}

	return f.exportTrackFunc(
		ctx,
		hikeID,
	)
}

func TestGPXExportHandler(
	t *testing.T,
) {
	recordedAt := time.Date(
		2026,
		time.August,
		17,
		8,
		30,
		0,
		0,
		time.UTC,
	)

	elevation := 215.5

	service := &fakeGPXExportService{
		exportTrackFunc: func(
			ctx context.Context,
			hikeID int64,
		) (TrackExport, error) {
			if got, want :=
				hikeID,
				int64(42); got != want {
				t.Errorf(
					"hike ID = %d, want %d",
					got,
					want,
				)
			}

			return TrackExport{
				Hike: Hike{
					ID: 42,

					Name: "Forest & River",

					Description: "Quiet route",
				},

				Track: Track{
					ID: 7,

					HikeID: 42,

					PointCount: 3,
				},

				Points: []TrackPoint{
					{
						SegmentIndex: 0,

						Latitude: 51.1,

						Longitude: 22.1,

						Elevation: &elevation,

						RecordedAt: &recordedAt,
					},
					{
						SegmentIndex: 0,

						Latitude: 51.2,

						Longitude: 22.2,
					},
					{
						SegmentIndex: 1,

						Latitude: 51.3,

						Longitude: 22.3,
					},
				},
			}, nil
		},
	}

	gin.SetMode(
		gin.TestMode,
	)

	router := gin.New()

	handler := NewGPXExportHandler(
		service,
	)

	handler.RegisterRoutes(
		router,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/hikes/42/export.gpx",
		nil,
	)

	response := httptest.NewRecorder()

	router.ServeHTTP(
		response,
		request,
	)

	if got, want :=
		response.Code,
		http.StatusOK; got != want {
		t.Fatalf(
			"status = %d, want %d",
			got,
			want,
		)
	}

	if got := response.Header().Get(
		"Content-Type",
	); got !=
		"application/xml; charset=utf-8" {

		t.Errorf(
			"Content-Type = %q",
			got,
		)
	}

	disposition := response.Header().Get(
		"Content-Disposition",
	)

	if !strings.Contains(
		disposition,
		"attachment",
	) {
		t.Errorf(
			"Content-Disposition = %q, want attachment",
			disposition,
		)
	}

	if !strings.Contains(
		disposition,
		"traillog-hike-42.gpx",
	) {
		t.Errorf(
			"Content-Disposition = %q, want GPX filename",
			disposition,
		)
	}

	exported, err := gpx.Parse(
		bytes.NewReader(
			response.Body.Bytes(),
		),
		100,
	)
	if err != nil {
		t.Fatalf(
			"parse exported response: %v",
			err,
		)
	}

	if got, want :=
		exported.Name,
		"Forest & River"; got != want {
		t.Errorf(
			"name = %q, want %q",
			got,
			want,
		)
	}

	if got, want :=
		len(exported.Segments),
		2; got != want {
		t.Fatalf(
			"segments = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		len(
			exported.
				Segments[0].
				Points,
		),
		2; got != want {
		t.Errorf(
			"first segment points = %d, want %d",
			got,
			want,
		)
	}
}
