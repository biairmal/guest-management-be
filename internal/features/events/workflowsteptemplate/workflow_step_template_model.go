package workflowsteptemplate

import (
	"time"

	"github.com/google/uuid"
)

// WorkflowStepTemplate represents a row in the workflow_step_templates
// table — one default workflow step of an event category, at one template
// version. The category's template set is saved as a whole by
// category.Service (B15); EventService.Create copies the current version's
// rows onto a new event's workflow steps. Supports soft delete via
// deleted_at (a save soft-deletes the previous version's rows, kept as
// history). Uses db tags for reflection-based scanning.
type WorkflowStepTemplate struct {
	ID             uuid.UUID  `json:"id"                   db:"id"`
	CategoryID     uuid.UUID  `json:"category_id"          db:"category_id"`
	Version        int        `json:"version"              db:"version"`
	Name           string     `json:"name"                 db:"name"`
	OrderIndex     int        `json:"order_index"          db:"order_index"`
	AllowsMultiple bool       `json:"allows_multiple"      db:"allows_multiple"`
	CreatedAt      time.Time  `json:"created_at"           db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"           db:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// TableName returns the database table name.
func (WorkflowStepTemplate) TableName() string {
	return "workflow_step_templates"
}
