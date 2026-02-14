package events

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/biairmal/go-sdk/errorz"
	"github.com/biairmal/go-sdk/httpkit/response"
	"github.com/biairmal/go-sdk/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CategoryHandlerOptions struct{}

// CategoryHandler exposes HTTP handlers for event category CRUD.
type CategoryHandler struct {
	options CategoryHandlerOptions
	service *CategoryService
}

// NewCategoryHandler returns a CategoryHandler that uses the given service.
func NewCategoryHandler(options CategoryHandlerOptions, service *CategoryService) *CategoryHandler {
	return &CategoryHandler{options: options, service: service}
}

// List handles GET /event-categories with optional query: limit, offset, sort_field, sort_dir.
func (h *CategoryHandler) List(r *http.Request) (any, error) {
	ctx := r.Context()
	limit, offset := parseLimitOffset(r)
	sort := parseSort(r)
	filter := repository.Filter{} // Optional: parse from query e.g. source=app, tenant_id=...
	result, err := h.service.List(ctx, filter, sort, limit, offset)
	if err != nil {
		return nil, err
	}
	return response.OK(result), nil
}

// GetByID handles GET /event-categories/{id}.
func (h *CategoryHandler) GetByID(r *http.Request) (any, error) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid event category id")
	}
	entity, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// Create handles POST /event-categories.
func (h *CategoryHandler) Create(r *http.Request) (any, error) {
	var body CreateInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid request body")
	}
	entity, err := h.service.Create(r.Context(), body)
	if err != nil {
		return nil, err
	}
	return response.Created(entity), nil
}

// Update handles PUT /event-categories/{id}.
func (h *CategoryHandler) Update(r *http.Request) (any, error) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid event category id")
	}
	var body UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid request body")
	}
	entity, err := h.service.Update(r.Context(), id, body)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// Delete handles DELETE /event-categories/{id}.
func (h *CategoryHandler) Delete(r *http.Request) (any, error) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid event category id")
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		return nil, err
	}
	return response.NoContent(), nil
}

func parseLimitOffset(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
			if limit > 100 {
				limit = 100
			}
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

func parseSort(r *http.Request) repository.Sort {
	field := r.URL.Query().Get("sort_field")
	if field == "" {
		field = "created_at"
	}
	dir := repository.SortAsc
	if r.URL.Query().Get("sort_dir") == "desc" {
		dir = repository.SortDesc
	}
	return repository.Sort{Field: field, Direction: dir}
}
