// Package webui contains Web UI resources
package webui

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"time"
)

//go:embed templates
var templateFiles embed.FS

// Templates returns an HTTP file system containing the embedded HTML templates.
func Templates() http.FileSystem {
	subtree, err := fs.Sub(templateFiles, "templates")
	if err != nil {
		panic(fmt.Errorf("create template filesystem: %w", err))
	}

	return http.FS(subtree)
}

// FuncMap returns the functions exposed to TrailLog HTML templates.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"formatDistance":   formatDistance,
		"formatDate":       formatDate,
		"formatVisitDate":  formatVisitDate,
		"seq":              sequence,
		"eqStringInt":      equalStringInt,
		"formatKilometers": formatKilometers,
		"formatRating":     formatRating,
		"formatMonth":      formatMonth,
		"toJSON":           toJSON,
		"formatMeters":     formatMeters,
		"formatDuration":   formatDuration,
	}
}

func formatDistance(distance *float64) string {
	if distance == nil {
		return "Not specified"
	}

	return fmt.Sprintf("%.1f km", *distance)
}

func formatDate(value time.Time) string {
	if value.IsZero() {
		return "-"
	}

	return value.UTC().UTC().Format("02 Jan 2006, 15:04 UTC")
}

func sequence(from int, to int) []int {
	if to < from {
		return []int{}
	}

	values := make(
		[]int,
		0,
		to-from+1,
	)

	for value := from; value <= to; value++ {
		values = append(values, value)
	}

	return values
}

func formatVisitDate(
	value time.Time,
) string {
	if value.IsZero() {
		return "-"
	}

	return value.Format("02 Jan 2006")
}

func equalStringInt(
	value string,
	number int,
) bool {
	return value == strconv.Itoa(number)
}

func formatKilometers(
	value float64,
) string {
	return fmt.Sprintf("%.1f km", value)
}

func formatRating(
	value *float64,
) string {
	if value == nil {
		return "Not rated yet'"
	}

	return fmt.Sprintf("%.1f / 5", *value)
}

func formatMonth(
	value string,
) string {
	parsed, err := time.Parse(
		"2006-01",
		value,
	)
	if err != nil {
		return value
	}

	return parsed.Format("Jan 2006")
}

func toJSON(
	value any,
) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "null"
	}

	return string(encoded)
}

func formatMeters(
	value float64,
) string {
	return fmt.Sprintf(
		"%.0f m",
		value,
	)
}

func formatDuration(
	value *time.Duration,
) string {
	if value == nil {
		return "Not recorded"
	}

	duration := *value

	hours := int(
		duration / time.Hour,
	)

	minutes := int(
		(duration % time.Hour) /
			time.Minute,
	)

	switch {
	case hours > 0:
		return fmt.Sprintf(
			"%dh %02dm",
			hours,
			minutes,
		)

	default:
		return fmt.Sprintf(
			"%d min",
			minutes,
		)
	}
}
