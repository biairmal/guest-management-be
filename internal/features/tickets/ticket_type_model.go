package tickets

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TicketType represents a row in the ticket_types table — a ticket type
// scoped to one event (e.g. Regular, VIP). Rules is an opaque JSON document
// (entry rules and other config), passed through unvalidated. WorkflowStepIDs
// is not a database column (db:"-") — it is populated only by
// TicketTypeService.GetByID/ReplaceWorkflowSteps from the
// ticket_type_workflow_steps junction, and left empty on List/Update to
// avoid an N+1 lookup. Supports soft delete via deleted_at. Uses db tags for
// reflection-based scanning.
//
// swagger:model TicketType
type TicketType struct {
	ID              uuid.UUID       `json:"id"                          db:"id"`
	EventID         uuid.UUID       `json:"event_id"                    db:"event_id"`
	Name            string          `json:"name"                        db:"name"`
	Rules           json.RawMessage `json:"rules"                       db:"rules"`
	WorkflowStepIDs []uuid.UUID     `json:"workflow_step_ids,omitempty" db:"-"`
	CreatedAt       time.Time       `json:"created_at"                  db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"                  db:"updated_at"`
	DeletedAt       *time.Time      `json:"deleted_at,omitempty"        db:"deleted_at"`
}

// TableName returns the database table name.
func (TicketType) TableName() string {
	return "ticket_types"
}
