package hike

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"go.opentelemetry.io/otel/codes"
)

type fakeHikeTransaction struct {
	markHikeVisitedFunc func(
		context.Context,
		int64,
	) error

	createVisitFunc func(
		context.Context,
		int64,
		CreateVisitParams,
	) (Visit, error)

	createHikeFunc func(
		context.Context,
		CreateParams,
	) (Hike, error)

	createTrackFunc func(
		context.Context,
		int64,
		ImportTrackParams,
	) (Track, error)

	updateHikeDistanceFunc func(
		context.Context,
		int64,
		*float64,
	) error

	deleteTrackByHikeIDFunc func(
		context.Context,
		int64,
	) (bool, error)
}

func (f *fakeHikeTransaction) MarkHikeVisited(
	ctx context.Context,
	hikeID int64,
) error {
	if f.markHikeVisitedFunc == nil {
		panic("unexpected MarkHikeVisited call")
	}

	return f.markHikeVisitedFunc(
		ctx,
		hikeID,
	)
}

func (f *fakeHikeTransaction) CreateVisit(
	ctx context.Context,
	hikeID int64,
	params CreateVisitParams,
) (Visit, error) {
	if f.createVisitFunc == nil {
		panic("unexpected CreateVisit call")
	}

	return f.createVisitFunc(
		ctx,
		hikeID,
		params,
	)
}

func (f *fakeHikeTransaction) CreateHike(
	ctx context.Context,
	params CreateParams,
) (Hike, error) {
	if f.createHikeFunc == nil {
		panic(
			"unexpected CreateHike call",
		)
	}

	return f.createHikeFunc(
		ctx,
		params,
	)
}

func (f *fakeHikeTransaction) CreateTrack(
	ctx context.Context,
	hikeID int64,
	params ImportTrackParams,
) (Track, error) {
	if f.createTrackFunc == nil {
		panic(
			"unexpected CreateTrack call",
		)
	}

	return f.createTrackFunc(
		ctx,
		hikeID,
		params,
	)
}

// ---- Tests ----

func TestServiceCreateValidation(t *testing.T) {
	tests := []struct {
		name        string
		params      CreateParams
		field       string
		wantMessage string
	}{
		{
			name: "missing name",
			params: CreateParams{
				Name:     "   ",
				Location: "Lublin",
				Status:   StatusPlanned,
			},
			field:       "name",
			wantMessage: "Name is required",
		},
		{
			name: "missing location",
			params: CreateParams{
				Name:     "Forest Loop",
				Location: "   ",
				Status:   StatusPlanned,
			},
			field:       "location",
			wantMessage: "Location is required",
		},
		{
			name: "negative distance",
			params: CreateParams{
				Name:       "Forest Loop",
				Location:   "Lublin",
				DistanceKM: new(-5.0),
				Status:     StatusPlanned,
			},
			field:       "distance_km",
			wantMessage: "Distance cannot be negative",
		},
		{
			name: "invalid status",
			params: CreateParams{
				Name:     "Forest Loop",
				Location: "Lublin",
				Status:   Status("finished"),
			},
			field:       "status",
			wantMessage: "Status must be planned or visited",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakeRepository{
				createFunc: func(
					context.Context,
					CreateParams,
				) (Hike, error) {
					t.Fatal("repository.Create was called for invalid input")

					return Hike{}, nil
				},
			}

			service := newTestService(repository)

			_, err := service.Create(
				context.Background(),
				test.params,
			)

			if err == nil {
				t.Fatal(
					"Create() returned nil error",
				)
			}

			var validationError *ValidationError

			if !errors.As(
				err,
				&validationError,
			) {
				t.Fatalf(
					"error = %T, want *ValidationError",
					err,
				)
			}

			if got := validationError.Field(
				test.field,
			); got != test.wantMessage {
				t.Errorf(
					"validation message for %q = %q, want %q",
					test.field,
					got,
					test.wantMessage,
				)
			}
		})
	}
}

func TestServiceCreateNormalizesInputAndDefaultsStatus(t *testing.T) {
	var received CreateParams

	repository := &fakeRepository{
		createFunc: func(
			ctx context.Context,
			params CreateParams,
		) (Hike, error) {
			received = params

			return Hike{
				ID:          1,
				Name:        params.Name,
				Location:    params.Location,
				Description: params.Description,
				DistanceKM:  params.DistanceKM,
				Status:      params.Status,
			}, nil
		},
	}

	service := newTestService(repository)

	created, err := service.Create(
		context.Background(),
		CreateParams{
			Name:        "  Forest Loop  ",
			Location:    "  Lublin  ",
			Description: "  Quiet walk  ",
		},
	)
	if err != nil {
		t.Fatalf(
			"Create() returned error: %v",
			err,
		)
	}

	if got, want := received.Name,
		"Forest Loop"; got != want {
		t.Errorf(
			"repository name = %q, want %q",
			got,
			want,
		)
	}

	if got, want := received.Location,
		"Lublin"; got != want {
		t.Errorf(
			"repository location = %q, want %q",
			got,
			want,
		)
	}

	if got, want := received.Description,
		"Quiet walk"; got != want {
		t.Errorf(
			"repository description = %q, want %q",
			got,
			want,
		)
	}

	if got, want := received.Status,
		StatusPlanned; got != want {
		t.Errorf(
			"repository status = %q, want %q",
			got,
			want,
		)
	}

	if got, want := created.ID,
		int64(1); got != want {
		t.Errorf(
			"created ID = %d, want %d",
			got,
			want,
		)
	}
}

func TestServiceCreatePreservesRepositoryError(t *testing.T) {
	repositoryError := errors.New(
		"database unavailable",
	)

	repository := &fakeRepository{
		createFunc: func(
			context.Context,
			CreateParams,
		) (Hike, error) {
			return Hike{}, repositoryError
		},
	}

	service := newTestService(repository)

	_, err := service.Create(
		context.Background(),
		CreateParams{
			Name:     "Forest Loop",
			Location: "Lublin",
			Status:   StatusPlanned,
		},
	)

	if !errors.Is(err, repositoryError) {
		t.Fatalf(
			"error = %v, want wrapped repository error",
			err,
		)
	}
}

func TestServiceFindByIDPreservesNotFound(t *testing.T) {
	repository := &fakeRepository{
		findByIDFunc: func(
			context.Context,
			int64,
		) (Hike, error) {
			return Hike{}, ErrNotFound
		},
	}

	service := newTestService(repository)

	_, err := service.FindByID(
		context.Background(),
		42,
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"error = %v, want ErrNotFound",
			err,
		)
	}
}

func TestServiceCreateRecordsSpan(
	t *testing.T,
) {
	provider, recorder := newRecordingTracerProvider(t)

	repository := &fakeRepository{
		createFunc: func(
			context.Context,
			CreateParams,
		) (Hike, error) {
			return Hike{
				ID: 1,
			}, nil
		},
	}

	service := NewService(
		repository,
		provider.Tracer(
			"traillog/test/service",
		),
	)

	_, err := service.Create(
		context.Background(),
		CreateParams{
			Name:     "Forest Loop",
			Location: "Lublin",
			Status:   StatusPlanned,
		},
	)
	if err != nil {
		t.Fatalf(
			"Create() returned error: %v",
			err,
		)
	}

	span := findEndedSpan(
		t,
		recorder,
		"hike.service.create",
	)

	if got, want :=
		span.Name(),
		"hike.service.create"; got != want {
		t.Errorf(
			"span name = %q, want %q",
			got,
			want,
		)
	}

	if got := stringSpanAttribute(
		span,
		"traillog.hike.status",
	); got != "planned" {
		t.Errorf(
			"status attribute = %q, want %q",
			got,
			"planned",
		)
	}
}

func TestServiceValidationDoesNotMarkSpanAsError(
	t *testing.T,
) {
	provider, recorder := newRecordingTracerProvider(t)

	repository := &fakeRepository{
		createFunc: func(
			context.Context,
			CreateParams,
		) (Hike, error) {
			t.Fatal(
				"repository called for invalid input",
			)

			return Hike{}, nil
		},
	}

	service := NewService(
		repository,
		provider.Tracer(
			"traillog/test/service",
		),
	)

	_, err := service.Create(
		context.Background(),
		CreateParams{
			Name:     "",
			Location: "Lublin",
			Status:   StatusPlanned,
		},
	)

	if err == nil {
		t.Fatal(
			"Create() returned nil error",
		)
	}

	span := findEndedSpan(
		t,
		recorder,
		"hike.service.create",
	)

	if got := span.Status().Code; got != codes.Unset {
		t.Errorf(
			"span status = %s, want UNSET",
			got,
		)
	}

	foundEvent := false

	for _, event := range span.Events() {
		if event.Name ==
			"validation failed" {

			foundEvent = true
			break
		}
	}

	if !foundEvent {
		t.Error(
			"validation event was not recorded",
		)
	}
}

func TestServiceSearchNormalizesParams(
	t *testing.T,
) {
	var received ListParams

	repository := &fakeRepository{
		searchFunc: func(
			ctx context.Context,
			params ListParams,
		) (ListResult, error) {
			received = params

			return ListResult{
				Total: 25,
			}, nil
		},
	}

	service := newTestService(
		repository,
	)

	result, err := service.Search(
		context.Background(),
		ListParams{
			Query: "  forest  ",
		},
	)
	if err != nil {
		t.Fatalf(
			"Search() returned error: %v",
			err,
		)
	}

	if got, want :=
		received.Query,
		"forest"; got != want {
		t.Errorf(
			"query = %q, want %q",
			got,
			want,
		)
	}

	if got, want :=
		received.Sort,
		SortNewest; got != want {
		t.Errorf(
			"sort = %q, want %q",
			got,
			want,
		)
	}

	if got, want :=
		received.Page,
		1; got != want {
		t.Errorf(
			"page = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		received.PageSize,
		12; got != want {
		t.Errorf(
			"page size = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		result.TotalPages,
		3; got != want {
		t.Errorf(
			"total pages = %d, want %d",
			got,
			want,
		)
	}
}

func TestServiceSearchRejectsInvalidSort(
	t *testing.T,
) {
	repository := &fakeRepository{
		searchFunc: func(
			context.Context,
			ListParams,
		) (ListResult, error) {
			t.Fatal(
				"repository.Search called for invalid sort",
			)

			return ListResult{}, nil
		},
	}

	service := newTestService(
		repository,
	)

	_, err := service.Search(
		context.Background(),
		ListParams{
			Sort: Sort(
				"DROP TABLE hikes",
			),
		},
	)

	var validationError *ValidationError

	if !errors.As(
		err,
		&validationError,
	) {
		t.Fatalf(
			"error = %v, want ValidationError",
			err,
		)
	}

	if !validationError.Has("sort") {
		t.Error(
			"sort validation error missing",
		)
	}
}

func TestServiceAddVisit(
	t *testing.T,
) {
	visitedAt := time.Date(
		2026,
		time.August,
		17,
		0,
		0,
		0,
		0,
		time.UTC,
	)

	rating := 5

	var operations []string

	tx := &fakeHikeTransaction{
		markHikeVisitedFunc: func(
			ctx context.Context,
			hikeID int64,
		) error {
			operations = append(
				operations,
				"mark-visited",
			)

			if got, want :=
				hikeID,
				int64(42); got != want {
				t.Errorf(
					"hike ID = %d, want %d",
					got,
					want,
				)
			}

			return nil
		},

		createVisitFunc: func(
			ctx context.Context,
			hikeID int64,
			params CreateVisitParams,
		) (Visit, error) {
			operations = append(
				operations,
				"create-visit",
			)

			return Visit{
				ID:        7,
				HikeID:    hikeID,
				VisitedAt: params.VisitedAt,
				Weather:   params.Weather,
				Notes:     params.Notes,
				Rating:    params.Rating,
			}, nil
		},
	}

	repository := &fakeRepository{
		withinTransactionFunc: func(
			ctx context.Context,
			fn func(hikeTransaction) error,
		) error {
			return fn(tx)
		},
	}

	service := newTestService(
		repository,
	)

	created, err := service.AddVisit(
		context.Background(),
		42,
		CreateVisitParams{
			VisitedAt: visitedAt,
			Weather:   "  Sunny  ",
			Notes:     "  Quiet trail  ",
			Rating:    &rating,
		},
	)
	if err != nil {
		t.Fatalf(
			"AddVisit() returned error: %v",
			err,
		)
	}

	if got, want :=
		operations,
		[]string{
			"mark-visited",
			"create-visit",
		}; !slices.Equal(got, want) {
		t.Errorf(
			"operations = %v, want %v",
			got,
			want,
		)
	}

	if got, want :=
		created.ID,
		int64(7); got != want {
		t.Errorf(
			"visit ID = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		created.Weather,
		"Sunny"; got != want {
		t.Errorf(
			"weather = %q, want %q",
			got,
			want,
		)
	}
}

func TestServiceAddVisitValidation(
	t *testing.T,
) {
	tests := []struct {
		name   string
		params CreateVisitParams
		field  string
	}{
		{
			name: "missing visit date",

			params: CreateVisitParams{},

			field: "visited_at",
		},

		{
			name: "rating too low",

			params: CreateVisitParams{
				VisitedAt: time.Now(),
				Rating:    new(0),
			},

			field: "rating",
		},

		{
			name: "rating too high",

			params: CreateVisitParams{
				VisitedAt: time.Now(),
				Rating:    new(6),
			},

			field: "rating",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				repository := &fakeRepository{
					withinTransactionFunc: func(
						context.Context,
						func(hikeTransaction) error,
					) error {
						t.Fatal(
							"transaction started for invalid input",
						)

						return nil
					},
				}

				service := newTestService(
					repository,
				)

				_, err := service.AddVisit(
					context.Background(),
					42,
					test.params,
				)

				var validationError *ValidationError

				if !errors.As(
					err,
					&validationError,
				) {
					t.Fatalf(
						"error = %v, want ValidationError",
						err,
					)
				}

				if !validationError.Has(
					test.field,
				) {
					t.Errorf(
						"missing validation error for %q",
						test.field,
					)
				}
			},
		)
	}
}

func TestRepositoryTransactionRollsBackAllChanges(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	ctx := context.Background()

	createdHike, err := repository.Create(
		ctx,
		CreateParams{
			Name:     "Forest Loop",
			Location: "Lublin",
			Status:   StatusPlanned,
		},
	)
	if err != nil {
		t.Fatalf(
			"create hike: %v",
			err,
		)
	}

	forcedError := errors.New(
		"force rollback",
	)

	err = repository.WithinTransaction(
		ctx,

		func(
			tx hikeTransaction,
		) error {
			if err := tx.MarkHikeVisited(
				ctx,
				createdHike.ID,
			); err != nil {
				return err
			}

			_, err := tx.CreateVisit(
				ctx,
				createdHike.ID,
				CreateVisitParams{
					VisitedAt: time.Now().UTC(),

					Weather: "Sunny",
				},
			)
			if err != nil {
				return err
			}

			return forcedError
		},
	)

	if !errors.Is(
		err,
		forcedError,
	) {
		t.Fatalf(
			"error = %v, want forced error",
			err,
		)
	}

	found, err := repository.FindByID(
		ctx,
		createdHike.ID,
	)
	if err != nil {
		t.Fatalf(
			"find hike: %v",
			err,
		)
	}

	if got, want :=
		found.Status,
		StatusPlanned; got != want {
		t.Errorf(
			"status = %q, want %q",
			got,
			want,
		)
	}

	visits, err := repository.ListVisits(
		ctx,
		createdHike.ID,
	)
	if err != nil {
		t.Fatalf(
			"list visits: %v",
			err,
		)
	}

	if got := len(visits); got != 0 {
		t.Errorf(
			"visit count = %d, want 0",
			got,
		)
	}
}

func TestRepositoryTransactionCommitsVisit(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	ctx := context.Background()

	hike, err := repository.Create(
		ctx,
		CreateParams{
			Name:     "Roztocze",
			Location: "Lubelskie",
			Status:   StatusPlanned,
		},
	)
	if err != nil {
		t.Fatalf(
			"create hike: %v",
			err,
		)
	}

	err = repository.WithinTransaction(
		ctx,

		func(
			tx hikeTransaction,
		) error {
			if err := tx.MarkHikeVisited(
				ctx,
				hike.ID,
			); err != nil {
				return err
			}

			rating := 5

			_, err := tx.CreateVisit(
				ctx,
				hike.ID,
				CreateVisitParams{
					VisitedAt: time.Now().UTC(),

					Weather: "Sunny",

					Rating: &rating,
				},
			)

			return err
		},
	)
	if err != nil {
		t.Fatalf(
			"transaction: %v",
			err,
		)
	}

	found, err := repository.FindByID(
		ctx,
		hike.ID,
	)
	if err != nil {
		t.Fatalf(
			"find hike: %v",
			err,
		)
	}

	if found.Status != StatusVisited {
		t.Errorf(
			"status = %q, want visited",
			found.Status,
		)
	}

	visits, err := repository.ListVisits(
		ctx,
		hike.ID,
	)
	if err != nil {
		t.Fatalf(
			"list visits: %v",
			err,
		)
	}

	if got, want :=
		len(visits),
		1; got != want {
		t.Fatalf(
			"visit count = %d, want %d",
			got,
			want,
		)
	}
}

func TestVisitRequiresExistingHike(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	err := repository.WithinTransaction(
		context.Background(),

		func(
			tx hikeTransaction,
		) error {
			_, err := tx.CreateVisit(
				context.Background(),
				999999,
				CreateVisitParams{
					VisitedAt: time.Now().UTC(),
				},
			)

			return err
		},
	)

	if err == nil {
		t.Fatal(
			"visit with nonexistent hike was accepted",
		)
	}
}

func TestServiceSetTagsNormalizesNames(
	t *testing.T,
) {
	var received []string

	repository := &fakeRepository{
		replaceHikeTagsFunc: func(
			ctx context.Context,
			hikeID int64,
			names []string,
		) ([]Tag, error) {
			received = names

			return []Tag{}, nil
		},
	}

	service := newTestService(
		repository,
	)

	_, err := service.SetTags(
		context.Background(),
		42,
		[]string{
			" Forest ",
			"MACRO",
			"forest",
			"  quiet ",
			"",
		},
	)
	if err != nil {
		t.Fatalf(
			"SetTags() returned error: %v",
			err,
		)
	}

	want := []string{
		"forest",
		"macro",
		"quiet",
	}

	if !slices.Equal(
		received,
		want,
	) {
		t.Errorf(
			"tags = %v, want %v",
			received,
			want,
		)
	}
}

func TestRepositoryReplaceHikeTags(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	ctx := context.Background()

	hike, err := repository.Create(
		ctx,
		CreateParams{
			Name:     "Forest Loop",
			Location: "Lublin",
			Status:   StatusPlanned,
		},
	)
	if err != nil {
		t.Fatalf(
			"create hike: %v",
			err,
		)
	}

	first, err := repository.ReplaceHikeTags(
		ctx,
		hike.ID,
		[]string{
			"forest",
			"easy",
			"macro",
		},
	)
	if err != nil {
		t.Fatalf(
			"assign first tags: %v",
			err,
		)
	}

	if got, want :=
		len(first),
		3; got != want {
		t.Fatalf(
			"tag count = %d, want %d",
			got,
			want,
		)
	}

	second, err := repository.ReplaceHikeTags(
		ctx,
		hike.ID,
		[]string{
			"forest",
			"quiet",
		},
	)
	if err != nil {
		t.Fatalf(
			"replace tags: %v",
			err,
		)
	}

	if got, want :=
		len(second),
		2; got != want {
		t.Fatalf(
			"tag count = %d, want %d",
			got,
			want,
		)
	}

	found, err := repository.ListHikeTags(
		ctx,
		hike.ID,
	)
	if err != nil {
		t.Fatalf(
			"list hike tags: %v",
			err,
		)
	}

	got := []string{
		found[0].Name,
		found[1].Name,
	}

	want := []string{
		"forest",
		"quiet",
	}

	if !slices.Equal(
		got,
		want,
	) {
		t.Errorf(
			"tags = %v, want %v",
			got,
			want,
		)
	}
}

func TestRepositoryReplaceHikeTagsCanClearTags(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	ctx := context.Background()

	hike, err := repository.Create(
		ctx,
		CreateParams{
			Name:     "Forest Loop",
			Location: "Lublin",
			Status:   StatusPlanned,
		},
	)
	if err != nil {
		t.Fatalf(
			"create hike: %v",
			err,
		)
	}

	_, err = repository.ReplaceHikeTags(
		ctx,
		hike.ID,
		[]string{
			"forest",
			"macro",
		},
	)
	if err != nil {
		t.Fatalf(
			"assign tags: %v",
			err,
		)
	}

	_, err = repository.ReplaceHikeTags(
		ctx,
		hike.ID,
		[]string{},
	)
	if err != nil {
		t.Fatalf(
			"clear tags: %v",
			err,
		)
	}

	found, err := repository.ListHikeTags(
		ctx,
		hike.ID,
	)
	if err != nil {
		t.Fatalf(
			"list tags: %v",
			err,
		)
	}

	if got := len(found); got != 0 {
		t.Errorf(
			"tag count = %d, want 0",
			got,
		)
	}
}

func TestServiceImportTrack(
	t *testing.T,
) {
	var operations []string

	tx := &fakeHikeTransaction{
		createHikeFunc: func(
			ctx context.Context,
			params CreateParams,
		) (Hike, error) {
			operations = append(
				operations,
				"create-hike",
			)

			return Hike{
				ID:       42,
				Name:     params.Name,
				Location: params.Location,
				Status:   params.Status,
			}, nil
		},

		createTrackFunc: func(
			ctx context.Context,
			hikeID int64,
			params ImportTrackParams,
		) (Track, error) {
			operations = append(
				operations,
				"create-track",
			)

			return Track{
				ID:         7,
				HikeID:     hikeID,
				PointCount: len(params.Points),
			}, nil
		},
	}

	repository := &fakeRepository{
		withinTransactionFunc: func(
			ctx context.Context,
			fn func(hikeTransaction) error,
		) error {
			return fn(tx)
		},
	}

	service := newTestService(
		repository,
	)

	distance := 8.5

	hike, track, err := service.ImportTrack(
		context.Background(),
		ImportTrackParams{
			Hike: CreateParams{
				Name: " Forest Loop ",

				Location: " Lublin ",

				DistanceKM: &distance,
			},

			SourceFilename: " forest.gpx ",

			Points: []TrackPoint{
				{
					Latitude:  51,
					Longitude: 22,
				},
				{
					Latitude:  51.1,
					Longitude: 22.1,
				},
			},

			DistanceKM: 8.5,

			StartLat: 51,

			StartLon: 22,

			EndLat: 51.1,

			EndLon: 22.1,
		},
	)
	if err != nil {
		t.Fatalf(
			"ImportTrack() returned error: %v",
			err,
		)
	}

	if got, want :=
		operations,
		[]string{
			"create-hike",
			"create-track",
		}; !slices.Equal(
		got,
		want,
	) {

		t.Errorf(
			"operations = %v, want %v",
			got,
			want,
		)
	}

	if got, want :=
		hike.Name,
		"Forest Loop"; got != want {
		t.Errorf(
			"name = %q, want %q",
			got,
			want,
		)
	}

	if got, want :=
		track.HikeID,
		int64(42); got != want {
		t.Errorf(
			"track hike ID = %d, want %d",
			got,
			want,
		)
	}
}

func TestServiceTrackVisualizationNoTrack(
	t *testing.T,
) {
	repository := &fakeRepository{
		findTrackByHikeIDFunc: func(
			context.Context,
			int64,
		) (Track, error) {
			return Track{}, ErrTrackNotFound
		},
	}

	visualization, err := newTestService(repository).TrackVisualization(
		context.Background(),
		42,
	)
	if err != nil {
		t.Fatalf("TrackVisualization() returned error: %v", err)
	}

	if visualization != nil {
		t.Errorf("TrackVisualization() = %#v, want nil", visualization)
	}
}

func TestServiceTrackVisualizationErrors(
	t *testing.T,
) {
	repositoryError := errors.New("repository error")

	tests := []struct {
		name       string
		repository *fakeRepository
	}{
		{
			name: "track lookup fails",
			repository: &fakeRepository{
				findTrackByHikeIDFunc: func(
					context.Context,
					int64,
				) (Track, error) {
					return Track{}, repositoryError
				},
			},
		},
		{
			name: "point lookup fails",
			repository: &fakeRepository{
				findTrackByHikeIDFunc: func(
					context.Context,
					int64,
				) (Track, error) {
					return Track{ID: 7}, nil
				},
				listTrackPointsFunc: func(
					context.Context,
					int64,
				) ([]TrackPoint, error) {
					return nil, repositoryError
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			visualization, err := newTestService(test.repository).TrackVisualization(
				context.Background(),
				42,
			)

			if !errors.Is(err, repositoryError) {
				t.Fatalf("error = %v, want wrapped repository error", err)
			}

			if visualization != nil {
				t.Errorf("TrackVisualization() = %#v, want nil", visualization)
			}
		})
	}
}

func TestServiceTrackVisualizationRejectsInsufficientPoints(
	t *testing.T,
) {
	repository := &fakeRepository{
		findTrackByHikeIDFunc: func(
			context.Context,
			int64,
		) (Track, error) {
			return Track{ID: 7}, nil
		},
		listTrackPointsFunc: func(
			context.Context,
			int64,
		) ([]TrackPoint, error) {
			return []TrackPoint{{Latitude: 51, Longitude: 22}}, nil
		},
	}

	visualization, err := newTestService(repository).TrackVisualization(
		context.Background(),
		42,
	)
	if err == nil {
		t.Fatal("TrackVisualization() returned nil error")
	}

	if visualization != nil {
		t.Errorf("TrackVisualization() = %#v, want nil", visualization)
	}
}

func TestServiceTrackVisualization(
	t *testing.T,
) {
	track := Track{ID: 7, HikeID: 42, SourceFilename: "forest.gpx"}
	points := []TrackPoint{
		{SegmentIndex: 0, Latitude: 51, Longitude: 22},
		{SegmentIndex: 0, Latitude: 51.1, Longitude: 22.1},
	}

	repository := &fakeRepository{
		findTrackByHikeIDFunc: func(
			_ context.Context,
			hikeID int64,
		) (Track, error) {
			if hikeID != 42 {
				t.Errorf("hike ID = %d, want 42", hikeID)
			}

			return track, nil
		},
		listTrackPointsFunc: func(
			_ context.Context,
			trackID int64,
		) ([]TrackPoint, error) {
			if trackID != track.ID {
				t.Errorf("track ID = %d, want %d", trackID, track.ID)
			}

			return points, nil
		},
	}

	visualization, err := newTestService(repository).TrackVisualization(
		context.Background(),
		42,
	)
	if err != nil {
		t.Fatalf("TrackVisualization() returned error: %v", err)
	}

	if visualization == nil {
		t.Fatal("TrackVisualization() returned nil visualization")
	}

	if visualization.Track != track {
		t.Errorf("track = %#v, want %#v", visualization.Track, track)
	}

	if got, want := len(visualization.Map.Segments), 1; got != want {
		t.Fatalf("map segment count = %d, want %d", got, want)
	}

	if got, want := len(visualization.Map.Segments[0].Points), 2; got != want {
		t.Errorf("map point count = %d, want %d", got, want)
	}
}

func TestServiceSetTrack(
	t *testing.T,
) {
	var operations []string

	tx := &fakeHikeTransaction{
		updateHikeDistanceFunc: func(
			ctx context.Context,
			hikeID int64,
			distance *float64,
		) error {
			operations = append(
				operations,
				"update-distance",
			)

			if got, want :=
				*distance,
				8.4; got != want {
				t.Errorf(
					"distance = %.1f, want %.1f",
					got,
					want,
				)
			}

			return nil
		},

		deleteTrackByHikeIDFunc: func(
			context.Context,
			int64,
		) (bool, error) {
			operations = append(
				operations,
				"delete-track",
			)

			return true, nil
		},

		createTrackFunc: func(
			ctx context.Context,
			hikeID int64,
			params ImportTrackParams,
		) (Track, error) {
			operations = append(
				operations,
				"create-track",
			)

			return Track{
				ID: 100,

				HikeID: hikeID,

				DistanceKM: params.DistanceKM,
			}, nil
		},
	}

	repository := &fakeRepository{
		withinTransactionFunc: func(
			ctx context.Context,
			fn func(hikeTransaction) error,
		) error {
			return fn(tx)
		},
	}

	service := newTestService(
		repository,
	)

	created, err := service.SetTrack(
		context.Background(),
		42,
		SetTrackParams{
			SourceFilename: " new.gpx ",

			Points: []TrackPoint{
				{
					Latitude: 51,

					Longitude: 22,
				},
				{
					Latitude: 51.1,

					Longitude: 22.1,
				},
			},

			DistanceKM: 8.4,

			StartLat: 51,

			StartLon: 22,

			EndLat: 51.1,

			EndLon: 22.1,
		},
	)
	if err != nil {
		t.Fatalf(
			"SetTrack() returned error: %v",
			err,
		)
	}

	wantOperations := []string{
		"update-distance",
		"delete-track",
		"create-track",
	}

	if !slices.Equal(
		operations,
		wantOperations,
	) {
		t.Errorf(
			"operations = %v, want %v",
			operations,
			wantOperations,
		)
	}

	if got, want :=
		created.ID,
		int64(100); got != want {
		t.Errorf(
			"track ID = %d, want %d",
			got,
			want,
		)
	}
}

// ---- Fake Repository ----

type fakeRepository struct {
	createFunc            func(context.Context, CreateParams) (Hike, error)
	findByIDFunc          func(context.Context, int64) (Hike, error)
	listFunc              func(context.Context) ([]Hike, error)
	searchFunc            func(context.Context, ListParams) (ListResult, error)
	updateFunc            func(context.Context, int64, UpdateParams) (Hike, error)
	deleteFunc            func(context.Context, int64) error
	withinTransactionFunc func(context.Context, func(hikeTransaction) error) error
	listVisitsFunc        func(context.Context, int64) ([]Visit, error)

	replaceHikeTagsFunc func(context.Context, int64, []string) ([]Tag, error)
	listTagsFunc        func(context.Context) ([]Tag, error)
	listHikeTagsFunc    func(context.Context, int64) ([]Tag, error)

	dashboardStatsFunc func(context.Context) (DashboardStats, error)

	findTrackByHikeIDFunc func(context.Context, int64) (Track, error)
	listTrackPointsFunc   func(context.Context, int64) ([]TrackPoint, error)
}

func (f *fakeRepository) Create(ctx context.Context, params CreateParams) (Hike, error) {
	if f.createFunc == nil {
		panic("unexpected Create call")
	}

	return f.createFunc(ctx, params)
}

func (f *fakeRepository) FindByID(ctx context.Context, id int64) (Hike, error) {
	if f.findByIDFunc == nil {
		panic("unexpected FindByID call")
	}

	return f.findByIDFunc(ctx, id)
}

func (f *fakeRepository) List(ctx context.Context) ([]Hike, error) {
	if f.listFunc == nil {
		panic("unexpected List call")
	}

	return f.listFunc(ctx)
}

func (f *fakeRepository) Search(ctx context.Context, params ListParams) (ListResult, error) {
	if f.searchFunc == nil {
		panic("unexpected search call")
	}

	return f.searchFunc(ctx, params)
}

func (f *fakeRepository) Update(ctx context.Context, id int64, params UpdateParams) (Hike, error) {
	if f.updateFunc == nil {
		panic("unexpected Update call")
	}

	return f.updateFunc(ctx, id, params)
}

func (f *fakeRepository) Delete(ctx context.Context, id int64) error {
	if f.deleteFunc == nil {
		panic("unexpected Delete call")
	}

	return f.deleteFunc(ctx, id)
}

func (f *fakeRepository) WithinTransaction(
	ctx context.Context,
	fn func(hikeTransaction) error,
) error {
	if f.withinTransactionFunc == nil {
		panic("unexpected WithinTransaction call")
	}

	return f.withinTransactionFunc(
		ctx,
		fn,
	)
}

func (f *fakeRepository) ListVisits(
	ctx context.Context,
	hikeID int64,
) ([]Visit, error) {
	if f.listVisitsFunc == nil {
		panic("unexpected ListVisits call")
	}

	return f.listVisitsFunc(
		ctx,
		hikeID,
	)
}

func (f *fakeRepository) ReplaceHikeTags(
	ctx context.Context,
	hikeID int64,
	names []string,
) ([]Tag, error) {
	if f.replaceHikeTagsFunc == nil {
		panic("unexpected ReplaceHikeTags call")
	}

	return f.replaceHikeTagsFunc(
		ctx,
		hikeID,
		names,
	)
}

func (f *fakeRepository) ListTags(
	ctx context.Context,
) ([]Tag, error) {
	if f.listTagsFunc == nil {
		panic("unexpected ListTags call")
	}

	return f.listTagsFunc(
		ctx,
	)
}

func (f *fakeRepository) ListHikeTags(
	ctx context.Context,
	hikeID int64,
) ([]Tag, error) {
	if f.listHikeTagsFunc == nil {
		panic("unexpected ListHikeTags call")
	}

	return f.listHikeTagsFunc(
		ctx,
		hikeID,
	)
}

func (f *fakeRepository) DashboardStats(
	ctx context.Context,
) (DashboardStats, error) {
	if f.dashboardStatsFunc == nil {
		panic("unexpected DashboardStats call")
	}

	return f.dashboardStatsFunc(
		ctx,
	)
}

func (f *fakeRepository) FindTrackByHikeID(
	ctx context.Context,
	hikeID int64,
) (Track, error) {
	if f.findTrackByHikeIDFunc == nil {
		panic("unexpected FindTrackByHikeID call")
	}

	return f.findTrackByHikeIDFunc(ctx, hikeID)
}

func (f *fakeRepository) ListTrackPoints(
	ctx context.Context,
	trackID int64,
) ([]TrackPoint, error) {
	if f.listTrackPointsFunc == nil {
		panic("unexpected ListTrackPoints call")
	}

	return f.listTrackPointsFunc(ctx, trackID)
}

func (f *fakeHikeTransaction) UpdateHikeDistance(
	ctx context.Context,
	hikeID int64,
	distanceKM *float64,
) error {
	if f.updateHikeDistanceFunc == nil {
		panic(
			"unexpected UpdateHikeDistance call",
		)
	}

	return f.updateHikeDistanceFunc(
		ctx,
		hikeID,
		distanceKM,
	)
}

func (f *fakeHikeTransaction) DeleteTrackByHikeID(
	ctx context.Context,
	hikeID int64,
) (bool, error) {
	if f.deleteTrackByHikeIDFunc == nil {
		panic(
			"unexpected DeleteTrackByHikeID call",
		)
	}

	return f.deleteTrackByHikeIDFunc(
		ctx,
		hikeID,
	)
}

// --- Helpers ---

func newTestService(
	repository hikeRepository,
) *Service {
	return NewService(
		repository,
		newNoopTracer(),
	)
}
