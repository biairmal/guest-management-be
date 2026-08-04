package tenants

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

func ptrString(s string) *string { return &s }

func TestTenantService_Create(t *testing.T) {
	tests := []struct {
		name    string
		in      CreateInput
		repoErr error
		wantErr string
	}{
		{
			name: "already exists maps to 409", in: CreateInput{Name: "Acme"},
			repoErr: repository.ErrAlreadyExists, wantErr: errorz.CodeConflict,
		},
		{
			name: "invalid entity maps to 422", in: CreateInput{Name: "Acme"},
			repoErr: repository.ErrInvalidEntity, wantErr: errorz.CodeUnprocessableEntity,
		},
		{
			name: "unexpected repo error maps to 500", in: CreateInput{Name: "Acme"},
			repoErr: errors.New("boom"), wantErr: errorz.CodeInternal,
		},
		{name: "happy path", in: CreateInput{Name: "Acme", Type: ptrString("enterprise")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[Tenant, uuid.UUID](ctrl)
			repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tt.repoErr)

			svc := NewTenantService(logger.NewNoOp(), repo)
			got, err := svc.Create(context.Background(), tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" {
				if got == nil {
					t.Fatal("expected non-nil entity on success")
				}
				if string(got.Settings) != "{}" || string(got.Branding) != "{}" {
					t.Errorf("Settings/Branding = %q/%q, want default {} when omitted", got.Settings, got.Branding)
				}
			}
		})
	}
}

func TestTenantService_GetByID(t *testing.T) {
	tests := []struct {
		name    string
		repoRes *Tenant
		repoErr error
		wantErr string
	}{
		{name: "found", repoRes: &Tenant{Name: "Acme"}},
		{name: "not found maps to 404", repoErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound},
		{name: "unexpected error maps to 500", repoErr: errors.New("boom"), wantErr: errorz.CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[Tenant, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.repoRes, tt.repoErr)

			svc := NewTenantService(logger.NewNoOp(), repo)
			_, err := svc.GetByID(context.Background(), uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestTenantService_Update(t *testing.T) {
	tests := []struct {
		name       string
		in         UpdateInput
		getRes     *Tenant
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
			name:    "get unexpected error maps to 500",
			getErr:  errors.New("boom"),
			wantErr: errorz.CodeInternal,
		},
		{
			name:       "update not found maps to 404",
			in:         UpdateInput{Name: ptrString("New Name")},
			getRes:     &Tenant{Name: "Acme"},
			expectsSet: true,
			updateErr:  repository.ErrNotFound,
			wantErr:    errorz.CodeNotFound,
		},
		{
			name:       "happy path partial update",
			in:         UpdateInput{Name: ptrString("New Name")},
			getRes:     &Tenant{Name: "Acme"},
			expectsSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[Tenant, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.getRes, tt.getErr)
			if tt.expectsSet {
				repo.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.updateErr)
			}

			svc := NewTenantService(logger.NewNoOp(), repo)
			got, err := svc.Update(context.Background(), uuid.New(), tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && got.Name != "New Name" {
				t.Errorf("Name = %q, want %q", got.Name, "New Name")
			}
		})
	}
}

func TestTenantService_Delete(t *testing.T) {
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
			repo := mockrepository.NewMockRepository[Tenant, uuid.UUID](ctrl)
			repo.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(tt.repoErr)

			svc := NewTenantService(logger.NewNoOp(), repo)
			err := svc.Delete(context.Background(), uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestTenantService_List(t *testing.T) {
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
			repo := mockrepository.NewMockRepository[Tenant, uuid.UUID](ctrl)
			repo.EXPECT().
				List(gomock.Any(), gomock.Any()).
				Return([]*Tenant{{Name: "Acme"}}, int64(1), tt.repoErr)

			svc := NewTenantService(logger.NewNoOp(), repo)
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
