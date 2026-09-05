package event

import (
	"encoding/json"
	"net/http"

	common "github.com/biairmal/go-sdk/lib/common/dto"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/httpkit/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/core/validation"
)

// Handler exposes HTTP handlers for event CRUD.
type Handler struct {
	service   Service
	validator validation.Validator
}

// NewHandler returns an Handler that uses the given service and
// validator. Both are interfaces, allowing easy testing and substitution.
func NewHandler(service Service, validator validation.Validator) *Handler {
	return &Handler{service: service, validator: validator}
}

// List handles GET /events with query parameters.
//
// Query format: tenant_id=...&page=1&size=20&sort=column1,DESC&sort=column2,ASC
//
// List godoc
//
//	@Summary		List events
//	@Description	Returns a paginated list of events. Query: page, size, sort=field,dir (repeatable), filter by allowed fields (tenant_id, category_id, name).
//	@Tags			events
//	@Accept			json
//	@Produce		json
//	@Param			page		query		int		false	"Page number (1-based)"
//	@Param			size		query		int		false	"Page size (default 20, max 100)"
//	@Param			sort		query		string	false	"Sort: field,dir (e.g. sort=start_date,ASC&sort=id,DESC)"
//	@Param			tenant_id	query		string	false	"Filter by tenant ID (exact match)"
//	@Param			category_id	query		string	false	"Filter by category ID (exact match)"
//	@Param			name		query		string	false	"Filter by name (exact match)"
//	@Success		200			{object}	common.PageResponse[event.Event]
//	@Failure		400			{object}	object	"Invalid query (e.g. invalid sort field)"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events [get]
func (h *Handler) List(r *http.Request) (any, error) {
	params, err := query.ParseListParams(r.URL.Query(), EventListConfig)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage(err.Error())
	}
	// Explicitly typed (rather than :=) so this file imports common/dto — swag
	// resolves the generic @Success type below against this file's imports.
	var result *common.PageResponse[Event]
	result, err = h.service.List(r.Context(), params)
	if err != nil {
		return nil, err
	}
	return response.OK(result), nil
}

// GetByID handles GET /events/{id}.
//
// GetByID godoc
//
//	@Summary		Get event by ID
//	@Description	Returns a single event by UUID.
//	@Tags			events
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Event UUID"
//	@Success		200	{object}	event.Event
//	@Failure		400	{object}	object	"Invalid ID format"
//	@Failure		404	{object}	object	"Event not found"
//	@Failure		500	{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{id} [get]
func (h *Handler) GetByID(r *http.Request) (any, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid event id")
	}
	entity, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// Create handles POST /events.
//
// Create godoc
//
//	@Summary		Create event
//	@Description	Creates a new event scoped to a tenant and category. is_multi_day is derived from start_date/end_date.
//	@Tags			events
//	@Accept			json
//	@Produce		json
//	@Param			body	body		event.CreateEventInput	true	"Event payload"
//	@Success		201		{object}	event.Event
//	@Failure		400		{object}	object	"Invalid request body or validation error"
//	@Failure		409		{object}	object	"Conflict"
//	@Failure		422		{object}	object	"Unprocessable entity"
//	@Failure		500		{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events [post]
func (h *Handler) Create(r *http.Request) (any, error) {
	var body CreateEventInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid request body")
	}
	if err := h.validator.Struct(body); err != nil {
		return nil, err
	}
	entity, err := h.service.Create(r.Context(), body)
	if err != nil {
		return nil, err
	}
	return response.Created(entity), nil
}

// Update handles PUT /events/{id}.
//
// Update godoc
//
//	@Summary		Update event
//	@Description	Updates an existing event by ID. Only provided fields are applied (partial update); tenant_id is immutable.
//	@Tags			events
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Event UUID"
//	@Param			body	body		event.UpdateEventInput	true	"Fields to update"
//	@Success		200		{object}	event.Event
//	@Failure		400		{object}	object	"Invalid ID or request body"
//	@Failure		404		{object}	object	"Event not found"
//	@Failure		500		{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{id} [put]
func (h *Handler) Update(r *http.Request) (any, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid event id")
	}
	var body UpdateEventInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid request body")
	}
	if err := h.validator.Struct(body); err != nil {
		return nil, err
	}
	entity, err := h.service.Update(r.Context(), id, body)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// Delete handles DELETE /events/{id}.
//
// Delete godoc
//
//	@Summary		Delete event
//	@Description	Soft-deletes an event by ID.
//	@Tags			events
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string	true	"Event UUID"
//	@Success		204	"No content"
//	@Failure		400	{object}	object	"Invalid ID format"
//	@Failure		404	{object}	object	"Event not found"
//	@Failure		500	{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{id} [delete]
func (h *Handler) Delete(r *http.Request) (any, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid event id")
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		return nil, err
	}
	return response.NoContent(), nil
}
