package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	sdkauth "github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	mockauth "github.com/biairmal/go-sdk/mocks/auth"
	mockrepository "github.com/biairmal/go-sdk/mocks/repository"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"

	"github.com/biairmal/guest-management-be/internal/features/users"
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

func hashOf(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword() error = %v", err)
	}
	return string(hash)
}

func TestAuthService_Login(t *testing.T) {
	validHash := hashOf(t, "correct-password")

	tests := []struct {
		name      string
		repoRes   []*users.User
		repoErr   error
		password  string
		issuerErr error
		wantErr   string
	}{
		{
			name: "unknown email maps to 401", repoRes: nil, password: "correct-password",
			wantErr: errorz.CodeUnauthorized,
		},
		{
			name: "repo error maps to 500", repoErr: errors.New("boom"), password: "correct-password",
			wantErr: errorz.CodeInternal,
		},
		{
			name:     "wrong password maps to 401",
			repoRes:  []*users.User{{ID: uuid.New(), TenantID: uuid.New(), PasswordHash: validHash}},
			password: "wrong-password",
			wantErr:  errorz.CodeUnauthorized,
		},
		{
			name:      "issuer error maps to 500",
			repoRes:   []*users.User{{ID: uuid.New(), TenantID: uuid.New(), PasswordHash: validHash}},
			password:  "correct-password",
			issuerErr: errors.New("sign failed"),
			wantErr:   errorz.CodeInternal,
		},
		{
			name:     "happy path",
			repoRes:  []*users.User{{ID: uuid.New(), TenantID: uuid.New(), PasswordHash: validHash}},
			password: "correct-password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[users.User, uuid.UUID](ctrl)
			repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(tt.repoRes, int64(len(tt.repoRes)), tt.repoErr)

			issuer := mockauth.NewMockIssuer(ctrl)
			if tt.repoErr == nil && len(tt.repoRes) > 0 && tt.password == "correct-password" {
				wantCalls := 2 // access + refresh
				if tt.issuerErr != nil {
					wantCalls = 1 // issueTokenPair returns as soon as the access-token Issue fails
				}
				issuer.EXPECT().Issue(gomock.Any(), gomock.Any(), gomock.Any()).
					Return("signed-token", tt.issuerErr).Times(wantCalls)
			}

			svc := NewService(logger.NewNoOp(), repo, issuer, nil, 15*time.Minute, 7*24*time.Hour)
			got, err := svc.Login(context.Background(), LoginInput{Email: "a@acme.com", Password: tt.password})
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" {
				if got.AccessToken == "" || got.RefreshToken == "" {
					t.Error("expected non-empty access and refresh tokens")
				}
				if got.TokenType != "Bearer" {
					t.Errorf("TokenType = %q, want %q", got.TokenType, "Bearer")
				}
				if got.ExpiresIn != int64((15 * time.Minute).Seconds()) {
					t.Errorf("ExpiresIn = %d, want %d", got.ExpiresIn, int64((15 * time.Minute).Seconds()))
				}
			}
		})
	}
}

func TestAuthService_Refresh(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()

	tests := []struct {
		name        string
		claimsType  string
		claimsSub   string
		validateErr error
		repoRes     *users.User
		repoErr     error
		wantErr     string
	}{
		{
			// go-sdk's Validator returns a coded *errorz.Error (never a bare
			// sentinel) on failure; this mirrors that.
			name:        "validator error propagates",
			validateErr: errorz.Wrap(sdkauth.ErrTokenExpired).WithCode(errorz.CodeUnauthorized).WithMessage("token expired"),
			wantErr:     errorz.CodeUnauthorized,
		},
		{
			name: "wrong token type maps to 401", claimsType: tokenTypeAccess, claimsSub: userID.String(),
			wantErr: errorz.CodeUnauthorized,
		},
		{
			name: "non-uuid subject maps to 401", claimsType: tokenTypeRefresh, claimsSub: "not-a-uuid",
			wantErr: errorz.CodeUnauthorized,
		},
		{
			name: "user no longer exists maps to 401", claimsType: tokenTypeRefresh, claimsSub: userID.String(),
			repoErr: repository.ErrNotFound, wantErr: errorz.CodeUnauthorized,
		},
		{
			name: "happy path", claimsType: tokenTypeRefresh, claimsSub: userID.String(),
			repoRes: &users.User{ID: userID, TenantID: tenantID},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			validator := mockauth.NewMockValidator(ctrl)
			if tt.validateErr != nil {
				validator.EXPECT().Validate(gomock.Any(), gomock.Any()).Return(nil, tt.validateErr)
			} else {
				claims := mockauth.NewMockClaims(ctrl)
				claims.EXPECT().Get("type").Return(tt.claimsType, true).AnyTimes()
				claims.EXPECT().Subject().Return(tt.claimsSub).AnyTimes()
				validator.EXPECT().Validate(gomock.Any(), gomock.Any()).Return(claims, nil)
			}

			repo := mockrepository.NewMockRepository[users.User, uuid.UUID](ctrl)
			if tt.validateErr == nil && tt.claimsType == tokenTypeRefresh && tt.claimsSub == userID.String() {
				repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.repoRes, tt.repoErr)
			}

			issuer := mockauth.NewMockIssuer(ctrl)
			if tt.wantErr == "" {
				issuer.EXPECT().Issue(gomock.Any(), gomock.Any(), gomock.Any()).
					Return("signed-token", nil).Times(2) // access + refresh
			}

			svc := NewService(logger.NewNoOp(), repo, issuer, validator, 15*time.Minute, 7*24*time.Hour)
			got, err := svc.Refresh(context.Background(), RefreshInput{RefreshToken: "some-token"})
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && (got.AccessToken == "" || got.RefreshToken == "") {
				t.Error("expected non-empty access and refresh tokens")
			}
		})
	}
}
