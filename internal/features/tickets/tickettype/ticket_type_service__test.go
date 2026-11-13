package tickettype

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
	"github.com/biairmal/guest-management-be/internal/features/tickets/tickettypetemplate"
	mocktickettype "github.com/biairmal/guest-management-be/mocks/tickets/tickettype"
	mocktickettypetemplate "github.com/biairmal/guest-management-be/mocks/tickets/tickettypetemplate"
)

func newTestService(
	repo repository.Repository[TicketType, uuid.UUID],
	junctionRepo TicketTypeWorkflowStepRepository,
) TicketTypeService {
	return NewTicketTypeService(logger.NewNoOp(), repo, junctionRepo, nil, nil)
}

func TestTicketTypeService_GetByID(t *testing.T) {
	eventID := uuid.New()
	id := uuid.New()

	t.Run("not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(nil, repository.ErrNotFound)
		svc := newTestService(repo, nil)

		_, err := svc.GetByID(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("found for different event maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: uuid.New()}, nil)
		svc := newTestService(repo, nil)

		_, err := svc.GetByID(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("unexpected get error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(nil, errors.New("boom"))
		svc := newTestService(repo, nil)

		_, err := svc.GetByID(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("junction lookup failure maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		junctionRepo := mocktickettype.NewMockTicketTypeWorkflowStepRepository(ctrl)
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: eventID}, nil)
		junctionRepo.EXPECT().WorkflowStepIDsByTicketTypeID(gomock.Any(), id).Return(nil, errors.New("boom"))
		svc := newTestService(repo, junctionRepo)

		_, err := svc.GetByID(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("happy path populates workflow_step_ids", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		junctionRepo := mocktickettype.NewMockTicketTypeWorkflowStepRepository(ctrl)
		stepID := uuid.New()
		repo.EXPECT().GetByID(gomock.Any(), id).Return(&TicketType{ID: id, EventID: eventID}, nil)
		junctionRepo.EXPECT().WorkflowStepIDsByTicketTypeID(gomock.Any(), id).Return([]uuid.UUID{stepID}, nil)
		svc := newTestService(repo, junctionRepo)

		got, err := svc.GetByID(context.Background(), eventID, id)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if len(got.WorkflowStepIDs) != 1 || got.WorkflowStepIDs[0] != stepID {
			t.Errorf("WorkflowStepIDs = %v, want [%v]", got.WorkflowStepIDs, stepID)
		}
	})
}

func TestTicketTypeService_List(t *testing.T) {
	eventID := uuid.New()

	t.Run("repo error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("boom"))
		svc := newTestService(repo, nil)

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
		svc := newTestService(repo, nil)

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

func TestTicketTypeService_SeedFromCategoryTemplates(t *testing.T) {
	eventID := uuid.New()
	categoryID := uuid.New()
	stepTmplA, stepTmplB := uuid.New(), uuid.New()
	eventStepA, eventStepB := uuid.New(), uuid.New()
	stepIDs := map[uuid.UUID]uuid.UUID{stepTmplA: eventStepA, stepTmplB: eventStepB}
	vip := &tickettypetemplate.TicketTypeTemplate{ID: uuid.New(), Name: "VIP", Rules: json.RawMessage(`{"max":10}`)}
	crew := &tickettypetemplate.TicketTypeTemplate{ID: uuid.New(), Name: "Crew", Rules: emptyJSONObject}

	type mocks struct {
		repo         *mockrepository.MockRepository[TicketType, uuid.UUID]
		junctionRepo *mocktickettype.MockTicketTypeWorkflowStepRepository
		templateRepo *mockrepository.MockRepository[tickettypetemplate.TicketTypeTemplate, uuid.UUID]
		stepRepo     *mocktickettypetemplate.MockWorkflowStepRepository
	}
	listTemplates := func(m mocks) {
		m.templateRepo.EXPECT().List(gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ context.Context, opts *repository.ListOptions,
			) ([]*tickettypetemplate.TicketTypeTemplate, int64, error) {
				want := map[string]any{"category_id": categoryID, "version": 3}
				for _, c := range opts.Filter.Conditions {
					if want[c.Field] == c.Value {
						delete(want, c.Field)
					}
				}
				if len(want) != 0 {
					t.Errorf("missing filter conditions %v", want)
				}
				return []*tickettypetemplate.TicketTypeTemplate{vip, crew}, 2, nil
			})
	}

	tests := []struct {
		name    string
		setup   func(m mocks)
		wantErr string
	}{
		{
			name: "template lookup failure maps to 500",
			setup: func(m mocks) {
				m.templateRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("boom"))
			},
			wantErr: errorz.CodeInternal,
		},
		{
			name: "template step lookup failure maps to 500",
			setup: func(m mocks) {
				listTemplates(m)
				m.stepRepo.EXPECT().WorkflowStepTemplateIDs(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))
			},
			wantErr: errorz.CodeInternal,
		},
		{
			name: "a create failure aborts, no more templates are attempted",
			setup: func(m mocks) {
				listTemplates(m)
				m.stepRepo.EXPECT().WorkflowStepTemplateIDs(gomock.Any(), gomock.Any()).Return(nil, nil)
				m.repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrAlreadyExists)
			},
			wantErr: errorz.CodeConflict,
		},
		{
			name: "a step link failure propagates instead of failing open",
			setup: func(m mocks) {
				listTemplates(m)
				m.stepRepo.EXPECT().WorkflowStepTemplateIDs(gomock.Any(), gomock.Any()).Return(nil, nil)
				m.repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
				m.junctionRepo.EXPECT().SetWorkflowStepIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("boom"))
			},
			wantErr: errorz.CodeInternal,
		},
		{
			name: "each ticket type gets exactly its template's steps, mapped to event steps",
			setup: func(m mocks) {
				listTemplates(m)
				m.stepRepo.EXPECT().WorkflowStepTemplateIDs(gomock.Any(), []uuid.UUID{vip.ID, crew.ID}).
					Return(map[uuid.UUID][]uuid.UUID{vip.ID: {stepTmplB, stepTmplA}}, nil)
				m.repo.EXPECT().Create(gomock.Any(), gomock.Any()).Times(2).Return(nil)
				gomock.InOrder(
					m.junctionRepo.EXPECT().SetWorkflowStepIDs(gomock.Any(), gomock.Any(), []uuid.UUID{eventStepB, eventStepA}),
					m.junctionRepo.EXPECT().SetWorkflowStepIDs(gomock.Any(), gomock.Any(), []uuid.UUID{}),
				)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := mocks{
				repo:         mockrepository.NewMockRepository[TicketType, uuid.UUID](ctrl),
				junctionRepo: mocktickettype.NewMockTicketTypeWorkflowStepRepository(ctrl),
				templateRepo: mockrepository.NewMockRepository[tickettypetemplate.TicketTypeTemplate, uuid.UUID](ctrl),
				stepRepo:     mocktickettypetemplate.NewMockWorkflowStepRepository(ctrl),
			}
			tt.setup(m)
			svc := NewTicketTypeService(logger.NewNoOp(), m.repo, m.junctionRepo, m.templateRepo, m.stepRepo)

			err := svc.SeedFromCategoryTemplates(context.Background(), eventID, categoryID, 3, stepIDs)
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}
