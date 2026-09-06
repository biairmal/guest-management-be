package guests

import (
	"time"

	"github.com/google/uuid"
)

// Ticket represents a row in the tickets table — the QR-based admission
// artifact issued to a guest for one ticket type. This phase ships model +
// repository only: no HTTP surface of its own (mirrors roles shipping
// model+repository-only in B6) — a ticket is only ever created internally by
// GuestService (on invitation-send when the event doesn't require RSVP, or on
// RSVP confirm) and surfaced through GET .../guests/{id} (see Guest.Ticket).
// Supports soft delete via deleted_at. Uses db tags for reflection-based
// scanning.
//
// swagger:model Ticket
type Ticket struct {
	ID           uuid.UUID  `json:"id"             db:"id"`
	GuestID      uuid.UUID  `json:"guest_id"       db:"guest_id"`
	EventID      uuid.UUID  `json:"event_id"       db:"event_id"`
	TicketTypeID uuid.UUID  `json:"ticket_type_id" db:"ticket_type_id"`
	QRCode       string     `json:"qr_code"        db:"qr_code"`
	Status       string     `json:"status"         db:"status"`
	CreatedAt    time.Time  `json:"created_at"     db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"     db:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// TableName returns the database table name.
func (Ticket) TableName() string {
	return "tickets"
}
