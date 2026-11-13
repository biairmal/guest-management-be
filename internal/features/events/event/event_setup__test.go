package event

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	mockrepository "github.com/biairmal/go-sdk/mocks/repository"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/biairmal/guest-management-be/internal/features/events/workflowstep"
	mockeventtenant "github.com/biairmal/guest-management-be/mocks/events/eventtenant"
	mockworkflowstep "github.com/biairmal/guest-management-be/mocks/events/workflowstep"
)

// stubTicketTypeStore is a real, minimal TicketTypeStore. Hand-written for the
// same reason as stubTicketTypeSeeder: a generated mock would live in
// mocks/events/event, which imports this package, and this is this package's own
// same-package test — an import cycle (see ticket_type_store.go).
type stubTicketTypeStore struct {
	list       []TicketTypeStoreView
	listErr    error
	written    []TicketTypeDraft
	writeCalls int
	writeErr   error
}

func (s *stubTicketTypeStore) ListForEvent(_ context.Context, _ uuid.UUID) ([]TicketTypeStoreView, error) {
	return s.list, s.listErr
}

func (s *stubTicketTypeStore) ReplaceForEvent(
	_ context.Context, _ uuid.UUID, in []TicketTypeDraft,
) ([]TicketTypeStoreView, error) {
	s.writeCalls++
	s.written = in
	if s.writeErr != nil {
		return nil, s.writeErr
	}
	views := make([]TicketTypeStoreView, len(in))
	for i, d := range in {
		id := uuid.New()
		if d.ID != nil {
			id = *d.ID
		}
		views[i] = TicketTypeStoreView{ID: id, Name: d.Name, Rules: d.Rules, WorkflowStepIDs: d.WorkflowStepIDs}
	}
	return views, nil
}

// stubGuestStats is a real, minimal GuestStats — hand-written for the same
// reason as stubTicketTypeStore.
type stubGuestStats struct {
	byEvent            map[uuid.UUID]GuestCounts
	eventsErr          error
	countByEventsCalls int
	byTicketType       map[uuid.UUID]int
	ticketTypeErr      error
}

func (s *stubGuestStats) CountByEvents(
	_ context.Context, _ []uuid.UUID,
) (map[uuid.UUID]GuestCounts, error) {
	s.countByEventsCalls++
	return s.byEvent, s.eventsErr
}

func (s *stubGuestStats) CountByTicketType(_ context.Context, _ uuid.UUID) (map[uuid.UUID]int, error) {
	return s.byTicketType, s.ticketTypeErr
}

// lockedTenant returns an EventTenantRepository mock expecting one FOR UPDATE
// read that reports tenantID/found.
func lockedTenant(
	ctrl *gomock.Controller, tenantID uuid.UUID, found bool, err error,
) *mockeventtenant.MockEventTenantRepository {
	m := mockeventtenant.NewMockEventTenantRepository(ctrl)
	m.EXPECT().TenantIDForEventUpdate(gomock.Any(), gomock.Any()).Return(tenantID, found, err)
	return m
}

// eventStepsReturning returns a workflow step repository mock whose single List
// call returns steps.
func eventStepsReturning(
	ctrl *gomock.Controller, steps []*workflowstep.WorkflowStep, err error,
) *mockrepository.MockRepository[workflowstep.WorkflowStep, uuid.UUID] {
	m := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
	m.EXPECT().List(gomock.Any(), gomock.Any()).Return(steps, int64(len(steps)), err)
	return m
}

// TestValidateSetup covers AC5: per-field detail keyed by JSON path, the same
// rules as a category's template set plus the id-ownership check.
func TestValidateSetup(t *testing.T) {
	liveStep, liveType := uuid.New(), uuid.New()
	strayID := uuid.New()
	steps := []*workflowstep.WorkflowStep{{ID: liveStep, Name: "Check-in"}}
	ticketTypes := []TicketTypeStoreView{{ID: liveType, Name: "VIP"}}

	tests := []struct {
		name      string
		in        SetupInput
		wantField string
	}{
		{name: "empty payload is valid — it clears the setup"},
		{
			name: "valid payload",
			in: SetupInput{
				WorkflowSteps: []WorkflowStepInput{{ID: &liveStep, Name: "Check-in"}, {Name: "Dinner"}},
				TicketTypes:   []TicketTypeInput{{ID: &liveType, Name: "VIP", Steps: []int{0, 1}}},
			},
		},
		{
			name: "step id from another event is rejected by path",
			in: SetupInput{
				WorkflowSteps: []WorkflowStepInput{{Name: "A"}, {ID: &strayID, Name: "B"}},
			},
			wantField: "workflow_steps[1].id",
		},
		{
			name: "ticket type id from another event is rejected by path",
			in: SetupInput{
				TicketTypes: []TicketTypeInput{{ID: &strayID, Name: "VIP"}},
			},
			wantField: "ticket_types[0].id",
		},
		{
			name: "duplicate ticket type name, case-insensitive, is rejected",
			in: SetupInput{
				TicketTypes: []TicketTypeInput{{Name: "VIP"}, {Name: " vip "}},
			},
			wantField: "ticket_types[1].name",
		},
		{
			name: "out-of-range step reference is rejected",
			in: SetupInput{
				WorkflowSteps: []WorkflowStepInput{{Name: "A"}},
				TicketTypes:   []TicketTypeInput{{Name: "VIP", Steps: []int{1}}},
			},
			wantField: "ticket_types[0].steps[0]",
		},
		{
			name: "negative step reference is rejected",
			in: SetupInput{
				WorkflowSteps: []WorkflowStepInput{{Name: "A"}},
				TicketTypes:   []TicketTypeInput{{Name: "VIP", Steps: []int{-1}}},
			},
			wantField: "ticket_types[0].steps[0]",
		},
		{
			name: "repeated step reference is rejected",
			in: SetupInput{
				WorkflowSteps: []WorkflowStepInput{{Name: "A"}},
				TicketTypes:   []TicketTypeInput{{Name: "VIP", Steps: []int{0, 0}}},
			},
			wantField: "ticket_types[0].steps[1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSetup(tt.in, steps, ticketTypes)
			if tt.wantField == "" {
				assertErrorzCode(t, err, "")
				return
			}
			assertErrorzCode(t, err, errorz.CodeBadRequest)

			var e *errorz.Error
			if !errors.As(err, &e) {
				t.Fatalf("expected *errorz.Error, got %T", err)
			}
			fields, ok := e.Meta["fields"].(map[string]string)
			if !ok {
				t.Fatalf("meta[fields] = %#v, want map[string]string", e.Meta["fields"])
			}
			if _, ok := fields[tt.wantField]; !ok {
				t.Errorf("fields = %v, want a key %q", fields, tt.wantField)
			}
		})
	}
}

// TestEventService_replaceSetup exercises ReplaceSetup's transaction body
// directly against mocked collaborators, without a live transaction — the same
// reasoning as TestEventService_createEvent.
func TestEventService_replaceSetup(t *testing.T) {
	eventID := uuid.New()

	t.Run("no tenant claim maps to 401", func(t *testing.T) {
		svc := &eventServiceImpl{logger: logger.NewNoOp()}

		_, err := svc.replaceSetup(context.Background(), eventID, SetupInput{})
		assertErrorzCode(t, err, errorz.CodeUnauthorized)
	})

	t.Run("missing event maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc := &eventServiceImpl{tenantRepo: lockedTenant(ctrl, uuid.Nil, false, nil), logger: logger.NewNoOp()}

		_, err := svc.replaceSetup(tenantCtx(), eventID, SetupInput{})
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("another tenant's event maps to 404, not 403", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc := &eventServiceImpl{tenantRepo: lockedTenant(ctrl, otherTenantID, true, nil), logger: logger.NewNoOp()}

		_, err := svc.replaceSetup(tenantCtx(), eventID, SetupInput{})
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("lock failure maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc := &eventServiceImpl{
			tenantRepo: lockedTenant(ctrl, uuid.Nil, false, errors.New("boom")), logger: logger.NewNoOp(),
		}

		_, err := svc.replaceSetup(tenantCtx(), eventID, SetupInput{})
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("removing a ticket type guests hold is rejected and nothing is written", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		heldID := uuid.New()
		store := &stubTicketTypeStore{list: []TicketTypeStoreView{{ID: heldID, Name: "VIP"}}}
		steps := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		steps.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		svc := &eventServiceImpl{
			tenantRepo: lockedTenant(ctrl, callerTenantID, true, nil), workflowStepRepo: steps,
			ticketTypes: store, guestStats: &stubGuestStats{byTicketType: map[uuid.UUID]int{heldID: 3}},
			logger: logger.NewNoOp(),
		}

		// VIP is left out of the payload, so the save would delete it.
		_, err := svc.replaceSetup(tenantCtx(), eventID, SetupInput{})
		assertErrorzCode(t, err, errorz.CodeBadRequest)

		var e *errorz.Error
		if !errors.As(err, &e) {
			t.Fatalf("expected *errorz.Error, got %T", err)
		}
		fields, _ := e.Meta["fields"].(map[string]string)
		if got := fields["ticket_types"]; got != "VIP is held by 3 guests; reassign them first" {
			t.Errorf("fields[ticket_types] = %q, want it to name the type and the count", got)
		}
		if store.writeCalls != 0 {
			t.Error("ReplaceForEvent must not run when a held ticket type would be removed")
		}
	})

	t.Run("removing an unheld ticket type is allowed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := &stubTicketTypeStore{list: []TicketTypeStoreView{{ID: uuid.New(), Name: "VIP"}}}
		steps := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		steps.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		stepSvc := mockworkflowstep.NewMockService(ctrl)
		stepSvc.EXPECT().Sync(gomock.Any(), eventID, gomock.Any()).Return(nil, nil)
		svc := &eventServiceImpl{
			tenantRepo: lockedTenant(ctrl, callerTenantID, true, nil), workflowStepRepo: steps,
			steps: stepSvc, ticketTypes: store, guestStats: &stubGuestStats{}, logger: logger.NewNoOp(),
		}

		got, err := svc.replaceSetup(tenantCtx(), eventID, SetupInput{})
		if err != nil {
			t.Fatalf("replaceSetup() error = %v", err)
		}
		if len(got.WorkflowSteps) != 0 || len(got.TicketTypes) != 0 {
			t.Errorf("setup = %+v, want empty", got)
		}
	})

	t.Run("a step sync failure aborts before any ticket type is written", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := &stubTicketTypeStore{}
		stepSvc := mockworkflowstep.NewMockService(ctrl)
		stepSvc.EXPECT().Sync(gomock.Any(), eventID, gomock.Any()).
			Return(nil, errorz.Conflict().WithMessage("order index clash"))
		svc := &eventServiceImpl{
			tenantRepo:       lockedTenant(ctrl, callerTenantID, true, nil),
			workflowStepRepo: eventStepsReturning(ctrl, nil, nil), steps: stepSvc,
			ticketTypes: store, guestStats: &stubGuestStats{}, logger: logger.NewNoOp(),
		}

		_, err := svc.replaceSetup(tenantCtx(), eventID, SetupInput{
			WorkflowSteps: []WorkflowStepInput{{Name: "Check-in"}},
		})
		assertErrorzCode(t, err, errorz.CodeConflict)
		if store.writeCalls != 0 {
			t.Error("ReplaceForEvent must not run after the step sync fails (the transaction aborts)")
		}
	})

	t.Run("steps keep their ids and array order becomes order_index", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		keptStep, keptType := uuid.New(), uuid.New()
		newStepID := uuid.New()
		store := &stubTicketTypeStore{list: []TicketTypeStoreView{{ID: keptType, Name: "VIP"}}}
		stepSvc := mockworkflowstep.NewMockService(ctrl)
		stepSvc.EXPECT().Sync(gomock.Any(), eventID, gomock.Any()).
			DoAndReturn(func(
				_ context.Context, _ uuid.UUID, in []workflowstep.SyncWorkflowStepInput,
			) ([]*workflowstep.WorkflowStep, error) {
				if len(in) != 2 {
					t.Fatalf("Sync got %d steps, want 2", len(in))
				}
				if in[0].ID == nil || *in[0].ID != keptStep {
					t.Errorf("Sync step 0 id = %v, want the kept id %v", in[0].ID, keptStep)
				}
				if in[0].OrderIndex != 0 || in[1].OrderIndex != 1 {
					t.Errorf("order indexes = (%d, %d), want (0, 1) from array position",
						in[0].OrderIndex, in[1].OrderIndex)
				}
				return []*workflowstep.WorkflowStep{
					{ID: keptStep, Name: in[0].Name, OrderIndex: 0},
					{ID: newStepID, Name: in[1].Name, OrderIndex: 1},
				}, nil
			})
		svc := &eventServiceImpl{
			tenantRepo:       lockedTenant(ctrl, callerTenantID, true, nil),
			workflowStepRepo: eventStepsReturning(ctrl, []*workflowstep.WorkflowStep{{ID: keptStep}}, nil),
			steps:            stepSvc, ticketTypes: store, guestStats: &stubGuestStats{}, logger: logger.NewNoOp(),
		}

		got, err := svc.replaceSetup(tenantCtx(), eventID, SetupInput{
			WorkflowSteps: []WorkflowStepInput{{ID: &keptStep, Name: "Check-in"}, {Name: "Dinner"}},
			TicketTypes: []TicketTypeInput{
				{ID: &keptType, Name: "VIP", Rules: json.RawMessage(`{"max":2}`), Steps: []int{1, 0}},
			},
		})
		if err != nil {
			t.Fatalf("replaceSetup() error = %v", err)
		}

		// Step indexes resolve to the ids Sync actually saved, in payload order.
		if len(store.written) != 1 ||
			len(store.written[0].WorkflowStepIDs) != 2 ||
			store.written[0].WorkflowStepIDs[0] != newStepID ||
			store.written[0].WorkflowStepIDs[1] != keptStep {
			t.Fatalf("draft step ids = %v, want [%v %v]",
				store.written[0].WorkflowStepIDs, newStepID, keptStep)
		}
		if store.written[0].ID == nil || *store.written[0].ID != keptType {
			t.Errorf("draft ticket type id = %v, want the kept id %v", store.written[0].ID, keptType)
		}
		// ...and come back as ascending indexes into the response's step array.
		if len(got.TicketTypes) != 1 || len(got.TicketTypes[0].Steps) != 2 ||
			got.TicketTypes[0].Steps[0] != 0 || got.TicketTypes[0].Steps[1] != 1 {
			t.Errorf("response steps = %v, want [0 1]", got.TicketTypes[0].Steps)
		}
		if len(got.WorkflowSteps) != 2 || got.WorkflowSteps[0].ID != keptStep {
			t.Errorf("response steps = %+v, want the kept step first", got.WorkflowSteps)
		}
	})
}

// TestEventService_GetSetup covers the read shape: another tenant's event is
// 404, and a ticket type's step ids come back as ascending indexes.
func TestEventService_GetSetup(t *testing.T) {
	eventID := uuid.New()
	stepA, stepB := uuid.New(), uuid.New()

	t.Run("another tenant's event maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Event, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), eventID).Return(&Event{ID: eventID, TenantID: otherTenantID}, nil)
		svc := &eventServiceImpl{repo: repo, logger: logger.NewNoOp()}

		_, err := svc.GetSetup(tenantCtx(), eventID)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("step lookup failure maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Event, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), eventID).Return(&Event{ID: eventID, TenantID: callerTenantID}, nil)
		svc := &eventServiceImpl{
			repo: repo, workflowStepRepo: eventStepsReturning(ctrl, nil, errors.New("boom")),
			logger: logger.NewNoOp(),
		}

		_, err := svc.GetSetup(tenantCtx(), eventID)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("reports each ticket type's steps as ascending indexes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Event, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), eventID).Return(&Event{ID: eventID, TenantID: callerTenantID}, nil)
		steps := []*workflowstep.WorkflowStep{
			{ID: stepA, Name: "Check-in", OrderIndex: 0},
			{ID: stepB, Name: "Dinner", OrderIndex: 1, AllowsMultiple: true},
		}
		store := &stubTicketTypeStore{list: []TicketTypeStoreView{
			{ID: uuid.New(), Name: "VIP", WorkflowStepIDs: []uuid.UUID{stepB, stepA}},
			{ID: uuid.New(), Name: "Crew"},
		}}
		svc := &eventServiceImpl{
			repo: repo, workflowStepRepo: eventStepsReturning(ctrl, steps, nil),
			ticketTypes: store, logger: logger.NewNoOp(),
		}

		got, err := svc.GetSetup(tenantCtx(), eventID)
		if err != nil {
			t.Fatalf("GetSetup() error = %v", err)
		}
		if len(got.WorkflowSteps) != 2 || !got.WorkflowSteps[1].AllowsMultiple {
			t.Errorf("workflow steps = %+v, want 2 with the second allowing multiple", got.WorkflowSteps)
		}
		if len(got.TicketTypes[0].Steps) != 2 ||
			got.TicketTypes[0].Steps[0] != 0 || got.TicketTypes[0].Steps[1] != 1 {
			t.Errorf("VIP steps = %v, want [0 1] (sorted, not junction order)", got.TicketTypes[0].Steps)
		}
		if len(got.TicketTypes[1].Steps) != 0 {
			t.Errorf("Crew steps = %v, want empty", got.TicketTypes[1].Steps)
		}
	})
}

// TestEventService_eventSteps asserts the setup read is scoped to the event and
// ordered by step order — the order the response's step indexes refer to.
func TestEventService_eventSteps(t *testing.T) {
	ctrl := gomock.NewController(t)
	eventID := uuid.New()
	repo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, opts *repository.ListOptions,
		) ([]*workflowstep.WorkflowStep, int64, error) {
			scoped := false
			for _, c := range opts.Filter.Conditions {
				if c.Field == "event_id" && c.Value == eventID {
					scoped = true
				}
			}
			if !scoped {
				t.Error("expected an event_id filter condition")
			}
			if len(opts.Sorts) != 1 || opts.Sorts[0].Field != "order_index" {
				t.Errorf("sorts = %+v, want order_index ASC", opts.Sorts)
			}
			return nil, 0, nil
		})
	svc := &eventServiceImpl{workflowStepRepo: repo, logger: logger.NewNoOp()}

	if _, err := svc.eventSteps(context.Background(), eventID); err != nil {
		t.Fatalf("eventSteps() error = %v", err)
	}
}
