package scans

import (
	"context"
	"errors"
	"testing"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	mockrepository "github.com/biairmal/go-sdk/mocks/repository"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowstep"
	"github.com/biairmal/guest-management-be/internal/features/guests"
	"github.com/biairmal/guest-management-be/internal/features/tickets"
	mocktickets "github.com/biairmal/guest-management-be/mocks/tickets"
)

func newTestService(
	repo repository.Repository[ScanLog, uuid.UUID],
	ticketRepo repository.Repository[guests.Ticket, uuid.UUID],
	workflowStepRepo repository.Repository[workflowstep.WorkflowStep, uuid.UUID],
	ticketTypeWorkflowStepRepo tickets.TicketTypeWorkflowStepRepository,
) ScanLogService {
	return NewScanLogService(logger.NewNoOp(), repo, ticketRepo, workflowStepRepo, ticketTypeWorkflowStepRepo)
}

func TestScanLogService_RecordScan(t *testing.T) {
	eventID, ticketID, stepID, ticketTypeID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	in := RecordScanInput{QRCode: "qr-code", WorkflowStepID: stepID}

	t.Run("ticket not found for event maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ticketRepo := mockrepository.NewMockRepository[guests.Ticket, uuid.UUID](ctrl)
		ticketRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*guests.Ticket{}, int64(0), nil)
		svc := newTestService(nil, ticketRepo, nil, nil)

		_, err := svc.RecordScan(context.Background(), eventID, in)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("ticket lookup error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ticketRepo := mockrepository.NewMockRepository[guests.Ticket, uuid.UUID](ctrl)
		ticketRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("boom"))
		svc := newTestService(nil, ticketRepo, nil, nil)

		_, err := svc.RecordScan(context.Background(), eventID, in)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("invalidated ticket maps to 409", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ticketRepo := mockrepository.NewMockRepository[guests.Ticket, uuid.UUID](ctrl)
		ticket := &guests.Ticket{ID: ticketID, EventID: eventID, Status: guests.TicketStatusInvalidated}
		ticketRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*guests.Ticket{ticket}, int64(1), nil)
		svc := newTestService(nil, ticketRepo, nil, nil)

		_, err := svc.RecordScan(context.Background(), eventID, in)
		assertErrorzCode(t, err, errorz.CodeConflict)
	})

	t.Run("workflow step not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ticketRepo := mockrepository.NewMockRepository[guests.Ticket, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		ticket := &guests.Ticket{ID: ticketID, EventID: eventID, Status: guests.TicketStatusActive}
		ticketRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*guests.Ticket{ticket}, int64(1), nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).Return(nil, repository.ErrNotFound)
		svc := newTestService(nil, ticketRepo, workflowStepRepo, nil)

		_, err := svc.RecordScan(context.Background(), eventID, in)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("workflow step lookup error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ticketRepo := mockrepository.NewMockRepository[guests.Ticket, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		ticket := &guests.Ticket{ID: ticketID, EventID: eventID, Status: guests.TicketStatusActive}
		ticketRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*guests.Ticket{ticket}, int64(1), nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).Return(nil, errors.New("boom"))
		svc := newTestService(nil, ticketRepo, workflowStepRepo, nil)

		_, err := svc.RecordScan(context.Background(), eventID, in)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("workflow step from a different event maps to 400", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ticketRepo := mockrepository.NewMockRepository[guests.Ticket, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		ticket := &guests.Ticket{ID: ticketID, EventID: eventID, Status: guests.TicketStatusActive}
		ticketRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*guests.Ticket{ticket}, int64(1), nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).
			Return(&workflowstep.WorkflowStep{ID: stepID, EventID: uuid.New()}, nil)
		svc := newTestService(nil, ticketRepo, workflowStepRepo, nil)

		_, err := svc.RecordScan(context.Background(), eventID, in)
		assertErrorzCode(t, err, errorz.CodeBadRequest)
	})

	t.Run("ticket type not entitled to step maps to 400", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ticketRepo := mockrepository.NewMockRepository[guests.Ticket, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		ticket := &guests.Ticket{
			ID: ticketID, EventID: eventID, TicketTypeID: ticketTypeID, Status: guests.TicketStatusActive,
		}
		ticketRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*guests.Ticket{ticket}, int64(1), nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).
			Return(&workflowstep.WorkflowStep{ID: stepID, EventID: eventID}, nil)
		junctionRepo.EXPECT().WorkflowStepIDsByTicketTypeID(gomock.Any(), ticketTypeID).
			Return([]uuid.UUID{uuid.New()}, nil)
		svc := newTestService(nil, ticketRepo, workflowStepRepo, junctionRepo)

		_, err := svc.RecordScan(context.Background(), eventID, in)
		assertErrorzCode(t, err, errorz.CodeBadRequest)
	})

	t.Run("entitlement lookup error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ticketRepo := mockrepository.NewMockRepository[guests.Ticket, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		ticket := &guests.Ticket{
			ID: ticketID, EventID: eventID, TicketTypeID: ticketTypeID, Status: guests.TicketStatusActive,
		}
		ticketRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*guests.Ticket{ticket}, int64(1), nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).
			Return(&workflowstep.WorkflowStep{ID: stepID, EventID: eventID}, nil)
		junctionRepo.EXPECT().WorkflowStepIDsByTicketTypeID(gomock.Any(), ticketTypeID).
			Return(nil, errors.New("boom"))
		svc := newTestService(nil, ticketRepo, workflowStepRepo, junctionRepo)

		_, err := svc.RecordScan(context.Background(), eventID, in)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("non-repeatable step already completed maps to 409", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[ScanLog, uuid.UUID](ctrl)
		ticketRepo := mockrepository.NewMockRepository[guests.Ticket, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		ticket := &guests.Ticket{
			ID: ticketID, EventID: eventID, TicketTypeID: ticketTypeID, Status: guests.TicketStatusActive,
		}
		ticketRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*guests.Ticket{ticket}, int64(1), nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).
			Return(&workflowstep.WorkflowStep{ID: stepID, EventID: eventID, AllowsMultiple: false}, nil)
		junctionRepo.EXPECT().WorkflowStepIDsByTicketTypeID(gomock.Any(), ticketTypeID).
			Return([]uuid.UUID{stepID}, nil)
		repo.EXPECT().Count(gomock.Any(), gomock.Any()).Return(int64(1), nil)
		svc := newTestService(repo, ticketRepo, workflowStepRepo, junctionRepo)

		_, err := svc.RecordScan(context.Background(), eventID, in)
		assertErrorzCode(t, err, errorz.CodeConflict)
	})

	t.Run("completion count error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[ScanLog, uuid.UUID](ctrl)
		ticketRepo := mockrepository.NewMockRepository[guests.Ticket, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		ticket := &guests.Ticket{
			ID: ticketID, EventID: eventID, TicketTypeID: ticketTypeID, Status: guests.TicketStatusActive,
		}
		ticketRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*guests.Ticket{ticket}, int64(1), nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).
			Return(&workflowstep.WorkflowStep{ID: stepID, EventID: eventID, AllowsMultiple: false}, nil)
		junctionRepo.EXPECT().WorkflowStepIDsByTicketTypeID(gomock.Any(), ticketTypeID).
			Return([]uuid.UUID{stepID}, nil)
		repo.EXPECT().Count(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("boom"))
		svc := newTestService(repo, ticketRepo, workflowStepRepo, junctionRepo)

		_, err := svc.RecordScan(context.Background(), eventID, in)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("scan log create error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[ScanLog, uuid.UUID](ctrl)
		ticketRepo := mockrepository.NewMockRepository[guests.Ticket, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		ticket := &guests.Ticket{
			ID: ticketID, EventID: eventID, TicketTypeID: ticketTypeID, Status: guests.TicketStatusActive,
		}
		ticketRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*guests.Ticket{ticket}, int64(1), nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).
			Return(&workflowstep.WorkflowStep{ID: stepID, EventID: eventID, AllowsMultiple: true}, nil)
		junctionRepo.EXPECT().WorkflowStepIDsByTicketTypeID(gomock.Any(), ticketTypeID).
			Return([]uuid.UUID{stepID}, nil)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("boom"))
		svc := newTestService(repo, ticketRepo, workflowStepRepo, junctionRepo)

		_, err := svc.RecordScan(context.Background(), eventID, in)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("happy path on active ticket flips status to used", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[ScanLog, uuid.UUID](ctrl)
		ticketRepo := mockrepository.NewMockRepository[guests.Ticket, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		ticket := &guests.Ticket{
			ID: ticketID, EventID: eventID, TicketTypeID: ticketTypeID, Status: guests.TicketStatusActive,
		}
		ticketRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*guests.Ticket{ticket}, int64(1), nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).
			Return(&workflowstep.WorkflowStep{ID: stepID, EventID: eventID, AllowsMultiple: false}, nil)
		junctionRepo.EXPECT().WorkflowStepIDsByTicketTypeID(gomock.Any(), ticketTypeID).
			Return([]uuid.UUID{stepID}, nil)
		repo.EXPECT().Count(gomock.Any(), gomock.Any()).Return(int64(0), nil)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *ScanLog) error {
			if e.TicketID != ticketID || e.WorkflowStepID != stepID || e.EventID != eventID {
				t.Errorf("unexpected scan log entity: %+v", e)
			}
			return nil
		})
		ticketRepo.EXPECT().Update(gomock.Any(), ticketID, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uuid.UUID, tk *guests.Ticket) error {
				if tk.Status != guests.TicketStatusUsed {
					t.Errorf("Status = %q, want %q", tk.Status, guests.TicketStatusUsed)
				}
				return nil
			})
		svc := newTestService(repo, ticketRepo, workflowStepRepo, junctionRepo)

		got, err := svc.RecordScan(context.Background(), eventID, in)
		if err != nil {
			t.Fatalf("RecordScan() error = %v", err)
		}
		if got.TicketID != ticketID || got.WorkflowStepID != stepID {
			t.Errorf("got = %+v", got)
		}
	})

	t.Run("repeatable step on an already-used ticket succeeds without a status update", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[ScanLog, uuid.UUID](ctrl)
		ticketRepo := mockrepository.NewMockRepository[guests.Ticket, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		ticket := &guests.Ticket{ID: ticketID, EventID: eventID, TicketTypeID: ticketTypeID, Status: guests.TicketStatusUsed}
		ticketRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*guests.Ticket{ticket}, int64(1), nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).
			Return(&workflowstep.WorkflowStep{ID: stepID, EventID: eventID, AllowsMultiple: true}, nil)
		junctionRepo.EXPECT().WorkflowStepIDsByTicketTypeID(gomock.Any(), ticketTypeID).
			Return([]uuid.UUID{stepID}, nil)
		// AllowsMultiple == true skips the Count check entirely (AC3): no
		// repo.Count expectation set here.
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		// No ticketRepo.Update expectation: status is already used, so
		// flipTicketToUsed must not be called (gomock fails on any
		// unexpected call).
		svc := newTestService(repo, ticketRepo, workflowStepRepo, junctionRepo)

		if _, err := svc.RecordScan(context.Background(), eventID, in); err != nil {
			t.Fatalf("RecordScan() error = %v", err)
		}
	})

	t.Run("ticket status update failure is best-effort and does not fail the scan", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[ScanLog, uuid.UUID](ctrl)
		ticketRepo := mockrepository.NewMockRepository[guests.Ticket, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		ticket := &guests.Ticket{
			ID: ticketID, EventID: eventID, TicketTypeID: ticketTypeID, Status: guests.TicketStatusActive,
		}
		ticketRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*guests.Ticket{ticket}, int64(1), nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).
			Return(&workflowstep.WorkflowStep{ID: stepID, EventID: eventID, AllowsMultiple: true}, nil)
		junctionRepo.EXPECT().WorkflowStepIDsByTicketTypeID(gomock.Any(), ticketTypeID).
			Return([]uuid.UUID{stepID}, nil)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		ticketRepo.EXPECT().Update(gomock.Any(), ticketID, gomock.Any()).Return(errors.New("boom"))
		svc := newTestService(repo, ticketRepo, workflowStepRepo, junctionRepo)

		if _, err := svc.RecordScan(context.Background(), eventID, in); err != nil {
			t.Fatalf("RecordScan() error = %v, want scan to succeed despite status update failure", err)
		}
	})
}

func TestScanLogService_ListScans(t *testing.T) {
	eventID := uuid.New()

	t.Run("disallowed filter field maps to 400", func(t *testing.T) {
		svc := newTestService(nil, nil, nil, nil)
		params := &query.ListParams{Filters: map[string]query.FilterValue{"bogus": {Value: "x"}}}

		_, err := svc.ListScans(context.Background(), eventID, params)
		assertErrorzCode(t, err, errorz.CodeBadRequest)
	})

	t.Run("repo error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[ScanLog, uuid.UUID](ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("boom"))
		svc := newTestService(repo, nil, nil, nil)

		_, err := svc.ListScans(context.Background(), eventID, &query.ListParams{})
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("always injects event_id filter regardless of query params", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[ScanLog, uuid.UUID](ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, opts *repository.ListOptions) ([]*ScanLog, int64, error) {
				found := false
				for _, c := range opts.Filter.Conditions {
					if c.Field == "event_id" && c.Value == eventID {
						found = true
					}
				}
				if !found {
					t.Error("expected event_id filter condition to be present and match eventID")
				}
				return []*ScanLog{{ID: uuid.New(), EventID: eventID}}, int64(1), nil
			})
		svc := newTestService(repo, nil, nil, nil)

		got, err := svc.ListScans(context.Background(), eventID, &query.ListParams{})
		if err != nil {
			t.Fatalf("ListScans() error = %v", err)
		}
		if got.Total != 1 {
			t.Errorf("Total = %d, want 1", got.Total)
		}
	})
}
