package tickettypetemplate

import (
	"context"
	"fmt"

	"github.com/biairmal/go-sdk/lib/logger"
	reposql "github.com/biairmal/go-sdk/lib/repository/sql"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../../mocks/tickets/tickettypetemplate/mock_workflow_step_repository.go -package=mocktickettypetemplate github.com/biairmal/guest-management-be/internal/features/tickets/tickettypetemplate WorkflowStepRepository

// WorkflowStepRepository reads and writes ticket_type_template_workflow_steps
// — which workflow step templates a ticket type template includes. Rows are
// write-once per template version (a new version gets new template rows), so
// there is no update or delete. Runs on the transaction in ctx when present.
type WorkflowStepRepository interface {
	// Insert links ticketTypeTemplateID to each of workflowStepTemplateIDs.
	Insert(ctx context.Context, ticketTypeTemplateID uuid.UUID, workflowStepTemplateIDs []uuid.UUID) error
	// WorkflowStepTemplateIDs returns the linked step template IDs for each
	// of ticketTypeTemplateIDs (absent from the map when none are linked).
	WorkflowStepTemplateIDs(
		ctx context.Context, ticketTypeTemplateIDs []uuid.UUID,
	) (map[uuid.UUID][]uuid.UUID, error)
}

// sqlWorkflowStepRepository implements WorkflowStepRepository with direct SQL.
type sqlWorkflowStepRepository struct {
	base *reposql.BaseRepository
	log  logger.Logger
}

// NewWorkflowStepRepository returns a WorkflowStepRepository backed by direct SQL.
func NewWorkflowStepRepository(log logger.Logger, db *sqlkit.DB) WorkflowStepRepository {
	return &sqlWorkflowStepRepository{
		base: reposql.NewBaseRepository(db, "ticket_type_template_workflow_steps"), log: log,
	}
}

// Insert implements WorkflowStepRepository.
func (r *sqlWorkflowStepRepository) Insert(
	ctx context.Context, ticketTypeTemplateID uuid.UUID, workflowStepTemplateIDs []uuid.UUID,
) error {
	const q = `INSERT INTO ticket_type_template_workflow_steps (ticket_type_template_id, workflow_step_template_id)
VALUES ($1, $2)`
	conn := r.base.GetConnection(ctx)
	for _, stepID := range workflowStepTemplateIDs {
		if r.log != nil {
			r.log.DebugfWithContext(ctx, "query: %s args: %v", q, []any{ticketTypeTemplateID, stepID})
		}
		if _, err := conn.ExecContext(ctx, q, ticketTypeTemplateID, stepID); err != nil {
			return fmt.Errorf("ticket_type_template_workflow_steps: insert: %w", err)
		}
	}
	return nil
}

// WorkflowStepTemplateIDs implements WorkflowStepRepository with one query
// for all ids.
func (r *sqlWorkflowStepRepository) WorkflowStepTemplateIDs(
	ctx context.Context, ticketTypeTemplateIDs []uuid.UUID,
) (map[uuid.UUID][]uuid.UUID, error) {
	result := make(map[uuid.UUID][]uuid.UUID, len(ticketTypeTemplateIDs))
	if len(ticketTypeTemplateIDs) == 0 {
		return result, nil
	}
	ids := make([]string, len(ticketTypeTemplateIDs))
	for i, id := range ticketTypeTemplateIDs {
		ids[i] = id.String()
	}

	const q = `SELECT ticket_type_template_id, workflow_step_template_id
FROM ticket_type_template_workflow_steps WHERE ticket_type_template_id = ANY($1::uuid[])`
	if r.log != nil {
		r.log.DebugfWithContext(ctx, "query: %s args: %v", q, []any{ids})
	}
	rows, err := r.base.GetReadConnection(ctx).QueryContext(ctx, q, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("ticket_type_template_workflow_steps: query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var templateID, stepID uuid.UUID
		if err := rows.Scan(&templateID, &stepID); err != nil {
			return nil, fmt.Errorf("ticket_type_template_workflow_steps: scan: %w", err)
		}
		result[templateID] = append(result[templateID], stepID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ticket_type_template_workflow_steps: read: %w", err)
	}
	return result, nil
}
