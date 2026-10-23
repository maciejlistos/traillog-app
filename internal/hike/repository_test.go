package hike

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/maciejlistos/traillog/internal/config"
	"github.com/maciejlistos/traillog/internal/database"
	"github.com/maciejlistos/traillog/internal/gpx"
	"go.opentelemetry.io/otel/trace"
)

// --- Unit Tests ---

func TestRepositoryCreate(t *testing.T) {
	now := time.Date(
		2026,
		time.August,
		11,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	repository, _ := newTestRepository(
		t,
		func() time.Time {
			return now
		},
	)

	distance := 8.4

	created, err := repository.Create(
		context.Background(),
		CreateParams{
			Name:        "Forest loop",
			Location:    "Lublin",
			Description: "A quiet forest walk",
			DistanceKM:  &distance,
			Status:      StatusPlanned,
		},
	)
	if err != nil {
		t.Fatalf("create hike: %v", err)
	}

	if created.ID <= 0 {
		t.Errorf("ID = %d, want positive value", created.ID)
	}

	if got, want := created.Name, "Forest loop"; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}

	if got, want := created.Location, "Lublin"; got != want {
		t.Errorf("location = %q, want %q", got, want)
	}

	if created.DistanceKM == nil {
		t.Fatal("dinstance is nil")
	}

	if got, want := *created.DistanceKM, 8.4; got != want {
		t.Errorf("distance %f, want %f", got, want)
	}

	if got, want := created.Status, StatusPlanned; got != want {
		t.Errorf("status %q, want %q", got, want)
	}

	if !created.CreatedAt.Equal(now) {
		t.Errorf("created at = %s, want %s", created.CreatedAt, now)
	}

	if !created.UpdatedAt.Equal(now) {
		t.Errorf("updated at = %s, want %s", created.UpdatedAt, now)
	}
}

func TestRepositoryCreateWithoutDistance(t *testing.T) {
	now := time.Date(
		2026,
		time.August,
		11,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	repository, _ := newTestRepository(t, func() time.Time { return now })

	created, err := repository.Create(
		context.Background(),
		CreateParams{
			Name:        "Unknown Route",
			Location:    "Roztocze",
			Description: "",
			DistanceKM:  nil,
			Status:      StatusPlanned,
		},
	)
	if err != nil {
		t.Fatalf("create hike: %v", err)
	}

	if created.DistanceKM != nil {
		t.Errorf("distance = %f, want nil", *created.DistanceKM)
	}
}

func TestRepositoryFindByID(t *testing.T) {
	now := time.Date(
		2026,
		time.August,
		11,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	repository, _ := newTestRepository(t, func() time.Time { return now })

	created, err := repository.Create(
		context.Background(),
		CreateParams{
			Name:     "Stare Brusno",
			Location: "Podkarpacie",
			Status:   StatusVisited,
		},
	)
	if err != nil {
		t.Fatalf("create hike: %v", err)
	}

	found, err := repository.FindByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("find hike: %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("ID = %d, want %d", found.ID, created.ID)
	}

	if found.Name != created.Name {
		t.Errorf("name = %q, want %q", found.Name, created.Name)
	}
}

func TestRepositoryFindByIDNotFound(t *testing.T) {
	repository, _ := newTestRepository(t, time.Now)

	_, err := repository.FindByID(context.Background(), 9999)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

// -- List --
func TestRepositoryList(t *testing.T) {
	now := time.Date(
		2026,
		time.August,
		11,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	repository, _ := newTestRepository(
		t,
		func() time.Time {
			return now
		},
	)

	first, err := repository.Create(
		context.Background(),
		CreateParams{
			Name:     "First hike",
			Location: "Lublin",
			Status:   StatusPlanned,
		},
	)
	if err != nil {
		t.Fatalf("create first hike: %v", err)
	}

	second, err := repository.Create(
		context.Background(),
		CreateParams{
			Name:     "Second hike",
			Location: "Roztocze",
			Status:   StatusVisited,
		},
	)
	if err != nil {
		t.Fatalf("create second hike: %v", err)
	}

	results, err := repository.List(
		context.Background(),
	)
	if err != nil {
		t.Fatalf("list hikes: %v", err)
	}

	if got, want := len(results), 2; got != want {
		t.Fatalf(
			"hike count = %d, want %d",
			got,
			want,
		)
	}

	if got, want := results[0].ID,
		second.ID; got != want {
		t.Errorf(
			"first result ID = %d, want %d",
			got,
			want,
		)
	}

	if got, want := results[1].ID,
		first.ID; got != want {
		t.Errorf(
			"second result ID = %d, want %d",
			got,
			want,
		)
	}
}

func TestRepositoryListReturnsEmptySlice(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	results, err := repository.List(
		context.Background(),
	)
	if err != nil {
		t.Fatalf("list hikes: %v", err)
	}

	if results == nil {
		t.Fatal(
			"results = nil, want empty slice",
		)
	}

	if got := len(results); got != 0 {
		t.Errorf(
			"hike count = %d, want 0",
			got,
		)
	}
}

// -- Update --
func TestRepositoryUpdate(t *testing.T) {
	currentTime := time.Date(
		2026,
		time.August,
		11,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	repository, _ := newTestRepository(
		t,
		func() time.Time {
			return currentTime
		},
	)

	created, err := repository.Create(
		context.Background(),
		CreateParams{
			Name:     "Old name",
			Location: "Lublin",
			Status:   StatusPlanned,
		},
	)
	if err != nil {
		t.Fatalf("create hike: %v", err)
	}

	currentTime = currentTime.Add(2 * time.Hour)

	distance := 12.3

	updated, err := repository.Update(
		context.Background(),
		created.ID,
		UpdateParams{
			Name:        "New name",
			Location:    "Roztocze",
			Description: "Updated description",
			DistanceKM:  &distance,
			Status:      StatusVisited,
		},
	)
	if err != nil {
		t.Fatalf("update hike: %v", err)
	}

	if got, want := updated.Name,
		"New name"; got != want {
		t.Errorf(
			"name = %q, want %q",
			got,
			want,
		)
	}

	if got, want := updated.Status,
		StatusVisited; got != want {
		t.Errorf(
			"status = %q, want %q",
			got,
			want,
		)
	}

	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Error(
			"CreatedAt changed during update",
		)
	}

	if !updated.UpdatedAt.Equal(currentTime) {
		t.Errorf(
			"updated at = %s, want %s",
			updated.UpdatedAt,
			currentTime,
		)
	}
}

func TestRepositoryUpdateReturnsNotFound(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	_, err := repository.Update(
		context.Background(),
		12345,
		UpdateParams{
			Name:     "Missing",
			Location: "Nowhere",
			Status:   StatusPlanned,
		},
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"error = %v, want ErrNotFound",
			err,
		)
	}
}

// -- Delete --
func TestRepositoryDelete(t *testing.T) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	created, err := repository.Create(
		context.Background(),
		CreateParams{
			Name:     "To be deleted",
			Location: "Lublin",
			Status:   StatusPlanned,
		},
	)
	if err != nil {
		t.Fatalf("create hike: %v", err)
	}

	err = repository.Delete(
		context.Background(),
		created.ID,
	)
	if err != nil {
		t.Fatalf("delete hike: %v", err)
	}

	_, err = repository.FindByID(
		context.Background(),
		created.ID,
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"find after delete error = %v, want ErrNotFound",
			err,
		)
	}
}

func TestRepositoryDeleteReturnsNotFound(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	err := repository.Delete(
		context.Background(),
		9876,
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"error = %v, want ErrNotFound",
			err,
		)
	}
}

// -- Others --
func TestRepositoryCreateRejectsNegativeDistance(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	distance := -5.0

	_, err := repository.Create(
		context.Background(),
		CreateParams{
			Name:       "Impossible hike",
			Location:   "Lublin",
			DistanceKM: &distance,
			Status:     StatusPlanned,
		},
	)

	if err == nil {
		t.Fatal(
			"Create() returned nil error for negative distance",
		)
	}
}

func TestRepositoryListRecordsDatabaseSpan(
	t *testing.T,
) {
	provider, recorder := newRecordingTracerProvider(t)

	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	repository.tracer = provider.Tracer(
		"traillog/test/repository",
	)

	_, err := repository.List(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"List() returned error: %v",
			err,
		)
	}

	span := findEndedSpan(
		t,
		recorder,
		"SELECT hikes",
	)

	if got, want :=
		span.SpanKind(),
		trace.SpanKindClient; got != want {
		t.Errorf(
			"span kind = %s, want %s",
			got,
			want,
		)
	}

	if got := stringSpanAttribute(
		span,
		"db.system.name",
	); got != "sqlite" {
		t.Errorf(
			"db.system.name = %q, want sqlite",
			got,
		)
	}

	if got := stringSpanAttribute(
		span,
		"db.operation.name",
	); got != "SELECT" {
		t.Errorf(
			"db.operation.name = %q, want SELECT",
			got,
		)
	}
}

func TestServiceSpanUsesContextParent(
	t *testing.T,
) {
	provider, recorder := newRecordingTracerProvider(t)

	tracer := provider.Tracer(
		"traillog/test",
	)

	repository := &fakeRepository{
		listFunc: func(
			context.Context,
		) ([]Hike, error) {
			return []Hike{}, nil
		},
	}

	service := NewService(
		repository,
		tracer,
	)

	ctx, parent := tracer.Start(
		context.Background(),
		"http request",
	)

	parentSpanID := parent.SpanContext().
		SpanID()

	_, err := service.List(ctx)
	if err != nil {
		t.Fatalf(
			"List() returned error: %v",
			err,
		)
	}

	parent.End()

	serviceSpan := findEndedSpan(
		t,
		recorder,
		"hike.service.list",
	)

	if got, want :=
		serviceSpan.Parent().SpanID(),
		parentSpanID; got != want {
		t.Errorf(
			"parent span ID = %s, want %s",
			got,
			want,
		)
	}
}

func TestRepositorySearchByQuery(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	ctx := context.Background()

	_, err := repository.Create(
		ctx,
		CreateParams{
			Name:     "Forest Loop",
			Location: "Lublin",
			Status:   StatusPlanned,
		},
	)
	if err != nil {
		t.Fatalf(
			"create first hike: %v",
			err,
		)
	}

	_, err = repository.Create(
		ctx,
		CreateParams{
			Name:     "Mountain Trail",
			Location: "Beskid Niski",
			Status:   StatusVisited,
		},
	)
	if err != nil {
		t.Fatalf(
			"create second hike: %v",
			err,
		)
	}

	result, err := repository.Search(
		ctx,
		ListParams{
			Query:    "forest",
			Sort:     SortNewest,
			Page:     1,
			PageSize: 12,
		},
	)
	if err != nil {
		t.Fatalf(
			"search hikes: %v",
			err,
		)
	}

	if got, want :=
		result.Total,
		1; got != want {
		t.Fatalf(
			"total = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		result.Hikes[0].Name,
		"Forest Loop"; got != want {
		t.Errorf(
			"name = %q, want %q",
			got,
			want,
		)
	}
}

func TestRepositorySearchByStatus(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	ctx := context.Background()

	for _, params := range []CreateParams{
		{
			Name:     "Planned hike",
			Location: "Lublin",
			Status:   StatusPlanned,
		},
		{
			Name:     "Visited hike",
			Location: "Roztocze",
			Status:   StatusVisited,
		},
	} {
		if _, err := repository.Create(
			ctx,
			params,
		); err != nil {
			t.Fatalf(
				"create hike: %v",
				err,
			)
		}
	}

	status := StatusVisited

	result, err := repository.Search(
		ctx,
		ListParams{
			Status:   &status,
			Sort:     SortNewest,
			Page:     1,
			PageSize: 12,
		},
	)
	if err != nil {
		t.Fatalf(
			"search hikes: %v",
			err,
		)
	}

	if got, want :=
		result.Total,
		1; got != want {
		t.Fatalf(
			"total = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		result.Hikes[0].Status,
		StatusVisited; got != want {
		t.Errorf(
			"status = %q, want %q",
			got,
			want,
		)
	}
}

func TestRepositorySearchPagination(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		_, err := repository.Create(
			ctx,
			CreateParams{
				Name: fmt.Sprintf(
					"Hike %d",
					i,
				),
				Location: "Lublin",
				Status:   StatusPlanned,
			},
		)
		if err != nil {
			t.Fatalf(
				"create hike %d: %v",
				i,
				err,
			)
		}
	}

	result, err := repository.Search(
		ctx,
		ListParams{
			Sort:     SortNewest,
			Page:     2,
			PageSize: 2,
		},
	)
	if err != nil {
		t.Fatalf(
			"search hikes: %v",
			err,
		)
	}

	if got, want :=
		result.Total,
		5; got != want {
		t.Errorf(
			"total = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		len(result.Hikes),
		2; got != want {
		t.Fatalf(
			"result count = %d, want %d",
			got,
			want,
		)
	}

	// Newest ordering is ID DESC.
	if got, want :=
		result.Hikes[0].Name,
		"Hike 3"; got != want {
		t.Errorf(
			"first hike = %q, want %q",
			got,
			want,
		)
	}
}

func TestRepositorySearchEscapesLikeWildcards(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	ctx := context.Background()

	for _, name := range []string{
		"100% Trail",
		"1000 Trail",
	} {
		_, err := repository.Create(
			ctx,
			CreateParams{
				Name:     name,
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
	}

	result, err := repository.Search(
		ctx,
		ListParams{
			Query:    "%",
			Sort:     SortNewest,
			Page:     1,
			PageSize: 12,
		},
	)
	if err != nil {
		t.Fatalf(
			"search hikes: %v",
			err,
		)
	}

	if got, want :=
		result.Total,
		1; got != want {
		t.Fatalf(
			"total = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		result.Hikes[0].Name,
		"100% Trail"; got != want {
		t.Errorf(
			"name = %q, want %q",
			got,
			want,
		)
	}
}

func TestRepositorySearchRequiresAllTags(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	ctx := context.Background()

	forestMacro, err := repository.Create(
		ctx,
		CreateParams{
			Name: "Forest Macro Trail",

			Location: "Lublin",

			Status: StatusVisited,
		},
	)
	if err != nil {
		t.Fatalf(
			"create first hike: %v",
			err,
		)
	}

	forestOnly, err := repository.Create(
		ctx,
		CreateParams{
			Name: "Forest Walk",

			Location: "Lublin",

			Status: StatusVisited,
		},
	)
	if err != nil {
		t.Fatalf(
			"create second hike: %v",
			err,
		)
	}

	_, err = repository.ReplaceHikeTags(
		ctx,
		forestMacro.ID,
		[]string{
			"forest",
			"macro",
		},
	)
	if err != nil {
		t.Fatalf(
			"assign first tags: %v",
			err,
		)
	}

	_, err = repository.ReplaceHikeTags(
		ctx,
		forestOnly.ID,
		[]string{
			"forest",
		},
	)
	if err != nil {
		t.Fatalf(
			"assign second tags: %v",
			err,
		)
	}

	result, err := repository.Search(
		ctx,
		ListParams{
			Tags: []string{
				"forest",
				"macro",
			},

			Sort: SortNewest,

			Page: 1,

			PageSize: 12,
		},
	)
	if err != nil {
		t.Fatalf(
			"search hikes: %v",
			err,
		)
	}

	if got, want :=
		result.Total,
		1; got != want {
		t.Fatalf(
			"total = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		result.Hikes[0].Name,
		"Forest Macro Trail"; got != want {
		t.Errorf(
			"hike = %q, want %q",
			got,
			want,
		)
	}

	if got, want := len(result.Hikes[0].Tags), 2; got != want {
		t.Fatalf("tag count = %d, want %d", got, want)
	}

	if got, want := result.Hikes[0].Tags[0].Name, "forest"; got != want {
		t.Errorf("first tag = %q, want %q", got, want)
	}

	if got, want := result.Hikes[0].Tags[1].Name, "macro"; got != want {
		t.Errorf("second tag = %q, want %q", got, want)
	}
}

func TestRepositoryDashboardStats(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	ctx := context.Background()

	forestLoop, err := repository.Create(
		ctx,
		CreateParams{
			Name: "Forest Loop",

			Location: "Lublin",

			DistanceKM: new(10.0),

			Status: StatusVisited,
		},
	)
	if err != nil {
		t.Fatalf(
			"create Forest Loop: %v",
			err,
		)
	}

	wetlandTrail, err := repository.Create(
		ctx,
		CreateParams{
			Name: "Wetland Trail",

			Location: "Polesie",

			DistanceKM: new(5.0),

			Status: StatusVisited,
		},
	)
	if err != nil {
		t.Fatalf(
			"create Wetland Trail: %v",
			err,
		)
	}

	hillView, err := repository.Create(
		ctx,
		CreateParams{
			Name: "Hill View",

			Location: "Roztocze",

			Status: StatusPlanned,
		},
	)
	if err != nil {
		t.Fatalf(
			"create Hill View: %v",
			err,
		)
	}

	_, err = repository.ReplaceHikeTags(
		ctx,
		forestLoop.ID,
		[]string{
			"forest",
			"macro",
		},
	)
	if err != nil {
		t.Fatalf(
			"tag Forest Loop: %v",
			err,
		)
	}

	_, err = repository.ReplaceHikeTags(
		ctx,
		wetlandTrail.ID,
		[]string{
			"forest",
			"macro",
		},
	)
	if err != nil {
		t.Fatalf(
			"tag Wetland Trail: %v",
			err,
		)
	}

	_, err = repository.ReplaceHikeTags(
		ctx,
		hillView.ID,
		[]string{
			"forest",
		},
	)
	if err != nil {
		t.Fatalf(
			"tag Hill View: %v",
			err,
		)
	}

	addVisit := func(
		hikeID int64,
		visitedAt time.Time,
		rating *int,
	) {
		t.Helper()

		err := repository.WithinTransaction(
			ctx,

			func(
				tx hikeTransaction,
			) error {
				_, err := tx.CreateVisit(
					ctx,
					hikeID,
					CreateVisitParams{
						VisitedAt: visitedAt,

						Rating: rating,
					},
				)

				return err
			},
		)
		if err != nil {
			t.Fatalf(
				"create visit: %v",
				err,
			)
		}
	}

	addVisit(
		forestLoop.ID,
		time.Date(
			2026,
			time.August,
			1,
			0,
			0,
			0,
			0,
			time.UTC,
		),
		new(5),
	)

	addVisit(
		forestLoop.ID,
		time.Date(
			2026,
			time.August,
			12,
			0,
			0,
			0,
			0,
			time.UTC,
		),
		nil,
	)

	addVisit(
		wetlandTrail.ID,
		time.Date(
			2026,
			time.July,
			20,
			0,
			0,
			0,
			0,
			time.UTC,
		),
		new(3),
	)

	stats, err := repository.DashboardStats(
		ctx,
	)
	if err != nil {
		t.Fatalf(
			"DashboardStats() returned error: %v",
			err,
		)
	}

	if got, want :=
		stats.TotalHikes,
		3; got != want {
		t.Errorf(
			"total hikes = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		stats.VisitedHikes,
		2; got != want {
		t.Errorf(
			"visited hikes = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		stats.PlannedHikes,
		1; got != want {
		t.Errorf(
			"planned hikes = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		stats.TotalDistanceKM,
		15.0; got != want {
		t.Errorf(
			"distance = %.1f, want %.1f",
			got,
			want,
		)
	}

	if got, want :=
		stats.TotalVisits,
		3; got != want {
		t.Errorf(
			"total visits = %d, want %d",
			got,
			want,
		)
	}

	if stats.AverageRating == nil {
		t.Fatal(
			"average rating is nil",
		)
	}

	if got, want :=
		*stats.AverageRating,
		4.0; got != want {
		t.Errorf(
			"average rating = %.1f, want %.1f",
			got,
			want,
		)
	}

	if got, want :=
		stats.TopTags[0].Name,
		"forest"; got != want {
		t.Errorf(
			"top tag = %q, want %q",
			got,
			want,
		)
	}

	if got, want :=
		stats.TopTags[0].HikeCount,
		3; got != want {
		t.Errorf(
			"forest count = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		stats.MostVisitedHikes[0].Name,
		"Forest Loop"; got != want {
		t.Errorf(
			"most visited hike = %q, want %q",
			got,
			want,
		)
	}

	if got, want :=
		stats.MostVisitedHikes[0].VisitCount,
		2; got != want {
		t.Errorf(
			"visit count = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		len(stats.MonthlyVisits),
		2; got != want {
		t.Fatalf(
			"monthly rows = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		stats.MonthlyVisits[0].Month,
		"2026-07"; got != want {
		t.Errorf(
			"first month = %q, want %q",
			got,
			want,
		)
	}

	if got, want :=
		stats.MonthlyVisits[1].VisitCount,
		2; got != want {
		t.Errorf(
			"August visits = %d, want %d",
			got,
			want,
		)
	}
}

func TestRepositoryDashboardStatsEmptyDatabase(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	stats, err := repository.DashboardStats(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"DashboardStats() returned error: %v",
			err,
		)
	}

	if stats.TotalHikes != 0 {
		t.Errorf(
			"total hikes = %d, want 0",
			stats.TotalHikes,
		)
	}

	if stats.TotalVisits != 0 {
		t.Errorf(
			"total visits = %d, want 0",
			stats.TotalVisits,
		)
	}

	if stats.TotalDistanceKM != 0 {
		t.Errorf(
			"distance = %f, want 0",
			stats.TotalDistanceKM,
		)
	}

	if stats.AverageRating != nil {
		t.Errorf(
			"average rating = %v, want nil",
			stats.AverageRating,
		)
	}

	if len(stats.TopTags) != 0 {
		t.Errorf(
			"top tags = %v, want empty",
			stats.TopTags,
		)
	}

	if len(stats.MonthlyVisits) != 0 {
		t.Errorf(
			"monthly visits = %v, want empty",
			stats.MonthlyVisits,
		)
	}
}

func TestImportTrackPersistsAllPoints(
	t *testing.T,
) {
	repository, db := newTestRepository(
		t,
		time.Now,
	)

	service := NewService(
		repository,
		newNoopTracer(),
	)

	distance := 2.4

	hike, track, err := service.ImportTrack(
		context.Background(),
		ImportTrackParams{
			Hike: CreateParams{
				Name: "Imported Route",

				Location: "Lublin",

				DistanceKM: &distance,

				Status: StatusPlanned,
			},

			SourceFilename: "route.gpx",

			Points: []TrackPoint{
				{
					SegmentIndex: 0,
					Latitude:     51,
					Longitude:    22,
				},
				{
					SegmentIndex: 0,
					Latitude:     51.01,
					Longitude:    22.01,
				},
				{
					SegmentIndex: 1,
					Latitude:     51.02,
					Longitude:    22.02,
				},
			},

			DistanceKM: 2.4,

			ElevationGainM: 43,

			StartLat: 51,

			StartLon: 22,

			EndLat: 51.02,

			EndLon: 22.02,
		},
	)
	if err != nil {
		t.Fatalf(
			"ImportTrack() returned error: %v",
			err,
		)
	}

	if hike.ID == 0 {
		t.Error(
			"hike ID is zero",
		)
	}

	if track.ID == 0 {
		t.Error(
			"track ID is zero",
		)
	}

	var pointCount int

	err = db.GetContext(
		context.Background(),
		&pointCount,
		`
		SELECT COUNT(*)
		FROM track_points
		WHERE track_id = ?
		`,
		track.ID,
	)
	if err != nil {
		t.Fatalf(
			"count track points: %v",
			err,
		)
	}

	if got, want :=
		pointCount,
		3; got != want {
		t.Errorf(
			"point count = %d, want %d",
			got,
			want,
		)
	}

	var storedSegment int

	err = db.GetContext(
		context.Background(),
		&storedSegment,
		`
		SELECT segment_index
		FROM track_points
		WHERE
			track_id = ?
			AND sequence = 2
		`,
		track.ID,
	)
	if err != nil {
		t.Fatalf(
			"read segment: %v",
			err,
		)
	}

	if got, want :=
		storedSegment,
		1; got != want {
		t.Errorf(
			"segment = %d, want %d",
			got,
			want,
		)
	}
}

func TestImportTrackTransactionRollsBackHike(
	t *testing.T,
) {
	repository, db := newTestRepository(
		t,
		time.Now,
	)

	ctx := context.Background()

	err := repository.WithinTransaction(
		ctx,

		func(
			tx hikeTransaction,
		) error {
			hike, err := tx.CreateHike(
				ctx,
				CreateParams{
					Name: "Broken Import",

					Location: "Lublin",

					Status: StatusPlanned,
				},
			)
			if err != nil {
				return err
			}

			_, err = tx.CreateTrack(
				ctx,
				hike.ID,
				ImportTrackParams{
					Points: []TrackPoint{
						{
							Latitude: 200,

							Longitude: 22,
						},

						{
							Latitude: 201,

							Longitude: 22,
						},
					},

					DistanceKM: 1,

					StartLat: 200,

					StartLon: 22,

					EndLat: 201,

					EndLon: 22,
				},
			)

			return err
		},
	)

	if err == nil {
		t.Fatal(
			"transaction unexpectedly succeeded",
		)
	}

	var count int

	err = db.GetContext(
		ctx,
		&count,
		`
		SELECT COUNT(*)
		FROM hikes
		WHERE name = ?
		`,
		"Broken Import",
	)
	if err != nil {
		t.Fatalf(
			"count hikes: %v",
			err,
		)
	}

	if got := count; got != 0 {
		t.Errorf(
			"hike count = %d, want 0",
			got,
		)
	}
}

func TestRepositoryReadsImportedTrackPointsInOrder(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	service := NewService(
		repository,
		newNoopTracer(),
	)

	distance := 2.0

	hike, _, err := service.ImportTrack(
		context.Background(),
		ImportTrackParams{
			Hike: CreateParams{
				Name: "Imported Track",

				Location: "Lublin",

				DistanceKM: &distance,

				Status: StatusPlanned,
			},

			SourceFilename: "track.gpx",

			Points: []TrackPoint{
				{
					SegmentIndex: 0,
					Latitude:     51.1,
					Longitude:    22.1,
				},
				{
					SegmentIndex: 0,
					Latitude:     51.2,
					Longitude:    22.2,
				},
				{
					SegmentIndex: 1,
					Latitude:     51.3,
					Longitude:    22.3,
				},
			},

			DistanceKM: 2,

			StartLat: 51.1,

			StartLon: 22.1,

			EndLat: 51.3,

			EndLon: 22.3,
		},
	)
	if err != nil {
		t.Fatalf(
			"ImportTrack() returned error: %v",
			err,
		)
	}

	track, err := repository.FindTrackByHikeID(
		context.Background(),
		hike.ID,
	)
	if err != nil {
		t.Fatalf(
			"FindTrackByHikeID() returned error: %v",
			err,
		)
	}

	points, err := repository.ListTrackPoints(
		context.Background(),
		track.ID,
	)
	if err != nil {
		t.Fatalf(
			"ListTrackPoints() returned error: %v",
			err,
		)
	}

	if got, want :=
		len(points),
		3; got != want {
		t.Fatalf(
			"points = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		points[0].Latitude,
		51.1; got != want {
		t.Errorf(
			"first latitude = %f, want %f",
			got,
			want,
		)
	}

	if got, want :=
		points[2].SegmentIndex,
		1; got != want {
		t.Errorf(
			"segment index = %d, want %d",
			got,
			want,
		)
	}
}

func TestTrackImportExportRoundTrip(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	service := NewService(
		repository,
		newNoopTracer(),
	)

	firstElevation := 100.5

	secondElevation := 125.25

	recordedAt := time.Date(
		2026,
		time.July,
		12,
		7,
		30,
		0,
		0,
		time.UTC,
	)

	distance := 5.2

	importedHike,
		_,
		err := service.ImportTrack(
		context.Background(),

		ImportTrackParams{
			Hike: CreateParams{
				Name: "Round Trip Route",

				Location: "Lublin",

				Description: "Test route",

				DistanceKM: &distance,

				Status: StatusPlanned,
			},

			SourceFilename: "original.gpx",

			Points: []TrackPoint{
				{
					SegmentIndex: 0,

					Latitude: 51.1,

					Longitude: 22.1,

					Elevation: &firstElevation,

					RecordedAt: &recordedAt,
				},
				{
					SegmentIndex: 0,

					Latitude: 51.2,

					Longitude: 22.2,

					Elevation: &secondElevation,
				},
				{
					SegmentIndex: 1,

					Latitude: 51.3,

					Longitude: 22.3,
				},
			},

			DistanceKM: distance,

			ElevationGainM: 24.75,

			StartLat: 51.1,

			StartLon: 22.1,

			EndLat: 51.3,

			EndLon: 22.3,
		},
	)
	if err != nil {
		t.Fatalf(
			"ImportTrack() returned error: %v",
			err,
		)
	}

	export, err := service.ExportTrack(
		context.Background(),
		importedHike.ID,
	)
	if err != nil {
		t.Fatalf(
			"ExportTrack() returned error: %v",
			err,
		)
	}

	if got, want :=
		len(export.Points),
		3; got != want {
		t.Fatalf(
			"export points = %d, want %d",
			got,
			want,
		)
	}

	if got, want :=
		export.Points[2].
			SegmentIndex,
		1; got != want {
		t.Errorf(
			"segment = %d, want %d",
			got,
			want,
		)
	}

	document := gpxTrackFromExport(
		export,
	)

	var output bytes.Buffer

	if err := gpx.Write(
		&output,
		document,
	); err != nil {
		t.Fatalf(
			"write exported GPX: %v",
			err,
		)
	}

	parsed, err := gpx.Parse(
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
		"Round Trip Route"; got != want {
		t.Errorf(
			"name = %q, want %q",
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
		parsed.Segments[0].
			Points[1].
			Latitude,
		51.2; got != want {
		t.Errorf(
			"latitude = %f, want %f",
			got,
			want,
		)
	}
}

func TestTrackReplacementRollbackRestoresOldTrack(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	ctx := context.Background()

	service := NewService(
		repository,
		newNoopTracer(),
	)

	oldDistance := 8.0

	hike, oldTrack, err := service.ImportTrack(
		ctx,
		ImportTrackParams{
			Hike: CreateParams{
				Name: "Forest Loop",

				Location: "Lublin",

				DistanceKM: &oldDistance,

				Status: StatusPlanned,
			},

			SourceFilename: "old.gpx",

			Points: []TrackPoint{
				{
					Latitude: 51,

					Longitude: 22,
				},
				{
					Latitude: 51.1,

					Longitude: 22.1,
				},
				{
					Latitude: 51.2,

					Longitude: 22.2,
				},
			},

			DistanceKM: oldDistance,

			StartLat: 51,

			StartLon: 22,

			EndLat: 51.2,

			EndLon: 22.2,
		},
	)
	if err != nil {
		t.Fatalf(
			"ImportTrack() returned error: %v",
			err,
		)
	}

	newDistance := 99.0

	err = repository.WithinTransaction(
		ctx,

		func(
			tx hikeTransaction,
		) error {
			if err := tx.UpdateHikeDistance(
				ctx,
				hike.ID,
				&newDistance,
			); err != nil {
				return err
			}

			deleted, err := tx.DeleteTrackByHikeID(
				ctx,
				hike.ID,
			)
			if err != nil {
				return err
			}

			if !deleted {
				t.Fatal(
					"existing track was not deleted",
				)
			}

			_, err = tx.CreateTrack(
				ctx,
				hike.ID,

				ImportTrackParams{
					SourceFilename: "broken.gpx",

					Points: []TrackPoint{
						{
							Latitude: 200,

							Longitude: 22,
						},
						{
							Latitude: 201,

							Longitude: 22,
						},
					},

					DistanceKM: newDistance,

					StartLat: 200,

					StartLon: 22,

					EndLat: 201,

					EndLon: 22,
				},
			)

			return err
		},
	)

	if err == nil {
		t.Fatal(
			"replacement unexpectedly succeeded",
		)
	}

	foundHike, err := repository.FindByID(
		ctx,
		hike.ID,
	)
	if err != nil {
		t.Fatalf(
			"find hike after rollback: %v",
			err,
		)
	}

	if foundHike.DistanceKM == nil {
		t.Fatal(
			"hike distance is nil",
		)
	}

	if got, want :=
		*foundHike.DistanceKM,
		oldDistance; got != want {
		t.Errorf(
			"distance after rollback = %.1f, want %.1f",
			got,
			want,
		)
	}

	foundTrack, err := repository.FindTrackByHikeID(
		ctx,
		hike.ID,
	)
	if err != nil {
		t.Fatalf(
			"find old track after rollback: %v",
			err,
		)
	}

	if got, want :=
		foundTrack.ID,
		oldTrack.ID; got != want {
		t.Errorf(
			"track ID after rollback = %d, want old ID %d",
			got,
			want,
		)
	}

	points, err := repository.ListTrackPoints(
		ctx,
		foundTrack.ID,
	)
	if err != nil {
		t.Fatalf(
			"list restored points: %v",
			err,
		)
	}

	if got, want :=
		len(points),
		3; got != want {
		t.Errorf(
			"restored point count = %d, want %d",
			got,
			want,
		)
	}
}

func TestServiceSetTrackReplacesExistingTrack(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	ctx := context.Background()

	service := NewService(
		repository,
		newNoopTracer(),
	)

	oldDistance := 5.0

	hike, oldTrack, err := service.ImportTrack(
		ctx,
		ImportTrackParams{
			Hike: CreateParams{
				Name: "Forest Loop",

				Location: "Lublin",

				DistanceKM: &oldDistance,

				Status: StatusPlanned,
			},

			SourceFilename: "old.gpx",

			Points: []TrackPoint{
				{
					Latitude:  51,
					Longitude: 22,
				},
				{
					Latitude:  51.1,
					Longitude: 22.1,
				},
				{
					Latitude:  51.2,
					Longitude: 22.2,
				},
			},

			DistanceKM: oldDistance,

			StartLat: 51,

			StartLon: 22,

			EndLat: 51.2,

			EndLon: 22.2,
		},
	)
	if err != nil {
		t.Fatalf(
			"ImportTrack() returned error: %v",
			err,
		)
	}

	newTrack, err := service.SetTrack(
		ctx,
		hike.ID,

		SetTrackParams{
			SourceFilename: "new.gpx",

			Points: []TrackPoint{
				{
					Latitude:  52,
					Longitude: 23,
				},
				{
					Latitude:  52.1,
					Longitude: 23.1,
				},
			},

			DistanceKM: 9.25,

			ElevationGainM: 250,

			StartLat: 52,

			StartLon: 23,

			EndLat: 52.1,

			EndLon: 23.1,
		},
	)
	if err != nil {
		t.Fatalf(
			"SetTrack() returned error: %v",
			err,
		)
	}

	if newTrack.ID ==
		oldTrack.ID {

		t.Errorf(
			"replacement reused old track ID %d",
			oldTrack.ID,
		)
	}

	foundHike, err := repository.FindByID(
		ctx,
		hike.ID,
	)
	if err != nil {
		t.Fatalf(
			"find hike: %v",
			err,
		)
	}

	if got, want :=
		*foundHike.DistanceKM,
		9.25; got != want {
		t.Errorf(
			"distance = %.2f, want %.2f",
			got,
			want,
		)
	}

	points, err := repository.ListTrackPoints(
		ctx,
		newTrack.ID,
	)
	if err != nil {
		t.Fatalf(
			"list replacement points: %v",
			err,
		)
	}

	if got, want :=
		len(points),
		2; got != want {
		t.Errorf(
			"point count = %d, want %d",
			got,
			want,
		)
	}
}

func TestServiceRemoveTrackPreservesHike(
	t *testing.T,
) {
	repository, _ := newTestRepository(
		t,
		time.Now,
	)

	ctx := context.Background()

	service := NewService(
		repository,
		newNoopTracer(),
	)

	distance := 7.5

	hike, track, err := service.ImportTrack(
		ctx,
		ImportTrackParams{
			Hike: CreateParams{
				Name: "Forest Loop",

				Location: "Lublin",

				DistanceKM: &distance,

				Status: StatusPlanned,
			},

			SourceFilename: "route.gpx",

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

			DistanceKM: distance,

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

	if err := service.RemoveTrack(
		ctx,
		hike.ID,
	); err != nil {
		t.Fatalf(
			"RemoveTrack() returned error: %v",
			err,
		)
	}

	foundHike, err := repository.FindByID(
		ctx,
		hike.ID,
	)
	if err != nil {
		t.Fatalf(
			"Hike disappeared: %v",
			err,
		)
	}

	if got, want :=
		*foundHike.DistanceKM,
		distance; got != want {
		t.Errorf(
			"distance = %.1f, want preserved %.1f",
			got,
			want,
		)
	}

	_, err = repository.FindTrackByHikeID(
		ctx,
		hike.ID,
	)

	if !errors.Is(
		err,
		ErrTrackNotFound,
	) {
		t.Errorf(
			"FindTrackByHikeID() error = %v, want ErrTrackNotFound",
			err,
		)
	}

	var pointCount int

	err = repository.db.GetContext(
		ctx,
		&pointCount,
		`
		SELECT COUNT(*)
		FROM track_points
		WHERE track_id = ?
		`,
		track.ID,
	)
	if err != nil {
		t.Fatalf(
			"count track points: %v",
			err,
		)
	}

	if pointCount != 0 {
		t.Errorf(
			"point count = %d, want 0",
			pointCount,
		)
	}
}

// --- Helpers ---

func newTestRepository(t *testing.T, now func() time.Time) (*Repository, *sqlx.DB) {
	t.Helper()

	cfg := config.DatabaseConfig{
		Path:               filepath.Join(t.TempDir(), "traillog-test.db"),
		BusyTimeout:        5 * time.Second,
		ConnectTimeout:     5 * time.Second,
		MaxOpenConnections: 5,
		MaxIdleConnections: 5,
	}

	db, err := database.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	if _, err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	return newRepository(
		db,
		now,
		newNoopTracer(),
	), db
}
