package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultGPXMaxUploadBytes = int64(20 << 20)

	defaultGPXMaxTrackPoints = 200_000

	defaultGpxMaxTextFieldBytes = int64(64 << 10)

	defaultMapTileURL = "https://tile.openstreetmap.org/{z}/{x}/{y}.png"

	defaultMapAttribution = `&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors`
)

func loadGPXConfig() (
	GPXConfig,
	error,
) {
	maxUploadBytes, err := positiveInt64Environment(
		"TRAILLOG_GPX_MAX_UPLOAD_BYTES",
		defaultGPXMaxUploadBytes,
	)
	if err != nil {
		return GPXConfig{}, err
	}

	maxTrackPoints, err := positiveIntEnvironment(
		"TRAILLOG_GPX_MAX_TRACK_POINTS",
		defaultGPXMaxTrackPoints,
	)
	if err != nil {
		return GPXConfig{}, err
	}

	maxTextFieldBytes, err := positiveInt64Environment(
		"TRAILLOG_GPX_MAX_TEXT_FIELD_BYTES",
		defaultGpxMaxTextFieldBytes,
	)
	if err != nil {
		return GPXConfig{}, err
	}

	return GPXConfig{
		MaxUploadBytes:     maxUploadBytes,
		MaxTrackPoints:     maxTrackPoints,
		MaxTextFieldsBytes: maxTextFieldBytes,
	}, nil
}

func loadMapConfig() MapConfig {
	return MapConfig{
		TileURL: stringEnvironment(
			"TRAILLOG_MAP_TILE_URL",
			defaultMapTileURL,
		),
		Attribution: stringEnvironment(
			"TRAILLOG_MAP_ATTRIBUTION",
			defaultMapAttribution,
		),
	}
}

func stringEnvironment(
	name string,
	defaultVaule string,
) string {
	value, exists := os.LookupEnv(name)
	if !exists {
		return defaultVaule
	}

	value = strings.TrimSpace(value)

	if value == "" {
		return defaultVaule
	}

	return value
}

func positiveIntEnvironment(
	name string,
	defaultValue int,
) (int, error) {
	value, exists := os.LookupEnv(name)
	if !exists || strings.TrimSpace(value) == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}

	return parsed, nil
}

func positiveInt64Environment(
	name string,
	defaultValue int64,
) (int64, error) {
	value, exists := os.LookupEnv(name)
	if !exists || strings.TrimSpace(value) == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}

	return parsed, nil
}
