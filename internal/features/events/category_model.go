package events

import (
	"time"

	"github.com/google/uuid"
)

const (
	// SourceApp denotes a system-defined category (tenant_id must be nil).
	SourceApp = "app"
	// SourceTenant denotes a tenant-defined category (tenant_id required).
	SourceTenant = "tenant"
)

// EventCategory represents a row in the event_categories table.
// Supports soft delete via deleted_at. Uses db tags for reflection-based scanning.
//
// swagger:model EventCategory
type EventCategory struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	Source    string     `json:"source" db:"source"`
	TenantID  *uuid.UUID `json:"tenant_id,omitempty" db:"tenant_id"`
	Name      string     `json:"name" db:"name"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// TableName returns the database table name.
func (EventCategory) TableName() string {
	return "event_categories"
}
