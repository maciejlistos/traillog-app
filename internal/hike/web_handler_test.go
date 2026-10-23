package hike

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestWebHandlerListPage(t *testing.T) {
	service := &fakeHikeService{
		searchFunc: func(
			context.Context,
			ListParams,
		) (ListResult, error) {
			distance := 8.4

			return ListResult{
				Hikes: []Hike{
					{
						ID:          1,
						Name:        "Forest Loop",
						Location:    "Lublin",
						Description: "Quiet forest walk",
						DistanceKM:  &distance,
						Status:      StatusPlanned,
						Tags: []Tag{
							{Name: "forest"},
							{Name: "family"},
						},
					},
				},
				Total:      1,
				Page:       1,
				TotalPages: 1,
			}, nil
		},
		listTagsFunc: func(context.Context) ([]Tag, error) {
			return []Tag{}, nil
		},
	}

	router := newWebHandlerTestRouter(t, service)

	request := httptest.NewRequest(http.MethodGet, "/hikes", nil)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	body := recorder.Body.String()

	if !strings.Contains(body, "Forest Loop") {
		t.Error("response does not contain hike name")
	}

	if !strings.Contains(body, "8.4 km") {
		t.Error("response does not contain distance")
	}

	if !strings.Contains(body, `href="/hikes/1`) {
		t.Error("response does not contain hike link")
	}

	if !strings.Contains(body, "forest") || !strings.Contains(body, "family") {
		t.Error("response does not contain hike tags")
	}
}

func TestWebHandlerListPageShowsEmptyState(
	t *testing.T,
) {
	service := &fakeHikeService{
		searchFunc: func(
			context.Context,
			ListParams,
		) (ListResult, error) {
			return ListResult{
				Hikes:      []Hike{},
				Total:      0,
				Page:       1,
				TotalPages: 1,
			}, nil
		},
		listTagsFunc: func(context.Context) ([]Tag, error) {
			return []Tag{}, nil
		},
	}

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/hikes",
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

	if !strings.Contains(
		recorder.Body.String(),
		"No hikes yet",
	) {
		t.Error(
			"response does not contain empty state",
		)
	}
}

func TestWebHandlerShowPageRendersVisitHistory(t *testing.T) {
	distance := 8.4

	service := &fakeHikeService{
		findByIDFunc: func(context.Context, int64) (Hike, error) {
			return Hike{
				ID:         42,
				Name:       "Forest Loop",
				Location:   "Lublin",
				DistanceKM: &distance,
				Status:     StatusPlanned,
			}, nil
		},
		listVisitFunc: func(context.Context, int64) ([]Visit, error) {
			return []Visit{}, nil
		},
		listHikeTagsFunc: func(context.Context, int64) ([]Tag, error) {
			return []Tag{}, nil
		},
	}

	router := newWebHandlerTestRouter(t, service)
	request := httptest.NewRequest(http.MethodGet, "/hikes/42", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	body := recorder.Body.String()

	if !strings.Contains(body, "Visit history") {
		t.Error("response does not contain visit history")
	}

	if !strings.Contains(body, "8.4 km") {
		t.Error("response does not contain hike distance")
	}

	if !strings.Contains(body, "No visits recorded yet.") {
		t.Error("response does not contain empty visit history")
	}

	if !strings.Contains(body, `hx-get="/hikes/42/track"`) {
		t.Error("response does not load the track panel")
	}

	if !strings.Contains(body, `src="/static/js/htmx.min.js"`) {
		t.Error("response does not load HTMX")
	}
}

func TestWebHandlerCreateRedirectsToHike(
	t *testing.T,
) {
	var received CreateParams

	service := &fakeHikeService{
		createFunc: func(
			ctx context.Context,
			params CreateParams,
		) (Hike, error) {
			received = params

			return Hike{
				ID: 42,
			}, nil
		},
	}

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	form := url.Values{
		"name": {
			"Forest Loop",
		},
		"location": {
			"Lublin",
		},
		"description": {
			"Quiet forest walk",
		},
		"distance_km": {
			"8.4",
		},
		"status": {
			"planned",
		},
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/hikes",
		strings.NewReader(
			form.Encode(),
		),
	)

	request.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if got, want := recorder.Code,
		http.StatusSeeOther; got != want {
		t.Fatalf(
			"status = %d, want %d",
			got,
			want,
		)
	}

	if got, want := recorder.Header().Get(
		"Location",
	), "/hikes/42"; got != want {
		t.Errorf(
			"Location = %q, want %q",
			got,
			want,
		)
	}

	if got, want := received.Name,
		"Forest Loop"; got != want {
		t.Errorf(
			"name = %q, want %q",
			got,
			want,
		)
	}

	if received.DistanceKM == nil {
		t.Fatal(
			"distance is nil",
		)
	}

	if got, want := *received.DistanceKM,
		8.4; got != want {
		t.Errorf(
			"distance = %f, want %f",
			got,
			want,
		)
	}
}

func TestWebHandlerCreateRejectsInvalidDistance(
	t *testing.T,
) {
	service := &fakeHikeService{
		createFunc: func(
			context.Context,
			CreateParams,
		) (Hike, error) {
			t.Fatal(
				"service.Create called for invalid distance",
			)

			return Hike{}, nil
		},
	}

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	form := url.Values{
		"name": {
			"Forest Loop",
		},
		"location": {
			"Lublin",
		},
		"distance_km": {
			"banana",
		},
		"status": {
			"planned",
		},
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/hikes",
		strings.NewReader(
			form.Encode(),
		),
	)

	request.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
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

	body := recorder.Body.String()

	if !strings.Contains(
		body,
		"Distance must be a number",
	) {
		t.Error(
			"response does not contain distance error",
		)
	}

	if !strings.Contains(
		body,
		`value="banana"`,
	) {
		t.Error(
			"form did not preserve invalid distance value",
		)
	}
}

func TestWebHandlerCreateRendersValidationErrors(
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

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	form := url.Values{
		"name": {
			"",
		},
		"location": {
			"Lublin",
		},
		"status": {
			"planned",
		},
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/hikes",
		strings.NewReader(
			form.Encode(),
		),
	)

	request.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
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

	body := recorder.Body.String()

	if !strings.Contains(
		body,
		"Name is required",
	) {
		t.Error(
			"validation error was not rendered",
		)
	}

	if !strings.Contains(
		body,
		`value="Lublin"`,
	) {
		t.Error(
			"location field was not preserved",
		)
	}
}

func TestWebHandlerEscapesHikeContent(
	t *testing.T,
) {
	service := &fakeHikeService{
		searchFunc: func(
			context.Context,
			ListParams,
		) (ListResult, error) {
			return ListResult{
				Hikes: []Hike{
					{
						ID:       1,
						Name:     `<script>alert("boom")</script>`,
						Location: "Lublin",
						Status:   StatusPlanned,
					},
				},
				Total:    1,
				Page:     1,
				PageSize: 1,
			}, nil
		},
		listTagsFunc: func(context.Context) ([]Tag, error) {
			return []Tag{}, nil
		},
	}

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/hikes",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	body := recorder.Body.String()

	if strings.Contains(
		body,
		`<script>alert`,
	) {
		t.Error(
			"unescaped script tag was rendered",
		)
	}

	if !strings.Contains(
		body,
		"&lt;script&gt;",
	) {
		t.Error(
			"escaped content was not rendered",
		)
	}
}

func TestWebHandlerNewPageReturnsFullDocument(t *testing.T) {
	service := &fakeHikeService{}

	router := newWebHandlerTestRouter(t, service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/hikes/new",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	body := recorder.Body.String()

	if !strings.Contains(body, "<!doctype html>") {
		t.Errorf("response 	is not a full HTML document")
	}
}

func TestWebHandlerNewPageReturnsHTMLFragment(t *testing.T) {
	service := &fakeHikeService{}

	router := newWebHandlerTestRouter(t, service)

	request := httptest.NewRequest(http.MethodGet, "/hikes/new", nil)
	request.Header.Set("HX-Request", "true")

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	body := recorder.Body.String()

	if strings.Contains(body, "<!doctype html>") {
		t.Errorf("HTMX response unexpectedly contains full document")
	}

	if !strings.Contains(body, `id="new-hike-form-panel"`) {
		t.Errorf("response does not contain form fragment")
	}

	if !strings.Contains(body, `hx-post="/hikes"`) {
		t.Errorf("HTMX form is not enhanced")
	}

	if got := recorder.Header().Get("Vary"); !strings.Contains(got, "HX-Request") {
		t.Errorf("Vary = %q, want HX-Request", got)
	}
}

func TestWebHandlerCreateHTMXRendersValidationFragment(
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

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	form := url.Values{
		"name": {
			"",
		},
		"location": {
			"Lublin",
		},
		"status": {
			"planned",
		},
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/hikes",
		strings.NewReader(
			form.Encode(),
		),
	)

	request.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	request.Header.Set(
		"HX-Request",
		"true",
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

	body := recorder.Body.String()

	if strings.Contains(
		body,
		"<!doctype html>",
	) {
		t.Error(
			"validation response contains full document",
		)
	}

	if !strings.Contains(
		body,
		"Name is required",
	) {
		t.Error(
			"validation message not rendered",
		)
	}

	if !strings.Contains(
		body,
		`value="Lublin"`,
	) {
		t.Error(
			"form values were not preserved",
		)
	}
}

func TestWebHandlerCreateHTMXReturnsNewHikeCard(t *testing.T) {
	distance := 7.2

	service := &fakeHikeService{
		createFunc: func(context.Context, CreateParams) (Hike, error) {
			return Hike{
				ID:          42,
				Name:        "Roztocze Loop",
				Location:    "Roztocze",
				Description: "Forest route",
				DistanceKM:  &distance,
				Status:      StatusPlanned,
			}, nil
		},
	}

	router := newWebHandlerTestRouter(t, service)

	form := url.Values{
		"name": {
			"Roztocze Loop",
		},
		"location": {
			"Roztocze",
		},
		"description": {
			"Forest route",
		},
		"distance_km": {
			"7.2",
		},
		"status": {
			"planned",
		},
	}

	request := httptest.NewRequest(http.MethodPost, "/hikes", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("HX-Request", "true")

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	body := recorder.Body.String()

	if !strings.Contains(body, "Hike saved") {
		t.Errorf("response does not contains success message")
	}

	if !strings.Contains(body, `hx-swap-oob="afterbegin:#hike-list"`) {
		t.Error("response does not contain hike-list OOB swap")
	}

	if !strings.Contains(body, `id="hike-42"`) {
		t.Error("response does not contain new hike card")
	}

	if !strings.Contains(body, "Roztocze Loop") {
		t.Error("response does not contain hike name")
	}

	if recorder.Header().Get("Location") != "" {
		t.Error("HTMX request unexpectedly received redirect")
	}
}

func TestWebHandlerEditReturnsHTMXFragment(
	t *testing.T,
) {
	distance := 8.4

	service := &fakeHikeService{
		findByIDFunc: func(
			context.Context,
			int64,
		) (Hike, error) {
			return Hike{
				ID:          42,
				Name:        "Forest Loop",
				Location:    "Lublin",
				Description: "Quiet walk",
				DistanceKM:  &distance,
				Status:      StatusPlanned,
			}, nil
		},
	}

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/hikes/42/edit",
		nil,
	)

	request.Header.Set(
		"HX-Request",
		"true",
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

	body := recorder.Body.String()

	if strings.Contains(
		body,
		"<!doctype html>",
	) {
		t.Error(
			"HTMX edit response contains full document",
		)
	}

	if !strings.Contains(
		body,
		`id="hike-detail"`,
	) {
		t.Error(
			"edit fragment does not target hike-detail",
		)
	}

	if !strings.Contains(
		body,
		`hx-put="/hikes/42"`,
	) {
		t.Error(
			"edit form does not contain hx-put",
		)
	}

	if !strings.Contains(
		body,
		`value="Forest Loop"`,
	) {
		t.Error(
			"edit form does not contain current name",
		)
	}

	if !strings.Contains(
		body,
		`value="8.4"`,
	) {
		t.Error(
			"edit form does not contain current distance",
		)
	}
}

func TestWebHandlerUpdateHTMXReturnsDetailFragment(
	t *testing.T,
) {
	var receivedID int64
	var received UpdateParams

	service := &fakeHikeService{
		updateFunc: func(
			ctx context.Context,
			id int64,
			params UpdateParams,
		) (Hike, error) {
			receivedID = id
			received = params

			return Hike{
				ID:          id,
				Name:        params.Name,
				Location:    params.Location,
				Description: params.Description,
				DistanceKM:  params.DistanceKM,
				Status:      params.Status,
			}, nil
		},
	}

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	form := url.Values{
		"name": {
			"Updated Forest Loop",
		},
		"location": {
			"Lublin",
		},
		"description": {
			"Updated notes",
		},
		"distance_km": {
			"9.1",
		},
		"status": {
			"visited",
		},
	}

	request := httptest.NewRequest(
		http.MethodPut,
		"/hikes/42",
		strings.NewReader(
			form.Encode(),
		),
	)

	request.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	request.Header.Set(
		"HX-Request",
		"true",
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

	if got, want := receivedID,
		int64(42); got != want {
		t.Errorf(
			"ID = %d, want %d",
			got,
			want,
		)
	}

	if got, want := received.Name,
		"Updated Forest Loop"; got != want {
		t.Errorf(
			"name = %q, want %q",
			got,
			want,
		)
	}

	if got, want := received.Status,
		StatusVisited; got != want {
		t.Errorf(
			"status = %q, want %q",
			got,
			want,
		)
	}

	body := recorder.Body.String()

	if !strings.Contains(
		body,
		`id="hike-detail"`,
	) {
		t.Error(
			"response does not contain detail fragment",
		)
	}

	if !strings.Contains(
		body,
		"Updated Forest Loop",
	) {
		t.Error(
			"updated name was not rendered",
		)
	}

	if strings.Contains(
		body,
		"<!doctype html>",
	) {
		t.Error(
			"HTMX response contains full page",
		)
	}
}

func TestWebHandlerUpdateRedirectsWithoutHTMX(
	t *testing.T,
) {
	service := &fakeHikeService{
		updateFunc: func(
			ctx context.Context,
			id int64,
			params UpdateParams,
		) (Hike, error) {
			return Hike{
				ID:     id,
				Name:   params.Name,
				Status: params.Status,
			}, nil
		},
	}

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	form := url.Values{
		"name": {
			"Forest Loop",
		},
		"location": {
			"Lublin",
		},
		"status": {
			"visited",
		},
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/hikes/42",
		strings.NewReader(
			form.Encode(),
		),
	)

	request.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if got, want := recorder.Code,
		http.StatusSeeOther; got != want {
		t.Fatalf(
			"status = %d, want %d",
			got,
			want,
		)
	}

	if got, want := recorder.Header().Get(
		"Location",
	), "/hikes/42"; got != want {
		t.Errorf(
			"Location = %q, want %q",
			got,
			want,
		)
	}
}

func TestWebHandlerDeleteHTMX(
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

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	request := httptest.NewRequest(
		http.MethodDelete,
		"/hikes/42",
		nil,
	)

	request.Header.Set(
		"HX-Request",
		"true",
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
			"body = %q, want empty",
			recorder.Body.String(),
		)
	}
}

func TestWebHandlerSearchPage(t *testing.T) {
	var received ListParams

	service := &fakeHikeService{
		searchFunc: func(ctx context.Context, params ListParams) (ListResult, error) {
			received = params

			return ListResult{
				Hikes: []Hike{
					{
						ID:       1,
						Name:     "Forest Loop",
						Location: "Lublin",
						Status:   StatusVisited,
					},
				},
				Total:      1,
				Page:       2,
				PageSize:   12,
				TotalPages: 3,
			}, nil
		},
		listTagsFunc: func(context.Context) ([]Tag, error) {
			return []Tag{}, nil
		},
	}

	router := newWebHandlerTestRouter(t, service)

	request := httptest.NewRequest(http.MethodGet, "/hikes?q=forest&status=visited&sort=name&page=2", nil)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("stataus = %d, want %d", got, want)
	}

	if got, want := received.Query, "forest"; got != want {
		t.Errorf("query = %q, want %q", got, want)
	}

	if received.Status == nil {
		t.Fatalf("status filter is nil")
	}

	if got, want := *received.Status, StatusVisited; got != want {
		t.Errorf("status = %q, want %q", got, want)
	}

	if got, want := received.Sort, SortName; got != want {
		t.Errorf("sort = %q, want %q", got, want)
	}

	if got, want := received.Page, 2; got != want {
		t.Errorf("page = %d, want %d", got, want)
	}

	body := recorder.Body.String()

	if !strings.Contains(body, "<!doctype html>") {
		t.Error("normal request did not render full document")
	}
}

func TestWebHandlerSearchReturnsHTMXFragment(
	t *testing.T,
) {
	service := &fakeHikeService{
		searchFunc: func(
			context.Context,
			ListParams,
		) (ListResult, error) {
			return ListResult{
				Hikes: []Hike{
					{
						ID:       1,
						Name:     "Forest Loop",
						Location: "Lublin",
						Status:   StatusPlanned,
					},
				},
				Total:      1,
				Page:       1,
				PageSize:   12,
				TotalPages: 1,
			}, nil
		},
	}

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/hikes?q=forest",
		nil,
	)

	request.Header.Set(
		"HX-Request",
		"true",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if got, want :=
		recorder.Code,
		http.StatusOK; got != want {
		t.Fatalf(
			"status = %d, want %d",
			got,
			want,
		)
	}

	body := recorder.Body.String()

	if strings.Contains(
		body,
		"<!doctype html>",
	) {
		t.Error(
			"HTMX search returned full page",
		)
	}

	if !strings.Contains(
		body,
		`id="hike-results"`,
	) {
		t.Error(
			"response does not contain result fragment",
		)
	}

	if !strings.Contains(
		body,
		"Forest Loop",
	) {
		t.Error(
			"response does not contain matching hike",
		)
	}

	if got := recorder.Header().Get("Vary"); !strings.Contains(
		got,
		"HX-Request",
	) {

		t.Errorf(
			"Vary = %q, want HX-Request",
			got,
		)
	}
}

func TestWebHandlerSetTags(
	t *testing.T,
) {
	var received []string

	service := &fakeHikeService{
		setTagsFunc: func(
			ctx context.Context,
			hikeID int64,
			names []string,
		) ([]Tag, error) {
			received = names

			return []Tag{
				{
					ID:   1,
					Name: "forest",
				},
				{
					ID:   2,
					Name: "macro",
				},
			}, nil
		},
	}

	router := newWebHandlerTestRouter(
		t,
		service,
	)

	form := url.Values{
		"tags": {
			" Forest, macro ",
		},
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/hikes/42/tags",
		strings.NewReader(
			form.Encode(),
		),
	)

	request.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
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

	wantInput := []string{
		" Forest",
		" macro ",
	}

	if !slices.Equal(
		received,
		wantInput,
	) {
		t.Errorf(
			"handler tags = %v, want %v",
			received,
			wantInput,
		)
	}

	body := response.Body.String()

	if !strings.Contains(
		body,
		"#forest",
	) {
		t.Error(
			"response does not contain forest tag",
		)
	}

	if !strings.Contains(
		body,
		"#macro",
	) {
		t.Error(
			"response does not contain macro tag",
		)
	}
}
