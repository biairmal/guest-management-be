package tickets

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

const ticketTypesTable = "ticket_types"

// ticketTypeColumns are the columns selected on reads (GetByID, List).
// WorkflowStepIDs is not a column (see ticket_type_model.go) and is excluded.
var ticketTypeColumns = []string{"id", "event_id", "name", "rules", "created_at", "updated_at", "deleted_at"}

// NewTicketTypeRepository returns a soft-delete-aware repository for ticket
// types. TID is uuid.UUID — kept typed all the way through the service layer.
func NewTicketTypeRepository(
	log logger.Logger, db *sqlkit.DB, cacheOpts corerepository.CacheOptions,
) repository.Repository[TicketType, uuid.UUID] {
	return corerepository.NewRepository[TicketType, uuid.UUID](
		log, db, ticketTypesTable, ticketTypeColumns, cacheOpts,
	)
}
