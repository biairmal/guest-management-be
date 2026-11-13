package tickettype

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

	"github.com/biairmal/guest-management-be/internal/features/events/event"
	mocktickettype "github.com/biairmal/guest-management-be/mocks/tickets/tickettype"
)

// listReturning stubs the store's single live-ticket-type read.
func listReturning(
	ctrl *gomock.Controller, types []*TicketType, err error,
) *mockrepository.MockRepository[TicketType, uuid.UUID] {
	m := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
	m.EXPECT().List(gomock.Any(), gomock.Any()).Return(types, int64(len(types)), err)
	return m
}

func TestSetupStore_ListForEvent(t *testing.T) {
	eventID := uuid.New()

	t.Run("list failure maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := NewSetupStore(logger.NewNoOp(), listReturning(ctrl, nil, errors.New("boom")), nil)

		_, err := store.ListForEvent(context.Background(), eventID)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("junction failure maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		id := uuid.New()
		junction := mocktickettype.NewMockTicketTypeWorkflowStepRepository(ctrl)
		junction.EXPECT().WorkflowStepIDsByTicketTypeID(gomock.Any(), id).Return(nil, errors.New("boom"))
		store := NewSetupStore(logger.NewNoOp(),
			listReturning(ctrl, []*TicketType{{ID: id, EventID: eventID}}, nil), junction)

		_, err := store.ListForEvent(context.Background(), eventID)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("reports each type's step ids", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		id, stepID := uuid.New(), uuid.New()
		junction := mocktickettype.NewMockTicketTypeWorkflowStepRepository(ctrl)
		junction.EXPECT().WorkflowStepIDsByTicketTypeID(gomock.Any(), id).Return([]uuid.UUID{stepID}, nil)
		store := NewSetupStore(logger.NewNoOp(), listReturning(ctrl,
			[]*TicketType{{ID: id, EventID: eventID, Name: "VIP", Rules: emptyJSONObject}}, nil), junction)

		got, err := store.ListForEvent(context.Background(), eventID)
		if err != nil {
			t.Fatalf("ListForEvent() error = %v", err)
		}
		if len(got) != 1 || got[0].ID != id || len(got[0].WorkflowStepIDs) != 1 || got[0].WorkflowStepIDs[0] != stepID {
			t.Errorf("views = %+v, want one VIP carrying step %v", got, stepID)
		}
	})
}

// TestSetupStore_ReplaceForEvent covers AC3: an id keeps its row (so the tickets
// issued against it stay valid), no id creates one, and a live type left out of
// the payload is soft-deleted.
func TestSetupStore_ReplaceForEvent(t *testing.T) {
	eventID := uuid.New()

	t.Run("reconciles: kept updated in place, missing deleted, new created", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		keptID, droppedID := uuid.New(), uuid.New()
		stepID := uuid.New()
		existing := []*TicketType{
			{ID: keptID, EventID: eventID, Name: "VIP", Rules: emptyJSONObject},
			{ID: droppedID, EventID: eventID, Name: "Crew", Rules: emptyJSONObject},
		}
		repo := listReturning(ctrl, existing, nil)
		junction := mocktickettype.NewMockTicketTypeWorkflowStepRepository(ctrl)

		var created *TicketType
		gomock.InOrder(
			// The delete runs before any write, so a freed name is reusable.
			repo.EXPECT().Delete(gomock.Any(), droppedID).Return(nil),
			repo.EXPECT().Update(gomock.Any(), keptID, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ uuid.UUID, e *TicketType) error {
					if e.Name != "VIP Plus" {
						t.Errorf("updated name = %q, want %q", e.Name, "VIP Plus")
					}
					return nil
				}),
			repo.EXPECT().Create(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, e *TicketType) error { created = e; return nil }),
		)
		junction.EXPECT().SetWorkflowStepIDs(gomock.Any(), keptID, []uuid.UUID{stepID}).Return(nil)
		junction.EXPECT().SetWorkflowStepIDs(gomock.Any(), gomock.Any(), nil).Return(nil)

		store := NewSetupStore(logger.NewNoOp(), repo, junction)
		got, err := store.ReplaceForEvent(context.Background(), eventID, []event.TicketTypeDraft{
			{ID: &keptID, Name: "VIP Plus", Rules: json.RawMessage(`{"max":3}`), WorkflowStepIDs: []uuid.UUID{stepID}},
			{Name: "Staff"},
		})
		if err != nil {
			t.Fatalf("ReplaceForEvent() error = %v", err)
		}
		if len(got) != 2 || got[0].ID != keptID {
			t.Fatalf("views = %+v, want the kept id first", got)
		}
		if created == nil || created.EventID != eventID || created.Name != "Staff" {
			t.Errorf("created = %+v, want a Staff type under %v", created, eventID)
		}
		// rules defaults to {} on a create that omits it, matching the column default.
		if created != nil && string(created.Rules) != "{}" {
			t.Errorf("created rules = %s, want {}", created.Rules)
		}
	})

	t.Run("name clash with a soft-deleted row maps to 409, not 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		droppedID := uuid.New()
		repo := listReturning(ctrl, []*TicketType{{ID: droppedID, EventID: eventID, Name: "VIP"}}, nil)
		repo.EXPECT().Delete(gomock.Any(), droppedID).Return(nil)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrAlreadyExists)

		store := NewSetupStore(logger.NewNoOp(), repo, nil)
		_, err := store.ReplaceForEvent(context.Background(), eventID, []event.TicketTypeDraft{{Name: "VIP"}})
		assertErrorzCode(t, err, errorz.CodeConflict)
	})

	t.Run("an id from another event maps to 400 rather than panicking", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		stray := uuid.New()
		store := NewSetupStore(logger.NewNoOp(), listReturning(ctrl, nil, nil), nil)

		_, err := store.ReplaceForEvent(context.Background(), eventID,
			[]event.TicketTypeDraft{{ID: &stray, Name: "VIP"}})
		assertErrorzCode(t, err, errorz.CodeBadRequest)
	})

	t.Run("a step link failure maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := listReturning(ctrl, nil, nil)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		junction := mocktickettype.NewMockTicketTypeWorkflowStepRepository(ctrl)
		junction.EXPECT().SetWorkflowStepIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("boom"))

		store := NewSetupStore(logger.NewNoOp(), repo, junction)
		_, err := store.ReplaceForEvent(context.Background(), eventID, []event.TicketTypeDraft{{Name: "VIP"}})
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("an empty payload deletes every live ticket type", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		a, b := uuid.New(), uuid.New()
		repo := listReturning(ctrl, []*TicketType{{ID: a, EventID: eventID}, {ID: b, EventID: eventID}}, nil)
		repo.EXPECT().Delete(gomock.Any(), a).Return(nil)
		repo.EXPECT().Delete(gomock.Any(), b).Return(nil)

		store := NewSetupStore(logger.NewNoOp(), repo, nil)
		got, err := store.ReplaceForEvent(context.Background(), eventID, nil)
		if err != nil {
			t.Fatalf("ReplaceForEvent() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("views = %+v, want empty", got)
		}
	})
}
