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

func TestEventService_Create(t *testing.T) {
	start := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		in      CreateEventInput
		expects bool
		repoErr error
		wantErr string
	}{
		{
			name: "end_date before start_date rejected",
			in: CreateEventInput{
				TenantID: uuid.New(), CategoryID: uuid.New(), Name: "x", StartDate: start, EndDate: start.Add(-time.Hour),
			},
			wantErr: errorz.CodeBadRequest,
		},
		{
			name: "already exists maps to 409",
			in: CreateEventInput{
				TenantID: uuid.New(), CategoryID: uuid.New(), Name: "x", StartDate: start, EndDate: start.Add(time.Hour),
			},
			expects: true,
			repoErr: repository.ErrAlreadyExists,
			wantErr: errorz.CodeConflict,
		},
		{
			name: "invalid entity maps to 422",
			in: CreateEventInput{
				TenantID: uuid.New(), CategoryID: uuid.New(), Name: "x", StartDate: start, EndDate: start.Add(time.Hour),
			},
			expects: true,
			repoErr: repository.ErrInvalidEntity,
			wantErr: errorz.CodeUnprocessableEntity,
		},
		{
			name: "unexpected repo error maps to 500",
			in: CreateEventInput{
				TenantID: uuid.New(), CategoryID: uuid.New(), Name: "x", StartDate: start, EndDate: start.Add(time.Hour),
			},
			expects: true,
			repoErr: errors.New("boom"),
			wantErr: errorz.CodeInternal,
		},
		{
			name: "happy path",
			in: CreateEventInput{
				TenantID: uuid.New(), CategoryID: uuid.New(), Name: "x", StartDate: start, EndDate: start.Add(24 * time.Hour),
			},
			expects: true,
		},
	}
	falseVal := false
	tests = append(tests, struct {
		name    string
		in      CreateEventInput
		expects bool
		repoErr error
		wantErr string
	}{
		name: "rsvp_required false is preserved, not defaulted",
		in: CreateEventInput{
			TenantID: uuid.New(), CategoryID: uuid.New(), Name: "x",
			StartDate: start, EndDate: start.Add(time.Hour), RsvpRequired: &falseVal,
		},
		expects: true,
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[Event, uuid.UUID](ctrl)
			templateRepo := mockrepository.NewMockRepository[workflowsteptemplate.WorkflowStepTemplate, uuid.UUID](ctrl)
			workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
			if tt.expects {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tt.repoErr)
			}
			if tt.expects && tt.repoErr == nil {
				// Create succeeded: copyWorkflowStepTemplates always runs.
				templateRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
			}

			svc := NewService(logger.NewNoOp(), repo, templateRepo, workflowStepRepo)
			got, err := svc.Create(context.Background(), tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" {
				if got == nil {
					t.Fatal("expected non-nil entity on success")
				}
				if got.IsMultiDay != isMultiDay(tt.in.StartDate, tt.in.EndDate) {
					t.Errorf("IsMultiDay = %v, want %v", got.IsMultiDay, isMultiDay(tt.in.StartDate, tt.in.EndDate))
				}
				wantRsvpRequired := true
				if tt.in.RsvpRequired != nil {
					wantRsvpRequired = *tt.in.RsvpRequired
				}
				if got.RsvpRequired != wantRsvpRequired {
					t.Errorf("RsvpRequired = %v, want %v", got.RsvpRequired, wantRsvpRequired)
				}
			}
		})
	}
}

func TestEventService_Create_CopiesWorkflowStepTemplates(t *testing.T) {
	categoryID := uuid.New()
	in := CreateEventInput{
		TenantID: uuid.New(), CategoryID: categoryID, Name: "x",
		StartDate: time.Now(), EndDate: time.Now().Add(time.Hour),
	}

	tests := []struct {
		name          string
		templates     []*workflowsteptemplate.WorkflowStepTemplate
		templateErr   error
		stepCreateErr error
		wantStepCalls int
	}{
		{name: "no templates for category copies nothing"},
		{
			name:        "template lookup failure is non-fatal",
			templateErr: errors.New("boom"),
		},
		{
			name: "templates are copied as workflow steps",
			templates: []*workflowsteptemplate.WorkflowStepTemplate{
				{ID: uuid.New(), CategoryID: categoryID, Name: "Check-in", OrderIndex: 0},
				{ID: uuid.New(), CategoryID: categoryID, Name: "Photo booth", OrderIndex: 1, AllowsMultiple: true},
			},
			wantStepCalls: 2,
		},
		{
			name: "a single step copy failure is non-fatal and does not block the rest",
			templates: []*workflowsteptemplate.WorkflowStepTemplate{
				{ID: uuid.New(), CategoryID: categoryID, Name: "Check-in", OrderIndex: 0},
			},
			stepCreateErr: errors.New("boom"),
			wantStepCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[Event, uuid.UUID](ctrl)
			templateRepo := mockrepository.NewMockRepository[workflowsteptemplate.WorkflowStepTemplate, uuid.UUID](ctrl)
			workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)

			repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			templateRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(tt.templates, int64(len(tt.templates)), tt.templateErr)
			if tt.wantStepCalls > 0 {
				workflowStepRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tt.stepCreateErr).Times(tt.wantStepCalls)
			}

			svc := NewService(logger.NewNoOp(), repo, templateRepo, workflowStepRepo)
			got, err := svc.Create(context.Background(), in)
			if err != nil {
				t.Fatalf("Create() error = %v, want nil (template copy is best-effort)", err)
			}
			if got == nil {
				t.Fatal("expected non-nil entity")
			}
		})
	}
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

			svc := NewService(logger.NewNoOp(), repo, nil, nil)
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

			svc := NewService(logger.NewNoOp(), repo, nil, nil)
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

			svc := NewService(logger.NewNoOp(), repo, nil, nil)
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

			svc := NewService(logger.NewNoOp(), repo, nil, nil)
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
