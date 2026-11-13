package workflowstep

import (
	"net/http"

	common "github.com/biairmal/go-sdk/lib/common/dto"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/httpkit/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/biairmal/guest-management-be/internal/core/query"
)

// Handler exposes HTTP handlers for reading an event's workflow steps. There
// are no write handlers since B16: the event's setup save owns every change to
// an event's steps (see workflow_step_routes.go), so this handler takes no
// validator — neither read decodes a body.
type Handler struct {
	service Service
}

// NewHandler returns a Handler that uses the given service.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// eventIDFromPath parses the {event_id} path parameter.
func eventIDFromPath(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "event_id"))
	if err != nil {
		return uuid.Nil, errorz.BadRequest().WithMessage("invalid event id")
	}
	return id, nil
}

// List handles GET /events/{event_id}/workflow-steps with query parameters.
//
// List godoc
//
//	@Summary		List workflow steps
//	@Description	Returns a paginated list of workflow steps for an event. Query: page, size, sort=field,dir (repeatable), filter by allowed fields (name).
//	@Tags			workflow-steps
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string	true	"Event UUID"
//	@Param			page		query		int		false	"Page number (1-based)"
//	@Param			size		query		int		false	"Page size (default 20, max 100)"
//	@Param			sort		query		string	false	"Sort: field,dir (e.g. sort=order_index,ASC)"
//	@Param			name		query		string	false	"Filter by name (exact match)"
//	@Success		200			{object}	common.PageResponse[workflowstep.WorkflowStep]
//	@Failure		400			{object}	object	"Invalid event id or query"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/workflow-steps [get]
func (h *Handler) List(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	params, err := query.ParseListParams(r.URL.Query(), WorkflowStepListConfig)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage(err.Error())
	}
	// Explicitly typed (rather than :=) so this file imports common/dto — swag
	// resolves the generic @Success type below against this file's imports.
	var result *common.PageResponse[WorkflowStep]
	result, err = h.service.List(r.Context(), eventID, params)
	if err != nil {
		return nil, err
	}
	return response.OK(result), nil
}

// GetByID handles GET /events/{event_id}/workflow-steps/{id}.
//
// GetByID godoc
//
//	@Summary		Get workflow step by ID
//	@Description	Returns a single workflow step by UUID, scoped to its event.
//	@Tags			workflow-steps
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string	true	"Event UUID"
//	@Param			id			path		string	true	"Workflow step UUID"
//	@Success		200			{object}	workflowstep.WorkflowStep
//	@Failure		400			{object}	object	"Invalid ID format"
//	@Failure		404			{object}	object	"Workflow step not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/workflow-steps/{id} [get]
func (h *Handler) GetByID(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid workflow step id")
	}
	entity, err := h.service.GetByID(r.Context(), eventID, id)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}
