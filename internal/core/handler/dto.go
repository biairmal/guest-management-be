package handler

import (
	"math"
)

// PaginatedRequest is the request parameters for paginated requests.
// Prefer common.PageRequest with common.ParsePageRequest for new list endpoints.
type PaginatedRequest struct {
	Page int `json:"page" validate:"required,min=1"`
	Size int `json:"size" validate:"required,min=1" max:"100"`
}

// PaginatedResponse is the response body for paginated responses.
// Prefer common.PageResponse for new list endpoints.
type PaginatedResponse[T any] struct {
	Items      []*T  `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	Size       int   `json:"size"`
	TotalPages int   `json:"total_pages"`
	HasPrev    bool  `json:"has_prev"`
	HasNext    bool  `json:"has_next"`
}

// NewPaginatedResponse creates a new PaginatedResponse.
func NewPaginatedResponse[T any](items []*T, total int64, page, size int) *PaginatedResponse[T] {
	return &PaginatedResponse[T]{
		Items:      items,
		Total:      total,
		Page:       page,
		Size:       size,
		TotalPages: int(math.Ceil(float64(total) / float64(size))),
		HasPrev:    page > 1,
		HasNext:    page*size < int(total),
	}
}
