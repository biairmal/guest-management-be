package tickettypetemplate

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

const ticketTypeTemplatesTable = "ticket_type_templates"

// ticketTypeTemplateColumns are the columns selected on reads (GetByID, List).
var ticketTypeTemplateColumns = []string{
	"id", "category_id", "version", "name", "rules", "created_at", "updated_at", "deleted_at",
}

// NewTicketTypeTemplateRepository returns a soft-delete-aware repository for
// ticket type templates. TID is uuid.UUID — kept typed all the way through
// the service layer.
func NewTicketTypeTemplateRepository(
	log logger.Logger, db *sqlkit.DB, cacheOpts corerepository.CacheOptions,
) repository.Repository[TicketTypeTemplate, uuid.UUID] {
	return corerepository.NewRepository[TicketTypeTemplate, uuid.UUID](
		log, db, ticketTypeTemplatesTable, ticketTypeTemplateColumns, cacheOpts,
	)
}
