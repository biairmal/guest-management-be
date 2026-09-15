package event

import (
	"context"

	"github.com/google/uuid"
)

// TicketTypeSeeder seeds new ticket types for a newly created event from its
// category's ticket-type templates (B12). Implemented by tickets'
// tickettype.Service; wired in internal/app — the only layer allowed to know
// both events and tickets — so this package never imports tickets, keeping
// each slice independently extractable (docs/ARCHITECTURE.md). Same
// interface-at-the-consumer idiom already used for guests'
// PIIEncryptor/InvitationPublisher.
//
// Returns error, not best-effort: Create runs this inside a transaction (see
// event_service.go's Create), so a seeding failure must propagate to roll
// back the whole event creation rather than being swallowed.
//
// No //go:generate mock is declared for this interface: a generated mock
// would need to live in mocks/events/event (alongside Service's own mock,
// which already imports package event) and this package's own test file
// (event_service__test.go) is the only thing that needs to stub it — that
// would import a package that imports event back into event's own
// same-package test, an import cycle. event_service__test.go uses a small
// hand-written stub instead (stubTicketTypeSeeder), the same accepted
// exception as guests' noopInvitationPublisher.
type TicketTypeSeeder interface {
	SeedFromCategoryTemplates(ctx context.Context, eventID, categoryID uuid.UUID) error
}
