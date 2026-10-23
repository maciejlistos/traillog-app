package hike

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type statisticsService interface {
	DashboardStats(
		context.Context,
	) (DashboardStats, error)
}

var _ statisticsService = (*Service)(nil)

// StatsHandler serves TrailLog's server-rendered statistics dashboard.
type StatsHandler struct {
	service statisticsService
}

// NewStatsHandler creates a statistics web handler.
func NewStatsHandler(
	service statisticsService,
) *StatsHandler {
	return &StatsHandler{
		service: service,
	}
}

// RegisterRoutes registers statistics routes on router.
func (h *StatsHandler) RegisterRoutes(
	router *gin.Engine,
) {
	router.GET("/stats", h.index)
}

func (h *StatsHandler) index(
	c *gin.Context,
) {
	stats, err := h.service.DashboardStats(c.Request.Context())
	if err != nil {
		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to load statistics",
			"TrailLog could not calculate the statistics",
			err,
		)

		return
	}

	c.HTML(
		http.StatusOK,
		"stats/index.html",
		statisticsPageData{
			Title:            "Statistics",
			Stats:            stats,
			MaxTagHikes:      maxTagUsage(stats.TopTags),
			MaxHikeVisits:    maxHikeVisitCount(stats.MostVisitedHikes),
			MaxMonthlyVisits: maxMonthlyVisitCount(stats.MonthlyVisits),
		},
	)
}

// ---------------------------------------------------------

type statisticsPageData struct {
	Title string
	Stats DashboardStats

	MaxTagHikes      int
	MaxHikeVisits    int
	MaxMonthlyVisits int
}

// ---- Helpers ----

func maxTagUsage(
	values []TagUsage,
) int {
	maximum := 0

	for _, value := range values {
		if value.HikeCount > maximum {
			maximum = value.HikeCount
		}
	}

	return maximum
}

func maxHikeVisitCount(
	values []HikeVisitCount,
) int {
	maximum := 0

	for _, value := range values {
		if value.VisitCount > maximum {
			maximum = value.VisitCount
		}
	}

	return maximum
}

func maxMonthlyVisitCount(
	values []MonthlyVisitCount,
) int {
	maximum := 0

	for _, value := range values {
		if value.VisitCount > maximum {
			maximum = value.VisitCount
		}
	}

	return maximum
}
