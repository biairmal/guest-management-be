package workflowsteptemplate

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// WorkflowStepTemplate represents a row in the workflow_step_templates
// table — the per-category default workflow steps that EventService.Create
// copies onto a newly created event's workflow steps (if any exist for the
// event's category; see EventService). Supports soft delete via deleted_at.
// Uses db tags for reflection-based scanning.
//
// swagger:model WorkflowStepTemplate
type WorkflowStepTemplate struct {
	ID                      uuid.UUID        `json:"id"                                   db:"id"`
	CategoryID              uuid.UUID        `json:"category_id"                          db:"category_id"`
	Name                    string           `json:"name"                                 db:"name"`
	OrderIndex              int              `json:"order_index"                          db:"order_index"`
	AllowsMultiple          bool             `json:"allows_multiple"                      db:"allows_multiple"`
	TicketTypeApplicability *json.RawMessage `json:"ticket_type_applicability,omitempty" db:"ticket_type_applicability"`
	CreatedAt               time.Time        `json:"created_at"                           db:"created_at"`
	UpdatedAt               time.Time        `json:"updated_at"                           db:"updated_at"`
	DeletedAt               *time.Time       `json:"deleted_at,omitempty"                 db:"deleted_at"`
}

// TableName returns the database table name.
func (WorkflowStepTemplate) TableName() string {
	return "workflow_step_templates"
}
