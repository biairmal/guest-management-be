package auth

import (
	"context"
	"errors"
	"time"

	sdkauth "github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/biairmal/guest-management-be/internal/features/users"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../mocks/auth/mock_service.go -package=mockappauth github.com/biairmal/guest-management-be/internal/features/auth Service

// Service verifies credentials and issues/refreshes tokens.
type Service interface {
	Login(ctx context.Context, in LoginInput) (*TokenPair, error)
	Refresh(ctx context.Context, in RefreshInput) (*TokenPair, error)
}

// authServiceImpl implements Service. It reads users.User directly off
// the users repository — the same repository internal/app wires into
// users.UserService — rather than depending on users.UserService itself, so
// this feature never imports another feature's service layer (see
// docs/ARCHITECTURE.md "Feature slice = future service boundary") and its
// tests can use go-sdk's generated repository.Repository mock instead of a
// second, module-cyclic mock of an app-defined service interface.
type authServiceImpl struct {
	repo       repository.Repository[users.User, uuid.UUID]
	issuer     sdkauth.Issuer
	validator  sdkauth.Validator
	logger     logger.Logger
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewService returns an Service. accessTTL/refreshTTL set the
// lifetime of the two tokens issued together by Login (see issueTokenPair);
// validator is the same one used to authenticate protected routes (wrapped
// in AccessOnlyValidator for that purpose) and is reused here, unwrapped, to
// verify an incoming refresh token.
func NewService(
	logger logger.Logger, repo repository.Repository[users.User, uuid.UUID],
	issuer sdkauth.Issuer, validator sdkauth.Validator, accessTTL, refreshTTL time.Duration,
) Service {
	return &authServiceImpl{
		repo: repo, issuer: issuer, validator: validator, logger: logger,
		accessTTL: accessTTL, refreshTTL: refreshTTL,
	}
}

// errInvalidCredentials is returned for both an unknown email and a
// password mismatch, so a caller cannot enumerate valid emails.
var errInvalidCredentials = errorz.Unauthorized().WithMessage("invalid email or password")

// Login verifies email/password and, on success, issues a fresh access +
// refresh token pair.
func (s *authServiceImpl) Login(ctx context.Context, in LoginInput) (*TokenPair, error) {
	user, err := s.getByEmail(ctx, in.Email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errInvalidCredentials
		}
		s.logger.ErrorWithContext(ctx, "auth login lookup failed", logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to look up user")
	}

	if cmpErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)); cmpErr != nil {
		return nil, errInvalidCredentials
	}

	return s.issueTokenPair(ctx, user.ID, user.TenantID, user.RoleID, user.MustChangePassword)
}

// Refresh validates a refresh token and issues a new token pair for the same
// subject. The user is re-loaded so a deleted account cannot refresh past
// its removal.
func (s *authServiceImpl) Refresh(ctx context.Context, in RefreshInput) (*TokenPair, error) {
	claims, err := s.validator.Validate(ctx, in.RefreshToken)
	if err != nil {
		return nil, err
	}
	if typ, _ := claims.Get("type"); typ != tokenTypeRefresh {
		return nil, errorz.Unauthorized().WithMessage("not a refresh token")
	}

	userID, err := uuid.Parse(claims.Subject())
	if err != nil {
		return nil, errorz.Unauthorized().WithMessage("invalid token subject")
	}
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, errorz.Unauthorized().WithMessage("user no longer exists")
	}

	// user.RoleID is read from the freshly-reloaded user (never carried over
	// from the presented refresh token's claims), so a role change takes
	// effect on the caller's very next refresh rather than being stuck for
	// the remainder of the refresh token's TTL (see docs/STAFFING_RBAC.md ss6).
	// MustChangePassword is likewise read fresh — this is free reuse of a
	// value already loaded, not a new business rule attached to refresh.
	return s.issueTokenPair(ctx, user.ID, user.TenantID, user.RoleID, user.MustChangePassword)
}

// getByEmail looks up a user by email (unique across all tenants; migration
// 000012), returning repository.ErrNotFound when none matches.
func (s *authServiceImpl) getByEmail(ctx context.Context, email string) (*users.User, error) {
	opts := &repository.ListOptions{
		Filter: repository.Filter{Conditions: []repository.FilterCondition{
			{Field: "email", Operator: repository.FilterOperatorEq, Value: email},
		}},
		Pagination: repository.Pagination{Limit: 1},
	}
	items, _, err := s.repo.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, repository.ErrNotFound
	}
	return items[0], nil
}

// issueTokenPair mints an access token (accessTTL, type=access) and a
// refresh token (refreshTTL, type=refresh) for userID, both carrying
// tenant_id and role_id as extra claims — this package keeps tenant/role
// scoping in JWT claims rather than ctxkit (see docs/DEVELOPMENT_PLAN.md B3,
// docs/STAFFING_RBAC.md ss6). Callers must pass the caller's *current*
// roleID (e.g. freshly reloaded from the repository), never one carried
// over from a previously-issued token, so a role change takes effect
// promptly rather than persisting for the life of an old token.
func (s *authServiceImpl) issueTokenPair(
	ctx context.Context, userID, tenantID, roleID uuid.UUID, mustChangePassword bool,
) (*TokenPair, error) {
	subject := userID.String()
	claims := func(typ string) map[string]any {
		return map[string]any{"type": typ, "tenant_id": tenantID.String(), "role_id": roleID.String()}
	}

	access, err := s.issuer.Issue(subject,
		sdkauth.WithTTL(s.accessTTL),
		sdkauth.WithExtraClaims(claims(tokenTypeAccess)),
	)
	if err != nil {
		s.logger.ErrorWithContext(ctx, "issue access token failed", logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to issue access token")
	}

	refresh, err := s.issuer.Issue(subject,
		sdkauth.WithTTL(s.refreshTTL),
		sdkauth.WithExtraClaims(claims(tokenTypeRefresh)),
	)
	if err != nil {
		s.logger.ErrorWithContext(ctx, "issue refresh token failed", logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to issue refresh token")
	}

	return &TokenPair{
		AccessToken: access, RefreshToken: refresh,
		TokenType: "Bearer", ExpiresIn: int64(s.accessTTL.Seconds()),
		MustChangePassword: mustChangePassword,
	}, nil
}
