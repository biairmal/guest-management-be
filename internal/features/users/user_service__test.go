package users

import (
	"context"
	"errors"
	"net/url"
	"testing"

	sdkauth "github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	mockrepository "github.com/biairmal/go-sdk/mocks/repository"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"

	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/features/roles"
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

// systemRole is a valid system-scope role fixture for tests that need
// validateSystemScopeRole to succeed.
func systemRole() *roles.Role { return &roles.Role{Scope: roles.ScopeSystem} }

// ctxWithClaims builds a context carrying JWT claims for sub/tenant_id/role_id
// (any left "" is simply omitted), the same shape auth.issueTokenPair issues.
func ctxWithClaims(sub, tenantID, roleID string) context.Context {
	raw := map[string]any{}
	if sub != "" {
		raw["sub"] = sub
	}
	if tenantID != "" {
		raw["tenant_id"] = tenantID
	}
	if roleID != "" {
		raw["role_id"] = roleID
	}
	return sdkauth.ContextWithClaims(context.Background(), sdkauth.NewClaims(raw))
}

// ctxWithTenant returns a context with a tenant_id claim only (no user/role
// claim) — enough for the tenant-scoping checks most user_service tests need.
func ctxWithTenant(tenantID uuid.UUID) context.Context {
	return ctxWithClaims(uuid.NewString(), tenantID.String(), "")
}

// ctxWithUser returns a context with sub, tenant_id, and role_id claims — the
// shape SetOwnPassword/TransferMaster's caller-identity resolution needs.
func ctxWithUser(userID, tenantID uuid.UUID) context.Context {
	return ctxWithClaims(userID.String(), tenantID.String(), uuid.NewString())
}

func TestUserService_Create(t *testing.T) {
	tenantID := uuid.New()

	tests := []struct {
		name        string
		ctx         context.Context //nolint:containedctx // table-driven fixture, not a stored context
		in          CreateInput
		expectsRole bool
		roleRes     *roles.Role
		roleErr     error
		expectsRepo bool
		repoErr     error
		wantErr     string
	}{
		{
			name: "no tenant claim maps to 401", ctx: context.Background(),
			in: CreateInput{Email: "a@acme.com", Password: "password1"}, wantErr: errorz.CodeUnauthorized,
		},
		{
			name: "role not found maps to 404", ctx: ctxWithTenant(tenantID),
			in:          CreateInput{Email: "a@acme.com", Password: "password1"},
			expectsRole: true, roleErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound,
		},
		{
			name: "role wrong scope maps to 400", ctx: ctxWithTenant(tenantID),
			in:          CreateInput{Email: "a@acme.com", Password: "password1"},
			expectsRole: true, roleRes: &roles.Role{Scope: roles.ScopeEvent}, wantErr: errorz.CodeBadRequest,
		},
		{
			name: "role lookup error maps to 500", ctx: ctxWithTenant(tenantID),
			in:          CreateInput{Email: "a@acme.com", Password: "password1"},
			expectsRole: true, roleErr: errors.New("boom"), wantErr: errorz.CodeInternal,
		},
		{
			name: "already exists maps to 409", ctx: ctxWithTenant(tenantID),
			in:          CreateInput{Email: "a@acme.com", Password: "password1"},
			expectsRole: true, roleRes: systemRole(), expectsRepo: true,
			repoErr: repository.ErrAlreadyExists, wantErr: errorz.CodeConflict,
		},
		{
			name: "invalid entity maps to 422", ctx: ctxWithTenant(tenantID),
			in:          CreateInput{Email: "a@acme.com", Password: "password1"},
			expectsRole: true, roleRes: systemRole(), expectsRepo: true,
			repoErr: repository.ErrInvalidEntity, wantErr: errorz.CodeUnprocessableEntity,
		},
		{
			name: "unexpected repo error maps to 500", ctx: ctxWithTenant(tenantID),
			in:          CreateInput{Email: "a@acme.com", Password: "password1"},
			expectsRole: true, roleRes: systemRole(), expectsRepo: true,
			repoErr: errors.New("boom"), wantErr: errorz.CodeInternal,
		},
		{
			name: "happy path", ctx: ctxWithTenant(tenantID),
			in:          CreateInput{Email: "a@acme.com", Password: "password1", IsTenantMaster: true},
			expectsRole: true, roleRes: systemRole(), expectsRepo: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[User, uuid.UUID](ctrl)
			roleRepo := mockrepository.NewMockRepository[roles.Role, uuid.UUID](ctrl)
			if tt.expectsRole {
				roleRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.roleRes, tt.roleErr)
			}
			if tt.expectsRepo {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tt.repoErr)
			}

			svc := NewUserService(logger.NewNoOp(), repo, roleRepo, nil)
			got, err := svc.Create(tt.ctx, tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" {
				if got == nil {
					t.Fatal("expected non-nil entity on success")
				}
				if got.TenantID != tenantID {
					t.Errorf("TenantID = %v, want %v", got.TenantID, tenantID)
				}
				if !got.MustChangePassword {
					t.Error("MustChangePassword must be true on create")
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
	tenantID := uuid.New()
	otherTenant := uuid.New()

	tests := []struct {
		name       string
		ctx        context.Context //nolint:containedctx // table-driven fixture, not a stored context
		expectsGet bool
		repoRes    *User
		repoErr    error
		wantErr    string
	}{
		{name: "no tenant claim maps to 401", ctx: context.Background(), wantErr: errorz.CodeUnauthorized},
		{
			name: "not found maps to 404", ctx: ctxWithTenant(tenantID), expectsGet: true,
			repoErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound,
		},
		{
			name: "unexpected error maps to 500", ctx: ctxWithTenant(tenantID), expectsGet: true,
			repoErr: errors.New("boom"), wantErr: errorz.CodeInternal,
		},
		{
			name: "cross-tenant user maps to 404", ctx: ctxWithTenant(tenantID), expectsGet: true,
			repoRes: &User{TenantID: otherTenant}, wantErr: errorz.CodeNotFound,
		},
		{
			name: "found", ctx: ctxWithTenant(tenantID), expectsGet: true,
			repoRes: &User{TenantID: tenantID, Email: "a@acme.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[User, uuid.UUID](ctrl)
			if tt.expectsGet {
				repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.repoRes, tt.repoErr)
			}

			svc := NewUserService(logger.NewNoOp(), repo, nil, nil)
			_, err := svc.GetByID(tt.ctx, uuid.New())
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

			svc := NewUserService(logger.NewNoOp(), repo, nil, nil)
			got, err := svc.GetByEmail(context.Background(), "a@acme.com")
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && got.Email != "a@acme.com" {
				t.Errorf("Email = %q, want %q", got.Email, "a@acme.com")
			}
		})
	}
}

func TestUserService_Update(t *testing.T) {
	tenantID := uuid.New()
	otherTenant := uuid.New()
	roleID := uuid.New()

	tests := []struct {
		name        string
		ctx         context.Context //nolint:containedctx // table-driven fixture, not a stored context
		in          UpdateInput
		expectsGet  bool
		getRes      *User
		getErr      error
		expectsRole bool
		roleRes     *roles.Role
		roleErr     error
		expectsSet  bool
		updateErr   error
		wantErr     string
	}{
		{name: "no tenant claim maps to 401", ctx: context.Background(), wantErr: errorz.CodeUnauthorized},
		{
			name: "get not found maps to 404", ctx: ctxWithTenant(tenantID), expectsGet: true,
			getErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound,
		},
		{
			name: "get unexpected error maps to 500", ctx: ctxWithTenant(tenantID), expectsGet: true,
			getErr: errors.New("boom"), wantErr: errorz.CodeInternal,
		},
		{
			name: "cross-tenant user maps to 404", ctx: ctxWithTenant(tenantID), expectsGet: true,
			getRes: &User{TenantID: otherTenant}, wantErr: errorz.CodeNotFound,
		},
		{
			name: "role not found maps to 404", ctx: ctxWithTenant(tenantID),
			in: UpdateInput{RoleID: &roleID}, expectsGet: true, getRes: &User{TenantID: tenantID, Email: "a@acme.com"},
			expectsRole: true, roleErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound,
		},
		{
			name: "role wrong scope maps to 400", ctx: ctxWithTenant(tenantID),
			in: UpdateInput{RoleID: &roleID}, expectsGet: true, getRes: &User{TenantID: tenantID, Email: "a@acme.com"},
			expectsRole: true, roleRes: &roles.Role{Scope: roles.ScopeEvent}, wantErr: errorz.CodeBadRequest,
		},
		{
			name: "update not found maps to 404", ctx: ctxWithTenant(tenantID),
			in:         UpdateInput{Email: ptrString("new@acme.com")},
			expectsGet: true, getRes: &User{TenantID: tenantID, Email: "a@acme.com"},
			expectsSet: true, updateErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound,
		},
		{
			name: "update already exists maps to 409", ctx: ctxWithTenant(tenantID),
			in:         UpdateInput{Email: ptrString("new@acme.com")},
			expectsGet: true, getRes: &User{TenantID: tenantID, Email: "a@acme.com"},
			expectsSet: true, updateErr: repository.ErrAlreadyExists, wantErr: errorz.CodeConflict,
		},
		{
			name: "happy path partial update", ctx: ctxWithTenant(tenantID),
			in:         UpdateInput{Email: ptrString("new@acme.com")},
			expectsGet: true, getRes: &User{TenantID: tenantID, Email: "a@acme.com"}, expectsSet: true,
		},
		{
			name: "happy path with role change re-validates scope", ctx: ctxWithTenant(tenantID),
			in: UpdateInput{RoleID: &roleID}, expectsGet: true, getRes: &User{TenantID: tenantID, Email: "a@acme.com"},
			expectsRole: true, roleRes: systemRole(), expectsSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[User, uuid.UUID](ctrl)
			roleRepo := mockrepository.NewMockRepository[roles.Role, uuid.UUID](ctrl)
			if tt.expectsGet {
				repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.getRes, tt.getErr)
			}
			if tt.expectsRole {
				roleRepo.EXPECT().GetByID(gomock.Any(), roleID).Return(tt.roleRes, tt.roleErr)
			}
			if tt.expectsSet {
				repo.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.updateErr)
			}

			svc := NewUserService(logger.NewNoOp(), repo, roleRepo, nil)
			got, err := svc.Update(tt.ctx, uuid.New(), tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" {
				if tt.in.Email != nil && got.Email != *tt.in.Email {
					t.Errorf("Email = %q, want %q", got.Email, *tt.in.Email)
				}
				if tt.in.RoleID != nil && got.RoleID != *tt.in.RoleID {
					t.Errorf("RoleID = %v, want %v", got.RoleID, *tt.in.RoleID)
				}
			}
		})
	}
}

func TestUserService_Delete(t *testing.T) {
	tenantID := uuid.New()
	otherTenant := uuid.New()

	tests := []struct {
		name       string
		ctx        context.Context //nolint:containedctx // table-driven fixture, not a stored context
		expectsGet bool
		getRes     *User
		getErr     error
		expectsDel bool
		delErr     error
		wantErr    string
	}{
		{name: "no tenant claim maps to 401", ctx: context.Background(), wantErr: errorz.CodeUnauthorized},
		{
			name: "get not found maps to 404", ctx: ctxWithTenant(tenantID), expectsGet: true,
			getErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound,
		},
		{
			name: "cross-tenant user maps to 404", ctx: ctxWithTenant(tenantID), expectsGet: true,
			getRes: &User{TenantID: otherTenant}, wantErr: errorz.CodeNotFound,
		},
		{
			name: "delete unexpected error maps to 500", ctx: ctxWithTenant(tenantID), expectsGet: true,
			getRes: &User{TenantID: tenantID}, expectsDel: true, delErr: errors.New("boom"), wantErr: errorz.CodeInternal,
		},
		{
			name: "happy path", ctx: ctxWithTenant(tenantID), expectsGet: true,
			getRes: &User{TenantID: tenantID}, expectsDel: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[User, uuid.UUID](ctrl)
			if tt.expectsGet {
				repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.getRes, tt.getErr)
			}
			if tt.expectsDel {
				repo.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(tt.delErr)
			}

			svc := NewUserService(logger.NewNoOp(), repo, nil, nil)
			err := svc.Delete(tt.ctx, uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestUserService_List(t *testing.T) {
	tenantID := uuid.New()

	tests := []struct {
		name        string
		ctx         context.Context //nolint:containedctx // table-driven fixture, not a stored context
		expectsRepo bool
		repoErr     error
		wantErr     string
		wantSize    int
	}{
		{name: "no tenant claim maps to 401", ctx: context.Background(), wantErr: errorz.CodeUnauthorized},
		{
			name: "repo error maps to 500", ctx: ctxWithTenant(tenantID), expectsRepo: true,
			repoErr: errors.New("boom"), wantErr: errorz.CodeInternal,
		},
		{name: "happy path", ctx: ctxWithTenant(tenantID), expectsRepo: true, wantSize: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[User, uuid.UUID](ctrl)
			if tt.expectsRepo {
				repo.EXPECT().
					List(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, opts *repository.ListOptions) ([]*User, int64, error) {
						found := false
						for _, c := range opts.Filter.Conditions {
							if c.Field == "tenant_id" && c.Value == tenantID {
								found = true
							}
						}
						if !found {
							t.Error("expected a server-injected tenant_id filter condition")
						}
						return []*User{{Email: "a@acme.com"}}, int64(1), tt.repoErr
					})
			}

			svc := NewUserService(logger.NewNoOp(), repo, nil, nil)
			params, err := query.ParseListParams(url.Values{}, query.ListParseConfig{})
			if err != nil {
				t.Fatalf("ParseListParams() error = %v", err)
			}
			got, err := svc.List(tt.ctx, params)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && got.Size != tt.wantSize {
				t.Errorf("Size = %d, want %d", got.Size, tt.wantSize)
			}
		})
	}
}

// setPasswordTestCase is the shared table shape for TestUserService_SetPassword
// and TestUserService_SetOwnPassword — both exercise the same setPassword
// helper, differing only in how the target id is supplied.
type setPasswordTestCase struct {
	name       string
	ctx        context.Context //nolint:containedctx // table-driven fixture, not a stored context
	expectsGet bool
	getRes     *User
	getErr     error
	expectsSet bool
	updateErr  error
	wantErr    string
}

// assertSetPasswordResult asserts the persisted/returned entity's
// PasswordHash was re-hashed and MustChangePassword matches wantMustChange —
// true for an admin reset (SetPassword), false for self-service
// (SetOwnPassword) — per REQUIREMENT.md §3.2/§4.8 AC4/AC5.
func assertSetPasswordResult(t *testing.T, got *User, wantErr string, wantMustChange bool) {
	t.Helper()
	if wantErr != "" {
		return
	}
	if got.MustChangePassword != wantMustChange {
		t.Errorf("MustChangePassword = %v, want %v", got.MustChangePassword, wantMustChange)
	}
	if got.PasswordHash == "old" {
		t.Error("PasswordHash was not re-hashed")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("newpassword1")); err != nil {
		t.Errorf("stored hash does not match new password: %v", err)
	}
}

// TestUserService_SetPassword covers the admin-only route: no permission
// check happens in the service (the manage_users route group is the sole
// authorization check), id is always resolved through loadTenantScopedUser
// (so a cross-tenant target is errorz.NotFound), and MustChangePassword is
// forced back to true even when the target's was already false — an
// admin-assigned password wasn't chosen by the target (REQUIREMENT.md
// §3.2/§4.8 AC5).
func TestUserService_SetPassword(t *testing.T) {
	tenantID := uuid.New()
	otherTenant := uuid.New()
	targetID := uuid.New()

	tests := []setPasswordTestCase{
		{
			name: "not found maps to 404", ctx: ctxWithTenant(tenantID), expectsGet: true,
			getErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound,
		},
		{
			name: "cross-tenant target maps to 404", ctx: ctxWithTenant(tenantID), expectsGet: true,
			getRes: &User{ID: targetID, TenantID: otherTenant}, wantErr: errorz.CodeNotFound,
		},
		{
			name: "update failure maps to 500", ctx: ctxWithTenant(tenantID), expectsGet: true,
			getRes:     &User{ID: targetID, TenantID: tenantID, PasswordHash: "old"},
			expectsSet: true, updateErr: errors.New("boom"), wantErr: errorz.CodeInternal,
		},
		{
			name: "happy path sets must_change_password true", ctx: ctxWithTenant(tenantID), expectsGet: true,
			getRes:     &User{ID: targetID, TenantID: tenantID, PasswordHash: "old", MustChangePassword: false},
			expectsSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[User, uuid.UUID](ctrl)
			if tt.expectsGet {
				repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.getRes, tt.getErr)
			}
			if tt.expectsSet {
				repo.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.updateErr)
			}

			svc := NewUserService(logger.NewNoOp(), repo, nil, nil)
			got, err := svc.SetPassword(tt.ctx, targetID, SetPasswordInput{Password: "newpassword1"})
			assertErrorzCode(t, err, tt.wantErr)
			assertSetPasswordResult(t, got, tt.wantErr, true)
		})
	}
}

// TestUserService_SetOwnPassword covers the self-service route: the target
// id always comes from authz.UserIDFromContext, never a param, so a missing
// user claim maps to 401 before any repository call, and MustChangePassword
// is cleared to false (REQUIREMENT.md §3.2/§4.8 AC4).
func TestUserService_SetOwnPassword(t *testing.T) {
	tenantID := uuid.New()
	callerID := uuid.New()

	tests := []setPasswordTestCase{
		{name: "no user claim maps to 401", ctx: context.Background(), wantErr: errorz.CodeUnauthorized},
		{
			name: "update failure maps to 500", ctx: ctxWithUser(callerID, tenantID), expectsGet: true,
			getRes:     &User{ID: callerID, TenantID: tenantID, PasswordHash: "old"},
			expectsSet: true, updateErr: errors.New("boom"), wantErr: errorz.CodeInternal,
		},
		{
			name: "happy path clears must_change_password false", ctx: ctxWithUser(callerID, tenantID),
			expectsGet: true,
			getRes:     &User{ID: callerID, TenantID: tenantID, PasswordHash: "old", MustChangePassword: true},
			expectsSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[User, uuid.UUID](ctrl)
			if tt.expectsGet {
				repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.getRes, tt.getErr)
			}
			if tt.expectsSet {
				repo.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.updateErr)
			}

			svc := NewUserService(logger.NewNoOp(), repo, nil, nil)
			got, err := svc.SetOwnPassword(tt.ctx, SetPasswordInput{Password: "newpassword1"})
			assertErrorzCode(t, err, tt.wantErr)
			assertSetPasswordResult(t, got, tt.wantErr, false)
		})
	}
}

func TestUserService_TransferMaster_NoUserClaim(t *testing.T) {
	svc := NewUserService(logger.NewNoOp(), nil, nil, nil)
	_, err := svc.TransferMaster(context.Background(), uuid.New())
	assertErrorzCode(t, err, errorz.CodeUnauthorized)
}

// TestUserService_transferMaster exercises the transactional swap's business
// logic directly (permission check, tenant scoping, error translation)
// against a mocked repository. TransferMaster's public wrapper only adds the
// go-sdk (*sqlkit.DB).WithTransaction call around this, which needs a live
// database to exercise end to end (see docs/TESTING.md "Unit vs integration").
func TestUserService_transferMaster(t *testing.T) {
	tenantID := uuid.New()
	otherTenant := uuid.New()
	callerID := uuid.New()
	targetID := uuid.New()

	tests := []struct {
		name          string
		ctx           context.Context //nolint:containedctx // table-driven fixture, not a stored context
		expectsCaller bool
		callerRes     *User
		callerErr     error
		expectsTarget bool
		targetRes     *User
		targetErr     error
		expectsClear  bool
		clearErr      error
		expectsSet    bool
		setErr        error
		wantErr       string
	}{
		{name: "no tenant claim maps to 401", ctx: context.Background(), wantErr: errorz.CodeUnauthorized},
		{
			name: "caller not found maps to 404", ctx: ctxWithTenant(tenantID), expectsCaller: true,
			callerErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound,
		},
		{
			name: "caller not tenant master maps to 403", ctx: ctxWithTenant(tenantID), expectsCaller: true,
			callerRes: &User{ID: callerID, TenantID: tenantID, IsTenantMaster: false}, wantErr: errorz.CodeForbidden,
		},
		{
			name: "target not found maps to 404", ctx: ctxWithTenant(tenantID), expectsCaller: true,
			callerRes:     &User{ID: callerID, TenantID: tenantID, IsTenantMaster: true},
			expectsTarget: true, targetErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound,
		},
		{
			name: "target cross-tenant maps to 404", ctx: ctxWithTenant(tenantID), expectsCaller: true,
			callerRes:     &User{ID: callerID, TenantID: tenantID, IsTenantMaster: true},
			expectsTarget: true, targetRes: &User{ID: targetID, TenantID: otherTenant}, wantErr: errorz.CodeNotFound,
		},
		{
			name: "clear caller update failure maps to 500", ctx: ctxWithTenant(tenantID), expectsCaller: true,
			callerRes:     &User{ID: callerID, TenantID: tenantID, IsTenantMaster: true},
			expectsTarget: true, targetRes: &User{ID: targetID, TenantID: tenantID},
			expectsClear: true, clearErr: errors.New("boom"), wantErr: errorz.CodeInternal,
		},
		{
			name: "set target update failure maps to 500", ctx: ctxWithTenant(tenantID), expectsCaller: true,
			callerRes:     &User{ID: callerID, TenantID: tenantID, IsTenantMaster: true},
			expectsTarget: true, targetRes: &User{ID: targetID, TenantID: tenantID},
			expectsClear: true, expectsSet: true, setErr: errors.New("boom"), wantErr: errorz.CodeInternal,
		},
		{
			name: "happy path", ctx: ctxWithTenant(tenantID), expectsCaller: true,
			callerRes:     &User{ID: callerID, TenantID: tenantID, IsTenantMaster: true},
			expectsTarget: true, targetRes: &User{ID: targetID, TenantID: tenantID},
			expectsClear: true, expectsSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[User, uuid.UUID](ctrl)
			if tt.expectsCaller {
				repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.callerRes, tt.callerErr)
			}
			if tt.expectsTarget {
				repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.targetRes, tt.targetErr)
			}
			if tt.expectsClear {
				repo.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.clearErr)
			}
			if tt.expectsSet {
				repo.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.setErr)
			}

			svc := &userServiceImpl{repo: repo, logger: logger.NewNoOp()}
			got, err := svc.transferMaster(tt.ctx, callerID, targetID)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" {
				if got.ID != targetID {
					t.Errorf("ID = %v, want %v", got.ID, targetID)
				}
				if !got.IsTenantMaster {
					t.Error("target must hold IsTenantMaster after a successful transfer")
				}
			}
		})
	}
}
