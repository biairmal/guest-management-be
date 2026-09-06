package guests

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

const ticketsTable = "tickets"

// ticketColumns are the columns selected on reads (GetByID, List).
var ticketColumns = []string{
	"id", "guest_id", "event_id", "ticket_type_id", "qr_code", "status", "created_at", "updated_at", "deleted_at",
}

// NewTicketRepository returns a soft-delete-aware repository for tickets.
// TID is uuid.UUID — kept typed all the way through the service layer. No
// PII wrapping needed here (unlike NewGuestRepository) — nothing on Ticket
// is encrypted.
func NewTicketRepository(
	log logger.Logger, db *sqlkit.DB, cacheOpts corerepository.CacheOptions,
) repository.Repository[Ticket, uuid.UUID] {
	return corerepository.NewRepository[Ticket, uuid.UUID](
		log, db, ticketsTable, ticketColumns, cacheOpts,
	)
}
