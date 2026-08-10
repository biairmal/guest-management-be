package templates

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	// SourceApp denotes a system-defined message template, available to every
	// tenant (tenant_id and event_id must be nil).
	SourceApp = "app"
	// SourceTenant denotes a tenant-defined message template (tenant_id
	// required, event_id must be nil).
	SourceTenant = "tenant"
	// SourceEvent denotes an event-specific message template (tenant_id and
	// event_id both required).
	SourceEvent = "event"
)

const (
	// ChannelEmail sends the template body as an email; subject is required.
	ChannelEmail = "email"
	// ChannelWhatsapp sends the template body as a WhatsApp message; subject
	// must be empty.
	ChannelWhatsapp = "whatsapp"
)

// MessageTemplate represents a row in the message_templates table — a
// reusable email/WhatsApp message body (e.g. invitation, ticket delivery,
// thank you). Scoped by Source to app (global), tenant, or event; resolving
// which template applies to a given event prefers event, then tenant, then
// app (see REQUIREMENT.md ss2.2). Supports soft delete via deleted_at. Uses
// db tags for reflection-based scanning.
//
// swagger:model MessageTemplate
type MessageTemplate struct {
	ID        uuid.UUID       `json:"id"                   db:"id"`
	Source    string          `json:"source"                db:"source"`
	TenantID  *uuid.UUID      `json:"tenant_id,omitempty"   db:"tenant_id"`
	EventID   *uuid.UUID      `json:"event_id,omitempty"    db:"event_id"`
	Name      string          `json:"name"                  db:"name"`
	Channel   string          `json:"channel"                db:"channel"`
	Subject   *string         `json:"subject,omitempty"     db:"subject"`
	Body      string          `json:"body"                  db:"body"`
	Variables json.RawMessage `json:"variables,omitempty"   db:"variables"`
	CreatedAt time.Time       `json:"created_at"            db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"            db:"updated_at"`
	DeletedAt *time.Time      `json:"deleted_at,omitempty"  db:"deleted_at"`
}

// TableName returns the database table name.
func (MessageTemplate) TableName() string {
	return "message_templates"
}
