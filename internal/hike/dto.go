package hike

import "time"

type createHikeRequest struct {
	Name        string   `json:"name"`
	Location    string   `json:"location"`
	Description string   `json:"description"`
	DistanceKM  *float64 `json:"distance_km"`
	Status      Status   `json:"status"`
}

type updateHikeRequest struct {
	Name        string   `json:"name"`
	Location    string   `json:"location"`
	Description string   `json:"description"`
	DistanceKM  *float64 `json:"distance_km"`
	Status      string   `json:"status"`
}

type hikeResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Location    string    `json:"location"`
	Description string    `json:"description"`
	DistanceKM  *float64  `json:"distance_km"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type errorResponse struct {
	Error     string            `json:"error"`
	Fields    map[string]string `json:"fileds,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

func toHikeResponse(hike Hike) hikeResponse {
	return hikeResponse{
		ID:          hike.ID,
		Name:        hike.Name,
		Location:    hike.Location,
		Description: hike.Description,
		DistanceKM:  hike.DistanceKM,
		Status:      string(hike.Status),
		CreatedAt:   hike.CreatedAt,
		UpdatedAt:   hike.UpdatedAt,
	}
}
