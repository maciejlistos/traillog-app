package hike

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maciejlistos/traillog/internal/webui"
)

func TestHandlerCreateHike(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.August,
		12,
		10,
		0,
		0,
		0,
		time.UTC,
	)

	var received CreateParams

	service := &fakeHikeService{
		createFunc: func(
			ctx context.Context,
			params CreateParams,
		) (Hike, error) {
			received = params

			return Hike{
				ID:          42,
				Name:        params.Name,
				Location:    params.Location,
				Description: params.Description,
				DistanceKM:  params.DistanceKM,
				Status:      params.Status,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}

	router := newHandlerTestRouter(
		t,
		service,
	)

	body := []byte(`{
		"name": "Forest Loop",
		"location": "Lublin",
		"description": "Quiet walk",
		"distance_km": 8.4,
		"status": "planned"
	}`)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/hikes",
		bytes.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	response := recorder.Result()
	defer func() {
		_ = response.Body.Close()
	}()

	if got, want := response.StatusCode,
		http.StatusCreated; got != want {
		t.Fatalf(
			"status = %d, want %d",
			got,
			want,
		)
	}

	if got, want := response.Header.Get(
		"Location",
	), "/api/hikes/42"; got != want {
		t.Errorf(
			"Location = %q, want %q",
			got,
			want,
		)
	}

	if got, want := received.Name,
		"Forest Loop"; got != want {
		t.Errorf(
			"service name = %q, want %q",
			got,
			want,
		)
	}

	if got, want := received.Status,
		StatusPlanned; got != want {
		t.Errorf(
			"service status = %q, want %q",
			got,
			want,
		)
	}

	var responseBody hikeResponse

	if err := json.NewDecoder(
		response.Body,
	).Decode(&responseBody); err != nil {
		t.Fatalf(
			"decode response: %v",
			err,
		)
	}

	if got, want := responseBody.ID,
		int64(42); got != want {
		t.Errorf(
			"response ID = %d, want %d",
			got,
			want,
		)
	}
}

func TestHandlerCreateRejectsInvalidJSON(
	t *testing.T,
) {
	service := &fakeHikeService{
		createFunc: func(
			context.Context,
			CreateParams,
		) (Hike, error) {
			t.Fatal(
				"service.Create called for invalid JSON",
			)

			return Hike{}, nil
		},
	}

	router := newHandlerTestRouter(
		t,
		service,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/hikes",
		bytes.NewBufferString(
			`{"name":`,
		),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if got, want := recorder.Code,
		http.StatusBadRequest; got != want {
		t.Fatalf(
			"status = %d, want %d",
			got,
			want,
		)
	}
}

func TestHandlerCreateReturnsValidationErrors(
	t *testing.T,
) {
	service := &fakeHikeService{
		createFunc: func(
			context.Context,
			CreateParams,
		) (Hike, error) {
			return Hike{},
				&ValidationError{
					Fields: map[string]string{
						"name": "Name is required",
					},
				}
		},
	}

	router := newHandlerTestRouter(
		t,
		service,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/hikes",
		bytes.NewBufferString(`{
			"name": "",
			"location": "Lublin",
			"status": "planned"
		}`),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if got, want := recorder.Code,
		http.StatusUnprocessableEntity; got != want {
		t.Fatalf(
			"status = %d, want %d",
			got,
			want,
		)
	}

	var body errorResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"decode response: %v",
			err,
		)
	}

	if got, want := body.Fields["name"],
		"Name is required"; got != want {
		t.Errorf(
			"name error = %q, want %q",
			got,
			want,
		)
	}
}

func TestHandlerGetReturnsNotFound(
	t *testing.T,
) {
	service := &fakeHikeService{
		findByIDFunc: func(
			context.Context,
			int64,
		) (Hike, error) {
			return Hike{}, fmt.Errorf(
				"find hike: %w",
				ErrNotFound,
			)
		},
	}

	router := newHandlerTestRouter(
		t,
		service,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/hikes/999",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if got, want := recorder.Code,
		http.StatusNotFound; got != want {
		t.Errorf(
			"status = %d, want %d",
			got,
			want,
		)
	}
}

func TestHandlerRejectsInvalidHikeID(
	t *testing.T,
) {
	tests := []string{
		"abc",
		"0",
		"-1",
		"12.5",
	}

	for _, id := range tests {
		t.Run(id, func(t *testing.T) {
			service := &fakeHikeService{
				findByIDFunc: func(
					context.Context,
					int64,
				) (Hike, error) {
					t.Fatal(
						"service called for invalid ID",
					)

					return Hike{}, nil
				},
			}

			router := newHandlerTestRouter(
				t,
				service,
			)

			request := httptest.NewRequest(
				http.MethodGet,
				"/api/hikes/"+id,
				nil,
			)

			recorder := httptest.NewRecorder()

			router.ServeHTTP(
				recorder,
				request,
			)

			if got, want := recorder.Code,
				http.StatusBadRequest; got != want {
				t.Errorf(
					"status = %d, want %d",
					got,
					want,
				)
			}
		})
	}
}

func TestHandlerDeleteHike(
	t *testing.T,
) {
	var deletedID int64

	service := &fakeHikeService{
		deleteFunc: func(
			ctx context.Context,
			id int64,
		) error {
			deletedID = id
			return nil
		},
	}

	router := newHandlerTestRouter(
		t,
		service,
	)

	request := httptest.NewRequest(
		http.MethodDelete,
		"/api/hikes/42",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if got, want := recorder.Code,
		http.StatusNoContent; got != want {
		t.Fatalf(
			"status = %d, want %d",
			got,
			want,
		)
	}

	if got, want := deletedID,
		int64(42); got != want {
		t.Errorf(
			"deleted ID = %d, want %d",
			got,
			want,
		)
	}

	if recorder.Body.Len() != 0 {
		t.Errorf(
			"response body = %q, want empty",
			recorder.Body.String(),
		)
	}
}

func TestHandlerListHikes(
	t *testing.T,
) {
	service := &fakeHikeService{
		listFunc: func(
			context.Context,
		) ([]Hike, error) {
			return []Hike{
				{
					ID:       1,
					Name:     "Forest Loop",
					Location: "Lublin",
					Status:   StatusPlanned,
				},
				{
					ID:       2,
					Name:     "Roztocze Trail",
					Location: "Roztocze",
					Status:   StatusVisited,
				},
			}, nil
		},
	}

	router := newHandlerTestRouter(
		t,
		service,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/hikes",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if got, want := recorder.Code,
		http.StatusOK; got != want {
		t.Fatalf(
			"status = %d, want %d",
			got,
			want,
		)
	}

	var body []hikeResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"decode response: %v",
			err,
		)
	}

	if got, want := len(body), 2; got != want {
		t.Fatalf(
			"hike count = %d, want %d",
			got,
			want,
		)
	}

	if got, want := body[1].Name,
		"Roztocze Trail"; got != want {
		t.Errorf(
			"name = %q, want %q",
			got,
			want,
		)
	}
}

func TestWebHandlerAddVisit(
	t *testing.T,
) {
	var receivedHikeID int64
	var received CreateVisitParams

	service := &fakeHikeService{
		addVisitFunc: func(
			ctx context.Context,
			hikeID int64,
			params CreateVisitParams,
		) (Visit, error) {
			receivedHikeID = hikeID
			received = params

			return Visit{
				ID:     7,
				HikeID: hikeID,
			}, nil
		},
	}

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	form := url.Values{
		"visited_at": {
			"2026-08-17",
		},

		"weather": {
			"Sunny",
		},

		"notes": {
			"Very quiet route",
		},

		"rating": {
			"5",
		},
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/hikes/42/visits",
		strings.NewReader(
			form.Encode(),
		),
	)

	request.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	response := httptest.NewRecorder()

	router.ServeHTTP(
		response,
		request,
	)

	if got, want :=
		response.Code,
		http.StatusSeeOther; got != want {
		t.Fatalf(
			"status = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		receivedHikeID,
		int64(42); got != want {
		t.Errorf(
			"hike ID = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		received.Rating,
		new(5); got == nil ||
		*got != *want {

		t.Errorf(
			"rating = %v, want 5",
			got,
		)
	}

	if got, want :=
		response.Header().Get(
			"Location",
		),
		"/hikes/42"; got != want {
		t.Errorf(
			"Location = %q, want %q",
			got,
			want,
		)
	}
}

func TestWebHandlerImportsGPX(
	t *testing.T,
) {
	var received ImportTrackParams

	service := &fakeHikeService{
		importTrackFunc: func(
			ctx context.Context,
			params ImportTrackParams,
		) (Hike, Track, error) {
			received = params

			return Hike{
					ID:   42,
					Name: params.Hike.Name,
				},
				Track{
					ID:     7,
					HikeID: 42,
				},
				nil
		},
	}

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	gpxDocument := `
<gpx
	xmlns="http://www.topografix.com/GPX/1/1"
	version="1.1"
	creator="TrailLog test"
>
	<trk>
		<name>Forest Loop</name>

		<trkseg>
			<trkpt
				lat="51.1000"
				lon="22.1000"
			>
				<ele>200</ele>
			</trkpt>

			<trkpt
				lat="51.1010"
				lon="22.1020"
			>
				<ele>205</ele>
			</trkpt>
		</trkseg>
	</trk>
</gpx>
`

	var body bytes.Buffer

	writer := multipart.NewWriter(
		&body,
	)

	if err := writer.WriteField(
		"location",
		"Lublin",
	); err != nil {
		t.Fatalf(
			"write location: %v",
			err,
		)
	}

	if err := writer.WriteField(
		"status",
		"planned",
	); err != nil {
		t.Fatalf(
			"write status: %v",
			err,
		)
	}

	filePart, err := writer.CreateFormFile(
		"file",
		"forest-loop.gpx",
	)
	if err != nil {
		t.Fatalf(
			"create file part: %v",
			err,
		)
	}

	if _, err := filePart.Write(
		[]byte(gpxDocument),
	); err != nil {
		t.Fatalf(
			"write GPX: %v",
			err,
		)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf(
			"close multipart writer: %v",
			err,
		)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/hikes/import",
		&body,
	)

	request.Header.Set(
		"Content-Type",
		writer.FormDataContentType(),
	)

	response := httptest.NewRecorder()

	router.ServeHTTP(
		response,
		request,
	)

	if got, want :=
		response.Code,
		http.StatusSeeOther; got != want {
		t.Fatalf(
			"status = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		received.Hike.Name,
		"Forest Loop"; got != want {
		t.Errorf(
			"name = %q, want %q",
			got,
			want,
		)
	}

	if got, want :=
		received.Hike.Location,
		"Lublin"; got != want {
		t.Errorf(
			"location = %q, want %q",
			got,
			want,
		)
	}

	if got, want :=
		len(received.Points),
		2; got != want {
		t.Errorf(
			"points = %d, want %d",
			got,
			want,
		)
	}

	if received.DistanceKM <= 0 {
		t.Errorf(
			"distance = %f, want positive",
			received.DistanceKM,
		)
	}

	if got, want :=
		response.Header().Get(
			"Location",
		),
		"/hikes/42"; got != want {
		t.Errorf(
			"Location = %q, want %q",
			got,
			want,
		)
	}
}

func TestWebHandlerTrackReturnsHTMXFragment(
	t *testing.T,
) {
	service := &fakeHikeService{
		findByIDFunc: func(
			context.Context,
			int64,
		) (Hike, error) {
			return Hike{
				ID:   42,
				Name: "Forest Loop",
			}, nil
		},

		trackVisualizationFunc: func(
			context.Context,
			int64,
		) (*TrackVisualization, error) {
			return &TrackVisualization{
				Track: Track{
					ID:             7,
					HikeID:         42,
					SourceFilename: "forest.gpx",
					PointCount:     5000,
					DistanceKM:     8.4,
					ElevationGainM: 140,
				},

				Map: TrackMapData{
					Segments: []TrackMapSegment{
						{
							Points: []TrackMapPoint{
								{
									Latitude:  51.1,
									Longitude: 22.1,
								},
								{
									Latitude:  51.2,
									Longitude: 22.2,
								},
							},
						},
					},
				},

				Elevation: &ElevationProfile{
					DistanceKM: 8.4,

					MinElevationM: 180,

					MaxElevationM: 250,

					Segments: []ElevationProfileSegment{
						{
							Samples: []ElevationSample{
								{
									DistanceKM: 0,

									ElevationM: 180,
								},
								{
									DistanceKM: 8.4,

									ElevationM: 250,
								},
							},
						},
					},
				},
			}, nil
		},
	}

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/hikes/42/track",
		nil,
	)

	request.Header.Set(
		"HX-Request",
		"true",
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

	body := response.Body.String()

	if strings.Contains(
		body,
		"<!doctype html>",
	) {
		t.Error(
			"HTMX response contains a full document",
		)
	}

	if !strings.Contains(
		body,
		`data-track-map`,
	) {
		t.Error(
			"response does not contain map element",
		)
	}

	if !strings.Contains(
		body,
		`&#34;points&#34;`,
	) {
		t.Error(
			"response map data does not contain track points",
		)
	}

	if !strings.Contains(
		body,
		"8.4 km",
	) {
		t.Error(
			"response does not contain distance",
		)
	}

	if !strings.Contains(
		body,
		"Elevation profile",
	) {
		t.Error(
			"response does not contain elevation profile",
		)
	}
}

func TestWebHandlerReplacesTrackWithHTMX(
	t *testing.T,
) {
	var received SetTrackParams

	service := &fakeHikeService{
		findByIDFunc: func(
			context.Context,
			int64,
		) (Hike, error) {
			return Hike{
				ID: 42,

				Name: "Forest Loop",
			}, nil
		},

		setTrackFunc: func(
			ctx context.Context,
			hikeID int64,
			params SetTrackParams,
		) (Track, error) {
			received = params

			return Track{
				ID: 8,

				HikeID: hikeID,

				DistanceKM: params.DistanceKM,
			}, nil
		},

		trackVisualizationFunc: func(
			context.Context,
			int64,
		) (*TrackVisualization, error) {
			return &TrackVisualization{
				Track: Track{
					ID: 8,

					HikeID: 42,

					SourceFilename: "new.gpx",

					PointCount: 2,

					DistanceKM: 8,
				},

				Map: TrackMapData{
					Segments: []TrackMapSegment{
						{
							Points: []TrackMapPoint{
								{
									Latitude: 51,

									Longitude: 22,
								},
								{
									Latitude: 51.01,

									Longitude: 22.01,
								},
							},
						},
					},
				},
			}, nil
		},
	}

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	gpxDocument := `
<gpx
	xmlns="http://www.topografix.com/GPX/1/1"
	version="1.1"
	creator="test"
>
	<trk>
		<trkseg>
			<trkpt lat="51" lon="22"/>
			<trkpt lat="51.01" lon="22.01"/>
		</trkseg>
	</trk>
</gpx>
`

	var body bytes.Buffer

	writer := multipart.NewWriter(
		&body,
	)

	file, err := writer.CreateFormFile(
		"file",
		"new.gpx",
	)
	if err != nil {
		t.Fatalf(
			"CreateFormFile(): %v",
			err,
		)
	}

	if _, err := file.Write(
		[]byte(gpxDocument),
	); err != nil {
		t.Fatalf(
			"write GPX: %v",
			err,
		)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf(
			"close multipart writer: %v",
			err,
		)
	}

	request := httptest.NewRequest(
		http.MethodPut,
		"/hikes/42/track",
		&body,
	)

	request.Header.Set(
		"Content-Type",
		writer.FormDataContentType(),
	)

	request.Header.Set(
		"HX-Request",
		"true",
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

	if got, want :=
		received.SourceFilename,
		"new.gpx"; got != want {
		t.Errorf(
			"filename = %q, want %q",
			got,
			want,
		)
	}

	if got, want :=
		len(received.Points),
		2; got != want {
		t.Errorf(
			"points = %d, want %d",
			got,
			want,
		)
	}

	bodyText := response.Body.String()

	if !strings.Contains(
		bodyText,
		`id="hike-track-panel"`,
	) {
		t.Error(
			"response does not contain refreshed Track panel",
		)
	}

	if !strings.Contains(
		bodyText,
		"new.gpx",
	) {
		t.Error(
			"response does not contain replacement filename",
		)
	}
}

func TestWebHandlerRemovesTrackWithHTMX(
	t *testing.T,
) {
	var removedID int64

	service := &fakeHikeService{
		removeTrackFunc: func(
			ctx context.Context,
			hikeID int64,
		) error {
			removedID = hikeID

			return nil
		},

		findByIDFunc: func(
			context.Context,
			int64,
		) (Hike, error) {
			return Hike{
				ID: 42,

				Name: "Forest Loop",
			}, nil
		},

		trackVisualizationFunc: func(
			context.Context,
			int64,
		) (*TrackVisualization, error) {
			return nil, nil
		},
	}

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	request := httptest.NewRequest(
		http.MethodDelete,
		"/hikes/42/track",
		nil,
	)

	request.Header.Set(
		"HX-Request",
		"true",
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

	if got, want :=
		removedID,
		int64(42); got != want {
		t.Errorf(
			"removed ID = %d, want %d",
			got,
			want,
		)
	}

	body := response.Body.String()

	if !strings.Contains(
		body,
		"No GPX route",
	) {
		t.Error(
			"response does not contain empty route state",
		)
	}

	if !strings.Contains(
		body,
		"Attach GPX",
	) {
		t.Error(
			"response does not contain Attach GPX form",
		)
	}
}

// --- Helpers ---

type fakeHikeService struct {
	createFunc func(
		context.Context,
		CreateParams,
	) (Hike, error)

	findByIDFunc func(
		context.Context,
		int64,
	) (Hike, error)

	listFunc func(
		context.Context,
	) ([]Hike, error)

	searchFunc func(
		context.Context,
		ListParams,
	) (ListResult, error)

	updateFunc func(
		context.Context,
		int64,
		UpdateParams,
	) (Hike, error)

	deleteFunc func(
		context.Context,
		int64,
	) error

	addVisitFunc func(
		context.Context,
		int64,
		CreateVisitParams,
	) (Visit, error)

	listVisitFunc func(
		context.Context,
		int64,
	) ([]Visit, error)

	setTagsFunc func(
		context.Context,
		int64,
		[]string,
	) ([]Tag, error)

	listTagsFunc func(
		context.Context,
	) ([]Tag, error)

	listHikeTagsFunc func(
		context.Context,
		int64,
	) ([]Tag, error)

	importTrackFunc func(
		context.Context,
		ImportTrackParams,
	) (Hike, Track, error)

	trackVisualizationFunc func(
		context.Context,
		int64,
	) (*TrackVisualization, error)

	setTrackFunc func(
		context.Context,
		int64,
		SetTrackParams,
	) (Track, error)

	removeTrackFunc func(
		context.Context,
		int64,
	) error
}

func (f *fakeHikeService) Create(
	ctx context.Context,
	params CreateParams,
) (Hike, error) {
	if f.createFunc == nil {
		panic("unexpected Create call")
	}

	return f.createFunc(
		ctx,
		params,
	)
}

func (f *fakeHikeService) FindByID(
	ctx context.Context,
	id int64,
) (Hike, error) {
	if f.findByIDFunc == nil {
		panic("unexpected FindByID call")
	}

	return f.findByIDFunc(
		ctx,
		id,
	)
}

func (f *fakeHikeService) List(
	ctx context.Context,
) ([]Hike, error) {
	if f.listFunc == nil {
		panic("unexpected List call")
	}

	return f.listFunc(ctx)
}

func (f *fakeHikeService) Search(
	ctx context.Context,
	params ListParams,
) (ListResult, error) {
	if f.searchFunc == nil {
		panic("unexpected Search call")
	}

	return f.searchFunc(ctx, params)
}

func (f *fakeHikeService) Update(
	ctx context.Context,
	id int64,
	params UpdateParams,
) (Hike, error) {
	if f.updateFunc == nil {
		panic("unexpected Update call")
	}

	return f.updateFunc(
		ctx,
		id,
		params,
	)
}

func (f *fakeHikeService) Delete(
	ctx context.Context,
	id int64,
) error {
	if f.deleteFunc == nil {
		panic("unexpected Delete call")
	}

	return f.deleteFunc(
		ctx,
		id,
	)
}

func (f *fakeHikeService) AddVisit(
	ctx context.Context,
	hikeID int64,
	params CreateVisitParams,
) (Visit, error) {
	if f.addVisitFunc == nil {
		panic("unexpected AddVisit call")
	}

	return f.addVisitFunc(
		ctx,
		hikeID,
		params,
	)
}

func (f *fakeHikeService) ListVisits(
	ctx context.Context,
	hikeID int64,
) ([]Visit, error) {
	if f.listVisitFunc == nil {
		panic("unexpected ListVisits call")
	}

	return f.listVisitFunc(
		ctx,
		hikeID,
	)
}

func (f *fakeHikeService) SetTags(
	ctx context.Context,
	id int64,
	tags []string,
) ([]Tag, error) {
	if f.setTagsFunc == nil {
		panic("unexpected SetTags call")
	}

	return f.setTagsFunc(
		ctx,
		id,
		tags,
	)
}

func (f *fakeHikeService) ListTags(
	ctx context.Context,
) ([]Tag, error) {
	if f.listTagsFunc == nil {
		panic("unexpected ListTags call")
	}

	return f.listTagsFunc(ctx)
}

func (f *fakeHikeService) ListHikeTags(
	ctx context.Context,
	id int64,
) ([]Tag, error) {
	if f.listHikeTagsFunc == nil {
		panic("unexpected ListHikeTags call")
	}

	return f.listHikeTagsFunc(
		ctx,
		id,
	)
}

func (f *fakeHikeService) ImportTrack(
	ctx context.Context,
	params ImportTrackParams,
) (Hike, Track, error) {
	if f.importTrackFunc == nil {
		panic("unexpected ImportTrack call")
	}

	return f.importTrackFunc(
		ctx,
		params,
	)
}

func (f *fakeHikeService) TrackVisualization(
	ctx context.Context,
	hikeID int64,
) (*TrackVisualization, error) {
	if f.trackVisualizationFunc == nil {
		panic(
			"unexpected TrackVisualization call",
		)
	}

	return f.trackVisualizationFunc(
		ctx,
		hikeID,
	)
}

func (f *fakeHikeService) SetTrack(
	ctx context.Context,
	hikeID int64,
	params SetTrackParams,
) (Track, error) {
	if f.setTrackFunc == nil {
		panic(
			"unexpected SetTrack call",
		)
	}

	return f.setTrackFunc(
		ctx,
		hikeID,
		params,
	)
}

func (f *fakeHikeService) RemoveTrack(
	ctx context.Context,
	hikeID int64,
) error {
	if f.removeTrackFunc == nil {
		panic(
			"unexpected RemoveTrack call",
		)
	}

	return f.removeTrackFunc(
		ctx,
		hikeID,
	)
}

func newHandlerTestRouter(t *testing.T, service hikeService) http.Handler {
	t.Helper()

	gin.SetMode(
		gin.TestMode,
	)

	router := gin.New()

	handler := NewHandler(service)

	handler.RegisterRoutes(router)

	return router
}

func newWebHandlerTestRouter(t *testing.T, service hikeService) http.Handler {
	t.Helper()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.SetFuncMap(webui.FuncMap())
	router.LoadHTMLFS(webui.Templates(), "**/*.html")

	handler := NewWebHandler(service, testWebHandlerOptions())
	handler.RegisterRoutes(router)

	return router
}

func testWebHandlerOptions() WebHandlerOptions {
	return WebHandlerOptions{
		GPX: GPXUploadLimits{
			MaxUploadBytes: 20 << 20,

			MaxTrackPoints: 200_000,

			MaxTextFieldBytes: 64 << 10,
		},

		Map: MapOptions{
			TileURL: "https://tile.openstreetmap.org/{z}/{x}/{y}.png",

			Attribution: "OpenStreetMap contributors",
		},
	}
}
