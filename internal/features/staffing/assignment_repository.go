package staffing

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

const eventStaffAssignmentsTable = "event_staff_assignments"

// assignmentColumns are the columns selected on reads (GetByID, List).
var assignmentColumns = []string{"id", "event_id", "user_id", "role_id", "created_at", "updated_at", "deleted_at"}

// NewStaffAssignmentRepository returns a soft-delete-aware repository for
// event staff assignments. Unlike roles, event_staff_assignments has a
// deleted_at column, so this uses the ordinary audit-wrapped
// corerepository.NewRepository. TID is uuid.UUID — kept typed all the way
// through the service layer.
func NewStaffAssignmentRepository(
	log logger.Logger, db *sqlkit.DB, cacheOpts corerepository.CacheOptions,
) repository.Repository[EventStaffAssignment, uuid.UUID] {
	return corerepository.NewRepository[EventStaffAssignment, uuid.UUID](
		log, db, eventStaffAssignmentsTable, assignmentColumns, cacheOpts,
	)
}
