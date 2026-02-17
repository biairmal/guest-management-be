package events

import (
	"encoding/json"
	"net/http"

	"github.com/biairmal/go-sdk/errorz"
	"github.com/biairmal/go-sdk/httpkit/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// CategoryHandlerOptions holds configuration for the category handler.
type CategoryHandlerOptions struct{}

// CategoryHandler exposes HTTP handlers for event category CRUD.
type CategoryHandler struct {
	options CategoryHandlerOptions
	service CategoryService
}

// NewCategoryHandler returns a CategoryHandler that uses the given service.
// The service parameter is an interface, allowing easy testing and substitution.
func NewCategoryHandler(options CategoryHandlerOptions, service CategoryService) *CategoryHandler {
	return &CategoryHandler{options: options, service: service}
}

// List handles GET /event-categories with query parameters.
//
// Query format: name=Event1&page=1&size=20&sort=column1,DESC&sort=column2,ASC
//
// List godoc
//
//	@Summary		List event categories
//	@Description	Returns a paginated list of event categories. Query: page, size, sort=field,dir (repeatable), filter by allowed fields (name, source, tenant_id).
//	@Tags			event-categories
//	@Accept			json
//	@Produce		json
//	@Param			page	query		int		false	"Page number (1-based)"
//	@Param			size	query		int		false	"Page size (default 20, max 100)"
//	@Param			sort	query		string	false	"Sort: field,dir (e.g. sort=name,ASC&sort=id,DESC)"
//	@Param			name	query		string	false	"Filter by name (exact match)"
//	@Param			source	query		string	false	"Filter by source (exact match)"
//	@Success		200		{object}	common.PageResponse[events.EventCategory]
//	@Failure		400		{object}	object	"Invalid query (e.g. invalid sort field)"
//	@Failure		500		{object}	object	"Internal server error"
//	@Router			/api/v1/event-categories [get]
func (h *CategoryHandler) List(r *http.Request) (any, error) {
	params, err := ParseEventCategoryListParams(r.URL.Query(), eventCategoryListConfig)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage(err.Error())
	}
	result, err := h.service.List(r.Context(), params)
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
