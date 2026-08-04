package tenants

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Tenant represents a row in the tenants table — an organization using the
// platform. Settings and branding are opaque JSONB documents owned by the
// caller; the service defaults them to an empty object when omitted.
// Supports soft delete via deleted_at. Uses db tags for reflection-based scanning.
//
// swagger:model Tenant
type Tenant struct {
	ID        uuid.UUID       `json:"id" db:"id"`
	Name      string          `json:"name" db:"name"`
	Type      *string         `json:"type,omitempty" db:"type"`
	Settings  json.RawMessage `json:"settings" db:"settings"`
	Branding  json.RawMessage `json:"branding" db:"branding"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time      `json:"deleted_at,omitempty" db:"deleted_at"`
}

// TableName returns the database table name.
func (Tenant) TableName() string {
	return "tenants"
}
