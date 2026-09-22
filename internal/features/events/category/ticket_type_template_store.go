package category

import (
	"context"

	"github.com/google/uuid"
)

// TicketTypeTemplateStore reads and replaces a category's ticket type
// templates for one template version. Implemented by tickets'
// tickettypetemplate.Store; wired in internal/app so events never imports
// tickets (same consumer-side idiom as event.TicketTypeSeeder). Runs inside
// the caller's transaction (ctx) and returns errorz errors.
//
// No generated mock: it would import this package's DTOs, and this package's
// own test is its only consumer — an import cycle. category_service__test.go
// uses a small stub instead, the same accepted exception as event's
// stubTicketTypeSeeder.
type TicketTypeTemplateStore interface {
	ListForCategory(ctx context.Context, categoryID uuid.UUID, version int) ([]TicketTypeTemplateView, error)
	// ReplaceForCategory soft-deletes the category's live ticket type
	// templates and inserts in at version, returning what was written.
	ReplaceForCategory(
		ctx context.Context, categoryID uuid.UUID, version int, in []TicketTypeTemplateDraft,
	) ([]TicketTypeTemplateView, error)
}
