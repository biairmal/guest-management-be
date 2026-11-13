package tickettype

import (
	"net/http"

	common "github.com/biairmal/go-sdk/lib/common/dto"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/httpkit/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/biairmal/guest-management-be/internal/core/query"
)

// TicketTypeHandler exposes HTTP handlers for reading an event's ticket types.
// There are no write handlers since B16: the event's setup save owns every
// change to an event's ticket types (see ticket_type_routes.go), so this handler
// takes no validator — neither read decodes a body. Named as-is from the tickets
// package root (docs/DEVELOPMENT_PLAN.md B12); see ticket_type_service.go's
// TicketTypeService for why the stutter is left alone.
//
//nolint:revive // stutter is a side effect of the multi-entity split, not a fresh naming choice
type TicketTypeHandler struct {
	service TicketTypeService
}

// NewTicketTypeHandler returns a TicketTypeHandler that uses the given service.
func NewTicketTypeHandler(service TicketTypeService) *TicketTypeHandler {
	return &TicketTypeHandler{service: service}
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
//	@Description	Returns a paginated list of ticket types for an event. Query: page, size, sort=field,dir (repeatable), filter by allowed fields (name). Open to any authenticated caller of the event's tenant.
//	@Tags			ticket-types
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string	true	"Event UUID"
//	@Param			page		query		int		false	"Page number (1-based)"
//	@Param			size		query		int		false	"Page size (default 20, max 100)"
//	@Param			sort		query		string	false	"Sort: field,dir (e.g. sort=name,ASC)"
//	@Param			name		query		string	false	"Filter by name (exact match)"
//	@Success		200			{object}	common.PageResponse[tickettype.TicketType]
//	@Failure		400			{object}	object	"Invalid event id or query"
//	@Failure		401			{object}	object	"Unauthenticated"
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
//	@Description	Returns a single ticket type by UUID, scoped to its event, including which of the event's workflow steps it applies to (workflow_step_ids). Open to any authenticated caller of the event's tenant.
//	@Tags			ticket-types
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string	true	"Event UUID"
//	@Param			id			path		string	true	"Ticket type UUID"
//	@Success		200			{object}	tickettype.TicketType
//	@Failure		400			{object}	object	"Invalid ID format"
//	@Failure		401			{object}	object	"Unauthenticated"
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
