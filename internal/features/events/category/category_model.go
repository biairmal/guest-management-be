package category

import (
	"time"

	"github.com/google/uuid"
)

// EventCategory represents a row in the event_categories table.
// TemplateVersion is the version of the category's current template set
// (workflow step templates + ticket type templates), incremented by every
// Replace. Supports soft delete via deleted_at. Uses db tags for
// reflection-based scanning.
//
// swagger:model EventCategory
type EventCategory struct {
	ID              uuid.UUID  `json:"id"                   db:"id"`
	Source          string     `json:"source"               db:"source"`
	TenantID        *uuid.UUID `json:"tenant_id,omitempty"  db:"tenant_id"`
	Name            string     `json:"name"                 db:"name"`
	TemplateVersion int        `json:"template_version"     db:"template_version"`
	CreatedAt       time.Time  `json:"created_at"           db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"           db:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// TableName returns the database table name.
func (EventCategory) TableName() string {
	return "event_categories"
}
