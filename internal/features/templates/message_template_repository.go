package templates

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

const messageTemplatesTable = "message_templates"

// messageTemplateColumns are the columns selected on reads (GetByID, List).
var messageTemplateColumns = []string{
	"id", "source", "tenant_id", "event_id", "name", "channel",
	"subject", "body", "variables", "created_at", "updated_at", "deleted_at",
}

// NewMessageTemplateRepository returns a soft-delete-aware repository for
// message templates. TID is uuid.UUID — kept typed all the way through the
// service layer.
func NewMessageTemplateRepository(
	log logger.Logger, db *sqlkit.DB, cacheOpts corerepository.CacheOptions,
) repository.Repository[MessageTemplate, uuid.UUID] {
	return corerepository.NewRepository[MessageTemplate, uuid.UUID](
		log, db, messageTemplatesTable, messageTemplateColumns, cacheOpts,
	)
}
