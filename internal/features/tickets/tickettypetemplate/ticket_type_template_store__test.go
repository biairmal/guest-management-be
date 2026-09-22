package tickettypetemplate

import (
	"context"
	"errors"
	"testing"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	mockrepository "github.com/biairmal/go-sdk/mocks/repository"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/biairmal/guest-management-be/internal/features/events/category"
	mocktickettypetemplate "github.com/biairmal/guest-management-be/mocks/tickets/tickettypetemplate"
)

func TestStore_ReplaceForCategory(t *testing.T) {
	categoryID := uuid.New()
	oldID := uuid.New()
	stepIDs := []uuid.UUID{uuid.New()}
	drafts := []category.TicketTypeTemplateDraft{{Name: "VIP", WorkflowStepTemplateIDs: stepIDs}}

	t.Run("retires live rows, writes drafts at the version with default rules and step links", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketTypeTemplate, uuid.UUID](ctrl)
		stepRepo := mocktickettypetemplate.NewMockWorkflowStepRepository(ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*TicketTypeTemplate{{ID: oldID}}, int64(1), nil)
		repo.EXPECT().Delete(gomock.Any(), oldID).Return(nil)
		var created *TicketTypeTemplate
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, e *TicketTypeTemplate) error { created = e; return nil })
		stepRepo.EXPECT().Insert(gomock.Any(), gomock.Any(), stepIDs).Return(nil)

		store := NewStore(logger.NewNoOp(), repo, stepRepo)
		views, err := store.ReplaceForCategory(context.Background(), categoryID, 7, drafts)
		if err != nil {
			t.Fatalf("ReplaceForCategory() error = %v", err)
		}
		if created.Version != 7 || created.CategoryID != categoryID || string(created.Rules) != "{}" {
			t.Errorf("created = %+v, want version 7, category, rules {}", created)
		}
		if len(views) != 1 || views[0].ID != created.ID || len(views[0].WorkflowStepTemplateIDs) != 1 {
			t.Errorf("views = %+v", views)
		}
	})

	t.Run("a step link failure maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[TicketTypeTemplate, uuid.UUID](ctrl)
		stepRepo := mocktickettypetemplate.NewMockWorkflowStepRepository(ctrl)
		repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		stepRepo.EXPECT().Insert(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("boom"))

		_, err := NewStore(logger.NewNoOp(), repo, stepRepo).ReplaceForCategory(context.Background(), categoryID, 7, drafts)
		var e *errorz.Error
		if !errors.As(err, &e) || e.Code != errorz.CodeInternal {
			t.Errorf("err = %v, want internal", err)
		}
	})
}

func TestStore_ListForCategory(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mockrepository.NewMockRepository[TicketTypeTemplate, uuid.UUID](ctrl)
	stepRepo := mocktickettypetemplate.NewMockWorkflowStepRepository(ctrl)
	vip, crew := uuid.New(), uuid.New()
	step := uuid.New()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).
		Return([]*TicketTypeTemplate{{ID: vip, Name: "VIP"}, {ID: crew, Name: "Crew"}}, int64(2), nil)
	stepRepo.EXPECT().WorkflowStepTemplateIDs(gomock.Any(), []uuid.UUID{vip, crew}).
		Return(map[uuid.UUID][]uuid.UUID{vip: {step}}, nil)

	views, err := NewStore(logger.NewNoOp(), repo, stepRepo).ListForCategory(context.Background(), uuid.New(), 2)
	if err != nil {
		t.Fatalf("ListForCategory() error = %v", err)
	}
	if len(views) != 2 || len(views[0].WorkflowStepTemplateIDs) != 1 || len(views[1].WorkflowStepTemplateIDs) != 0 {
		t.Errorf("views = %+v", views)
	}
}
