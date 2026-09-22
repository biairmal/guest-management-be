package app

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	sdkauth "github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	"github.com/google/uuid"
	_ "github.com/lib/pq"

	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/biairmal/guest-management-be/internal/features/events/category"
	"github.com/biairmal/guest-management-be/internal/features/events/event"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowstep"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowsteptemplate"
	"github.com/biairmal/guest-management-be/internal/features/tickets/tickettype"
	"github.com/biairmal/guest-management-be/internal/features/tickets/tickettypetemplate"
)

// TestCategoryTemplateSet_EventSeeding runs B15 end to end against the
// local Postgres (DATABASE_* env, defaults matching .env.example; migrations
// applied): save a category template set, replace it (re-using the same
// names and positions), reject a stale save, then create an event and check
// the seeded ticket types include exactly the event steps their templates
// included. This also guards the transaction-aware junction repositories —
// before B15 the seeded ticket types' step inserts ran outside the event
// transaction and failed their FKs.
func TestCategoryTemplateSet_EventSeeding(t *testing.T) {
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
	junctionRepo := tickettype.NewTicketTypeWorkflowStepRepository(log, db)

	categories := category.NewService(log, category.NewCategoryRepository(log, db, noCache), versionRepo,
		stepTemplateRepo, tickettypetemplate.NewStore(log, ticketTemplateRepo, ticketTemplateStepRepo), db)
	ticketTypes := tickettype.NewTicketTypeService(log, tickettype.NewTicketTypeRepository(log, db, noCache),
		junctionRepo, workflowStepRepo, ticketTemplateRepo, ticketTemplateStepRepo)
	events := event.NewService(log, event.NewEventRepository(log, db, noCache),
		stepTemplateRepo, workflowStepRepo, ticketTypes, versionRepo, db)

	created, err := categories.Create(ctx, category.CreateInput{
		Source: category.SourceTenant, Name: "B15 integration " + uuid.NewString(),
		WorkflowSteps: []category.WorkflowStepInput{{Name: "Check-in"}, {Name: "Dinner"}},
		TicketTypes:   []category.TicketTypeInput{{Name: "VIP", Steps: []int{0, 1}}, {Name: "Crew", Steps: []int{1}}},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	var eventID uuid.UUID
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.Leader().ExecContext(cleanup, `DELETE FROM events WHERE id = $1`, eventID)
		_, _ = db.Leader().ExecContext(cleanup, `DELETE FROM event_categories WHERE id = $1`, created.ID)
	})

	// Same names and positions as version 1: must not collide with the
	// soft-deleted rows (the pre-B15 409 bug).
	replaced, err := categories.Replace(ctx, created.ID, category.ReplaceInput{
		Name: created.Name, TemplateVersion: 1,
		WorkflowSteps: []category.WorkflowStepInput{{Name: "Check-in"}, {Name: "Dinner"}, {Name: "Afterparty"}},
		TicketTypes:   []category.TicketTypeInput{{Name: "VIP", Steps: []int{0, 2}}, {Name: "Crew", Steps: []int{}}},
	})
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if replaced.TemplateVersion != 2 {
		t.Fatalf("TemplateVersion = %d, want 2", replaced.TemplateVersion)
	}

	_, err = categories.Replace(ctx, created.ID, category.ReplaceInput{Name: created.Name, TemplateVersion: 1})
	var ez *errorz.Error
	if !errors.As(err, &ez) || ez.Code != errorz.CodeConflict {
		t.Fatalf("stale Replace err = %v, want conflict", err)
	}

	start := time.Now().Add(24 * time.Hour)
	ev, err := events.Create(ctx, event.CreateEventInput{
		TenantID: category.PlatformTenantID, CategoryID: created.ID, Name: "B15 integration event",
		StartDate: start, EndDate: start.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("event Create: %v", err)
	}
	eventID = ev.ID
	if ev.CategoryTemplateVersion != 2 {
		t.Errorf("CategoryTemplateVersion = %d, want 2", ev.CategoryTemplateVersion)
	}

	assertSeededSteps(ctx, t, db, ev.ID, map[string][]string{"VIP": {"Check-in", "Afterparty"}, "Crew": {}})
}

// assertSeededSteps checks each of eventID's ticket types includes exactly
// the named workflow steps.
func assertSeededSteps(ctx context.Context, t *testing.T, db *sqlkit.DB, eventID uuid.UUID, want map[string][]string) {
	t.Helper()
	rows, err := db.Leader().QueryContext(ctx, `
SELECT tt.name, ws.name FROM ticket_types tt
LEFT JOIN ticket_type_workflow_steps j ON j.ticket_type_id = tt.id
LEFT JOIN workflow_steps ws ON ws.id = j.workflow_step_id
WHERE tt.event_id = $1`, eventID)
	if err != nil {
		t.Fatalf("query seeded steps: %v", err)
	}
	defer rows.Close()
	got := map[string]map[string]bool{}
	for rows.Next() {
		var ticketType string
		var step *string
		if err := rows.Scan(&ticketType, &step); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if got[ticketType] == nil {
			got[ticketType] = map[string]bool{}
		}
		if step != nil {
			got[ticketType][*step] = true
		}
	}
	if len(got) != len(want) {
		t.Fatalf("seeded ticket types = %v, want %v", got, want)
	}
	for ticketType, steps := range want {
		if len(got[ticketType]) != len(steps) {
			t.Errorf("%s steps = %v, want %v", ticketType, got[ticketType], steps)
		}
		for _, s := range steps {
			if !got[ticketType][s] {
				t.Errorf("%s is missing step %q (got %v)", ticketType, s, got[ticketType])
			}
		}
	}
}

// openTestDB connects to the database named by DATABASE_* env vars.
func openTestDB(t *testing.T) *sqlkit.DB {
	t.Helper()
	env := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return def
	}
	db, err := sqlkit.New(context.Background(), &sqlkit.Config{Leader: sqlkit.DBConfig{
		Driver: "postgres", Host: env("DATABASE_HOST", "localhost"), Port: 5432,
		Database: env("DATABASE_DATABASE", "guest_management"),
		Username: env("DATABASE_USERNAME", "postgres"), Password: env("DATABASE_PASSWORD", "postgres"),
		SSLMode: "disable",
	}})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
