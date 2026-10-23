package hike

import (
	"sort"
	"strings"
	"time"
)

const (
	defaultListPageSize = 12
	maximumListPageSize = 100
)

// Tags constants
const (
	maxTagsPerHike   = 12
	maxTagNameLength = 40
)

// GPX caontants
const maximumImportedTrackPoints = 200_000

// ValidationError reports invalid input by form or API field name.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return "hike validation failed"
}

// Field returns the validation message for name, or an empty string when absent.
func (e *ValidationError) Field(name string) string {
	return e.Fields[name]
}

// Has reports whether name has a validation error.
func (e *ValidationError) Has(name string) bool {
	_, ok := e.Fields[name]

	return ok
}

func validateHikeParams(name, location string, distanceKM *float64, status Status) error {
	fields := make(map[string]string)

	if name == "" {
		fields["name"] = "Name is required"
	}

	if location == "" {
		fields["location"] = "Location is required"
	}

	if distanceKM != nil && *distanceKM < 0 {
		fields["distance_km"] = "Distance cannot be negative"
	}

	if !status.Valid() {
		fields["status"] = "Status must be planned or visited"
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}

	return nil
}

func normalizeCreateParams(params CreateParams) CreateParams {
	params.Name = strings.TrimSpace(params.Name)
	params.Location = strings.TrimSpace(params.Location)
	params.Description = strings.TrimSpace(params.Description)

	if params.Status == "" {
		params.Status = StatusPlanned
	}

	return params
}

func normalizeUpdateParams(params UpdateParams) UpdateParams {
	params.Name = strings.TrimSpace(params.Name)
	params.Location = strings.TrimSpace(params.Location)
	params.Description = strings.TrimSpace(params.Description)

	return params
}

func normalizeListParams(params ListParams) ListParams {
	params.Query = strings.TrimSpace(params.Query)

	if params.Sort == "" {
		params.Sort = SortNewest
	}

	if params.Page <= 0 {
		params.Page = 1
	}

	if params.PageSize <= 0 {
		params.PageSize = defaultListPageSize
	}

	if params.PageSize > maximumListPageSize {
		params.PageSize = maximumListPageSize
	}

	params.Tags = normalizeTagNames(params.Tags)

	return params
}

func normalizeCreateVisitParams(params CreateVisitParams) CreateVisitParams {
	params.Weather = strings.TrimSpace(params.Weather)
	params.Notes = strings.TrimSpace(params.Notes)

	return params
}

func validateListParams(params ListParams) error {
	fields := make(map[string]string)

	if params.Status != nil && !params.Status.Valid() {
		fields["status"] = "Status must be planned or visited"
	}

	if !params.Sort.Valid() {
		fields["sort"] = "Sort must be newewst, name or distance"
	}

	if len(fields) > 0 {
		return &ValidationError{
			Fields: fields,
		}
	}

	return nil
}

func validateCreateVisitParams(params CreateVisitParams) error {
	fields := make(map[string]string)

	if params.VisitedAt.IsZero() {
		fields["visited_at"] = "Visit date is required"
	}

	if params.Rating != nil && (*params.Rating < 1 || *params.Rating > 5) {
		fields["rating"] = "Rating must be between 1 and 5"
	}

	if len(fields) > 0 {
		return &ValidationError{
			Fields: fields,
		}
	}

	return nil
}

func normalizeTagNames(names []string) []string {
	normalized := make([]string, 0, len(names))

	seen := make(map[string]struct{}, len(names))

	for _, value := range names {
		name := strings.ToLower(strings.TrimSpace(value))

		if name == "" {
			continue
		}

		if _, exists := seen[name]; exists {
			continue
		}

		seen[name] = struct{}{}

		normalized = append(normalized, name)
	}

	sort.Strings(normalized)

	return normalized
}

func validateTagNames(names []string) error {
	fields := make(map[string]string)

	if len(names) > maxTagsPerHike {
		fields["tags"] = "A hike can have at most 12 tags"
	}

	for _, name := range names {
		if len([]rune(name)) > maxTagNameLength {
			fields["tags"] = "Tag names can contain at most 40 characters"

			break
		}
	}

	if len(fields) > 0 {
		return &ValidationError{
			Fields: fields,
		}
	}

	return nil
}

func validateImportTrackParams(
	params ImportTrackParams,
) error {
	return validateTrackData(
		params.Points,
		params.DistanceKM,
		params.ElevationGainM,
		params.Duration,
		params.StartLat,
		params.StartLon,
		params.EndLat,
		params.EndLon,
	)
}

func validateSetTrackParams(
	params SetTrackParams,
) error {
	return validateTrackData(
		params.Points,
		params.DistanceKM,
		params.ElevationGainM,
		params.Duration,
		params.StartLat,
		params.StartLon,
		params.EndLat,
		params.EndLon,
	)
}

func validateTrackData(
	points []TrackPoint,
	distanceKM float64,
	elevationGainM float64,
	duration *time.Duration,
	startLat float64,
	startLon float64,
	endLat float64,
	endLon float64,
) error {
	fields := make(
		map[string]string,
	)

	if len(points) < 2 {
		fields["file"] = "GPX track must contain at least two points"
	}

	if len(points) >
		maximumImportedTrackPoints {

		fields["file"] = "GPX track contains too many points"
	}

	if distanceKM < 0 {
		fields["file"] = "Track distance cannot be negative"
	}

	if elevationGainM < 0 {
		fields["file"] = "Elevation gain cannot be negative"
	}

	if startLat < -90 ||
		startLat > 90 ||
		endLat < -90 ||
		endLat > 90 {

		fields["file"] = "Track contains invalid latitude"
	}

	if startLon < -180 ||
		startLon > 180 ||
		endLon < -180 ||
		endLon > 180 {

		fields["file"] = "Track contains invalid longitude"
	}

	if duration != nil &&
		*duration < 0 {

		fields["file"] = "Track duration cannot be negative"
	}

	for _, point := range points {
		if point.Latitude < -90 ||
			point.Latitude > 90 {

			fields["file"] = "Track contains invalid latitude"

			break
		}

		if point.Longitude < -180 ||
			point.Longitude > 180 {

			fields["file"] = "Track contains invalid longitude"

			break
		}
	}

	if len(fields) > 0 {
		return &ValidationError{
			Fields: fields,
		}
	}

	return nil
}
