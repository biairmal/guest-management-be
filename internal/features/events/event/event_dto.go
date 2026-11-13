package event

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CreateEventInput is the input for creating an event. tenant_id is not part
// of this input: the event belongs to the caller's tenant, taken from the JWT
// (B16 R4), so one tenant can never create an event under another.
//
// swagger:model CreateEventInput
type CreateEventInput struct {
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

// ListItem is one event in the list response: the event plus its guest
// progress (B16 R5). A list-item DTO rather than fields on Event — the counts
// are derived per page, not columns, and GET /{id} does not report them.
//
// swagger:model EventListItem
type ListItem struct {
	Event
	// GuestCount is how many guests are on the event.
	GuestCount int `json:"guest_count"`
	// ConfirmedCount is how many of them have rsvp_status = confirmed.
	ConfirmedCount int `json:"confirmed_count"`
}

// WorkflowStepInput is one workflow step of an event's setup. Its position in
// the workflow_steps array is its step order. An id updates that existing step
// in place (guests, tickets and scan logs reference step ids, so the setup save
// reconciles rather than recreating); omitting it creates a new step.
//
// swagger:model EventWorkflowStepInput
type WorkflowStepInput struct {
	ID             *uuid.UUID `json:"id,omitempty"`
	Name           string     `json:"name"            validate:"required"`
	AllowsMultiple bool       `json:"allows_multiple"`
}

// TicketTypeInput is one ticket type of an event's setup. Steps are indexes
// into the same payload's workflow_steps array; rules is an opaque JSON
// document, passed through unvalidated (defaults to {}). An id updates that
// existing ticket type in place, keeping the tickets already issued against it.
//
// swagger:model EventTicketTypeInput
type TicketTypeInput struct {
	ID    *uuid.UUID      `json:"id,omitempty"`
	Name  string          `json:"name"            validate:"required"`
	Rules json.RawMessage `json:"rules,omitempty" swaggertype:"object"`
	Steps []int           `json:"steps"`
}

// SetupInput is the input for replacing an event's whole setup in one request:
// its workflow steps in order, its ticket types, and each ticket type's step
// access. Items sent with an id are updated, items without one are created, and
// existing items left out are deleted. The max=100 array limits match
// query.DefaultMaxSize, the limit the setup read pages with, so a setup is
// never silently truncated.
//
// swagger:model EventSetupInput
type SetupInput struct {
	WorkflowSteps []WorkflowStepInput `json:"workflow_steps" validate:"max=100,dive"`
	TicketTypes   []TicketTypeInput   `json:"ticket_types"   validate:"max=100,dive"`
}

// Setup is an event's workflow steps and ticket types — the response of
// GET and PUT on /events/{event_id}/setup.
//
// swagger:model EventSetup
type Setup struct {
	WorkflowSteps []WorkflowStepView `json:"workflow_steps"`
	TicketTypes   []TicketTypeView   `json:"ticket_types"`
}

// WorkflowStepView is one of an event's workflow steps; array order is step
// order.
type WorkflowStepView struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	AllowsMultiple bool      `json:"allows_multiple"`
}

// TicketTypeView is one of an event's ticket types; Steps are indexes into
// Setup.WorkflowSteps, ascending — the same convention as a category's
// template set.
type TicketTypeView struct {
	ID    uuid.UUID       `json:"id"`
	Name  string          `json:"name"`
	Rules json.RawMessage `json:"rules" swaggertype:"object"`
	Steps []int           `json:"steps"`
}

// TicketTypeStoreView is a stored ticket type as TicketTypeStore reports it,
// with the step access as step ids rather than payload indexes.
type TicketTypeStoreView struct {
	ID              uuid.UUID
	Name            string
	Rules           json.RawMessage
	WorkflowStepIDs []uuid.UUID
}

// TicketTypeDraft is a ticket type to write through TicketTypeStore. A non-nil
// ID updates that ticket type in place; nil creates one. Step indexes are
// already resolved to the saved workflow step ids.
type TicketTypeDraft struct {
	ID              *uuid.UUID
	Name            string
	Rules           json.RawMessage
	WorkflowStepIDs []uuid.UUID
}

// GuestCounts is one event's guest progress as GuestStats reports it.
type GuestCounts struct {
	Total     int
	Confirmed int
}
