package hike

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type elevationChartData struct {
	Paths []string

	MinElevationM float64
	MaxElevationM float64
	DistanceKM    float64
}

type trackPanelData struct {
	Hike Hike

	Visualization *TrackVisualization
	Chart         *elevationChartData

	UploadError     string
	UploadSucceeded bool

	Map MapOptions
}

type trackPageData struct {
	Title string
	Panel trackPanelData
}

func buildElevationChart(
	profile *ElevationProfile,
) *elevationChartData {
	if profile == nil ||
		len(profile.Segments) == 0 {

		return nil
	}

	const (
		width = 800.0

		height = 220.0

		leftPadding = 46.0

		rightPadding = 16.0

		topPadding = 16.0

		bottomPadding = 32.0
	)

	plotWidth :=
		width -
			leftPadding -
			rightPadding

	plotHeight :=
		height -
			topPadding -
			bottomPadding

	elevationRange :=
		profile.MaxElevationM -
			profile.MinElevationM

	if elevationRange < 1 {
		elevationRange = 1
	}

	distance := profile.DistanceKM

	if distance <= 0 {
		distance = 1
	}

	paths := make(
		[]string,
		0,
		len(profile.Segments),
	)

	for _, segment := range profile.Segments {

		if len(segment.Samples) == 0 {
			continue
		}

		points := make(
			[]string,
			0,
			len(segment.Samples),
		)

		for _, sample := range segment.Samples {

			x :=
				leftPadding +
					(sample.DistanceKM/
						distance)*
						plotWidth

			y :=
				topPadding +
					(1-
						(sample.ElevationM-
							profile.MinElevationM)/
							elevationRange)*
						plotHeight

			points = append(
				points,
				fmt.Sprintf(
					"%.1f,%.1f",
					x,
					y,
				),
			)
		}

		paths = append(
			paths,
			strings.Join(
				points,
				" ",
			),
		)
	}

	return &elevationChartData{
		Paths: paths,

		MinElevationM: profile.MinElevationM,

		MaxElevationM: profile.MaxElevationM,

		DistanceKM: profile.DistanceKM,
	}
}

func (h *WebHandler) trackPage(
	c *gin.Context,
) {
	varyOnHXRequest(c)

	id, err := parseID(
		c.Param("id"),
	)
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

	found, err := h.service.FindByID(
		c.Request.Context(),
		id,
	)
	if err != nil {
		if errors.Is(
			err,
			ErrNotFound,
		) {
			renderWebError(
				c,
				http.StatusNotFound,
				"Hike not found",
				"The requested hike does not exist.",
				nil,
			)

			return
		}

		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to load hike",
			"TrailLog could not load this hike.",
			err,
		)

		return
	}

	visualization, err := h.service.TrackVisualization(
		c.Request.Context(),
		id,
	)
	if err != nil {
		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to load route",
			"TrailLog could not load this GPX route.",
			err,
		)

		return
	}

	panel, err := h.loadTrackPanelData(
		c.Request.Context(),
		found,
	)
	if err != nil {
		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to load route",
			"TrailLog could not load this GPX route.",
			err,
		)

		return
	}

	if visualization != nil {
		panel.Chart = buildElevationChart(
			visualization.Elevation,
		)
	}

	if isHTMXRequest(c) {
		c.HTML(
			http.StatusOK,
			"hikes/track-panel.html",
			panel,
		)

		return
	}

	c.HTML(
		http.StatusOK,
		"hikes/track.html",
		trackPageData{
			Title: found.Name + " route",
			Panel: panel,
		},
	)
}

func (h *WebHandler) loadTrackPanelData(
	ctx context.Context,
	found Hike,
) (trackPanelData, error) {
	visualization, err := h.service.TrackVisualization(
		ctx,
		found.ID,
	)
	if err != nil {
		return trackPanelData{},
			err
	}

	data := trackPanelData{
		Hike: found,

		Visualization: visualization,

		Map: h.options.Map,
	}

	if visualization != nil {
		data.Chart = buildElevationChart(
			visualization.Elevation,
		)
	}

	return data, nil
}

func (h *WebHandler) renderTrackPanel(
	c *gin.Context,
	status int,
	panel trackPanelData,
) {
	varyOnHXRequest(c)

	if isHTMXRequest(c) {
		c.HTML(
			status,
			"hikes/track-panel.html",
			panel,
		)

		return
	}

	c.HTML(
		status,
		"hikes/track.html",
		trackPageData{
			Title: panel.Hike.Name +
				" route",

			Panel: panel,
		},
	)
}

func (h *WebHandler) setTrackFromUpload(
	c *gin.Context,
) {
	id, err := parseID(
		c.Param("id"),
	)
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

	found, err := h.service.FindByID(
		c.Request.Context(),
		id,
	)
	if err != nil {
		if errors.Is(
			err,
			ErrNotFound,
		) {
			renderWebError(
				c,
				http.StatusNotFound,
				"Hike not found",
				"The requested hike does not exist.",
				nil,
			)

			return
		}

		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to load hike",
			"TrailLog could not load this hike.",
			err,
		)

		return
	}

	upload, err := parseGPXUpload(c, h.options.GPX)
	if err != nil {
		panel, panelErr := h.loadTrackPanelData(
			c.Request.Context(),
			found,
		)

		if panelErr != nil {
			renderWebError(
				c,
				http.StatusInternalServerError,
				"Unable to load route",
				"TrailLog could not load the existing route.",
				panelErr,
			)

			return
		}

		panel.UploadError = gpxUploadErrorMessage(
			err,
		)

		var maxBytesError *http.MaxBytesError

		status := http.StatusUnprocessableEntity

		if errors.As(
			err,
			&maxBytesError,
		) {
			status = http.StatusRequestEntityTooLarge

			panel.UploadError = "GPX upload is too large"
		}

		h.renderTrackPanel(
			c,
			status,
			panel,
		)

		return
	}

	if !upload.FileFound {
		panel, panelErr := h.loadTrackPanelData(
			c.Request.Context(),
			found,
		)

		if panelErr != nil {
			renderWebError(
				c,
				http.StatusInternalServerError,
				"Unable to load route",
				"TrailLog could not load the existing route.",
				panelErr,
			)

			return
		}

		panel.UploadError = "Select a GPX file"

		h.renderTrackPanel(
			c,
			http.StatusUnprocessableEntity,
			panel,
		)

		return
	}

	_, err = h.service.SetTrack(
		c.Request.Context(),
		id,
		setTrackParamsFromUpload(
			upload,
		),
	)
	if err != nil {
		var validationError *ValidationError

		if errors.As(
			err,
			&validationError,
		) {
			panel, panelErr := h.loadTrackPanelData(
				c.Request.Context(),
				found,
			)

			if panelErr != nil {
				renderWebError(
					c,
					http.StatusInternalServerError,
					"Unable to load route",
					"TrailLog could not load the existing route.",
					panelErr,
				)

				return
			}

			panel.UploadError = validationError.
				Fields["file"]

			h.renderTrackPanel(
				c,
				http.StatusUnprocessableEntity,
				panel,
			)

			return
		}

		if errors.Is(
			err,
			ErrNotFound,
		) {
			renderWebError(
				c,
				http.StatusNotFound,
				"Hike not found",
				"The requested hike does not exist.",
				nil,
			)

			return
		}

		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to replace GPX",
			"TrailLog could not replace this route.",
			err,
		)

		return
	}

	if !isHTMXRequest(c) {
		c.Redirect(
			http.StatusSeeOther,
			fmt.Sprintf(
				"/hikes/%d",
				id,
			),
		)

		return
	}

	// Reload both Hike metadata and route because SetTrack
	// updated Hike.DistanceKM.
	found, err = h.service.FindByID(
		c.Request.Context(),
		id,
	)
	if err != nil {
		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to reload hike",
			"TrailLog saved the route but could not reload the hike.",
			err,
		)

		return
	}

	panel, err := h.loadTrackPanelData(
		c.Request.Context(),
		found,
	)
	if err != nil {
		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to reload route",
			"TrailLog saved the route but could not reload it.",
			err,
		)

		return
	}

	panel.UploadSucceeded = true

	h.renderTrackPanel(
		c,
		http.StatusOK,
		panel,
	)
}

func (h *WebHandler) removeTrackFromForm(
	c *gin.Context,
) {
	id, err := parseID(
		c.Param("id"),
	)
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

	err = h.service.RemoveTrack(
		c.Request.Context(),
		id,
	)
	if err != nil {
		if errors.Is(
			err,
			ErrTrackNotFound,
		) {
			renderWebError(
				c,
				http.StatusNotFound,
				"GPX route not found",
				"This hike does not have a GPX route.",
				nil,
			)

			return
		}

		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to remove GPX",
			"TrailLog could not remove this route.",
			err,
		)

		return
	}

	c.Redirect(
		http.StatusSeeOther,
		fmt.Sprintf(
			"/hikes/%d",
			id,
		),
	)
}

func (h *WebHandler) removeTrackFromHTMX(
	c *gin.Context,
) {
	id, err := parseID(
		c.Param("id"),
	)
	if err != nil {
		c.Status(
			http.StatusBadRequest,
		)

		return
	}

	err = h.service.RemoveTrack(
		c.Request.Context(),
		id,
	)
	if err != nil {
		switch {
		case errors.Is(
			err,
			ErrTrackNotFound,
		):
			c.Status(
				http.StatusNotFound,
			)

		default:
			_ = c.Error(err)

			c.Status(
				http.StatusInternalServerError,
			)
		}

		return
	}

	found, err := h.service.FindByID(
		c.Request.Context(),
		id,
	)
	if err != nil {
		_ = c.Error(err)

		c.Status(
			http.StatusInternalServerError,
		)

		return
	}

	panel, err := h.loadTrackPanelData(
		c.Request.Context(),
		found,
	)
	if err != nil {
		_ = c.Error(err)

		c.Status(
			http.StatusInternalServerError,
		)

		return
	}

	h.renderTrackPanel(
		c,
		http.StatusOK,
		panel,
	)
}
