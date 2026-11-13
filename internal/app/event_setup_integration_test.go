package app

import (
	"context"
	"testing"
	"time"

	sdkauth "github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/google/uuid"

	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/biairmal/guest-management-be/internal/features/events/category"
	"github.com/biairmal/guest-management-be/internal/features/events/event"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowstep"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowsteptemplate"
	"github.com/biairmal/guest-management-be/internal/features/guests"
	"github.com/biairmal/guest-management-be/internal/features/tickets/tickettype"
	"github.com/biairmal/guest-management-be/internal/features/tickets/tickettypetemplate"
)

// failingTicketTypeStore wraps the real store and fails the ticket-type half of
// a setup save, so the test can assert the step half was rolled back with it.
// The steps are written first, on the same transaction, so nothing but a real
// rollback can undo them — which is the property this test exists for (B16 R1
// AC2).
type failingTicketTypeStore struct {
	event.TicketTypeStore
	err error
}

func (s failingTicketTypeStore) ReplaceForEvent(
	_ context.Context, _ uuid.UUID, _ []event.TicketTypeDraft,
) ([]event.TicketTypeStoreView, error) {
	return nil, s.err
}

// TestEventSetup_TransactionRollsBackSteps runs B16 R1 end to end against the
// local Postgres (DATABASE_* env, defaults matching .env.example; migrations
// applied): save an event's setup, then make the ticket-type write fail and
// check the workflow steps are exactly as they were before.
func TestEventSetup_TransactionRollsBackSteps(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: needs a live database")
	}
	ctx := sdkauth.ContextWithClaims(context.Background(), sdkauth.NewClaims(map[string]any{
		"sub": uuid.NewString(), "tenant_id": category.PlatformTenantID.String(),
	}))
	db := openTestDB(t)
	log := logger.NewNoOp()
	noCache := corerepository.CacheOptions{}

	versionRepo := category.NewTemplateVersionRepository(log, db)
	stepTemplateRepo := workflowsteptemplate.NewWorkflowStepTemplateRepository(log, db, noCache)
	ticketTemplateRepo := tickettypetemplate.NewTicketTypeTemplateRepository(log, db, noCache)
	ticketTemplateStepRepo := tickettypetemplate.NewWorkflowStepRepository(log, db)
	workflowStepRepo := workflowstep.NewWorkflowStepRepository(log, db, noCache)
	ticketTypeRepo := tickettype.NewTicketTypeRepository(log, db, noCache)
	junctionRepo := tickettype.NewTicketTypeWorkflowStepRepository(log, db)

	categories := category.NewService(log, category.NewCategoryRepository(log, db, noCache), versionRepo,
		stepTemplateRepo, tickettypetemplate.NewStore(log, ticketTemplateRepo, ticketTemplateStepRepo), db)
	ticketTypes := tickettype.NewTicketTypeService(log, ticketTypeRepo,
		junctionRepo, ticketTemplateRepo, ticketTemplateStepRepo)
	setupStore := tickettype.NewSetupStore(log, ticketTypeRepo, junctionRepo)

	newEvents := func(store event.TicketTypeStore) event.Service {
		return event.NewService(log, event.NewEventRepository(log, db, noCache),
			event.NewEventTenantRepository(log, db), stepTemplateRepo, workflowStepRepo,
			workflowstep.NewService(log, workflowStepRepo), ticketTypes,
			store, guests.NewGuestStats(log, db), versionRepo, db)
	}

	// A category with no templates, so the new event starts with an empty setup
	// and this test's own saves are the only writes to it.
	created, err := categories.Create(ctx, category.CreateInput{
		Source: category.SourceTenant, Name: "B16 integration " + uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("category Create: %v", err)
	}
	var eventID uuid.UUID
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.Leader().ExecContext(cleanup, `DELETE FROM events WHERE id = $1`, eventID)
		_, _ = db.Leader().ExecContext(cleanup, `DELETE FROM event_categories WHERE id = $1`, created.ID)
	})

	start := time.Now().Add(24 * time.Hour)
	ev, err := newEvents(setupStore).Create(ctx, event.CreateEventInput{
		CategoryID: created.ID, Name: "B16 integration event",
		StartDate: start, EndDate: start.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("event Create: %v", err)
	}
	eventID = ev.ID

	saved, err := newEvents(setupStore).ReplaceSetup(ctx, eventID, event.SetupInput{
		WorkflowSteps: []event.WorkflowStepInput{{Name: "Check-in"}, {Name: "Dinner"}},
		TicketTypes:   []event.TicketTypeInput{{Name: "VIP", Steps: []int{0, 1}}},
	})
	if err != nil {
		t.Fatalf("ReplaceSetup: %v", err)
	}
	if len(saved.WorkflowSteps) != 2 || len(saved.TicketTypes) != 1 {
		t.Fatalf("saved setup = %+v, want 2 steps and 1 ticket type", saved)
	}
	if len(saved.TicketTypes[0].Steps) != 2 {
		t.Errorf("VIP steps = %v, want both", saved.TicketTypes[0].Steps)
	}

	// Now a save that reorders the steps and adds a third, but whose ticket-type
	// write fails. Every step change must be rolled back with it.
	failing := newEvents(failingTicketTypeStore{
		TicketTypeStore: setupStore, err: context.DeadlineExceeded,
	})
	keptID := saved.WorkflowSteps[0].ID
	_, err = failing.ReplaceSetup(ctx, eventID, event.SetupInput{
		WorkflowSteps: []event.WorkflowStepInput{
			{Name: "Afterparty"}, {ID: &keptID, Name: "Check-in renamed"},
		},
		TicketTypes: []event.TicketTypeInput{{ID: &saved.TicketTypes[0].ID, Name: "VIP", Steps: []int{0}}},
	})
	if err == nil {
		t.Fatal("ReplaceSetup: expected the ticket-type failure to propagate")
	}

	after, err := newEvents(setupStore).GetSetup(ctx, eventID)
	if err != nil {
		t.Fatalf("GetSetup: %v", err)
	}
	if len(after.WorkflowSteps) != 2 {
		t.Fatalf("steps after the failed save = %+v, want the 2 from before (rolled back)", after.WorkflowSteps)
	}
	for i, want := range []string{"Check-in", "Dinner"} {
		if after.WorkflowSteps[i].Name != want {
			t.Errorf("step %d = %q, want %q — the rename was not rolled back",
				i, after.WorkflowSteps[i].Name, want)
		}
	}
	if after.WorkflowSteps[0].ID != keptID {
		t.Errorf("step 0 id = %v, want the original %v", after.WorkflowSteps[0].ID, keptID)
	}
}
