package events

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/repository"
	"github.com/biairmal/go-sdk/repository/sql"
	"github.com/biairmal/go-sdk/sqlkit"
	"github.com/biairmal/guest-management-be/internal/core/audit"
	"github.com/google/uuid"
)

const eventCategoriesTable = "event_categories"

// NewCategoryRepository returns a soft-delete-aware repository for event categories.
// TID is uuid.UUID — kept typed all the way through the service layer.
func NewCategoryRepository(log logger.Logger, db *sqlkit.DB) repository.Repository[EventCategory, uuid.UUID] {
	sqlRepo := sql.NewSQLRepository[EventCategory, uuid.UUID](
		log,
		db,
		eventCategoriesTable,
		sql.WithSelectColumns[EventCategory, uuid.UUID]([]string{
			"id", "source", "tenant_id", "name", "created_at", "updated_at", "deleted_at",
		}),
	)
	return audit.NewAuditableRepository[EventCategory, uuid.UUID](sqlRepo)
}
