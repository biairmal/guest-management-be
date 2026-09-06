package event

import (
	"time"

	"github.com/google/uuid"
)

// CreateEventInput is the input for creating an event.
//
// swagger:model CreateEventInput
type CreateEventInput struct {
	TenantID    uuid.UUID `json:"tenant_id"             validate:"required"`
	CategoryID  uuid.UUID `json:"category_id"           validate:"required"`
	Name        string    `json:"name"                  validate:"required"`
	Description *string   `json:"description,omitempty"`
	StartDate   time.Time `json:"start_date"            validate:"required"`
	EndDate     time.Time `json:"end_date"              validate:"required"`
	// RsvpRequired defaults to true when omitted — see event_service.go Create.
	// When false, a guest's ticket is issued at invitation time instead of
	// waiting on RSVP confirmation (see internal/features/guests).
	RsvpRequired *bool `json:"rsvp_required,omitempty"`
}

// UpdateEventInput is the input for updating an event. tenant_id is set once
// at creation and is not part of this input — events do not move tenants.
//
// swagger:model UpdateEventInput
type UpdateEventInput struct {
	CategoryID   *uuid.UUID `json:"category_id,omitempty"`
	Name         *string    `json:"name,omitempty"        validate:"omitempty,min=1"`
	Description  *string    `json:"description,omitempty"`
	StartDate    *time.Time `json:"start_date,omitempty"`
	EndDate      *time.Time `json:"end_date,omitempty"`
	RsvpRequired *bool      `json:"rsvp_required,omitempty"`
}
