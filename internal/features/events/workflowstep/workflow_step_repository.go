package workflowstep

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

const workflowStepsTable = "workflow_steps"

// workflowStepColumns are the columns selected on reads (GetByID, List).
var workflowStepColumns = []string{
	"id", "event_id", "name", "order_index", "allows_multiple", "created_at", "updated_at", "deleted_at",
}

// NewWorkflowStepRepository returns a soft-delete-aware repository for
// workflow steps. TID is uuid.UUID — kept typed all the way through the
// service layer.
func NewWorkflowStepRepository(
	log logger.Logger, db *sqlkit.DB, cacheOpts corerepository.CacheOptions,
) repository.Repository[WorkflowStep, uuid.UUID] {
	return corerepository.NewRepository[WorkflowStep, uuid.UUID](
		log, db, workflowStepsTable, workflowStepColumns, cacheOpts,
	)
}
