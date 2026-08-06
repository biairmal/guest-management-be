package users

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
	"golang.org/x/crypto/bcrypt"

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

func TestUserService_Create(t *testing.T) {
	tests := []struct {
		name    string
		in      CreateInput
		repoErr error
		wantErr string
	}{
		{
			name: "already exists maps to 409", in: CreateInput{Email: "a@acme.com", Password: "password1"},
			repoErr: repository.ErrAlreadyExists, wantErr: errorz.CodeConflict,
		},
		{
			name: "invalid entity maps to 422", in: CreateInput{Email: "a@acme.com", Password: "password1"},
			repoErr: repository.ErrInvalidEntity, wantErr: errorz.CodeUnprocessableEntity,
		},
		{
			name: "unexpected repo error maps to 500", in: CreateInput{Email: "a@acme.com", Password: "password1"},
			repoErr: errors.New("boom"), wantErr: errorz.CodeInternal,
		},
		{name: "happy path", in: CreateInput{Email: "a@acme.com", Password: "password1", IsTenantMaster: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[User, uuid.UUID](ctrl)
			repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tt.repoErr)

			svc := NewUserService(logger.NewNoOp(), repo)
			got, err := svc.Create(context.Background(), tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" {
				if got == nil {
					t.Fatal("expected non-nil entity on success")
				}
				if got.PasswordHash == tt.in.Password {
					t.Error("PasswordHash must not equal the plaintext password")
				}
				if err := bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte(tt.in.Password)); err != nil {
					t.Errorf("stored hash does not match input password: %v", err)
				}
			}
		})
	}
}

func TestUserService_GetByID(t *testing.T) {
	tests := []struct {
		name    string
		repoRes *User
		repoErr error
		wantErr string
	}{
		{name: "found", repoRes: &User{Email: "a@acme.com"}},
		{name: "not found maps to 404", repoErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound},
		{name: "unexpected error maps to 500", repoErr: errors.New("boom"), wantErr: errorz.CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[User, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.repoRes, tt.repoErr)

			svc := NewUserService(logger.NewNoOp(), repo)
			_, err := svc.GetByID(context.Background(), uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestUserService_GetByEmail(t *testing.T) {
	tests := []struct {
		name    string
		repoRes []*User
		repoErr error
		wantErr string
	}{
		{name: "found", repoRes: []*User{{Email: "a@acme.com"}}},
		{name: "no match maps to 404", repoRes: nil, wantErr: errorz.CodeNotFound},
		{name: "unexpected error maps to 500", repoErr: errors.New("boom"), wantErr: errorz.CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[User, uuid.UUID](ctrl)
			repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(tt.repoRes, int64(len(tt.repoRes)), tt.repoErr)

			svc := NewUserService(logger.NewNoOp(), repo)
			got, err := svc.GetByEmail(context.Background(), "a@acme.com")
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && got.Email != "a@acme.com" {
				t.Errorf("Email = %q, want %q", got.Email, "a@acme.com")
			}
		})
	}
}

func TestUserService_Update(t *testing.T) {
	tests := []struct {
		name       string
		in         UpdateInput
		getRes     *User
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
			in:         UpdateInput{Email: ptrString("new@acme.com")},
			getRes:     &User{Email: "a@acme.com"},
			expectsSet: true,
			updateErr:  repository.ErrNotFound,
			wantErr:    errorz.CodeNotFound,
		},
		{
			name:       "update already exists maps to 409",
			in:         UpdateInput{Email: ptrString("new@acme.com")},
			getRes:     &User{Email: "a@acme.com"},
			expectsSet: true,
			updateErr:  repository.ErrAlreadyExists,
			wantErr:    errorz.CodeConflict,
		},
		{
			name:       "happy path partial update re-hashes password",
			in:         UpdateInput{Email: ptrString("new@acme.com"), Password: ptrString("newpassword1")},
			getRes:     &User{Email: "a@acme.com", PasswordHash: "old-hash"},
			expectsSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[User, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.getRes, tt.getErr)
			if tt.expectsSet {
				repo.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.updateErr)
			}

			svc := NewUserService(logger.NewNoOp(), repo)
			got, err := svc.Update(context.Background(), uuid.New(), tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" {
				if got.Email != "new@acme.com" {
					t.Errorf("Email = %q, want %q", got.Email, "new@acme.com")
				}
				if tt.in.Password != nil {
					if got.PasswordHash == "old-hash" {
						t.Error("PasswordHash was not re-hashed")
					}
					if err := bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte(*tt.in.Password)); err != nil {
						t.Errorf("stored hash does not match new password: %v", err)
					}
				}
			}
		})
	}
}

func TestUserService_Delete(t *testing.T) {
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
			repo := mockrepository.NewMockRepository[User, uuid.UUID](ctrl)
			repo.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(tt.repoErr)

			svc := NewUserService(logger.NewNoOp(), repo)
			err := svc.Delete(context.Background(), uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestUserService_List(t *testing.T) {
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
			repo := mockrepository.NewMockRepository[User, uuid.UUID](ctrl)
			repo.EXPECT().
				List(gomock.Any(), gomock.Any()).
				Return([]*User{{Email: "a@acme.com"}}, int64(1), tt.repoErr)

			svc := NewUserService(logger.NewNoOp(), repo)
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
