package event

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	mockrepository "github.com/biairmal/go-sdk/mocks/repository"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowstep"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowsteptemplate"
)

// stubTicketTypeSeeder is a real, minimal TicketTypeSeeder for tests that
// exercise createEvent's transaction body without a live database — see
// ticket_type_seeder.go for why this isn't a generated mock (it would import
// package event's own types and cycle with this same-package test file, the
// same reasoning as guests' noopInvitationPublisher).
type stubTicketTypeSeeder struct {
	err              error
	called           bool
	calledEventID    uuid.UUID
	calledCategoryID uuid.UUID
}

func (s *stubTicketTypeSeeder) SeedFromCategoryTemplates(_ context.Context, eventID, categoryID uuid.UUID) error {
	s.called = true
	s.calledEventID = eventID
	s.calledCategoryID = categoryID
	return s.err
}

func TestIsMultiDay(t *testing.T) {
	day1 := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		start time.Time
		end   time.Time
		want  bool
	}{
		{name: "same calendar day", start: day1, end: day1.Add(2 * time.Hour), want: false},
		{name: "spans midnight", start: day1, end: day1.Add(24 * time.Hour), want: true},
		{name: "same instant", start: day1, end: day1, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMultiDay(tt.start, tt.end); got != tt.want {
				t.Errorf("isMultiDay() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildEvent(t *testing.T) {
	start := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	falseVal := false

	t.Run("rsvp_required defaults to true when omitted", func(t *testing.T) {
		got := buildEvent(CreateEventInput{StartDate: start, EndDate: start.Add(time.Hour)})
		if !got.RsvpRequired {
			t.Errorf("RsvpRequired = %v, want true", got.RsvpRequired)
		}
	})

	t.Run("rsvp_required false is preserved, not defaulted", func(t *testing.T) {
		got := buildEvent(CreateEventInput{StartDate: start, EndDate: start.Add(time.Hour), RsvpRequired: &falseVal})
		if got.RsvpRequired {
			t.Errorf("RsvpRequired = %v, want false", got.RsvpRequired)
		}
	})

	t.Run("is_multi_day derived from dates", func(t *testing.T) {
		got := buildEvent(CreateEventInput{StartDate: start, EndDate: start.Add(24 * time.Hour)})
		if !got.IsMultiDay {
			t.Error("IsMultiDay = false, want true")
		}
	})
}

// TestEventService_Create_ValidatesDates exercises only the validation that
// runs before Create's (*sqlkit.DB).WithTransaction call — the only part of
// the public wrapper reachable without a live database in a unit test (a nil
// db panics once WithTransaction is actually invoked). The transactional
// body is covered directly via TestEventService_createEvent below, mirroring
// users' transferMaster split.
func TestEventService_Create_ValidatesDates(t *testing.T) {
	start := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	svc := NewService(logger.NewNoOp(), nil, nil, nil, nil, nil)

	_, err := svc.Create(context.Background(), CreateEventInput{
		TenantID: uuid.New(), CategoryID: uuid.New(), Name: "x", StartDate: start, EndDate: start.Add(-time.Hour),
	})
	assertErrorzCode(t, err, errorz.CodeBadRequest)
}

// TestEventService_createEvent exercises Create's transactional body
// (insert + workflow-step-template copy + ticket-type-template seed)
// directly against mocked collaborators, without a live transaction — the
// same reasoning as users' TestUserService_transferMaster.
func TestEventService_createEvent(t *testing.T) {
	newEntity := func() *Event { return &Event{ID: uuid.New(), CategoryID: uuid.New()} }

	t.Run("already exists maps to 409, no further steps run", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Event, uuid.UUID](ctrl)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrAlreadyExists)
		seeder := &stubTicketTypeSeeder{}
		svc := &eventServiceImpl{repo: repo, ticketTypeSeeder: seeder, logger: logger.NewNoOp()}

		err := svc.createEvent(context.Background(), newEntity())
		assertErrorzCode(t, err, errorz.CodeConflict)
		if seeder.called {
			t.Error("ticketTypeSeeder should not be called when the event insert fails")
		}
	})

	t.Run("invalid entity maps to 422", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Event, uuid.UUID](ctrl)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrInvalidEntity)
		svc := &eventServiceImpl{repo: repo, logger: logger.NewNoOp()}

		err := svc.createEvent(context.Background(), newEntity())
		assertErrorzCode(t, err, errorz.CodeUnprocessableEntity)
	})

	t.Run("unexpected repo error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Event, uuid.UUID](ctrl)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("boom"))
		svc := &eventServiceImpl{repo: repo, logger: logger.NewNoOp()}

		err := svc.createEvent(context.Background(), newEntity())
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("workflow step template copy failure aborts before the ticket type seed runs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Event, uuid.UUID](ctrl)
		templateRepo := mockrepository.NewMockRepository[workflowsteptemplate.WorkflowStepTemplate, uuid.UUID](ctrl)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		templateRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("boom"))
		seeder := &stubTicketTypeSeeder{}
		svc := &eventServiceImpl{repo: repo, templateRepo: templateRepo, ticketTypeSeeder: seeder, logger: logger.NewNoOp()}

		err := svc.createEvent(context.Background(), newEntity())
		assertErrorzCode(t, err, errorz.CodeInternal)
		if seeder.called {
			t.Error("ticketTypeSeeder should not be called when the workflow step template copy fails (transaction aborts)")
		}
	})

	t.Run("ticket type seed failure propagates (aborts the transaction)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Event, uuid.UUID](ctrl)
		templateRepo := mockrepository.NewMockRepository[workflowsteptemplate.WorkflowStepTemplate, uuid.UUID](ctrl)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		templateRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		seeder := &stubTicketTypeSeeder{err: errorz.Conflict().WithMessage("ticket type template conflict")}
		svc := &eventServiceImpl{repo: repo, templateRepo: templateRepo, ticketTypeSeeder: seeder, logger: logger.NewNoOp()}

		err := svc.createEvent(context.Background(), newEntity())
		assertErrorzCode(t, err, errorz.CodeConflict)
	})

	t.Run("happy path seeds ticket types from the event's own id/category_id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[Event, uuid.UUID](ctrl)
		templateRepo := mockrepository.NewMockRepository[workflowsteptemplate.WorkflowStepTemplate, uuid.UUID](ctrl)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		templateRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		seeder := &stubTicketTypeSeeder{}
		svc := &eventServiceImpl{repo: repo, templateRepo: templateRepo, ticketTypeSeeder: seeder, logger: logger.NewNoOp()}

		entity := newEntity()
		if err := svc.createEvent(context.Background(), entity); err != nil {
			t.Fatalf("createEvent() error = %v", err)
		}
		if !seeder.called {
			t.Fatal("expected ticketTypeSeeder.SeedFromCategoryTemplates to be called")
		}
		if seeder.calledEventID != entity.ID || seeder.calledCategoryID != entity.CategoryID {
			t.Errorf("seeder called with (%v, %v), want (%v, %v)",
				seeder.calledEventID, seeder.calledCategoryID, entity.ID, entity.CategoryID)
		}
	})
}

func TestEventService_copyWorkflowStepTemplates(t *testing.T) {
	categoryID := uuid.New()
	entity := &Event{ID: uuid.New(), CategoryID: categoryID}

	t.Run("template lookup failure aborts and maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		templateRepo := mockrepository.NewMockRepository[workflowsteptemplate.WorkflowStepTemplate, uuid.UUID](ctrl)
		templateRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("boom"))
		svc := &eventServiceImpl{templateRepo: templateRepo, logger: logger.NewNoOp()}

		err := svc.copyWorkflowStepTemplates(context.Background(), entity)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("no templates for category copies nothing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		templateRepo := mockrepository.NewMockRepository[workflowsteptemplate.WorkflowStepTemplate, uuid.UUID](ctrl)
		templateRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		svc := &eventServiceImpl{templateRepo: templateRepo, logger: logger.NewNoOp()}

		if err := svc.copyWorkflowStepTemplates(context.Background(), entity); err != nil {
			t.Fatalf("copyWorkflowStepTemplates() error = %v", err)
		}
	})

	t.Run("templates are copied as workflow steps", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		templateRepo := mockrepository.NewMockRepository[workflowsteptemplate.WorkflowStepTemplate, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		templates := []*workflowsteptemplate.WorkflowStepTemplate{
			{ID: uuid.New(), CategoryID: categoryID, Name: "Check-in", OrderIndex: 0},
			{ID: uuid.New(), CategoryID: categoryID, Name: "Photo booth", OrderIndex: 1, AllowsMultiple: true},
		}
		templateRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(templates, int64(2), nil)
		workflowStepRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).Times(2)
		svc := &eventServiceImpl{templateRepo: templateRepo, workflowStepRepo: workflowStepRepo, logger: logger.NewNoOp()}

		if err := svc.copyWorkflowStepTemplates(context.Background(), entity); err != nil {
			t.Fatalf("copyWorkflowStepTemplates() error = %v", err)
		}
	})

	t.Run("a step copy failure aborts immediately, no more templates are attempted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		templateRepo := mockrepository.NewMockRepository[workflowsteptemplate.WorkflowStepTemplate, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		templates := []*workflowsteptemplate.WorkflowStepTemplate{
			{ID: uuid.New(), CategoryID: categoryID, Name: "Check-in", OrderIndex: 0},
			{ID: uuid.New(), CategoryID: categoryID, Name: "Photo booth", OrderIndex: 1},
		}
		templateRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(templates, int64(2), nil)
		// Only the first template's Create call is expected: a failure aborts
		// the loop instead of continuing to the second template.
		workflowStepRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("boom"))
		svc := &eventServiceImpl{templateRepo: templateRepo, workflowStepRepo: workflowStepRepo, logger: logger.NewNoOp()}

		err := svc.copyWorkflowStepTemplates(context.Background(), entity)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})
}

func TestEventService_GetByID(t *testing.T) {
	tests := []struct {
		name    string
		repoRes *Event
		repoErr error
		wantErr string
	}{
		{name: "found", repoRes: &Event{Name: "x"}},
		{name: "not found maps to 404", repoErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound},
		{name: "unexpected error maps to 500", repoErr: errors.New("boom"), wantErr: errorz.CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[Event, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.repoRes, tt.repoErr)

			svc := NewService(logger.NewNoOp(), repo, nil, nil, nil, nil)
			_, err := svc.GetByID(context.Background(), uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestEventService_Update(t *testing.T) {
	start := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	earlier := start.Add(-time.Hour)

	tests := []struct {
		name       string
		in         UpdateEventInput
		getRes     *Event
		getErr     error
		expectsGet bool
		expectsSet bool
		updateErr  error
		wantErr    string
	}{
		{
			name:       "get not found maps to 404",
			expectsGet: true,
			getErr:     repository.ErrNotFound,
			wantErr:    errorz.CodeNotFound,
		},
		{
			name:       "get unexpected error maps to 500",
			expectsGet: true,
			getErr:     errors.New("boom"),
			wantErr:    errorz.CodeInternal,
		},
		{
			name:       "resulting end_date before start_date rejected",
			in:         UpdateEventInput{EndDate: &earlier},
			expectsGet: true,
			getRes:     &Event{Name: "x", StartDate: start, EndDate: end},
			wantErr:    errorz.CodeBadRequest,
		},
		{
			name:       "update not found maps to 404",
			in:         UpdateEventInput{Name: ptrString("y")},
			expectsGet: true,
			getRes:     &Event{Name: "x", StartDate: start, EndDate: end},
			expectsSet: true,
			updateErr:  repository.ErrNotFound,
			wantErr:    errorz.CodeNotFound,
		},
		{
			name:       "happy path partial update recomputes is_multi_day",
			in:         UpdateEventInput{EndDate: ptrTime(start.Add(48 * time.Hour))},
			expectsGet: true,
			getRes:     &Event{Name: "x", StartDate: start, EndDate: end},
			expectsSet: true,
		},
		{
			name:       "rsvp_required can be toggled off",
			in:         UpdateEventInput{RsvpRequired: ptrBool(false)},
			expectsGet: true,
			getRes:     &Event{Name: "x", StartDate: start, EndDate: end, RsvpRequired: true},
			expectsSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[Event, uuid.UUID](ctrl)
			if tt.expectsGet {
				repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.getRes, tt.getErr)
			}
			if tt.expectsSet {
				repo.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.updateErr)
			}

			svc := NewService(logger.NewNoOp(), repo, nil, nil, nil, nil)
			got, err := svc.Update(context.Background(), uuid.New(), tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && tt.name == "happy path partial update recomputes is_multi_day" && !got.IsMultiDay {
				t.Errorf("IsMultiDay = %v, want true", got.IsMultiDay)
			}
			if tt.wantErr == "" && tt.in.RsvpRequired != nil && got.RsvpRequired != *tt.in.RsvpRequired {
				t.Errorf("RsvpRequired = %v, want %v", got.RsvpRequired, *tt.in.RsvpRequired)
			}
		})
	}
}

func TestEventService_Delete(t *testing.T) {
	tests := []struct {
		name    string
		repoErr error
		wantErr string
	}{
		{name: "not found maps to 404", repoErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound},
		{name: "unexpected error maps to 500", repoErr: errors.New("boom"), wantErr: errorz.CodeInternal},
		{name: "happy path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[Event, uuid.UUID](ctrl)
			repo.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(tt.repoErr)

			svc := NewService(logger.NewNoOp(), repo, nil, nil, nil, nil)
			err := svc.Delete(context.Background(), uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestEventService_List(t *testing.T) {
	tests := []struct {
		name     string
		repoErr  error
		wantErr  string
		wantSize int
	}{
		{name: "repo error maps to 500", repoErr: errors.New("boom"), wantErr: errorz.CodeInternal},
		{name: "happy path", wantSize: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[Event, uuid.UUID](ctrl)
			repo.EXPECT().
				List(gomock.Any(), gomock.Any()).
				Return([]*Event{{Name: "x"}}, int64(1), tt.repoErr)

			svc := NewService(logger.NewNoOp(), repo, nil, nil, nil, nil)
			params, err := query.ParseListParams(url.Values{}, query.ListParseConfig{})
			if err != nil {
				t.Fatalf("ParseListParams() error = %v", err)
			}
			got, err := svc.List(context.Background(), params)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && got.Size != tt.wantSize {
				t.Errorf("Size = %d, want %d", got.Size, tt.wantSize)
			}
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func ptrBool(b bool) *bool { return &b }
