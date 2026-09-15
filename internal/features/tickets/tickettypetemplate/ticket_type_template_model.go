package tickettypetemplate

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TicketTypeTemplate represents a row in the ticket_type_templates table —
// the per-category default ticket types that tickettype.Service's
// SeedFromCategoryTemplates copies onto a newly created event's ticket types
// (via EventService.Create), if any exist for the event's category. Rules is
// an opaque JSON document (entry rules and other config), passed through
// unvalidated, matching ticket_types.rules. Supports soft delete via
// deleted_at. Uses db tags for reflection-based scanning.
//
// swagger:model TicketTypeTemplate
type TicketTypeTemplate struct {
	ID         uuid.UUID       `json:"id"                   db:"id"`
	CategoryID uuid.UUID       `json:"category_id"          db:"category_id"`
	Name       string          `json:"name"                 db:"name"`
	Rules      json.RawMessage `json:"rules"                db:"rules"`
	CreatedAt  time.Time       `json:"created_at"           db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"           db:"updated_at"`
	DeletedAt  *time.Time      `json:"deleted_at,omitempty" db:"deleted_at"`
}

// TableName returns the database table name.
func (TicketTypeTemplate) TableName() string {
	return "ticket_type_templates"
}
