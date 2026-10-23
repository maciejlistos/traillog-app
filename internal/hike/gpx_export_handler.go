package hike

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/maciejlistos/traillog/internal/gpx"
)

type gpxExportService interface {
	ExportTrack(
		context.Context,
		int64,
	) (TrackExport, error)
}

var _ gpxExportService = (*Service)(nil)

// GPXExportHandler serves GPX representataion of imported TrailLog routes.
type GPXExportHandler struct {
	service gpxExportService
}

// NewGPXExportHandler creates a GPX export HTTP handler.
func NewGPXExportHandler(
	service gpxExportService,
) *GPXExportHandler {
	return &GPXExportHandler{
		service: service,
	}
}

// RegisterRoutes registers GPX export routes on router.
func (h *GPXExportHandler) RegisterRoutes(
	router *gin.Engine,
) {
	router.GET(
		"/hikes/:id/export.gpx",
		h.export,
	)
}

func gpxTrackFromExport(
	export TrackExport,
) gpx.Track {
	storedSegments := groupTrackPoints(export.Points)

	segments := make([]gpx.Segment, 0, len(storedSegments))

	for _, storedSegment := range storedSegments {
		points := make([]gpx.Point, 0, len(storedSegment))

		for _, storedPoint := range storedSegment {
			points = append(
				points,
				gpx.Point{
					Latitude:  storedPoint.Latitude,
					Longitude: storedPoint.Longitude,
					Elevation: storedPoint.Elevation,
					Time:      storedPoint.RecordedAt,
				},
			)
		}

		segments = append(segments, gpx.Segment{Points: points})
	}

	return gpx.Track{
		Name:        export.Hike.Name,
		Description: export.Hike.Description,
		Segments:    segments,
	}
}

func (h *GPXExportHandler) export(
	c *gin.Context,
) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		renderWebError(
			c,
			http.StatusBadRequest,
			"Invalid hike",
			"The hike ID is not valid.",
			nil,
		)

		return
	}

	export, err := h.service.ExportTrack(
		c.Request.Context(),
		id,
	)
	if err != nil {
		switch {
		case errors.Is(
			err,
			ErrNotFound,
		):
			renderWebError(
				c,
				http.StatusNotFound,
				"Hike not found",
				"The requested hike does not exists.",
				nil,
			)

		case errors.Is(
			err,
			ErrTrackNotFound,
		):
			renderWebError(
				c,
				http.StatusNotFound,
				"GPX route not found",
				"This hike does not have an imported GPX route.",
				nil,
			)

		default:
			renderWebError(
				c,
				http.StatusInternalServerError,
				"Unable to export GPX",
				"TrailLog could not export this route.",
				err,
			)
		}

		return
	}

	document := gpxTrackFromExport(export)

	filename := fmt.Sprintf("traillog-hike-%d.gpx", export.Hike.ID)

	disposition := mime.FormatMediaType(
		"attachment",
		map[string]string{
			"filename": filename,
		},
	)

	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.Header("Content-Disposition", disposition)
	c.Header("X-Content-Type-Options", "nosniff")

	if err := gpx.Write(
		c.Writer,
		document,
	); err != nil {
		_ = c.Error(
			fmt.Errorf("write GPX response %w", err),
		)
		return
	}
}
