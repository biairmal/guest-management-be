package workflowsteptemplate

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

const workflowStepTemplatesTable = "workflow_step_templates"

// workflowStepTemplateColumns are the columns selected on reads (GetByID, List).
var workflowStepTemplateColumns = []string{
	"id", "category_id", "version", "name", "order_index", "allows_multiple",
	"created_at", "updated_at", "deleted_at",
}

// NewWorkflowStepTemplateRepository returns a soft-delete-aware repository
// for workflow step templates. TID is uuid.UUID — kept typed all the way
// through the service layer.
func NewWorkflowStepTemplateRepository(
	log logger.Logger, db *sqlkit.DB, cacheOpts corerepository.CacheOptions,
) repository.Repository[WorkflowStepTemplate, uuid.UUID] {
	return corerepository.NewRepository[WorkflowStepTemplate, uuid.UUID](
		log, db, workflowStepTemplatesTable, workflowStepTemplateColumns, cacheOpts,
	)
}
