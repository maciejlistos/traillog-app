package hike

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/maciejlistos/traillog/internal/middleware"
)

type hikeService interface {
	Create(
		context.Context,
		CreateParams,
	) (Hike, error)

	FindByID(
		context.Context,
		int64,
	) (Hike, error)

	List(
		context.Context,
	) ([]Hike, error)

	Search(
		context.Context,
		ListParams,
	) (ListResult, error)

	Update(
		context.Context,
		int64,
		UpdateParams,
	) (Hike, error)

	Delete(
		context.Context,
		int64,
	) error

	AddVisit(
		context.Context,
		int64,
		CreateVisitParams,
	) (Visit, error)

	ListVisits(
		context.Context,
		int64,
	) ([]Visit, error)

	SetTags(
		context.Context,
		int64,
		[]string,
	) ([]Tag, error)

	ListTags(
		context.Context,
	) ([]Tag, error)

	ListHikeTags(
		context.Context,
		int64,
	) ([]Tag, error)

	ImportTrack(
		context.Context,
		ImportTrackParams,
	) (Hike, Track, error)

	TrackVisualization(
		context.Context,
		int64,
	) (*TrackVisualization, error)

	SetTrack(
		context.Context,
		int64,
		SetTrackParams,
	) (Track, error)

	RemoveTrack(
		context.Context,
		int64,
	) error
}

var _ hikeService = (*Service)(nil)

// Handler serves the TrailLog JSON API.
type Handler struct {
	service hikeService
}

// NewHandler creates a JSON API handler backed by service.
func NewHandler(service hikeService) *Handler {
	return &Handler{
		service: service,
	}
}

// RegisterRoutes registers the hike API endpoints on router.
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api")

	api.GET("/hikes", h.list)
	api.GET("/hikes/:id", h.get)
	api.POST("/hikes", h.create)
	api.PUT("/hikes/:id", h.update)
	api.DELETE("/hikes/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	hikes, err := h.service.List(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}

	response := make([]hikeResponse, 0, len(hikes))

	for _, hike := range hikes {
		response = append(response, toHikeResponse(hike))
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) get(c *gin.Context) {
	id, ok := parseHikeID(c)
	if !ok {
		return
	}

	hike, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, toHikeResponse(hike))
}

func (h *Handler) create(c *gin.Context) {
	var request createHikeRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid JSON request", RequestID: requestID(c)})
		return
	}

	created, err := h.service.Create(
		c.Request.Context(),
		CreateParams{
			Name:        request.Name,
			Location:    request.Location,
			Description: request.Description,
			DistanceKM:  request.DistanceKM,
			Status:      Status(request.Status),
		},
	)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	location := fmt.Sprintf("/api/hikes/%d", created.ID)

	c.Header("Location", location)

	c.JSON(
		http.StatusCreated,
		toHikeResponse(created),
	)
}

func (h *Handler) update(c *gin.Context) {
	id, ok := parseHikeID(c)
	if !ok {
		return
	}

	var request updateHikeRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			errorResponse{Error: "invalid JSON request", RequestID: requestID(c)},
		)
		return
	}

	updated, err := h.service.Update(
		c.Request.Context(),
		id,
		UpdateParams{
			Name:        request.Name,
			Location:    request.Location,
			Description: request.Description,
			DistanceKM:  request.DistanceKM,
			Status:      Status(request.Status),
		},
	)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	c.JSON(
		http.StatusOK,
		toHikeResponse(updated),
	)
}

func (h *Handler) delete(c *gin.Context) {
	id, ok := parseHikeID(c)
	if !ok {
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		writeServiceError(c, err)
	}

	c.Status(http.StatusNoContent)
}

// --- Helpers ---

func parseHikeID(c *gin.Context) (int64, bool) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid hike ID", RequestID: requestID(c)})
		return 0, false
	}

	return id, true
}

func requestID(c *gin.Context) string {
	return middleware.RequestIDFromGin(c)
}

func writeServiceError(c *gin.Context, err error) {
	var validationError *ValidationError

	switch {
	case errors.As(err, &validationError):
		c.JSON(http.StatusUnprocessableEntity, errorResponse{Error: "validation failed", Fields: validationError.Fields, RequestID: requestID(c)})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, errorResponse{Error: "hike not found", RequestID: requestID(c)})
	default:
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal service error", RequestID: requestID(c)})
	}
}

func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse ID %q: %w", value, err)
	}

	if id <= 0 {
		return 0, fmt.Errorf("ID must be positive: %d", id)
	}

	return id, nil
}
