package tickettypetemplate

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
)

func TestTicketTypeTemplateService_Create(t *testing.T) {
	tests := []struct {
		name    string
		in      CreateTicketTypeTemplateInput
		repoErr error
		wantErr string
	}{
		{
			name: "already exists maps to 409", in: CreateTicketTypeTemplateInput{Name: "Regular"},
			repoErr: repository.ErrAlreadyExists, wantErr: errorz.CodeConflict,
		},
		{
			name: "invalid entity maps to 422", in: CreateTicketTypeTemplateInput{Name: "Regular"},
			repoErr: repository.ErrInvalidEntity, wantErr: errorz.CodeUnprocessableEntity,
		},
		{
			name: "unexpected repo error maps to 500", in: CreateTicketTypeTemplateInput{Name: "Regular"},
			repoErr: errors.New("boom"), wantErr: errorz.CodeInternal,
		},
		{name: "happy path", in: CreateTicketTypeTemplateInput{Name: "Regular"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[TicketTypeTemplate, uuid.UUID](ctrl)
			var createdRules json.RawMessage
			repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *TicketTypeTemplate) error {
				createdRules = e.Rules
				return tt.repoErr
			})

			svc := NewService(logger.NewNoOp(), repo)
			got, err := svc.Create(context.Background(), uuid.New(), tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" {
				if got == nil {
					t.Fatal("expected non-nil entity on success")
				}
				if string(createdRules) != "{}" {
					t.Errorf("Rules = %s, want {}", createdRules)
				}
			}
		})
	}
}

func TestTicketTypeTemplateService_GetByID(t *testing.T) {
	categoryID := uuid.New()
	tests := []struct {
		name    string
		repoRes *TicketTypeTemplate
		repoErr error
		wantErr string
	}{
		{name: "found for matching category", repoRes: &TicketTypeTemplate{CategoryID: categoryID, Name: "x"}},
		{
			name:    "found for different category maps to 404",
			repoRes: &TicketTypeTemplate{CategoryID: uuid.New(), Name: "x"},
			wantErr: errorz.CodeNotFound,
		},
		{name: "not found maps to 404", repoErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound},
		{name: "unexpected error maps to 500", repoErr: errors.New("boom"), wantErr: errorz.CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[TicketTypeTemplate, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.repoRes, tt.repoErr)

			svc := NewService(logger.NewNoOp(), repo)
			_, err := svc.GetByID(context.Background(), categoryID, uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestTicketTypeTemplateService_Update(t *testing.T) {
	categoryID := uuid.New()
	tests := []struct {
		name       string
		in         UpdateTicketTypeTemplateInput
		getRes     *TicketTypeTemplate
		getErr     error
		expectsSet bool
		updateErr  error
		wantErr    string
	}{
		{name: "get not found maps to 404", getErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound},
		{
			name:    "get for different category maps to 404",
			getRes:  &TicketTypeTemplate{CategoryID: uuid.New(), Name: "x"},
			wantErr: errorz.CodeNotFound,
		},
		{
			name: "update conflict maps to 409", in: UpdateTicketTypeTemplateInput{Name: ptrString("y")},
			getRes: &TicketTypeTemplate{CategoryID: categoryID, Name: "x"}, expectsSet: true,
			updateErr: repository.ErrAlreadyExists, wantErr: errorz.CodeConflict,
		},
		{
			name: "happy path partial update", in: UpdateTicketTypeTemplateInput{Name: ptrString("y")},
			getRes: &TicketTypeTemplate{CategoryID: categoryID, Name: "x"}, expectsSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[TicketTypeTemplate, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.getRes, tt.getErr)
			if tt.expectsSet {
				repo.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.updateErr)
			}

			svc := NewService(logger.NewNoOp(), repo)
			got, err := svc.Update(context.Background(), categoryID, uuid.New(), tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" {
				if got == nil {
					t.Fatal("expected non-nil entity on success")
				}
				if got.Name != "y" {
					t.Errorf("Name = %q, want y", got.Name)
				}
			}
		})
	}
}

func TestTicketTypeTemplateService_Delete(t *testing.T) {
	categoryID := uuid.New()
	tests := []struct {
		name       string
		getRes     *TicketTypeTemplate
		getErr     error
		expectsDel bool
		delErr     error
		wantErr    string
	}{
		{name: "get not found maps to 404", getErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound},
		{
			name:    "get for different category maps to 404",
			getRes:  &TicketTypeTemplate{CategoryID: uuid.New(), Name: "x"},
			wantErr: errorz.CodeNotFound,
		},
		{
			name: "delete not found maps to 404", getRes: &TicketTypeTemplate{CategoryID: categoryID, Name: "x"},
			expectsDel: true, delErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound,
		},
		{name: "happy path", getRes: &TicketTypeTemplate{CategoryID: categoryID, Name: "x"}, expectsDel: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[TicketTypeTemplate, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.getRes, tt.getErr)
			if tt.expectsDel {
				repo.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(tt.delErr)
			}

			svc := NewService(logger.NewNoOp(), repo)
			err := svc.Delete(context.Background(), categoryID, uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestTicketTypeTemplateService_List(t *testing.T) {
	categoryID := uuid.New()
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
			repo := mockrepository.NewMockRepository[TicketTypeTemplate, uuid.UUID](ctrl)
			repo.EXPECT().List(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, opts *repository.ListOptions) ([]*TicketTypeTemplate, int64, error) {
					found := false
					for _, c := range opts.Filter.Conditions {
						if c.Field == "category_id" && c.Value == categoryID {
							found = true
						}
					}
					if !found {
						t.Error("expected category_id filter condition to be present and match categoryID")
					}
					return []*TicketTypeTemplate{{Name: "x"}}, 1, tt.repoErr
				})

			svc := NewService(logger.NewNoOp(), repo)
			params, err := query.ParseListParams(url.Values{}, query.ListParseConfig{})
			if err != nil {
				t.Fatalf("ParseListParams() error = %v", err)
			}
			got, err := svc.List(context.Background(), categoryID, params)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && got.Size != tt.wantSize {
				t.Errorf("Size = %d, want %d", got.Size, tt.wantSize)
			}
		})
	}
}
