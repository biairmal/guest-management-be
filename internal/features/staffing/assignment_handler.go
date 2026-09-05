package staffing

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

// StaffAssignmentHandler exposes HTTP handlers for event-scoped staff assignment CRUD.
type StaffAssignmentHandler struct {
	service   StaffAssignmentService
	validator validation.Validator
}

// NewStaffAssignmentHandler returns a StaffAssignmentHandler that uses the
// given service and validator. Both are interfaces, allowing easy testing
// and substitution.
func NewStaffAssignmentHandler(service StaffAssignmentService, validator validation.Validator) *StaffAssignmentHandler {
	return &StaffAssignmentHandler{service: service, validator: validator}
}

// eventIDFromPath parses the {event_id} path parameter.
func eventIDFromPath(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "event_id"))
	if err != nil {
		return uuid.Nil, errorz.BadRequest().WithMessage("invalid event id")
	}
	return id, nil
}

// List handles GET /events/{event_id}/staff with query parameters.
//
// List godoc
//
//	@Summary		List event staff
//	@Description	Returns a paginated list of staff assignments for an event. Query: page, size, sort=field,dir (repeatable), filter by allowed fields (user_id, role_id). Requires the manage_staff permission.
//	@Tags			staffing
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string	true	"Event UUID"
//	@Param			page		query		int		false	"Page number (1-based)"
//	@Param			size		query		int		false	"Page size (default 20, max 100)"
//	@Param			sort		query		string	false	"Sort: field,dir (e.g. sort=created_at,ASC)"
//	@Param			user_id		query		string	false	"Filter by user ID (exact match)"
//	@Param			role_id		query		string	false	"Filter by role ID (exact match)"
//	@Success		200			{object}	common.PageResponse[staffing.EventStaffAssignment]
//	@Failure		400			{object}	object	"Invalid event id or query"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_staff permission"
//	@Failure		404			{object}	object	"Event not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/staff [get]
func (h *StaffAssignmentHandler) List(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	params, err := query.ParseListParams(r.URL.Query(), AssignmentListConfig)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage(err.Error())
	}
	// Explicitly typed (rather than :=) so this file imports common/dto — swag
	// resolves the generic @Success type below against this file's imports.
	var result *common.PageResponse[EventStaffAssignment]
	result, err = h.service.List(r.Context(), eventID, params)
	if err != nil {
		return nil, err
	}
	return response.OK(result), nil
}

// GetByID handles GET /events/{event_id}/staff/{id}.
//
// GetByID godoc
//
//	@Summary		Get event staff assignment by ID
//	@Description	Returns a single staff assignment by UUID, scoped to its event. Requires the manage_staff permission.
//	@Tags			staffing
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string	true	"Event UUID"
//	@Param			id			path		string	true	"Staff assignment UUID"
//	@Success		200			{object}	staffing.EventStaffAssignment
//	@Failure		400			{object}	object	"Invalid ID format"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_staff permission"
//	@Failure		404			{object}	object	"Staff assignment or event not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/staff/{id} [get]
func (h *StaffAssignmentHandler) GetByID(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid staff assignment id")
	}
	entity, err := h.service.GetByID(r.Context(), eventID, id)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// Create handles POST /events/{event_id}/staff.
//
// Create godoc
//
//	@Summary		Assign staff to an event
//	@Description	Assigns a tenant user to an event with an event-scoped role. Requires the manage_staff permission.
//	@Tags			staffing
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string							true	"Event UUID"
//	@Param			body		body		staffing.CreateAssignmentInput	true	"Staff assignment payload"
//	@Success		201			{object}	staffing.EventStaffAssignment
//	@Failure		400			{object}	object	"Invalid request body, invalid event id, or role_id not event-scoped"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_staff permission"
//	@Failure		404			{object}	object	"Event, user, or role not found"
//	@Failure		409			{object}	object	"User already assigned to this event"
//	@Failure		422			{object}	object	"Unprocessable entity"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/staff [post]
func (h *StaffAssignmentHandler) Create(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	var body CreateAssignmentInput
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

// Update handles PUT /events/{event_id}/staff/{id}.
//
// Update godoc
//
//	@Summary		Update a staff assignment's role
//	@Description	Changes an existing assignment's event-scoped role. event_id/user_id are immutable; re-assigning a different user requires removing and re-creating the assignment. Requires the manage_staff permission.
//	@Tags			staffing
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string							true	"Event UUID"
//	@Param			id			path		string							true	"Staff assignment UUID"
//	@Param			body		body		staffing.UpdateAssignmentInput	true	"Fields to update"
//	@Success		200			{object}	staffing.EventStaffAssignment
//	@Failure		400			{object}	object	"Invalid ID, request body, or role_id not event-scoped"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_staff permission"
//	@Failure		404			{object}	object	"Staff assignment, event, or role not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/staff/{id} [put]
func (h *StaffAssignmentHandler) Update(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid staff assignment id")
	}
	var body UpdateAssignmentInput
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

// Delete handles DELETE /events/{event_id}/staff/{id}.
//
// Delete godoc
//
//	@Summary		Remove staff from an event
//	@Description	Soft-deletes a staff assignment by ID, scoped to its event. The same user may be re-assigned to the same event later as a new assignment. Requires the manage_staff permission.
//	@Tags			staffing
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path	string	true	"Event UUID"
//	@Param			id			path	string	true	"Staff assignment UUID"
//	@Success		204			"No content"
//	@Failure		400			{object}	object	"Invalid ID format"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_staff permission"
//	@Failure		404			{object}	object	"Staff assignment or event not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/staff/{id} [delete]
func (h *StaffAssignmentHandler) Delete(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid staff assignment id")
	}
	if err := h.service.Delete(r.Context(), eventID, id); err != nil {
		return nil, err
	}
	return response.NoContent(), nil
}
