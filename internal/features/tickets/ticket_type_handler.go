package tickets

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

// TicketTypeHandler exposes HTTP handlers for event-scoped ticket type CRUD
// and workflow-step applicability.
type TicketTypeHandler struct {
	service   TicketTypeService
	validator validation.Validator
}

// NewTicketTypeHandler returns a TicketTypeHandler that uses the given
// service and validator. Both are interfaces, allowing easy testing and
// substitution.
func NewTicketTypeHandler(service TicketTypeService, validator validation.Validator) *TicketTypeHandler {
	return &TicketTypeHandler{service: service, validator: validator}
}

// eventIDFromPath parses the {event_id} path parameter.
func eventIDFromPath(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "event_id"))
	if err != nil {
		return uuid.Nil, errorz.BadRequest().WithMessage("invalid event id")
	}
	return id, nil
}

// List handles GET /events/{event_id}/ticket-types with query parameters.
//
// List godoc
//
//	@Summary		List event ticket types
//	@Description	Returns a paginated list of ticket types for an event. Query: page, size, sort=field,dir (repeatable), filter by allowed fields (name). Requires the manage_events permission.
//	@Tags			ticket-types
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string	true	"Event UUID"
//	@Param			page		query		int		false	"Page number (1-based)"
//	@Param			size		query		int		false	"Page size (default 20, max 100)"
//	@Param			sort		query		string	false	"Sort: field,dir (e.g. sort=name,ASC)"
//	@Param			name		query		string	false	"Filter by name (exact match)"
//	@Success		200			{object}	common.PageResponse[tickets.TicketType]
//	@Failure		400			{object}	object	"Invalid event id or query"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_events permission"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/ticket-types [get]
func (h *TicketTypeHandler) List(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	params, err := query.ParseListParams(r.URL.Query(), TicketTypeListConfig)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage(err.Error())
	}
	// Explicitly typed (rather than :=) so this file imports common/dto — swag
	// resolves the generic @Success type below against this file's imports.
	var result *common.PageResponse[TicketType]
	result, err = h.service.List(r.Context(), eventID, params)
	if err != nil {
		return nil, err
	}
	return response.OK(result), nil
}

// GetByID handles GET /events/{event_id}/ticket-types/{id}.
//
// GetByID godoc
//
//	@Summary		Get ticket type by ID
//	@Description	Returns a single ticket type by UUID, scoped to its event, including which of the event's workflow steps it applies to (workflow_step_ids). Requires the manage_events permission.
//	@Tags			ticket-types
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string	true	"Event UUID"
//	@Param			id			path		string	true	"Ticket type UUID"
//	@Success		200			{object}	tickets.TicketType
//	@Failure		400			{object}	object	"Invalid ID format"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_events permission"
//	@Failure		404			{object}	object	"Ticket type not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/ticket-types/{id} [get]
func (h *TicketTypeHandler) GetByID(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid ticket type id")
	}
	entity, err := h.service.GetByID(r.Context(), eventID, id)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// Create handles POST /events/{event_id}/ticket-types.
//
// Create godoc
//
//	@Summary		Create ticket type
//	@Description	Creates a new ticket type under an event; name is unique per event. The new ticket type defaults to applying to ALL of the event's current workflow steps. Requires the manage_events permission.
//	@Tags			ticket-types
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string							true	"Event UUID"
//	@Param			body		body		tickets.CreateTicketTypeInput	true	"Ticket type payload"
//	@Success		201			{object}	tickets.TicketType
//	@Failure		400			{object}	object	"Invalid request body or validation error"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_events permission"
//	@Failure		409			{object}	object	"Ticket type name already exists for this event"
//	@Failure		422			{object}	object	"Unprocessable entity"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/ticket-types [post]
func (h *TicketTypeHandler) Create(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	var body CreateTicketTypeInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid request body")
	}
	if err := h.validator.Struct(body); err != nil {
		return nil, err
	}
	entity, err := h.service.Create(r.Context(), eventID, body)
	if err != nil {
		return nil, err
	}
	return response.Created(entity), nil
}

// Update handles PUT /events/{event_id}/ticket-types/{id}.
//
// Update godoc
//
//	@Summary		Update ticket type
//	@Description	Partially updates a ticket type's name/rules. Only provided fields are applied; event_id is immutable. Requires the manage_events permission.
//	@Tags			ticket-types
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string							true	"Event UUID"
//	@Param			id			path		string							true	"Ticket type UUID"
//	@Param			body		body		tickets.UpdateTicketTypeInput	true	"Fields to update"
//	@Success		200			{object}	tickets.TicketType
//	@Failure		400			{object}	object	"Invalid ID or request body"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_events permission"
//	@Failure		404			{object}	object	"Ticket type not found"
//	@Failure		409			{object}	object	"Ticket type name already exists for this event"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/ticket-types/{id} [put]
func (h *TicketTypeHandler) Update(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid ticket type id")
	}
	var body UpdateTicketTypeInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid request body")
	}
	if err := h.validator.Struct(body); err != nil {
		return nil, err
	}
	entity, err := h.service.Update(r.Context(), eventID, id, body)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// Delete handles DELETE /events/{event_id}/ticket-types/{id}.
//
// Delete godoc
//
//	@Summary		Delete ticket type
//	@Description	Soft-deletes a ticket type by ID, scoped to its event. Requires the manage_events permission.
//	@Tags			ticket-types
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path	string	true	"Event UUID"
//	@Param			id			path	string	true	"Ticket type UUID"
//	@Success		204			"No content"
//	@Failure		400			{object}	object	"Invalid ID format"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_events permission"
//	@Failure		404			{object}	object	"Ticket type not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/ticket-types/{id} [delete]
func (h *TicketTypeHandler) Delete(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid ticket type id")
	}
	if err := h.service.Delete(r.Context(), eventID, id); err != nil {
		return nil, err
	}
	return response.NoContent(), nil
}

// ReplaceWorkflowSteps handles PUT /events/{event_id}/ticket-types/{id}/workflow-steps.
//
// ReplaceWorkflowSteps godoc
//
//	@Summary		Replace ticket type workflow steps
//	@Description	Replaces the full set of workflow steps a ticket type applies to (full-replace, no incremental add/remove); every workflow_step_id must belong to the ticket type's own event. Requires the manage_events permission.
//	@Tags			ticket-types
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string								true	"Event UUID"
//	@Param			id			path		string								true	"Ticket type UUID"
//	@Param			body		body		tickets.ReplaceWorkflowStepsInput	true	"Full desired set of workflow step IDs"
//	@Success		200			{object}	tickets.TicketType
//	@Failure		400			{object}	object	"Invalid request body, ID, or a workflow_step_id not belonging to this event"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_events permission"
//	@Failure		404			{object}	object	"Ticket type not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/ticket-types/{id}/workflow-steps [put]
func (h *TicketTypeHandler) ReplaceWorkflowSteps(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid ticket type id")
	}
	var body ReplaceWorkflowStepsInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid request body")
	}
	entity, err := h.service.ReplaceWorkflowSteps(r.Context(), eventID, id, body.WorkflowStepIDs)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}
