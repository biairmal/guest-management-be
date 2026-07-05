package events

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/biairmal/go-sdk/errorz"
	"github.com/biairmal/go-sdk/logger"
	mockrepository "github.com/biairmal/go-sdk/mocks/repository"
	"github.com/biairmal/go-sdk/repository"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/biairmal/guest-management-be/internal/core/query"
)

// assertErrorzCode fails unless err carries the wanted errorz code (or is nil when want == "").
func assertErrorzCode(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	var e *errorz.Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *errorz.Error, got %T: %v", err, err)
	}
	if e.Code != want {
		t.Errorf("code = %q, want %q", e.Code, want)
	}
}

func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }

func ptrString(s string) *string { return &s }

func TestCategoryService_Create(t *testing.T) {
	tests := []struct {
		name    string
		in      CreateInput
		expects bool // whether repo.Create is reached (invariant failures short-circuit)
		repoErr error
		wantErr string
	}{
		{
			name:    "app source rejects tenant_id",
			in:      CreateInput{Source: SourceApp, TenantID: ptrUUID(uuid.New()), Name: "x"},
			wantErr: errorz.CodeBadRequest,
		},
		{
			name:    "tenant source requires tenant_id",
			in:      CreateInput{Source: SourceTenant, Name: "x"},
			wantErr: errorz.CodeBadRequest,
		},
		{
			name:    "already exists maps to 409",
			in:      CreateInput{Source: SourceApp, Name: "x"},
			expects: true,
			repoErr: repository.ErrAlreadyExists,
			wantErr: errorz.CodeConflict,
		},
		{
			name:    "invalid entity maps to 422",
			in:      CreateInput{Source: SourceApp, Name: "x"},
			expects: true,
			repoErr: repository.ErrInvalidEntity,
			wantErr: errorz.CodeUnprocessableEntity,
		},
		{
			name:    "unexpected repo error maps to 500",
			in:      CreateInput{Source: SourceApp, Name: "x"},
			expects: true,
			repoErr: errors.New("boom"),
			wantErr: errorz.CodeInternal,
		},
		{
			name:    "happy path",
			in:      CreateInput{Source: SourceTenant, TenantID: ptrUUID(uuid.New()), Name: "x"},
			expects: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[EventCategory, uuid.UUID](ctrl)
			if tt.expects {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tt.repoErr)
			}

			svc := NewCategoryService(logger.NewNoOp(), repo)
			got, err := svc.Create(context.Background(), tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && got == nil {
				t.Fatal("expected non-nil entity on success")
			}
		})
	}
}

func TestCategoryService_GetByID(t *testing.T) {
	tests := []struct {
		name    string
		repoRes *EventCategory
		repoErr error
		wantErr string
	}{
		{name: "found", repoRes: &EventCategory{Name: "x"}},
		{name: "not found maps to 404", repoErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound},
		{name: "unexpected error maps to 500", repoErr: errors.New("boom"), wantErr: errorz.CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[EventCategory, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.repoRes, tt.repoErr)

			svc := NewCategoryService(logger.NewNoOp(), repo)
			_, err := svc.GetByID(context.Background(), uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestCategoryService_Update(t *testing.T) {
	tests := []struct {
		name       string
		in         UpdateInput
		getRes     *EventCategory
		getErr     error
		expectsGet bool
		updateErr  error
		expectsSet bool
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
			name:       "update not found maps to 404",
			in:         UpdateInput{Name: ptrString("y")},
			expectsGet: true,
			getRes:     &EventCategory{Name: "x"},
			expectsSet: true,
			updateErr:  repository.ErrNotFound,
			wantErr:    errorz.CodeNotFound,
		},
		{
			name:       "happy path partial update",
			in:         UpdateInput{Name: ptrString("y")},
			expectsGet: true,
			getRes:     &EventCategory{Name: "x"},
			expectsSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[EventCategory, uuid.UUID](ctrl)
			if tt.expectsGet {
				repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.getRes, tt.getErr)
			}
			if tt.expectsSet {
				repo.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.updateErr)
			}

			svc := NewCategoryService(logger.NewNoOp(), repo)
			got, err := svc.Update(context.Background(), uuid.New(), tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && got.Name != "y" {
				t.Errorf("Name = %q, want %q", got.Name, "y")
			}
		})
	}
}

func TestCategoryService_Delete(t *testing.T) {
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
			repo := mockrepository.NewMockRepository[EventCategory, uuid.UUID](ctrl)
			repo.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(tt.repoErr)

			svc := NewCategoryService(logger.NewNoOp(), repo)
			err := svc.Delete(context.Background(), uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestCategoryService_List(t *testing.T) {
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
			repo := mockrepository.NewMockRepository[EventCategory, uuid.UUID](ctrl)
			repo.EXPECT().
				List(gomock.Any(), gomock.Any()).
				Return([]*EventCategory{{Name: "x"}}, int64(1), tt.repoErr)

			svc := NewCategoryService(logger.NewNoOp(), repo)
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
