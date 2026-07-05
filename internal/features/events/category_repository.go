package events

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/repository"
	"github.com/biairmal/go-sdk/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

const eventCategoriesTable = "event_categories"

// eventCategoryColumns are the columns selected on reads (GetByID, List).
var eventCategoryColumns = []string{
	"id", "source", "tenant_id", "name", "created_at", "updated_at", "deleted_at",
}

// NewCategoryRepository returns a soft-delete-aware repository for event categories.
// TID is uuid.UUID — kept typed all the way through the service layer.
func NewCategoryRepository(log logger.Logger, db *sqlkit.DB) repository.Repository[EventCategory, uuid.UUID] {
	return corerepository.NewRepository[EventCategory, uuid.UUID](log, db, eventCategoriesTable, eventCategoryColumns)
}
