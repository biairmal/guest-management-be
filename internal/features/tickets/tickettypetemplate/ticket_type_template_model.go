package tickettypetemplate

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TicketTypeTemplate represents a row in the ticket_type_templates table —
// one default ticket type of an event category, at one template version.
// Saved as part of the category's whole template set (Store, B15);
// tickettype's SeedFromCategoryTemplates copies the current version's rows
// onto a new event. Rules is an opaque JSON document, passed through
// unvalidated, matching ticket_types.rules. Supports soft delete via
// deleted_at (a save soft-deletes the previous version's rows, kept as
// history). Uses db tags for reflection-based scanning.
type TicketTypeTemplate struct {
	ID         uuid.UUID       `json:"id"                   db:"id"`
	CategoryID uuid.UUID       `json:"category_id"          db:"category_id"`
	Version    int             `json:"version"              db:"version"`
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
