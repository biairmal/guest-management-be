package event

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

const eventsTable = "events"

// eventColumns are the columns selected on reads (GetByID, List).
var eventColumns = []string{
	"id", "tenant_id", "category_id", "name", "description",
	"start_date", "end_date", "is_multi_day", "rsvp_required", "created_at", "updated_at", "deleted_at",
}

// NewEventRepository returns a soft-delete-aware repository for events.
// TID is uuid.UUID — kept typed all the way through the service layer.
func NewEventRepository(
	log logger.Logger, db *sqlkit.DB, cacheOpts corerepository.CacheOptions,
) repository.Repository[Event, uuid.UUID] {
	return corerepository.NewRepository[Event, uuid.UUID](
		log, db, eventsTable, eventColumns, cacheOpts,
	)
}
