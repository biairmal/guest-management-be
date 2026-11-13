package event

import (
	"context"

	"github.com/google/uuid"
)

// TicketTypeStore reads and reconciles an event's ticket types — the tickets
// half of an event's setup save (B16 R1). Implemented by tickets'
// tickettype.SetupStore; wired in internal/app so events never imports
// tickets, the same consumer-side idiom as TicketTypeSeeder and category's
// TicketTypeTemplateStore. Runs inside the caller's transaction (ctx) and
// returns errorz errors.
//
// No generated mock: it would import this package's DTOs, and this package's
// own test is its only consumer — an import cycle. event_service__test.go uses
// a small stub instead, the same accepted exception as stubTicketTypeSeeder.
type TicketTypeStore interface {
	// ListForEvent returns eventID's live ticket types with the steps each
	// one includes.
	ListForEvent(ctx context.Context, eventID uuid.UUID) ([]TicketTypeStoreView, error)
	// ReplaceForEvent reconciles eventID's ticket types to exactly match in:
	// a draft with an ID is updated in place and keeps that id (the tickets
	// already issued against it stay valid), one without is created, and any
	// live ticket type not in in is soft-deleted. Returns what was written,
	// in payload order — so the caller can build its response without a
	// second read, as category's ReplaceForCategory does.
	ReplaceForEvent(ctx context.Context, eventID uuid.UUID, in []TicketTypeDraft) ([]TicketTypeStoreView, error)
}

// GuestStats counts guests for event screens. Implemented by guests; wired in
// internal/app so events never imports guests. Same no-generated-mock reason
// as TicketTypeStore.
type GuestStats interface {
	// CountByEvents returns the guest progress of each of eventIDs in one
	// query. An event with no guests is absent from the map (read as zeros).
	CountByEvents(ctx context.Context, eventIDs []uuid.UUID) (map[uuid.UUID]GuestCounts, error)
	// CountByTicketType returns how many of eventID's guests hold each ticket
	// type, so the setup save can refuse to remove one that is still held.
	CountByTicketType(ctx context.Context, eventID uuid.UUID) (map[uuid.UUID]int, error)
}
