package hike

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/maciejlistos/traillog/internal/webui"
)

type fakeStatisticsService struct {
	dashboardStatsFunc func(
		context.Context,
	) (DashboardStats, error)
}

func (f *fakeStatisticsService) DashboardStats(
	ctx context.Context,
) (DashboardStats, error) {
	if f.dashboardStatsFunc == nil {
		panic("unexpected DashboardStats call")
	}

	return f.dashboardStatsFunc(ctx)
}

func newStatisticsTestRouter(
	t *testing.T,
	service statisticsService,
) http.Handler {
	t.Helper()

	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.SetFuncMap(
		webui.FuncMap(),
	)

	router.LoadHTMLFS(
		webui.Templates(),
		"**/*.html",
	)

	handler := NewStatsHandler(service)

	handler.RegisterRoutes(router)

	return router
}

func TestStatsHandlerRendersDashboard(
	t *testing.T,
) {
	averageRating := 4.5

	service := &fakeStatisticsService{
		dashboardStatsFunc: func(
			context.Context,
		) (DashboardStats, error) {
			return DashboardStats{
				TotalHikes: 12,

				VisitedHikes: 8,

				PlannedHikes: 4,

				TotalDistanceKM: 123.4,

				TotalVisits: 18,

				AverageRating: &averageRating,

				TopTags: []TagUsage{
					{
						Name: "forest",

						HikeCount: 7,
					},
				},

				MostVisitedHikes: []HikeVisitCount{
					{
						HikeID: 42,

						Name: "Forest Loop",

						VisitCount: 5,
					},
				},

				MonthlyVisits: []MonthlyVisitCount{
					{
						Month: "2026-08",

						VisitCount: 4,
					},
				},
			}, nil
		},
	}

	router := newStatisticsTestRouter(
		t,
		service,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/stats",
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

	body := response.Body.String()

	expected := []string{
		"Statistics",
		"123.4 km",
		"4.5 / 5",
		"#forest",
		"Forest Loop",
		"Aug 2026",
	}

	for _, value := range expected {
		if !strings.Contains(
			body,
			value,
		) {
			t.Errorf(
				"response does not contain %q",
				value,
			)
		}
	}
}
