package tickets

import (
	"context"
	"fmt"

	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	"github.com/google/uuid"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../mocks/tickets/mock_ticket_type_workflow_step_repository.go -package=mocktickets github.com/biairmal/guest-management-be/internal/features/tickets TicketTypeWorkflowStepRepository

// TicketTypeWorkflowStepRepository resolves and replaces which workflow
// steps a ticket type applies to via ticket_type_workflow_steps.
// ticket_type_workflow_steps has a composite primary key
// (ticket_type_id, workflow_step_id) and no soft delete — same shape as
// roles' role_permissions join (see roles.RolePermissionRepository) — so
// this is a small, purpose-built interface rather than the generic
// repository.Repository[T, TID] pattern (a junction row has no ID of its own
// to key a generic CRUD interface on).
type TicketTypeWorkflowStepRepository interface {
	// SetWorkflowStepIDs replaces the full set of workflow steps
	// ticketTypeID applies to with workflowStepIDs (full-replace semantics,
	// not incremental add/remove). Not wrapped in a transaction — no
	// feature in this codebase uses one yet; a failure partway through is
	// recovered by resubmitting the same call.
	SetWorkflowStepIDs(ctx context.Context, ticketTypeID uuid.UUID, workflowStepIDs []uuid.UUID) error
	// WorkflowStepIDsByTicketTypeID returns the workflow step IDs
	// ticketTypeID currently applies to. Returns an empty slice (not an
	// error) when none are set.
	WorkflowStepIDsByTicketTypeID(ctx context.Context, ticketTypeID uuid.UUID) ([]uuid.UUID, error)
}

// sqlTicketTypeWorkflowStepRepository implements TicketTypeWorkflowStepRepository
// with direct SQL over ticket_type_workflow_steps, bypassing the generic
// repository/sql machinery (which is built around a single-entity table).
type sqlTicketTypeWorkflowStepRepository struct {
	db  *sqlkit.DB
	log logger.Logger
}

// NewTicketTypeWorkflowStepRepository returns a TicketTypeWorkflowStepRepository
// backed by direct SQL.
func NewTicketTypeWorkflowStepRepository(log logger.Logger, db *sqlkit.DB) TicketTypeWorkflowStepRepository {
	return &sqlTicketTypeWorkflowStepRepository{db: db, log: log}
}

// WorkflowStepIDsByTicketTypeID reads the workflow_step_ids currently
// assigned to ticketTypeID. Reads use the follower connection
// (db.Follower()) since this is a read-only lookup.
func (r *sqlTicketTypeWorkflowStepRepository) WorkflowStepIDsByTicketTypeID(
	ctx context.Context, ticketTypeID uuid.UUID,
) ([]uuid.UUID, error) {
	const q = `SELECT workflow_step_id FROM ticket_type_workflow_steps WHERE ticket_type_id = $1`

	if r.log != nil {
		r.log.DebugfWithContext(ctx, "query: %s args: %v", q, []any{ticketTypeID})
	}

	rows, err := r.db.Follower().QueryContext(ctx, q, ticketTypeID)
	if err != nil {
		return nil, fmt.Errorf("ticket_type_workflow_steps: query workflow step ids: %w", err)
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if scanErr := rows.Scan(&id); scanErr != nil {
			return nil, fmt.Errorf("ticket_type_workflow_steps: scan workflow step id: %w", scanErr)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ticket_type_workflow_steps: read workflow step ids: %w", err)
	}
	return ids, nil
}

// SetWorkflowStepIDs replaces ticketTypeID's entire set of workflow step
// associations: every existing row is deleted, then one row per id in
// workflowStepIDs is inserted. Writes use the leader connection
// (db.Leader()).
func (r *sqlTicketTypeWorkflowStepRepository) SetWorkflowStepIDs(
	ctx context.Context, ticketTypeID uuid.UUID, workflowStepIDs []uuid.UUID,
) error {
	const deleteQuery = `DELETE FROM ticket_type_workflow_steps WHERE ticket_type_id = $1`
	if r.log != nil {
		r.log.DebugfWithContext(ctx, "query: %s args: %v", deleteQuery, []any{ticketTypeID})
	}
	if _, err := r.db.Leader().ExecContext(ctx, deleteQuery, ticketTypeID); err != nil {
		return fmt.Errorf("ticket_type_workflow_steps: delete existing: %w", err)
	}

	const insertQuery = `INSERT INTO ticket_type_workflow_steps (ticket_type_id, workflow_step_id) VALUES ($1, $2)`
	for _, workflowStepID := range workflowStepIDs {
		if r.log != nil {
			r.log.DebugfWithContext(ctx, "query: %s args: %v", insertQuery, []any{ticketTypeID, workflowStepID})
		}
		if _, err := r.db.Leader().ExecContext(ctx, insertQuery, ticketTypeID, workflowStepID); err != nil {
			return fmt.Errorf("ticket_type_workflow_steps: insert: %w", err)
		}
	}
	return nil
}
