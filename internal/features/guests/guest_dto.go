package guests

import "github.com/google/uuid"

// CreateGuestInput is the input for creating a guest under an event.
// event_id is taken from the URL, not part of this input. ticket_type_id is
// optional at creation — it can be assigned later via UpdateGuestInput, but
// must be set before SendInvitation is called.
//
// swagger:model CreateGuestInput
type CreateGuestInput struct {
	Name         string     `json:"name"                     validate:"required"`
	Email        string     `json:"email"                    validate:"required,email"`
	Phone        *string    `json:"phone,omitempty"`
	TicketTypeID *uuid.UUID `json:"ticket_type_id,omitempty"`
}

// UpdateGuestInput is the input for partially updating a guest. event_id is
// immutable and not part of this input. A provided ticket_type_id is
// re-validated to belong to the guest's own event.
//
// swagger:model UpdateGuestInput
type UpdateGuestInput struct {
	Name         *string    `json:"name,omitempty"           validate:"omitempty,min=1"`
	Email        *string    `json:"email,omitempty"          validate:"omitempty,email"`
	Phone        *string    `json:"phone,omitempty"`
	TicketTypeID *uuid.UUID `json:"ticket_type_id,omitempty"`
}

// SendInvitationOutput is the response for POST .../guests/{id}/invitation.
// InvitationToken is surfaced here — and only here — since there's no real
// email channel yet to deliver the RSVP link through.
//
// swagger:model SendInvitationOutput
type SendInvitationOutput struct {
	Guest           *Guest `json:"guest"`
	InvitationToken string `json:"invitation_token"`
}

// RSVPInput is the input for the public guest-facing RSVP endpoint.
//
// swagger:model RSVPInput
type RSVPInput struct {
	Status string `json:"status" validate:"required,oneof=confirmed declined"`
}
