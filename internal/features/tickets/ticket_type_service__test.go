package tickets

import (
	"context"
	"encoding/json"
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
	"github.com/biairmal/guest-management-be/internal/features/events/workflowstep"
	mocktickets "github.com/biairmal/guest-management-be/mocks/tickets"
)

func newTestService(
	repo repository.Repository[TicketType, uuid.UUID],
	junctionRepo TicketTypeWorkflowStepRepository,
	workflowStepRepo repository.Repository[workflowstep.WorkflowStep, uuid.UUID],
) TicketTypeService {
	return NewTicketTypeService(logger.NewNoOp(), repo, junctionRepo, workflowStepRepo)
}

func TestTicketTypeService_Create(t *testing.T) {
	eventID := uuid.New()

	t.Run("name conflict maps to 409", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrAlreadyExists)
		svc := newTestService(repo, junctionRepo, workflowStepRepo)

		_, err := svc.Create(context.Background(), eventID, CreateTicketTypeInput{Name: "VIP"})
		assertErrorzCode(t, err, errorz.CodeConflict)
	})

	t.Run("invalid entity maps to 422", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrInvalidEntity)
		svc := newTestService(repo, junctionRepo, workflowStepRepo)

		_, err := svc.Create(context.Background(), eventID, CreateTicketTypeInput{Name: "VIP"})
		assertErrorzCode(t, err, errorz.CodeUnprocessableEntity)
	})

	t.Run("unexpected create error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("boom"))
		svc := newTestService(repo, junctionRepo, workflowStepRepo)

		_, err := svc.Create(context.Background(), eventID, CreateTicketTypeInput{Name: "VIP"})
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("happy path defaults rules to empty object and seeds all event workflow steps", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)

		stepA, stepB := uuid.New(), uuid.New()
		var createdRules json.RawMessage
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *TicketType) error {
			createdRules = e.Rules
			return nil
		})
		workflowStepRepo.EXPECT().List(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, opts *repository.ListOptions) ([]*workflowstep.WorkflowStep, int64, error) {
				found := false
				for _, c := range opts.Filter.Conditions {
					if c.Field == "event_id" && c.Value == eventID {
						found = true
					}
				}
				if !found {
					t.Error("expected event_id filter condition to be present and match eventID")
				}
				return []*workflowstep.WorkflowStep{{ID: stepA}, {ID: stepB}}, int64(2), nil
			})
		junctionRepo.EXPECT().SetWorkflowStepIDs(gomock.Any(), gomock.Any(), []uuid.UUID{stepA, stepB}).Return(nil)
		svc := newTestService(repo, junctionRepo, workflowStepRepo)

		got, err := svc.Create(context.Background(), eventID, CreateTicketTypeInput{Name: "VIP"})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if string(createdRules) != "{}" {
			t.Errorf("Rules = %s, want {}", createdRules)
		}
		if len(got.WorkflowStepIDs) != 2 || got.WorkflowStepIDs[0] != stepA || got.WorkflowStepIDs[1] != stepB {
			t.Errorf("WorkflowStepIDs = %v, want [%v %v]", got.WorkflowStepIDs, stepA, stepB)
		}
	})

	t.Run("default workflow step lookup failure is best-effort and does not fail create", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		workflowStepRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("boom"))
		svc := newTestService(repo, junctionRepo, workflowStepRepo)

		got, err := svc.Create(context.Background(), eventID, CreateTicketTypeInput{Name: "VIP"})
		if err != nil {
			t.Fatalf("Create() error = %v, want create to succeed despite seeding failure", err)
		}
		if len(got.WorkflowStepIDs) != 0 {
			t.Errorf("WorkflowStepIDs = %v, want empty", got.WorkflowStepIDs)
		}
	})

	t.Run("default workflow step assign failure is best-effort and does not fail create", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		workflowStepRepo.EXPECT().List(gomock.Any(), gomock.Any()).
			Return([]*workflowstep.WorkflowStep{{ID: uuid.New()}}, int64(1), nil)
		junctionRepo.EXPECT().SetWorkflowStepIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("boom"))
		svc := newTestService(repo, junctionRepo, workflowStepRepo)

		got, err := svc.Create(context.Background(), eventID, CreateTicketTypeInput{Name: "VIP"})
		if err != nil {
			t.Fatalf("Create() error = %v, want create to succeed despite seeding failure", err)
		}
		if len(got.WorkflowStepIDs) != 0 {
			t.Errorf("WorkflowStepIDs = %v, want empty", got.WorkflowStepIDs)
		}
	})
}

func TestTicketTypeService_GetByID(t *testing.T) {
	eventID := uuid.New()
	id := uuid.New()

	t.Run("not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(nil, repository.ErrNotFound)
		svc := newTestService(repo, nil, nil)

		_, err := svc.GetByID(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("found for different event maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: uuid.New()}, nil)
		svc := newTestService(repo, nil, nil)

		_, err := svc.GetByID(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("unexpected get error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(nil, errors.New("boom"))
		svc := newTestService(repo, nil, nil)

		_, err := svc.GetByID(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("junction lookup failure maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: eventID}, nil)
		junctionRepo.EXPECT().WorkflowStepIDsByTicketTypeID(gomock.Any(), id).Return(nil, errors.New("boom"))
		svc := newTestService(repo, junctionRepo, nil)

		_, err := svc.GetByID(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("happy path populates workflow_step_ids", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		stepID := uuid.New()
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: eventID}, nil)
		junctionRepo.EXPECT().WorkflowStepIDsByTicketTypeID(gomock.Any(), id).Return([]uuid.UUID{stepID}, nil)
		svc := newTestService(repo, junctionRepo, nil)

		got, err := svc.GetByID(context.Background(), eventID, id)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if len(got.WorkflowStepIDs) != 1 || got.WorkflowStepIDs[0] != stepID {
			t.Errorf("WorkflowStepIDs = %v, want [%v]", got.WorkflowStepIDs, stepID)
		}
	})
}

func TestTicketTypeService_Update(t *testing.T) {
	eventID := uuid.New()
	id := uuid.New()

	t.Run("get not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(nil, repository.ErrNotFound)
		svc := newTestService(repo, nil, nil)

		_, err := svc.Update(context.Background(), eventID, id, UpdateTicketTypeInput{})
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("get for different event maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: uuid.New()}, nil)
		svc := newTestService(repo, nil, nil)

		_, err := svc.Update(context.Background(), eventID, id, UpdateTicketTypeInput{})
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	updateErrCases := []struct {
		name      string
		updateErr error
		wantErr   string
	}{
		{name: "update conflict maps to 409", updateErr: repository.ErrAlreadyExists, wantErr: errorz.CodeConflict},
		{name: "update not found maps to 404", updateErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound},
	}
	for _, tt := range updateErrCases {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: eventID, Name: "Regular"}, nil)
			repo.EXPECT().Update(gomock.Any(), id, gomock.Any()).Return(tt.updateErr)
			svc := newTestService(repo, nil, nil)

			_, err := svc.Update(context.Background(), eventID, id, UpdateTicketTypeInput{Name: ptrString("VIP")})
			assertErrorzCode(t, err, tt.wantErr)
		})
	}

	t.Run("happy path partial update leaves workflow_step_ids empty", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: eventID, Name: "Regular"}, nil)
		repo.EXPECT().Update(gomock.Any(), id, gomock.Any()).Return(nil)
		svc := newTestService(repo, nil, nil)

		got, err := svc.Update(context.Background(), eventID, id, UpdateTicketTypeInput{Name: ptrString("VIP")})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if got.Name != "VIP" {
			t.Errorf("Name = %q, want VIP", got.Name)
		}
		if got.WorkflowStepIDs != nil {
			t.Errorf("WorkflowStepIDs = %v, want nil", got.WorkflowStepIDs)
		}
	})
}

func TestTicketTypeService_Delete(t *testing.T) {
	eventID := uuid.New()
	id := uuid.New()

	t.Run("get not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(nil, repository.ErrNotFound)
		svc := newTestService(repo, nil, nil)

		err := svc.Delete(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("get for different event maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: uuid.New()}, nil)
		svc := newTestService(repo, nil, nil)

		err := svc.Delete(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("happy path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: eventID}, nil)
		repo.EXPECT().Delete(gomock.Any(), id).Return(nil)
		svc := newTestService(repo, nil, nil)

		if err := svc.Delete(context.Background(), eventID, id); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
	})
}

func TestTicketTypeService_List(t *testing.T) {
	eventID := uuid.New()

	t.Run("repo error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("boom"))
		svc := newTestService(repo, nil, nil)

		params, err := query.ParseListParams(url.Values{}, query.ListParseConfig{})
		if err != nil {
			t.Fatalf("ParseListParams() error = %v", err)
		}
		_, err = svc.List(context.Background(), eventID, params)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("always injects event_id filter regardless of query params", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, opts *repository.ListOptions) ([]*TicketType, int64, error) {
				found := false
				for _, c := range opts.Filter.Conditions {
					if c.Field == "event_id" && c.Value == eventID {
						found = true
					}
				}
				if !found {
					t.Error("expected event_id filter condition to be present and match eventID")
				}
				return []*TicketType{{ID: uuid.New(), EventID: eventID}}, int64(1), nil
			})
		svc := newTestService(repo, nil, nil)

		params, err := query.ParseListParams(url.Values{}, query.ListParseConfig{})
		if err != nil {
			t.Fatalf("ParseListParams() error = %v", err)
		}
		got, err := svc.List(context.Background(), eventID, params)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if got.Size != 20 {
			t.Errorf("Size = %d, want 20", got.Size)
		}
	})
}

func TestTicketTypeService_ReplaceWorkflowSteps(t *testing.T) {
	eventID := uuid.New()
	id := uuid.New()

	t.Run("ticket type not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(nil, repository.ErrNotFound)
		svc := newTestService(repo, nil, nil)

		_, err := svc.ReplaceWorkflowSteps(context.Background(), eventID, id, []uuid.UUID{uuid.New()})
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("workflow step not found maps to 400", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		stepID := uuid.New()
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: eventID}, nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).Return(nil, repository.ErrNotFound)
		svc := newTestService(repo, nil, workflowStepRepo)

		_, err := svc.ReplaceWorkflowSteps(context.Background(), eventID, id, []uuid.UUID{stepID})
		assertErrorzCode(t, err, errorz.CodeBadRequest)
	})

	t.Run("workflow step from a different event maps to 400", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		stepID := uuid.New()
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: eventID}, nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).
			Return(&workflowstep.WorkflowStep{ID: stepID, EventID: uuid.New()}, nil)
		svc := newTestService(repo, nil, workflowStepRepo)

		_, err := svc.ReplaceWorkflowSteps(context.Background(), eventID, id, []uuid.UUID{stepID})
		assertErrorzCode(t, err, errorz.CodeBadRequest)
	})

	t.Run("workflow step lookup failure maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		stepID := uuid.New()
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: eventID}, nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).Return(nil, errors.New("boom"))
		svc := newTestService(repo, nil, workflowStepRepo)

		_, err := svc.ReplaceWorkflowSteps(context.Background(), eventID, id, []uuid.UUID{stepID})
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("set failure maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		stepID := uuid.New()
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: eventID}, nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).
			Return(&workflowstep.WorkflowStep{ID: stepID, EventID: eventID}, nil)
		junctionRepo.EXPECT().SetWorkflowStepIDs(gomock.Any(), id, []uuid.UUID{stepID}).Return(errors.New("boom"))
		svc := newTestService(repo, junctionRepo, workflowStepRepo)

		_, err := svc.ReplaceWorkflowSteps(context.Background(), eventID, id, []uuid.UUID{stepID})
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("happy path replaces and returns updated workflow_step_ids", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		workflowStepRepo := mockrepository.NewMockRepository[workflowstep.WorkflowStep, uuid.UUID](ctrl)
		stepID := uuid.New()
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: eventID}, nil)
		workflowStepRepo.EXPECT().GetByID(gomock.Any(), stepID).
			Return(&workflowstep.WorkflowStep{ID: stepID, EventID: eventID}, nil)
		junctionRepo.EXPECT().SetWorkflowStepIDs(gomock.Any(), id, []uuid.UUID{stepID}).Return(nil)
		svc := newTestService(repo, junctionRepo, workflowStepRepo)

		got, err := svc.ReplaceWorkflowSteps(context.Background(), eventID, id, []uuid.UUID{stepID})
		if err != nil {
			t.Fatalf("ReplaceWorkflowSteps() error = %v", err)
		}
		if len(got.WorkflowStepIDs) != 1 || got.WorkflowStepIDs[0] != stepID {
			t.Errorf("WorkflowStepIDs = %v, want [%v]", got.WorkflowStepIDs, stepID)
		}
	})

	t.Run("empty set clears applicability", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		junctionRepo := mocktickets.NewMockTicketTypeWorkflowStepRepository(ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: eventID}, nil)
		junctionRepo.EXPECT().SetWorkflowStepIDs(gomock.Any(), id, []uuid.UUID{}).Return(nil)
		svc := newTestService(repo, junctionRepo, nil)

		got, err := svc.ReplaceWorkflowSteps(context.Background(), eventID, id, []uuid.UUID{})
		if err != nil {
			t.Fatalf("ReplaceWorkflowSteps() error = %v", err)
		}
		if len(got.WorkflowStepIDs) != 0 {
			t.Errorf("WorkflowStepIDs = %v, want empty", got.WorkflowStepIDs)
		}
	})
}
