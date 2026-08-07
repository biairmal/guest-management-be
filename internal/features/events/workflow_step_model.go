package events

import (
	"time"

	"github.com/google/uuid"
)

// WorkflowStep represents a row in the workflow_steps table — an event-level
// lifecycle stage (e.g. Check-in, Photo booth). OrderIndex is unique per
// event. Supports soft delete via deleted_at. Uses db tags for
// reflection-based scanning.
//
// swagger:model WorkflowStep
type WorkflowStep struct {
	ID             uuid.UUID  `json:"id"                  db:"id"`
	EventID        uuid.UUID  `json:"event_id"            db:"event_id"`
	Name           string     `json:"name"                db:"name"`
	OrderIndex     int        `json:"order_index"         db:"order_index"`
	AllowsMultiple bool       `json:"allows_multiple"     db:"allows_multiple"`
	CreatedAt      time.Time  `json:"created_at"          db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"          db:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// TableName returns the database table name.
func (WorkflowStep) TableName() string {
	return "workflow_steps"
}
