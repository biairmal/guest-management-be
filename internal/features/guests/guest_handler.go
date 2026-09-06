package guests

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

// GuestHandler exposes HTTP handlers for event-scoped guest CRUD, invitation
// sending, and the public guest-facing RSVP endpoint.
type GuestHandler struct {
	service   GuestService
	validator validation.Validator
}

// NewGuestHandler returns a GuestHandler that uses the given service and
// validator. Both are interfaces, allowing easy testing and substitution.
func NewGuestHandler(service GuestService, validator validation.Validator) *GuestHandler {
	return &GuestHandler{service: service, validator: validator}
}

// eventIDFromPath parses the {event_id} path parameter.
func eventIDFromPath(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "event_id"))
	if err != nil {
		return uuid.Nil, errorz.BadRequest().WithMessage("invalid event id")
	}
	return id, nil
}

// List handles GET /events/{event_id}/guests with query parameters.
//
// List godoc
//
//	@Summary		List event guests
//	@Description	Returns a paginated list of guests for an event. Query: page, size, sort=field,dir (repeatable), filter by allowed fields (name, email, phone, rsvp_status); name supports a partial match via "value;like", email/phone only support exact match. Requires the manage_guests permission.
//	@Tags			guests
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string	true	"Event UUID"
//	@Param			page		query		int		false	"Page number (1-based)"
//	@Param			size		query		int		false	"Page size (default 20, max 100)"
//	@Param			sort		query		string	false	"Sort: field,dir (e.g. sort=name,ASC)"
//	@Param			name		query		string	false	"Filter by name — exact, or partial via name=value;like"
//	@Success		200			{object}	common.PageResponse[guests.Guest]
//	@Failure		400			{object}	object	"Invalid event id or query"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_guests permission"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/guests [get]
func (h *GuestHandler) List(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	params, err := query.ParseListParams(r.URL.Query(), GuestListConfig)
	if err != nil {
		return nil, errorz.BadRequest().WithMessage(err.Error())
	}
	// Explicitly typed (rather than :=) so this file imports common/dto — swag
	// resolves the generic @Success type below against this file's imports.
	var result *common.PageResponse[Guest]
	result, err = h.service.List(r.Context(), eventID, params)
	if err != nil {
		return nil, err
	}
	return response.OK(result), nil
}

// GetByID handles GET /events/{event_id}/guests/{id}.
//
// GetByID godoc
//
//	@Summary		Get guest by ID
//	@Description	Returns a single guest by UUID, scoped to its event, including the issued ticket/QR summary once one exists. Requires the manage_guests permission.
//	@Tags			guests
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string	true	"Event UUID"
//	@Param			id			path		string	true	"Guest UUID"
//	@Success		200			{object}	guests.Guest
//	@Failure		400			{object}	object	"Invalid ID format"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_guests permission"
//	@Failure		404			{object}	object	"Guest not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/guests/{id} [get]
func (h *GuestHandler) GetByID(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid guest id")
	}
	entity, err := h.service.GetByID(r.Context(), eventID, id)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}

// Create handles POST /events/{event_id}/guests.
//
// Create godoc
//
//	@Summary		Create guest
//	@Description	Creates a new guest under an event; rsvp_status starts at none. ticket_type_id, if provided, must belong to this event. Requires the manage_guests permission.
//	@Tags			guests
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string					true	"Event UUID"
//	@Param			body		body		guests.CreateGuestInput	true	"Guest payload"
//	@Success		201			{object}	guests.Guest
//	@Failure		400			{object}	object	"Invalid request body or ticket type not on this event"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_guests permission"
//	@Failure		422			{object}	object	"Unprocessable entity"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/guests [post]
func (h *GuestHandler) Create(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	var body CreateGuestInput
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

// Update handles PUT /events/{event_id}/guests/{id}.
//
// Update godoc
//
//	@Summary		Update guest
//	@Description	Partially updates a guest's name/email/phone/ticket_type_id. Only provided fields are applied; event_id is immutable. A provided ticket_type_id must belong to this event. Requires the manage_guests permission.
//	@Tags			guests
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string					true	"Event UUID"
//	@Param			id			path		string					true	"Guest UUID"
//	@Param			body		body		guests.UpdateGuestInput	true	"Fields to update"
//	@Success		200			{object}	guests.Guest
//	@Failure		400			{object}	object	"Invalid ID, request body, or ticket type not on this event"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_guests permission"
//	@Failure		404			{object}	object	"Guest not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/guests/{id} [put]
func (h *GuestHandler) Update(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid guest id")
	}
	var body UpdateGuestInput
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

// Delete handles DELETE /events/{event_id}/guests/{id}.
//
// Delete godoc
//
//	@Summary		Delete guest
//	@Description	Soft-deletes a guest by ID, scoped to its event. Requires the manage_guests permission.
//	@Tags			guests
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path	string	true	"Event UUID"
//	@Param			id			path	string	true	"Guest UUID"
//	@Success		204			"No content"
//	@Failure		400			{object}	object	"Invalid ID format"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_guests permission"
//	@Failure		404			{object}	object	"Guest not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/guests/{id} [delete]
func (h *GuestHandler) Delete(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid guest id")
	}
	if err := h.service.Delete(r.Context(), eventID, id); err != nil {
		return nil, err
	}
	return response.NoContent(), nil
}

// SendInvitation handles POST /events/{event_id}/guests/{id}/invitation.
//
// SendInvitation godoc
//
//	@Summary		Send guest invitation
//	@Description	Requires a ticket type already assigned. Generates an invitation token (returned only in this response — there's no email channel yet) and sets rsvp_status to invited. If the event's rsvp_required is false, also issues the guest's ticket immediately. Requires the manage_guests permission.
//	@Tags			guests
//	@Accept			json
//	@Produce		json
//	@Param			event_id	path		string	true	"Event UUID"
//	@Param			id			path		string	true	"Guest UUID"
//	@Success		200			{object}	guests.SendInvitationOutput
//	@Failure		400			{object}	object	"No ticket type assigned"
//	@Failure		401			{object}	object	"Unauthenticated"
//	@Failure		403			{object}	object	"Missing manage_guests permission"
//	@Failure		404			{object}	object	"Guest not found"
//	@Failure		500			{object}	object	"Internal server error"
//	@Security		BearerAuth
//	@Router			/api/v1/events/{event_id}/guests/{id}/invitation [post]
func (h *GuestHandler) SendInvitation(r *http.Request) (any, error) {
	eventID, err := eventIDFromPath(r)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid guest id")
	}
	out, err := h.service.SendInvitation(r.Context(), eventID, id)
	if err != nil {
		return nil, err
	}
	return response.OK(out), nil
}

// RSVP handles POST /guests/rsvp/{token} — public, unauthenticated (see
// guest_routes.go / config.yaml's route policy). token alone identifies the
// guest, since the caller is never logged in.
//
// RSVP godoc
//
//	@Summary		Guest RSVP
//	@Description	Confirms or declines an invitation by its token — no authentication required. Confirming issues the guest's ticket if one hasn't already been issued.
//	@Tags			guests
//	@Accept			json
//	@Produce		json
//	@Param			token	path		string				true	"Invitation token"
//	@Param			body	body		guests.RSVPInput	true	"RSVP status"
//	@Success		200		{object}	guests.Guest
//	@Failure		400		{object}	object	"Invalid request body"
//	@Failure		404		{object}	object	"Invitation not found"
//	@Failure		500		{object}	object	"Internal server error"
//	@Router			/api/v1/guests/rsvp/{token} [post]
func (h *GuestHandler) RSVP(r *http.Request) (any, error) {
	token := chi.URLParam(r, "token")
	var body RSVPInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest().WithMessage("invalid request body")
	}
	if err := h.validator.Struct(body); err != nil {
		return nil, err
	}
	entity, err := h.service.RSVP(r.Context(), token, body.Status)
	if err != nil {
		return nil, err
	}
	return response.OK(entity), nil
}
