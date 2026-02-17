package domain

import (
	"time"

	"github.com/google/uuid"
)

// Entity is the base entity that has an ID.
type Entity struct {
	ID uuid.UUID `json:"id"`
}

// AuditableEntity is an entity that has created_at, updated_at, and deleted_at fields.
type AuditableEntity struct {
	ID        uuid.UUID  `json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}
