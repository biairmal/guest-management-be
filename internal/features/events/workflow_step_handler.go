package events

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

// WorkflowStepHandler exposes HTTP handlers for event-scoped workflow step CRUD.
type WorkflowStepHandler struct {
	service   WorkflowStepService
	validator validation.Validator
}

// workflowStepListConfig declares the allow-listed sort/filter fields for
// workflow step list queries. event_id is always scoped from the URL, not a
// query filter. Pagination falls back to the shared defaults.
var workflowStepListConfig = query.ListParseConfig{
	AllowedSortFields:   []string{"id", "name", "order_index", "allows_multiple", "created_at", "updated_at"},
	AllowedFilterFields: []string{"name"},
}

// NewWorkflowStepHandler returns a WorkflowStepHandler that uses the given
// service and validator. Both are interfaces, allowing easy testing and substitution.
func NewWorkflowStepHandler(service WorkflowStepService, validator validation.Validator) *WorkflowStepHandler {
	return &WorkflowStepHandler{service: service, validator: validator}
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
//	@Success		200			{object}	common.PageResponse[events.WorkflowStep]
//	@Failure		400			{object}	object	"Invalid event id or query"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/workflow-steps [get]
func (h *WorkflowStepHandler) List(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	params, err := query.ParseListParams(r.URL.Query(), workflowStepListConfig)
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
//	@Success		200			{object}	events.WorkflowStep
//	@Failure		400			{object}	object	"Invalid ID format"
//	@Failure		404			{object}	object	"Workflow step not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/workflow-steps/{id} [get]
func (h *WorkflowStepHandler) GetByID(r *http.Request) (any, error) {
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

// Create handles POST /events/{event_id}/workflow-steps.
//
// Create godoc
//
//	@Summary		Create workflow step
//	@Description	Creates a new workflow step under an event. order_index must be unique per event.
//	@Tags			workflow-steps
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string							true	"Event UUID"
//	@Param			body		body		events.CreateWorkflowStepInput	true	"Workflow step payload"
//	@Success		201			{object}	events.WorkflowStep
//	@Failure		400			{object}	object	"Invalid request body or validation error"
//	@Failure		409			{object}	object	"Conflict (order_index already used for this event)"
//	@Failure		422			{object}	object	"Unprocessable entity"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/workflow-steps [post]
//
//nolint:dupl // decode+validate+create+respond shape mirrors WorkflowStepTemplateHandler.Create (see PATTERNS.md)
func (h *WorkflowStepHandler) Create(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	var body CreateWorkflowStepInput
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

// Update handles PUT /events/{event_id}/workflow-steps/{id}.
//
// Update godoc
//
//	@Summary		Update workflow step
//	@Description	Updates an existing workflow step by ID. Only provided fields are applied (partial update); event_id is immutable.
//	@Tags			workflow-steps
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string							true	"Event UUID"
//	@Param			id			path		string							true	"Workflow step UUID"
//	@Param			body		body		events.UpdateWorkflowStepInput	true	"Fields to update"
//	@Success		200			{object}	events.WorkflowStep
//	@Failure		400			{object}	object	"Invalid ID or request body"
//	@Failure		404			{object}	object	"Workflow step not found"
//	@Failure		409			{object}	object	"Conflict (order_index already used for this event)"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/workflow-steps/{id} [put]
func (h *WorkflowStepHandler) Update(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid workflow step id")
	}
	var body UpdateWorkflowStepInput
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

// Delete handles DELETE /events/{event_id}/workflow-steps/{id}.
//
// Delete godoc
//
//	@Summary		Delete workflow step
//	@Description	Soft-deletes a workflow step by ID, scoped to its event.
//	@Tags			workflow-steps
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path	string	true	"Event UUID"
//	@Param			id			path	string	true	"Workflow step UUID"
//	@Success		204			"No content"
//	@Failure		400			{object}	object	"Invalid ID format"
//	@Failure		404			{object}	object	"Workflow step not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/workflow-steps/{id} [delete]
func (h *WorkflowStepHandler) Delete(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid workflow step id")
	}
	if err := h.service.Delete(r.Context(), eventID, id); err != nil {
		return nil, err
	}
	return response.NoContent(), nil
}

// Sync handles PUT /events/{event_id}/workflow-steps, replacing the event's
// entire workflow-step list in one call. Every entry with an id is updated,
// every entry without one is created, and any existing step left out of
// the payload is deleted — the single endpoint a "one screen, one save
// button" UI needs to add, update, delete, and reorder steps.
//
// Sync godoc
//
//	@Summary		Sync workflow steps
//	@Description	Replaces an event's workflow steps with the given list: entries with "id" are updated, entries without are created, and any existing step not present in the list is deleted.
//	@Tags			workflow-steps
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string							true	"Event UUID"
//	@Param			body		body		[]events.SyncWorkflowStepInput	true	"Full desired list of workflow steps"
//	@Success		200			{array}		events.WorkflowStep
//	@Failure		400			{object}	object	"Invalid request body, duplicate order_index, or an id not belonging to this event"
//	@Failure		422			{object}	object	"Unprocessable entity"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/workflow-steps [put]
func (h *WorkflowStepHandler) Sync(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	var body []SyncWorkflowStepInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid request body")
	}
	for _, item := range body {
		if err := h.validator.Struct(item); err != nil {
			return nil, err
		}
	}
	steps, err := h.service.Sync(r.Context(), eventID, body)
	if err != nil {
		return nil, err
	}
	return response.OK(steps), nil
}
