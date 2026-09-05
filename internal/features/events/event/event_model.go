package event

import (
	"time"

	"github.com/google/uuid"
)

// Event represents a row in the events table — a tenant's event, classified
// under an event category, with a start/end date range. IsMultiDay is
// derived by the service from StartDate/EndDate, not caller-supplied.
// Supports soft delete via deleted_at. Uses db tags for reflection-based
// scanning.
//
// swagger:model Event
type Event struct {
	ID          uuid.UUID  `json:"id"                     db:"id"`
	TenantID    uuid.UUID  `json:"tenant_id"               db:"tenant_id"`
	CategoryID  uuid.UUID  `json:"category_id"             db:"category_id"`
	Name        string     `json:"name"                    db:"name"`
	Description *string    `json:"description,omitempty"   db:"description"`
	StartDate   time.Time  `json:"start_date"              db:"start_date"`
	EndDate     time.Time  `json:"end_date"                db:"end_date"`
	IsMultiDay  bool       `json:"is_multi_day"            db:"is_multi_day"`
	CreatedAt   time.Time  `json:"created_at"              db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"              db:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"    db:"deleted_at"`
}

// TableName returns the database table name.
func (Event) TableName() string {
	return "events"
}
