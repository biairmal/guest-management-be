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
//
// List godoc
//
//	@Summary		List event categories
//	@Description	Returns a paginated list of event categories with optional sort and pagination.
//	@Tags			event-categories
//	@Accept			json
//	@Produce		json
//	@Param			limit		query		int		false	"Maximum number of items to return (default 20, max 100)"
//	@Param			offset		query		int		false	"Number of items to skip"
//	@Param			sort_field	query		string	false	"Sort field (default: created_at)"
//	@Param			sort_dir		query		string	false	"Sort direction: asc or desc"
//	@Success		200			{object}	events.ListResult
//	@Failure		500			{object}	object	"Internal server error"
//	@Router			/api/v1/event-categories [get]
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
//
// GetByID godoc
//
//	@Summary		Get event category by ID
//	@Description	Returns a single event category by UUID.
//	@Tags			event-categories
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Event category UUID"
//	@Success		200	{object}	events.EventCategory
//	@Failure		400	{object}	object	"Invalid ID format"
//	@Failure		404	{object}	object	"Event category not found"
//	@Failure		500	{object}	object	"Internal server error"
//	@Router			/api/v1/event-categories/{id} [get]
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
//
// Create godoc
//
//	@Summary		Create event category
//	@Description	Creates a new event category. Source must be "app" or "tenant"; tenant_id required when source is "tenant".
//	@Tags			event-categories
//	@Accept			json
//	@Produce		json
//	@Param			body	body		events.CreateInput	true	"Event category payload"
//	@Success		201		{object}	events.EventCategory
//	@Failure		400		{object}	object	"Invalid request body or validation error"
//	@Failure		409		{object}	object	"Conflict (e.g. already exists)"
//	@Failure		422		{object}	object	"Unprocessable entity"
//	@Failure		500		{object}	object	"Internal server error"
//	@Router			/api/v1/event-categories [post]
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
//
// Update godoc
//
//	@Summary		Update event category
//	@Description	Updates an existing event category by ID. Only provided fields are applied (partial update).
//	@Tags			event-categories
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Event category UUID"
//	@Param			body	body		events.UpdateInput	true	"Fields to update"
//	@Success		200		{object}	events.EventCategory
//	@Failure		400		{object}	object	"Invalid ID or request body"
//	@Failure		404		{object}	object	"Event category not found"
//	@Failure		500		{object}	object	"Internal server error"
//	@Router			/api/v1/event-categories/{id} [put]
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
//
// Delete godoc
//
//	@Summary		Delete event category
//	@Description	Soft-deletes an event category by ID.
//	@Tags			event-categories
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Event category UUID"
//	@Success		204	"No content"
//	@Failure		400	{object}	object	"Invalid ID format"
//	@Failure		404	{object}	object	"Event category not found"
//	@Failure		500	{object}	object	"Internal server error"
//	@Router			/api/v1/event-categories/{id} [delete]
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
