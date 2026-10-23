package hike

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maciejlistos/traillog/internal/middleware"
)

// GPXUploadLimits contains resource limits appliced by the GPX HTTP layer.
type GPXUploadLimits struct {
	MaxUploadBytes    int64
	MaxTrackPoints    int
	MaxTextFieldBytes int64
}

// MapOptions contains browser map configuration.
type MapOptions struct {
	TileURL     string
	Attribution string
}

// WebHandlerOptions contains configuration used by TrailLog's HTML handlers.
type WebHandlerOptions struct {
	GPX GPXUploadLimits
	Map MapOptions
}

type hikeCreateSuccessData struct {
	Hike Hike
}

// WebHandler serves TrailLog's HTML and HTMX endpoints.
type WebHandler struct {
	service hikeService

	options WebHandlerOptions
}

// NewWebHandler creates a web handler backed by service.
func NewWebHandler(
	service hikeService,
	options WebHandlerOptions,
) *WebHandler {
	return &WebHandler{
		service: service,
		options: options,
	}
}

// RegisterRoutes registers the hike web endpoints on router.
func (h *WebHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/hikes", h.listPage)
	router.GET("/hikes/new", h.newPage)
	router.POST("/hikes", h.createFromForm)
	router.GET("/hikes/import", h.importGPXPage)
	router.POST("/hikes/import", h.importGPX)
	router.GET("/hikes/:id/edit", h.editPage)

	router.GET("/hikes/:id/track", h.trackPage)
	router.POST("/hikes/:id/track", h.setTrackFromUpload)
	router.PUT("/hikes/:id/track", h.setTrackFromUpload)
	router.POST("/hikes/:id/track/delete", h.removeTrackFromForm)
	router.DELETE("/hikes/:id/track", h.removeTrackFromHTMX)

	router.GET("/hikes/:id", h.showPage)
	router.POST("/hikes/:id", h.updateFromForm)
	router.PUT("/hikes/:id", h.updateFromForm)
	router.DELETE("/hikes/:id", h.deleteFromHTMX)

	router.POST("/hikes/:id/delete", h.deleteFromForm)
	router.POST("/hikes/:id/visits", h.addVisit)
	router.POST("/hikes/:id/tags", h.setTags)
}

type hikeListFilters struct {
	Query  string
	Status string
	Sort   string
}

type hikeListPageData struct {
	Title   string
	Hikes   []Hike
	Filters hikeListFilters

	Total      int
	Page       int
	TotalPages int

	PreviousURL string
	NextURL     string

	HasFilters bool

	AvailableTags []Tag
	SelectedTags  map[string]bool
}

type hikeShowPageData struct {
	Title  string
	Hike   Hike
	Visits []Visit

	VisitForm visitForm

	TagsPanel hikeTagsPanelData
}

type visitForm struct {
	VisitedAt string
	Weather   string
	Notes     string
	Rating    string

	Errors map[string]string
}

type hikeFormPageData struct {
	Title       string
	Heading     string
	Intro       string
	Mode        string
	Action      string
	CancelURL   string
	SubmitLabel string
	PanelID     string
	Form        hikeForm
	HTMX        bool
}

type hikeForm struct {
	Name        string
	Location    string
	Description string
	DistanceKM  string
	Status      string
	Errors      map[string]string
}

type hikeTagsForm struct {
	Tags   string
	Errors map[string]string
}

type hikeTagsPanelData struct {
	HikeID int64
	Tags   []Tag
	Form   hikeTagsForm
}

type webErrorPageData struct {
	Title     string
	Message   string
	RequestID string
}

func (h *WebHandler) listPage(c *gin.Context) {
	varyOnHXRequest(c)

	params, err := listParamsFromRequest(c)
	if err != nil {
		renderWebError(
			c,
			http.StatusBadRequest,
			"Invalid filters",
			"The requested filters are not valid.",
			nil,
		)
		return
	}

	result, err := h.service.Search(
		c.Request.Context(),
		params,
	)
	if err != nil {
		if _, ok := errors.AsType[*ValidationError](err); ok {
			renderWebError(
				c,
				http.StatusBadRequest,
				"Invalid filters",
				"The requested filters are not valid.",
				nil,
			)
			return
		}

		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to load hikes",
			"TrailLog could not load your hikes.",
			err,
		)
		return
	}

	data := listPageData(
		params,
		result,
	)

	if !isHTMXRequest(c) {
		tags, err := h.service.ListTags(c.Request.Context())
		if err != nil {
			renderWebError(
				c,
				http.StatusInternalServerError,
				"Unable to load tags",
				"TrailLog could not load the available tags.",
				err,
			)
			return
		}

		data.AvailableTags = tags

		c.HTML(
			http.StatusOK,
			"hikes/index.html",
			data,
		)

		return
	}

	c.HTML(
		http.StatusOK,
		"hikes/list-results.html",
		data,
	)
}

func (h *WebHandler) newPage(c *gin.Context) {
	varyOnHXRequest(c)

	htmx := isHTMXRequest(c)

	form := hikeForm{
		Status: StatusPlanned.String(),
		Errors: make(map[string]string),
	}

	data := createFormPageData(form, htmx)

	if htmx {
		c.HTML(
			http.StatusOK,
			"hikes/form-panel.html",
			data,
		)
		return
	}

	c.HTML(
		http.StatusOK,
		"hikes/new.html",
		data,
	)
}

func (h *WebHandler) createFromForm(c *gin.Context) {
	form := formFromRequest(c)

	distance, err := parseFromDistance(form.DistanceKM)
	if err != nil {
		form.Errors["distance_km"] = "Distance must be a number"

		h.renderNewForm(c, http.StatusUnprocessableEntity, form)

		return
	}

	created, err := h.service.Create(
		c.Request.Context(),
		CreateParams{
			Name:        form.Name,
			Location:    form.Location,
			Description: form.Description,
			DistanceKM:  distance,
			Status:      Status(form.Status),
		},
	)
	if err != nil {
		if validationError, ok := errors.AsType[*ValidationError](err); ok {
			form.Errors = validationError.Fields

			h.renderNewForm(c, http.StatusUnprocessableEntity, form)

			return
		}

		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to save hike",
			"TrailLog could not save this hike.",
			err,
		)

		return
	}

	if isHTMXRequest(c) {
		varyOnHXRequest(c)

		c.HTML(
			http.StatusOK,
			"hikes/create-success.html",
			hikeCreateSuccessData{
				Hike: created,
			},
		)

		return
	}

	c.Redirect(
		http.StatusSeeOther,
		"/hikes/"+strconv.FormatInt(created.ID, 10),
	)
}

func (h *WebHandler) showPage(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		renderWebError(
			c,
			http.StatusBadRequest,
			"Invalid hike",
			"The hike ID is not valid",
			nil,
		)
	}

	found, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			renderWebError(
				c,
				http.StatusNotFound,
				"Hike not found",
				"The requested hike does not exists.",
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

	visits, err := h.service.ListVisits(
		c.Request.Context(),
		id,
	)
	if err != nil {
		renderWebError(
			c,
			http.StatusInternalServerError,
			"unable to load visits",
			"TrailLog could not load the visit history.",
			err,
		)

		return
	}

	tags, err := h.service.ListHikeTags(c.Request.Context(), id)
	if err != nil {
		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to load tags",
			"TrailLog could not load this hike's tags.",
			err,
		)
	}

	c.HTML(
		http.StatusOK,
		"hikes/show.html",
		hikeShowPageData{
			Title:  found.Name,
			Hike:   found,
			Visits: visits,
			VisitForm: visitForm{
				Errors: make(map[string]string),
			},
			TagsPanel: hikeTagsPanelData{
				HikeID: id,
				Tags:   tags,
				Form: hikeTagsForm{
					Tags:   tagInputValue(tags),
					Errors: make(map[string]string),
				},
			},
		},
	)
}

func (h *WebHandler) setTags(
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

	rawValue := c.PostForm("tags")

	names := parseTagInput(
		rawValue,
	)

	tags, err := h.service.SetTags(
		c.Request.Context(),
		id,
		names,
	)
	if err != nil {
		if validationError, ok := errors.AsType[*ValidationError](err); ok {
			data := hikeTagsPanelData{
				HikeID: id,
				Form: hikeTagsForm{
					Tags:   rawValue,
					Errors: validationError.Fields,
				},
			}

			c.HTML(
				http.StatusUnprocessableEntity,
				"hikes/tags-panel.html",
				data,
			)

			return
		}

		if errors.Is(err, ErrNotFound) {
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
			"Unable to save tags",
			"TrailLog could not update this hike's tags.",
			err,
		)

		return
	}

	if isHTMXRequest(c) {
		c.HTML(
			http.StatusOK,
			"hikes/tags-panel.html",
			hikeTagsPanelData{
				HikeID: id,
				Tags:   tags,
				Form: hikeTagsForm{
					Tags:   tagInputValue(tags),
					Errors: make(map[string]string),
				},
			},
		)

		return
	}

	c.Redirect(
		http.StatusSeeOther,
		"/hikes/"+strconv.FormatInt(id, 10),
	)
}

func (h *WebHandler) renderNewForm(c *gin.Context, status int, form hikeForm) {
	varyOnHXRequest(c)

	htmx := isHTMXRequest(c)

	data := createFormPageData(form, htmx)

	if htmx {
		c.HTML(
			status,
			"hikes/form-panel.html",
			data,
		)
		return
	}

	c.HTML(
		status,
		"hikes/new.html",
		data,
	)
}

func (h *WebHandler) editPage(c *gin.Context) {
	varyOnHXRequest(c)

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

	found, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			renderWebError(
				c,
				http.StatusNotFound,
				"Hike not found",
				"The request hike does not exists.",
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

	htmx := isHTMXRequest(c)

	data := editFormPageData(id, formFromHike(found), htmx)

	if htmx {
		c.HTML(
			http.StatusOK,
			"hikes/form-panel.html",
			data,
		)
		return
	}

	c.HTML(
		http.StatusOK,
		"hikes/edit.html",
		data,
	)
}

func (h *WebHandler) renderEditForm(c *gin.Context, status int, id int64, form hikeForm) {
	varyOnHXRequest(c)

	htmx := isHTMXRequest(c)

	data := editFormPageData(id, form, htmx)

	if htmx {
		c.HTML(
			status,
			"hikes/form-panel.html",
			data,
		)

		return
	}

	c.HTML(
		status,
		"hikes/edit.html",
		data,
	)
}

func (h *WebHandler) updateFromForm(c *gin.Context) {
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

	form := formFromRequest(c)

	distance, err := parseFromDistance(form.DistanceKM)
	if err != nil {
		form.Errors["distance_km"] = "Distance must be a number"

		h.renderEditForm(
			c,
			http.StatusUnprocessableEntity,
			id,
			form,
		)

		return
	}

	update, err := h.service.Update(
		c.Request.Context(),
		id,
		UpdateParams{
			Name:        form.Name,
			Location:    form.Location,
			Description: form.Description,
			DistanceKM:  distance,
			Status:      Status(form.Status),
		},
	)
	if err != nil {
		if validationError, ok := errors.AsType[*ValidationError](err); ok {
			form.Errors = validationError.Fields

			h.renderEditForm(
				c,
				http.StatusUnprocessableEntity,
				id,
				form,
			)

			return
		}

		if errors.Is(err, ErrNotFound) {
			renderWebError(
				c,
				http.StatusInternalServerError,
				"Unable to update hike",
				"TrailLog could not update this hike.",
				err,
			)

			return
		}
	}

	if isHTMXRequest(c) {
		varyOnHXRequest(c)

		c.HTML(
			http.StatusOK,
			"hikes/detail.html",
			hikeShowPageData{
				Title: update.Name,
				Hike:  update,
			},
		)

		return
	}

	c.Redirect(
		http.StatusSeeOther,
		"/hikes/"+strconv.FormatInt(update.ID, 10),
	)
}

func (h *WebHandler) deleteFromForm(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		renderWebError(
			c,
			http.StatusBadRequest,
			"Invalid hike",
			"The hike ID is not null",
			nil,
		)

		return
	}

	err = h.service.Delete(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			renderWebError(
				c,
				http.StatusNotFound,
				"Hike not found",
				"The requested hike does not exists",
				err,
			)

			return
		}

		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to delete hike",
			"TrailLog could not delete this hike.",
			err,
		)

		return
	}

	c.Redirect(http.StatusSeeOther, "/hikes")
}

func (h *WebHandler) deleteFromHTMX(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	err = h.service.Delete(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.Status(http.StatusNotFound)
			return
		}

		_ = c.Error(err)

		c.Status(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
}

func (h *WebHandler) addVisit(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		renderWebError(
			c,
			http.StatusBadRequest,
			"Invalid hike",
			"The hike ID is not valid.",
			err,
		)

		return
	}

	form := visitFormFromRequest(c)

	params, parsringError := parseVisitForm(form)

	if len(parsringError) > 0 {
		form.Errors = parsringError

		h.renderVisitFormError(
			c,
			id,
			form,
		)

		return
	}

	_, err = h.service.AddVisit(
		c.Request.Context(),
		id,
		params,
	)
	if err != nil {
		if validationError, ok := errors.AsType[*ValidationError](err); ok {
			form.Errors = validationError.Fields

			h.renderVisitFormError(
				c,
				id,
				form,
			)

			return
		}

		if errors.Is(err, ErrNotFound) {
			renderWebError(
				c,
				http.StatusNotFound,
				"Hike not found",
				"The requested hike does not exists.",
				nil,
			)

			return
		}

		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to add visit",
			"TrailLog could not save this visit.",
			err,
		)

		return
	}

	c.Redirect(
		http.StatusSeeOther,
		"/hikes/"+strconv.FormatInt(id, 10),
	)
}

// --- Helpers ---

const hxRequestHeader = "HX-Request"

func isHTMXRequest(c *gin.Context) bool {
	return strings.EqualFold(c.GetHeader(hxRequestHeader), "true")
}

func varyOnHXRequest(c *gin.Context) {
	c.Writer.Header().Add("Vary", hxRequestHeader)
}

func renderWebError(
	c *gin.Context,
	status int,
	title string,
	message string,
	err error,
) {
	if err != nil {
		_ = c.Error(err)
	}

	c.HTML(
		status,
		"error.html",
		webErrorPageData{
			Title:     title,
			Message:   message,
			RequestID: middleware.RequestIDFromGin(c),
		},
	)
}

func formFromRequest(c *gin.Context) hikeForm {
	request := c.Request
	return hikeForm{
		Name:        strings.TrimSpace(request.PostFormValue("name")),
		Location:    strings.TrimSpace(request.PostFormValue("location")),
		Description: strings.TrimSpace(request.PostFormValue("description")),
		DistanceKM:  strings.TrimSpace(request.PostFormValue("distance_km")),
		Status:      strings.TrimSpace(request.PostFormValue("status")),
		Errors:      make(map[string]string),
	}
}

func parseFromDistance(value string) (*float64, error) {
	if value == "" {
		return nil, nil
	}

	distance, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, err
	}

	return &distance, nil
}

func createFormPageData(form hikeForm, htmx bool) hikeFormPageData {
	return hikeFormPageData{
		Title:       "Add hike",
		Heading:     "Add a hike",
		Intro:       "Record a route of place you want to explore.",
		Mode:        "create",
		Action:      "/hikes",
		CancelURL:   "/hikes",
		SubmitLabel: "Save hike",
		PanelID:     "new-hike-form-panel",
		Form:        form,
		HTMX:        htmx,
	}
}

func editFormPageData(id int64, form hikeForm, htmx bool) hikeFormPageData {
	idValue := strconv.FormatInt(id, 10)

	return hikeFormPageData{
		Title:       "Edit hike",
		Heading:     "Edit hike",
		Intro:       "Update the details of this hike.",
		Mode:        "edit",
		Action:      "/hikes/" + idValue,
		CancelURL:   "/hikes/" + idValue,
		SubmitLabel: "Save changes",
		PanelID:     "hike-detail",
		Form:        form,
		HTMX:        htmx,
	}
}

func formFromHike(hike Hike) hikeForm {
	form := hikeForm{
		Name:        hike.Name,
		Location:    hike.Location,
		Description: hike.Description,
		Status:      hike.Status.String(),
		Errors:      make(map[string]string),
	}

	if hike.DistanceKM != nil {
		form.DistanceKM = strconv.FormatFloat(*hike.DistanceKM, 'f', -1, 64)
	}

	return form
}

func listParamsFromRequest(c *gin.Context) (ListParams, error) {
	pageValue := c.DefaultQuery("page", "1")

	page, err := strconv.Atoi(pageValue)
	if err != nil || page < 1 {
		return ListParams{}, fmt.Errorf("invalid page %q", pageValue)
	}

	params := ListParams{
		Query:    strings.TrimSpace(c.Query("q")),
		Sort:     Sort(strings.TrimSpace(c.Query("sort"))),
		Page:     page,
		PageSize: defaultListPageSize,
	}

	params.Tags = c.Request.URL.Query()["tag"]

	statusValue := strings.TrimSpace(c.Query("status"))

	if statusValue != "" {
		status := Status(statusValue)
		params.Status = &status
	}

	return params, nil
}

func hikeListURL(params ListParams, page int) string {
	values := url.Values{}

	if params.Query != "" {
		values.Set("q", params.Query)
	}

	if params.Status != nil {
		values.Set("status", params.Status.String())
	}

	if params.Sort != "" && params.Sort != SortNewest {
		values.Set("sort", params.Sort.String())
	}

	if page > 1 {
		values.Set("page", strconv.Itoa(page))
	}

	for _, tag := range params.Tags {
		values.Add("tag", tag)
	}

	encoded := values.Encode()

	if encoded == "" {
		return "/hikes"
	}

	return "/hikes?" + encoded
}

func listPageData(params ListParams, result ListResult) hikeListPageData {
	status := ""

	if params.Status != nil {
		status = params.Status.String()
	}

	sortValue := params.Sort.String()

	if sortValue == "" {
		sortValue = SortNewest.String()
	}

	data := hikeListPageData{
		Title: "Hikes",

		Hikes: result.Hikes,

		Filters: hikeListFilters{
			Query:  params.Query,
			Status: status,
			Sort:   sortValue,
		},

		Total:      result.Total,
		Page:       result.Page,
		TotalPages: result.TotalPages,

		HasFilters: params.Query != "" || params.Status != nil || len(params.Tags) > 0,

		SelectedTags: selectedTagMap(params.Tags),
	}

	if result.Page > 1 {
		data.PreviousURL = hikeListURL(params, result.Page-1)
	}

	if result.Page < result.TotalPages {
		data.NextURL = hikeListURL(params, result.Page+1)
	}

	return data
}

func visitFormFromRequest(c *gin.Context) visitForm {
	return visitForm{
		VisitedAt: strings.TrimSpace(c.PostForm("visited_at")),
		Weather:   strings.TrimSpace(c.PostForm("weather")),
		Notes:     strings.TrimSpace(c.PostForm("notes")),
		Rating:    strings.TrimSpace(c.PostForm("rating")),
		Errors:    make(map[string]string),
	}
}

func parseVisitForm(form visitForm) (CreateVisitParams, map[string]string) {
	errors := make(map[string]string)

	var visitedAt time.Time

	if form.VisitedAt == "" {
		errors["visited_at"] = "Visit date is required"
	} else {
		parsed, err := time.Parse(
			"2006-01-02",
			form.VisitedAt,
		)
		if err != nil {
			errors["visited_at"] = "Visit date is invalid"
		} else {
			visitedAt = parsed
		}
	}

	var rating *int

	if form.Rating != "" {
		value, err := strconv.Atoi(form.Rating)

		if err != nil {
			errors["rating"] = "Rating must be a number"
		} else {
			rating = &value
		}
	}

	return CreateVisitParams{
		VisitedAt: visitedAt,
		Weather:   form.Weather,
		Notes:     form.Notes,
		Rating:    rating,
	}, errors
}

func (h *WebHandler) renderVisitFormError(
	c *gin.Context,
	hikeID int64,
	form visitForm,
) {
	found, err := h.service.FindByID(
		c.Request.Context(),
		hikeID,
	)
	if err != nil {
		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to load hike",
			"TrailLog could not load this hike.",
			err,
		)

		return
	}

	visits, err := h.service.ListVisits(
		c.Request.Context(),
		hikeID,
	)
	if err != nil {
		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to load visits",
			"TrailLog could not load the visit history.",
			err,
		)

		return
	}

	tags, err := h.service.ListHikeTags(
		c.Request.Context(),
		hikeID,
	)
	if err != nil {
		renderWebError(
			c,
			http.StatusInternalServerError,
			"Unable to load tags",
			"TrailLog could not load this hike's tags.",
			err,
		)

		return
	}

	c.HTML(
		http.StatusUnprocessableEntity,
		"hikes/show.html",
		hikeShowPageData{
			Title:     found.Name,
			Hike:      found,
			Visits:    visits,
			VisitForm: form,
			TagsPanel: hikeTagsPanelData{
				HikeID: hikeID,
				Tags:   tags,

				Form: hikeTagsForm{
					Tags: tagInputValue(tags),

					Errors: make(
						map[string]string,
					),
				},
			},
		},
	)
}

func selectedTagMap(names []string) map[string]bool {
	result := make(
		map[string]bool,
		len(names),
	)
	for _, name := range names {
		result[name] = true
	}

	return result
}

func tagInputValue(
	tags []Tag,
) string {
	names := make([]string, 0, len(tags))

	for _, tag := range tags {
		names = append(names, tag.Name)
	}

	return strings.Join(names, ", ")
}

func parseTagInput(
	value string,
) []string {
	parts := strings.Split(value, ",")

	return parts
}
