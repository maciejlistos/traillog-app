package hike

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/maciejlistos/traillog/internal/gpx"
)

type gpxImportForm struct {
	Name     string
	Location string
	Status   string

	Errors map[string]string
}

type gpxImportPageData struct {
	Title string
	Form  gpxImportForm
}

type parsedGPXUpload struct {
	Form gpxImportForm

	Filename string
	Track    gpx.Track
	Summary  gpx.Summary

	FileFound bool
}

func (h *WebHandler) importGPXPage(
	c *gin.Context,
) {
	h.renderGPXImportPage(
		c,
		http.StatusOK,
		gpxImportForm{
			Status: StatusPlanned.String(),
			Errors: make(map[string]string),
		},
	)
}

func (h *WebHandler) renderGPXImportPage(
	c *gin.Context,
	status int,
	form gpxImportForm,
) {
	c.HTML(
		status,
		"hikes/import.html",
		gpxImportPageData{
			Title: "Import GPX",
			Form:  form,
		},
	)
}

func parseGPXUpload(
	c *gin.Context,
	limits GPXUploadLimits,
) (parsedGPXUpload, error) {
	c.Request.Body = http.MaxBytesReader(
		c.Writer,
		c.Request.Body,
		limits.MaxUploadBytes,
	)

	reader, err := c.Request.MultipartReader()
	if err != nil {
		return parsedGPXUpload{}, fmt.Errorf("create multipart reader: %w", err)
	}

	result := parsedGPXUpload{
		Form: gpxImportForm{
			Status: StatusPlanned.String(),
			Errors: make(map[string]string),
		},
	}

	for {
		part, err := reader.NextPart()

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return parsedGPXUpload{}, fmt.Errorf("read multipart part: %w", err)
		}

		switch part.FormName() {

		case "name":
			value, err := readTextPart(part, limits.MaxTextFieldBytes)
			if err != nil {
				part.Close()
				return parsedGPXUpload{}, err
			}

			result.Form.Name = strings.TrimSpace(
				value,
			)

		case "location":
			value, err := readTextPart(part, limits.MaxTextFieldBytes)
			if err != nil {
				part.Close()
				return parsedGPXUpload{}, err
			}

			result.Form.Location = strings.TrimSpace(
				value,
			)

		case "status":
			value, err := readTextPart(part, limits.MaxTextFieldBytes)
			if err != nil {
				part.Close()
				return parsedGPXUpload{}, err
			}

			result.Form.Status = strings.TrimSpace(
				value,
			)

		case "file":
			if result.FileFound {
				part.Close()

				return parsedGPXUpload{},
					fmt.Errorf(
						"only one GPX file may be uploaded",
					)
			}

			filename := strings.TrimSpace(
				part.FileName(),
			)

			if filename == "" {
				part.Close()
				continue
			}

			result.Filename = filepath.Base(
				filename,
			)

			track, err := gpx.Parse(
				part,
				limits.MaxTrackPoints,
			)
			if err != nil {
				part.Close()

				return parsedGPXUpload{},
					fmt.Errorf(
						"parse GPX: %w",
						err,
					)
			}

			summary, err := gpx.Summarize(
				track,
			)
			if err != nil {
				part.Close()

				return parsedGPXUpload{},
					fmt.Errorf(
						"summarize GPX: %w",
						err,
					)
			}

			result.Track = track

			result.Summary = summary

			result.FileFound = true
		}

		if err := part.Close(); err != nil {
			return parsedGPXUpload{},
				fmt.Errorf(
					"close multipart part: %w",
					err,
				)
		}
	}

	return result, nil
}

func readTextPart(
	reader io.Reader,
	maxBytes int64,
) (string, error) {
	limited := io.LimitReader(reader, maxBytes+1)

	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read from field: %w", err)
	}

	if int64(len(data)) > maxBytes {
		return "", fmt.Errorf("form field is too large")
	}

	return string(data), nil
}

func trackPointsFromGPX(
	track gpx.Track,
) []TrackPoint {
	pointCount := 0

	for _, segment := range track.Segments {
		pointCount += len(segment.Points)
	}

	points := make(
		[]TrackPoint,
		0,
		pointCount,
	)

	for segmentIndex, segment := range track.Segments {
		for _, point := range segment.Points {
			points = append(
				points,
				TrackPoint{
					SegmentIndex: segmentIndex,

					Latitude: point.Latitude,

					Longitude: point.Longitude,

					Elevation: point.Elevation,

					RecordedAt: point.Time,
				},
			)
		}
	}

	return points
}

func importedHikeName(
	formName string,
	trackName string,
	filename string,
) string {
	if value := strings.TrimSpace(
		formName,
	); value != "" {
		return value
	}

	if value := strings.TrimSpace(
		trackName,
	); value != "" {
		return value
	}

	base := filepath.Base(
		filename,
	)

	extension := filepath.Ext(base)

	return strings.TrimSpace(
		strings.TrimSuffix(
			base,
			extension,
		),
	)
}

func (h *WebHandler) importGPX(
	c *gin.Context,
) {
	upload, err := parseGPXUpload(c, h.options.GPX)
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			form := gpxImportForm{
				Status: StatusPlanned.String(),

				Errors: map[string]string{
					"file": "GPX upload is too large",
				},
			}

			h.renderGPXImportPage(
				c,
				http.StatusRequestEntityTooLarge,
				form,
			)

			return
		}

		form := gpxImportForm{
			Status: StatusPlanned.String(),

			Errors: map[string]string{
				"file": gpxUploadErrorMessage(
					err,
				),
			},
		}

		h.renderGPXImportPage(
			c,
			http.StatusUnprocessableEntity,
			form,
		)

		return
	}

	form := upload.Form

	if !upload.FileFound {
		form.Errors["file"] = "Select a GPX file"

		h.renderGPXImportPage(
			c,
			http.StatusUnprocessableEntity,
			form,
		)

		return
	}

	name := importedHikeName(
		form.Name,
		upload.Track.Name,
		upload.Filename,
	)

	distance := upload.Summary.DistanceKM

	params := ImportTrackParams{
		Hike: CreateParams{
			Name: name,

			Location: form.Location,

			Description: upload.Track.Description,

			DistanceKM: &distance,

			Status: Status(form.Status),
		},

		SourceFilename: upload.Filename,

		Points: trackPointsFromGPX(
			upload.Track,
		),

		DistanceKM: upload.Summary.DistanceKM,

		ElevationGainM: upload.Summary.ElevationGainM,

		Duration: upload.Summary.Duration,

		StartLat: upload.Summary.Start.Latitude,

		StartLon: upload.Summary.Start.Longitude,

		EndLat: upload.Summary.End.Latitude,

		EndLon: upload.Summary.End.Longitude,
	}

	created, _, err := h.service.ImportTrack(
		c.Request.Context(),
		params,
	)
	if err != nil {
		if validationError, ok := errors.AsType[*ValidationError](err); ok {
			form.Errors = validationError.Fields

			h.renderGPXImportPage(
				c,
				http.StatusUnprocessableEntity,
				form,
			)

			return
		}

		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to import GPX",
			"TrailLog could not import this GPX route.",
			err,
		)

		return
	}

	c.Redirect(
		http.StatusSeeOther,
		"/hikes/"+
			strconv.FormatInt(
				created.ID,
				10,
			),
	)
}

func gpxUploadErrorMessage(
	err error,
) string {
	switch {
	case errors.Is(err, gpx.ErrNoTrack):
		return "The GPX file does not contain a track"

	case errors.Is(
		err,
		gpx.ErrMultipleTracks,
	):
		return "TrailLog currently imports one GPX track per file"

	case errors.Is(
		err,
		gpx.ErrTooManyPoints,
	):
		return "The GPX track contains too many points"

	default:
		return "The uploaded file is not a valid supported GPX 1.1 track"
	}
}

func setTrackParamsFromUpload(
	upload parsedGPXUpload,
) SetTrackParams {
	return SetTrackParams{
		SourceFilename: upload.Filename,

		Points: trackPointsFromGPX(
			upload.Track,
		),

		DistanceKM: upload.Summary.DistanceKM,

		ElevationGainM: upload.Summary.ElevationGainM,

		Duration: upload.Summary.Duration,

		StartLat: upload.Summary.Start.Latitude,

		StartLon: upload.Summary.Start.Longitude,

		EndLat: upload.Summary.End.Latitude,

		EndLon: upload.Summary.End.Longitude,
	}
}
