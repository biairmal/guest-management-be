package events

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	mockrepository "github.com/biairmal/go-sdk/mocks/repository"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/biairmal/guest-management-be/internal/core/query"
)

//nolint:dupl // table-driven CRUD test shape intentionally mirrors WorkflowStepTemplateService tests (see PATTERNS.md)
func TestWorkflowStepService_Create(t *testing.T) {
	tests := []struct {
		name    string
		in      CreateWorkflowStepInput
		expects bool
		repoErr error
		wantErr string
	}{
		{
			name:    "already exists maps to 409",
			in:      CreateWorkflowStepInput{Name: "Check-in", OrderIndex: 0},
			expects: true,
			repoErr: repository.ErrAlreadyExists,
			wantErr: errorz.CodeConflict,
		},
		{
			name:    "invalid entity maps to 422",
			in:      CreateWorkflowStepInput{Name: "Check-in", OrderIndex: 0},
			expects: true,
			repoErr: repository.ErrInvalidEntity,
			wantErr: errorz.CodeUnprocessableEntity,
		},
		{
			name:    "unexpected repo error maps to 500",
			in:      CreateWorkflowStepInput{Name: "Check-in", OrderIndex: 0},
			expects: true,
			repoErr: errors.New("boom"),
			wantErr: errorz.CodeInternal,
		},
		{
			name:    "happy path",
			in:      CreateWorkflowStepInput{Name: "Check-in", OrderIndex: 0},
			expects: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[WorkflowStep, uuid.UUID](ctrl)
			if tt.expects {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tt.repoErr)
			}

			svc := NewWorkflowStepService(logger.NewNoOp(), repo)
			got, err := svc.Create(context.Background(), uuid.New(), tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && got == nil {
				t.Fatal("expected non-nil entity on success")
			}
		})
	}
}

func TestWorkflowStepService_GetByID(t *testing.T) {
	eventID := uuid.New()
	tests := []struct {
		name    string
		repoRes *WorkflowStep
		repoErr error
		wantErr string
	}{
		{name: "found for matching event", repoRes: &WorkflowStep{EventID: eventID, Name: "x"}},
		{
			name:    "found for different event maps to 404",
			repoRes: &WorkflowStep{EventID: uuid.New(), Name: "x"},
			wantErr: errorz.CodeNotFound,
		},
		{name: "not found maps to 404", repoErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound},
		{name: "unexpected error maps to 500", repoErr: errors.New("boom"), wantErr: errorz.CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[WorkflowStep, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.repoRes, tt.repoErr)

			svc := NewWorkflowStepService(logger.NewNoOp(), repo)
			_, err := svc.GetByID(context.Background(), eventID, uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestWorkflowStepService_Update(t *testing.T) {
	eventID := uuid.New()
	tests := []struct {
		name       string
		in         UpdateWorkflowStepInput
		getRes     *WorkflowStep
		getErr     error
		expectsSet bool
		updateErr  error
		wantErr    string
	}{
		{
			name:    "get not found maps to 404",
			getErr:  repository.ErrNotFound,
			wantErr: errorz.CodeNotFound,
		},
		{
			name:    "get for different event maps to 404",
			getRes:  &WorkflowStep{EventID: uuid.New(), Name: "x"},
			wantErr: errorz.CodeNotFound,
		},
		{
			name:       "update conflict maps to 409",
			in:         UpdateWorkflowStepInput{OrderIndex: ptrInt(1)},
			getRes:     &WorkflowStep{EventID: eventID, Name: "x"},
			expectsSet: true,
			updateErr:  repository.ErrAlreadyExists,
			wantErr:    errorz.CodeConflict,
		},
		{
			name:       "happy path partial update",
			in:         UpdateWorkflowStepInput{Name: ptrString("y")},
			getRes:     &WorkflowStep{EventID: eventID, Name: "x"},
			expectsSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[WorkflowStep, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.getRes, tt.getErr)
			if tt.expectsSet {
				repo.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.updateErr)
			}

			svc := NewWorkflowStepService(logger.NewNoOp(), repo)
			got, err := svc.Update(context.Background(), eventID, uuid.New(), tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && got == nil {
				t.Fatal("expected non-nil entity on success")
			}
		})
	}
}

func TestWorkflowStepService_Delete(t *testing.T) {
	eventID := uuid.New()
	tests := []struct {
		name       string
		getRes     *WorkflowStep
		getErr     error
		expectsDel bool
		delErr     error
		wantErr    string
	}{
		{name: "get not found maps to 404", getErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound},
		{
			name:    "get for different event maps to 404",
			getRes:  &WorkflowStep{EventID: uuid.New(), Name: "x"},
			wantErr: errorz.CodeNotFound,
		},
		{
			name:       "delete not found maps to 404",
			getRes:     &WorkflowStep{EventID: eventID, Name: "x"},
			expectsDel: true,
			delErr:     repository.ErrNotFound,
			wantErr:    errorz.CodeNotFound,
		},
		{name: "happy path", getRes: &WorkflowStep{EventID: eventID, Name: "x"}, expectsDel: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[WorkflowStep, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.getRes, tt.getErr)
			if tt.expectsDel {
				repo.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(tt.delErr)
			}

			svc := NewWorkflowStepService(logger.NewNoOp(), repo)
			err := svc.Delete(context.Background(), eventID, uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestWorkflowStepService_List(t *testing.T) {
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
			repo := mockrepository.NewMockRepository[WorkflowStep, uuid.UUID](ctrl)
			repo.EXPECT().
				List(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, opts *repository.ListOptions) ([]*WorkflowStep, int64, error) {
					found := false
					for _, c := range opts.Filter.Conditions {
						if c.Field == "event_id" {
							found = true
						}
					}
					if !found {
						t.Error("expected event_id filter condition to be present")
					}
					return []*WorkflowStep{{Name: "x"}}, int64(1), tt.repoErr
				})

			svc := NewWorkflowStepService(logger.NewNoOp(), repo)
			params, err := query.ParseListParams(url.Values{}, query.ListParseConfig{})
			if err != nil {
				t.Fatalf("ParseListParams() error = %v", err)
			}
			got, err := svc.List(context.Background(), uuid.New(), params)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && got.Size != tt.wantSize {
				t.Errorf("Size = %d, want %d", got.Size, tt.wantSize)
			}
		})
	}
}

func ptrInt(i int) *int { return &i }

func TestWorkflowStepService_Sync(t *testing.T) {
	eventID := uuid.New()

	t.Run("duplicate order_index rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[WorkflowStep, uuid.UUID](ctrl)
		svc := NewWorkflowStepService(logger.NewNoOp(), repo)

		_, err := svc.Sync(context.Background(), eventID, []SyncWorkflowStepInput{
			{Name: "a", OrderIndex: 0},
			{Name: "b", OrderIndex: 0},
		})
		assertErrorzCode(t, err, errorz.CodeBadRequest)
	})

	t.Run("list failure maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[WorkflowStep, uuid.UUID](ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("boom"))
		svc := NewWorkflowStepService(logger.NewNoOp(), repo)

		_, err := svc.Sync(context.Background(), eventID, []SyncWorkflowStepInput{{Name: "a", OrderIndex: 0}})
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("id not belonging to this event maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[WorkflowStep, uuid.UUID](ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		svc := NewWorkflowStepService(logger.NewNoOp(), repo)

		unknown := uuid.New()
		_, err := svc.Sync(context.Background(), eventID, []SyncWorkflowStepInput{{ID: &unknown, Name: "a", OrderIndex: 0}})
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("deletes steps missing from the payload", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[WorkflowStep, uuid.UUID](ctrl)
		existingID := uuid.New()
		repo.EXPECT().List(gomock.Any(), gomock.Any()).
			Return([]*WorkflowStep{{ID: existingID, EventID: eventID, Name: "old", OrderIndex: 0}}, int64(1), nil)
		repo.EXPECT().Delete(gomock.Any(), existingID).Return(nil)
		svc := NewWorkflowStepService(logger.NewNoOp(), repo)

		got, err := svc.Sync(context.Background(), eventID, []SyncWorkflowStepInput{})
		if err != nil {
			t.Fatalf("Sync() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("len(got) = %d, want 0", len(got))
		}
	})

	t.Run("delete failure maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[WorkflowStep, uuid.UUID](ctrl)
		existingID := uuid.New()
		repo.EXPECT().List(gomock.Any(), gomock.Any()).
			Return([]*WorkflowStep{{ID: existingID, EventID: eventID, Name: "old", OrderIndex: 0}}, int64(1), nil)
		repo.EXPECT().Delete(gomock.Any(), existingID).Return(errors.New("boom"))
		svc := NewWorkflowStepService(logger.NewNoOp(), repo)

		_, err := svc.Sync(context.Background(), eventID, []SyncWorkflowStepInput{})
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("creates new steps", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[WorkflowStep, uuid.UUID](ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		svc := NewWorkflowStepService(logger.NewNoOp(), repo)

		got, err := svc.Sync(context.Background(), eventID, []SyncWorkflowStepInput{{Name: "a", OrderIndex: 0}})
		if err != nil {
			t.Fatalf("Sync() error = %v", err)
		}
		if len(got) != 1 || got[0].Name != "a" || got[0].OrderIndex != 0 {
			t.Errorf("got = %+v", got)
		}
	})

	t.Run("create failure maps to 422 for invalid entity", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[WorkflowStep, uuid.UUID](ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrInvalidEntity)
		svc := NewWorkflowStepService(logger.NewNoOp(), repo)

		_, err := svc.Sync(context.Background(), eventID, []SyncWorkflowStepInput{{Name: "a", OrderIndex: 0}})
		assertErrorzCode(t, err, errorz.CodeUnprocessableEntity)
	})

	t.Run("updates an existing step via temp-shift then final order", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[WorkflowStep, uuid.UUID](ctrl)
		existingID := uuid.New()
		repo.EXPECT().List(gomock.Any(), gomock.Any()).
			Return([]*WorkflowStep{{ID: existingID, EventID: eventID, Name: "old", OrderIndex: 0}}, int64(1), nil)

		var seenOrders []int
		repo.EXPECT().Update(gomock.Any(), existingID, gomock.Any()).Times(2).DoAndReturn(
			func(_ context.Context, _ uuid.UUID, e *WorkflowStep) error {
				seenOrders = append(seenOrders, e.OrderIndex)
				return nil
			},
		)
		svc := NewWorkflowStepService(logger.NewNoOp(), repo)

		got, err := svc.Sync(context.Background(), eventID,
			[]SyncWorkflowStepInput{{ID: &existingID, Name: "new", OrderIndex: 5}})
		if err != nil {
			t.Fatalf("Sync() error = %v", err)
		}
		if len(got) != 1 || got[0].Name != "new" || got[0].OrderIndex != 5 {
			t.Errorf("got = %+v", got)
		}
		if len(seenOrders) != 2 || seenOrders[0] >= 0 || seenOrders[1] != 5 {
			t.Errorf("seenOrders = %v, want [negative, 5]", seenOrders)
		}
	})

	t.Run("swapping two steps' order_index does not collide", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[WorkflowStep, uuid.UUID](ctrl)
		idA, idB := uuid.New(), uuid.New()
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*WorkflowStep{
			{ID: idA, EventID: eventID, Name: "A", OrderIndex: 0},
			{ID: idB, EventID: eventID, Name: "B", OrderIndex: 1},
		}, int64(2), nil)
		repo.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Times(4).Return(nil) // 2 steps x (temp + final)
		svc := NewWorkflowStepService(logger.NewNoOp(), repo)

		got, err := svc.Sync(context.Background(), eventID, []SyncWorkflowStepInput{
			{ID: &idA, Name: "A", OrderIndex: 1},
			{ID: &idB, Name: "B", OrderIndex: 0},
		})
		if err != nil {
			t.Fatalf("Sync() error = %v", err)
		}
		if got[0].OrderIndex != 1 || got[1].OrderIndex != 0 {
			t.Errorf("got orders = [%d, %d], want [1, 0]", got[0].OrderIndex, got[1].OrderIndex)
		}
	})
}
