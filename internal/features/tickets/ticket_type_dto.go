package tickets

import (
	"encoding/json"

	"github.com/google/uuid"
)

// CreateTicketTypeInput is the input for creating a ticket type under an
// event. event_id is taken from the URL, not part of this input. rules is an
// opaque JSON document, passed through unvalidated; it defaults to an empty
// object when omitted, matching the column default.
//
// swagger:model CreateTicketTypeInput
type CreateTicketTypeInput struct {
	Name  string          `json:"name"            validate:"required"`
	Rules json.RawMessage `json:"rules,omitempty"`
}

// UpdateTicketTypeInput is the input for partially updating a ticket type's
// name/rules. event_id is immutable and not part of this input.
//
// swagger:model UpdateTicketTypeInput
type UpdateTicketTypeInput struct {
	Name  *string         `json:"name,omitempty"  validate:"omitempty,min=1"`
	Rules json.RawMessage `json:"rules,omitempty"`
}

// ReplaceWorkflowStepsInput is the input for
// PUT .../ticket-types/{id}/workflow-steps: the full desired set of workflow
// step IDs this ticket type applies to, replacing whatever was set before
// (full-replace semantics, no incremental add/remove).
//
// swagger:model ReplaceWorkflowStepsInput
type ReplaceWorkflowStepsInput struct {
	WorkflowStepIDs []uuid.UUID `json:"workflow_step_ids"`
}
