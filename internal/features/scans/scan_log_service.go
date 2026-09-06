package scans

import (
	"context"
	"errors"
	"time"

	common "github.com/biairmal/go-sdk/lib/common/dto"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/google/uuid"

	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowstep"
	"github.com/biairmal/guest-management-be/internal/features/guests"
	"github.com/biairmal/guest-management-be/internal/features/tickets"
)

// ScanLogListConfig declares the allow-listed sort/filter fields for scan
// history queries, enforced here in the service via query.ValidateListParams
// and reused by ScanLogHandler.List for query.ParseListParams. event_id is
// always scoped from the URL, not a query filter; ticket_id/workflow_step_id
// are exact-match only (both UUID equality — no ";like" use case).
var ScanLogListConfig = query.ListParseConfig{
	AllowedSortFields:   []string{"id", "scanned_at"},
	AllowedFilterFields: []string{"ticket_id", "workflow_step_id"},
}

// ScanLogService defines the application-level operations for event-scoped
// scan logs. Unlike every CRUD feature in this codebase, it exposes only
// RecordScan and ListScans — scan_logs is a permanent, append-only audit log
// with no Update/Delete/GetByID endpoint.
type ScanLogService interface {
	// RecordScan validates and records one ticket scan against one workflow
	// step, scoped to eventID. See the step-by-step validation order in
	// docs/DEVELOPMENT_PLAN.md §B9's Technical Design.
	RecordScan(ctx context.Context, eventID uuid.UUID, in RecordScanInput) (*ScanLog, error)
	// ListScans returns the scan_logs rows for eventID with filter, sort, and
	// pagination from query.ListParams.
	ListScans(ctx context.Context, eventID uuid.UUID, params *query.ListParams) (*common.PageResponse[ScanLog], error)
}

// scanLogServiceImpl is the concrete implementation of ScanLogService.
type scanLogServiceImpl struct {
	repo                       repository.Repository[ScanLog, uuid.UUID]
	ticketRepo                 repository.Repository[guests.Ticket, uuid.UUID]
	workflowStepRepo           repository.ReadRepository[workflowstep.WorkflowStep, uuid.UUID]
	ticketTypeWorkflowStepRepo tickets.TicketTypeWorkflowStepRepository
	logger                     logger.Logger
}

// NewScanLogService returns a ScanLogService with the given dependencies.
// ticketRepo/workflowStepRepo/ticketTypeWorkflowStepRepo are cross-feature
// repository-type dependencies — never another feature's service, the same
// direction as tickets'/guests' cross-feature repo dependencies. ticketRepo
// is the full Repository (RecordScan both reads and, on first successful
// scan, updates a ticket's status); workflowStepRepo is a ReadRepository —
// this service never writes workflow steps.
func NewScanLogService(
	logger logger.Logger,
	repo repository.Repository[ScanLog, uuid.UUID],
	ticketRepo repository.Repository[guests.Ticket, uuid.UUID],
	workflowStepRepo repository.ReadRepository[workflowstep.WorkflowStep, uuid.UUID],
	ticketTypeWorkflowStepRepo tickets.TicketTypeWorkflowStepRepository,
) ScanLogService {
	return &scanLogServiceImpl{
		repo: repo, ticketRepo: ticketRepo, workflowStepRepo: workflowStepRepo,
		ticketTypeWorkflowStepRepo: ticketTypeWorkflowStepRepo, logger: logger,
	}
}

// RecordScan runs the full validation order before persisting a scan_logs
// row:
//  1. resolve qr_code to a Ticket scoped to eventID (404 if none found —
//     a ticket that exists but belongs to another event is indistinguishable
//     from an unknown QR code, same no-cross-event-leak convention as
//     tickets/guests);
//  2. reject an invalidated ticket (409);
//  3. load the workflow step and confirm it belongs to eventID (404/400);
//  4. confirm the ticket's ticket type is entitled to that step (400);
//  5. for a non-repeatable step, reject a second scan (409);
//  6. insert the scan_logs row;
//  7. on the ticket's first successful scan of any step, flip its status to
//     used (one-way, best-effort — see flipTicketToUsed).
func (s *scanLogServiceImpl) RecordScan(
	ctx context.Context, eventID uuid.UUID, in RecordScanInput,
) (*ScanLog, error) {
	ticket, err := s.resolveTicket(ctx, eventID, in.QRCode)
	if err != nil {
		return nil, err
	}
	if ticket.Status == guests.TicketStatusInvalidated {
		return nil, errorz.Conflict().WithMessage("ticket is invalidated")
	}

	step, err := s.loadStepForEvent(ctx, eventID, in.WorkflowStepID)
	if err != nil {
		return nil, err
	}
	if err := s.confirmTicketTypeEntitled(ctx, ticket.TicketTypeID, step.ID); err != nil {
		return nil, err
	}
	if !step.AllowsMultiple {
		if err := s.rejectIfAlreadyCompleted(ctx, ticket.ID, step.ID); err != nil {
			return nil, err
		}
	}

	entity := &ScanLog{
		ID: uuid.New(), EventID: eventID, TicketID: ticket.ID, WorkflowStepID: step.ID, ScannedAt: time.Now(),
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		s.logger.ErrorWithContext(ctx, "scan log create failed", logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to record scan")
	}

	if ticket.Status == guests.TicketStatusActive {
		s.flipTicketToUsed(ctx, ticket)
	}

	s.logger.InfoWithContext(ctx, "scan recorded",
		logger.F("id", entity.ID), logger.F("ticket_id", ticket.ID), logger.F("workflow_step_id", step.ID))
	return entity, nil
}

// resolveTicket looks up the one Ticket matching qrCode, scoped to eventID.
// A ticket that exists but belongs to a different event is treated the same
// as an unknown QR code (404) — no cross-event leak.
func (s *scanLogServiceImpl) resolveTicket(
	ctx context.Context, eventID uuid.UUID, qrCode string,
) (*guests.Ticket, error) {
	items, _, err := s.ticketRepo.List(ctx, &repository.ListOptions{
		Filter: repository.Filter{Conditions: []repository.FilterCondition{
			{Field: "qr_code", Operator: repository.FilterOperatorEq, Value: qrCode},
			{Field: "event_id", Operator: repository.FilterOperatorEq, Value: eventID},
		}},
		Pagination: repository.Pagination{Limit: 1},
		SkipCount:  true,
	})
	if err != nil {
		s.logger.ErrorWithContext(ctx, "scan ticket lookup failed", logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to look up ticket")
	}
	if len(items) == 0 {
		return nil, errorz.NotFound().WithMessage("ticket not found for this event")
	}
	return items[0], nil
}

// loadStepForEvent loads the workflow step by ID and confirms it belongs to
// eventID. Returns errorz.NotFound if the step doesn't exist at all, and
// errorz.BadRequest if it exists but belongs to a different event (same
// status tickets already uses for "a step id not on this event").
func (s *scanLogServiceImpl) loadStepForEvent(
	ctx context.Context, eventID, stepID uuid.UUID,
) (*workflowstep.WorkflowStep, error) {
	step, err := s.workflowStepRepo.GetByID(ctx, stepID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorz.NotFound().WithMessage("workflow step not found")
		}
		s.logger.ErrorWithContext(ctx, "scan workflow step lookup failed",
			logger.F("workflow_step_id", stepID), logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to look up workflow step")
	}
	if step.EventID != eventID {
		return nil, errorz.BadRequest().WithMessage("workflow step does not belong to this event")
	}
	return step, nil
}

// confirmTicketTypeEntitled returns errorz.BadRequest if stepID is not
// among ticketTypeID's entitled workflow steps (AC5).
func (s *scanLogServiceImpl) confirmTicketTypeEntitled(ctx context.Context, ticketTypeID, stepID uuid.UUID) error {
	stepIDs, err := s.ticketTypeWorkflowStepRepo.WorkflowStepIDsByTicketTypeID(ctx, ticketTypeID)
	if err != nil {
		s.logger.ErrorWithContext(ctx, "scan ticket type entitlement lookup failed",
			logger.F("ticket_type_id", ticketTypeID), logger.F("error", err))
		return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to look up ticket type entitlement")
	}
	for _, id := range stepIDs {
		if id == stepID {
			return nil
		}
	}
	return errorz.BadRequest().WithMessage("ticket type is not entitled to this workflow step")
}

// rejectIfAlreadyCompleted returns errorz.Conflict when ticketID already has
// a scan_logs row for stepID (AC2 — only called for a non-repeatable step).
func (s *scanLogServiceImpl) rejectIfAlreadyCompleted(ctx context.Context, ticketID, stepID uuid.UUID) error {
	count, err := s.repo.Count(ctx, repository.Filter{Conditions: []repository.FilterCondition{
		{Field: "ticket_id", Operator: repository.FilterOperatorEq, Value: ticketID},
		{Field: "workflow_step_id", Operator: repository.FilterOperatorEq, Value: stepID},
	}})
	if err != nil {
		s.logger.ErrorWithContext(ctx, "scan completion count failed", logger.F("error", err))
		return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to check prior scans")
	}
	if count > 0 {
		return errorz.Conflict().WithMessage("this workflow step has already been completed for this ticket")
	}
	return nil
}

// flipTicketToUsed updates ticket's status to used — a one-way,
// first-successful-scan-of-any-step transition run at most once per ticket
// (the caller only calls this when ticket.Status is still active). A failure
// here is logged but does not fail RecordScan: the scan_logs row is already
// persisted and is the record of what actually happened, per the Technical
// Design's non-goal of a rollback/transaction around this step.
func (s *scanLogServiceImpl) flipTicketToUsed(ctx context.Context, ticket *guests.Ticket) {
	ticket.Status = guests.TicketStatusUsed
	if err := s.ticketRepo.Update(ctx, ticket.ID, ticket); err != nil {
		s.logger.ErrorWithContext(ctx, "scan ticket status update failed",
			logger.F("ticket_id", ticket.ID), logger.F("error", err))
	}
}

// ListScans returns the scan_logs rows for eventID with filter, sort, and
// pagination from query.ListParams. The event_id scope is enforced
// server-side regardless of query filters.
func (s *scanLogServiceImpl) ListScans(
	ctx context.Context, eventID uuid.UUID, params *query.ListParams,
) (*common.PageResponse[ScanLog], error) {
	if err := query.ValidateListParams(params, ScanLogListConfig); err != nil {
		return nil, errorz.BadRequest().WithMessage(err.Error())
	}
	opts := query.ToListOptions(params)
	opts.Filter.Conditions = append(opts.Filter.Conditions, repository.FilterCondition{
		Field:    "event_id",
		Operator: repository.FilterOperatorEq,
		Value:    eventID,
	})

	items, total, err := s.repo.List(ctx, opts)
	if err != nil {
		s.logger.ErrorWithContext(ctx, "scan log list failed", logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to list scans")
	}
	return common.NewPageResponse(items, total, params.Page, params.Size), nil
}
