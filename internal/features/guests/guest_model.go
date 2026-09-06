package guests

import (
	"time"

	"github.com/google/uuid"
)

// Guest represents a row in the guests table — a guest invited to one event.
// Email/Phone hold plaintext in memory; guestRepository transparently
// encrypts them before every write and decrypts them after every read (see
// guest_repository.go), so nothing outside this file's db tags ever sees
// ciphertext. EmailHash/PhoneHash are deterministic blind indexes used only
// for exact-match search (tagged json:"-" — never serialized).
// InvitationToken is also never serialized directly; it's returned only in
// SendInvitationOutput, since there's no other channel to deliver it through
// yet. Ticket is not a database column (db:"-") — populated only by
// GuestService.GetByID once a ticket has been issued. Supports soft delete
// via deleted_at. Uses db tags for reflection-based scanning.
//
// swagger:model Guest
type Guest struct {
	ID              uuid.UUID  `json:"id"                          db:"id"`
	EventID         uuid.UUID  `json:"event_id"                    db:"event_id"`
	Name            string     `json:"name"                        db:"name"`
	Email           string     `json:"email"                       db:"email"`
	Phone           *string    `json:"phone,omitempty"             db:"phone"`
	EmailHash       string     `json:"-"                           db:"email_hash"`
	PhoneHash       *string    `json:"-"                           db:"phone_hash"`
	TicketTypeID    *uuid.UUID `json:"ticket_type_id,omitempty"    db:"ticket_type_id"`
	InvitationToken *string    `json:"-"                           db:"invitation_token"`
	RsvpStatus      string     `json:"rsvp_status"                 db:"rsvp_status"`
	TicketID        *uuid.UUID `json:"ticket_id,omitempty"         db:"ticket_id"`
	Ticket          *Ticket    `json:"ticket,omitempty"            db:"-"`
	CreatedAt       time.Time  `json:"created_at"                  db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"                  db:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"        db:"deleted_at"`
}

// TableName returns the database table name.
func (Guest) TableName() string {
	return "guests"
}
