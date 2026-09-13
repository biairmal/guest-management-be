package users

import (
	"context"
	"errors"

	common "github.com/biairmal/go-sdk/lib/common/dto"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/biairmal/guest-management-be/internal/core/authz"
	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/features/roles"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../mocks/users/mock_service.go -package=mockusers github.com/biairmal/guest-management-be/internal/features/users UserService

// UserListConfig declares the allow-listed sort/filter fields for user list
// queries, enforced here in the service via query.ValidateListParams and
// reused by UserHandler.List for query.ParseListParams. tenant_id is
// deliberately absent from both lists: every list is already scoped to the
// caller's own tenant (server-injected from the JWT claim, see List below),
// so it is no longer a caller-supplied dimension.
var UserListConfig = query.ListParseConfig{
	AllowedSortFields:   []string{"id", "email", "role_id", "is_tenant_master", "created_at", "updated_at"},
	AllowedFilterFields: []string{"email", "role_id", "is_tenant_master"},
}

// UserService defines the application-level operations for users. Every
// operation resolves the caller's tenant from the "tenant_id" JWT claim
// (authz.TenantIDFromContext) rather than trusting a request body field, and
// a target user belonging to a different tenant is treated as not found
// (see loadTenantScopedUser) — mirroring staffing's precedent
// (docs/FEATURES.md#staffing).
type UserService interface {
	Create(ctx context.Context, in CreateInput) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	// GetByEmail returns a user by email (unique across all tenants), or
	// errorz.NotFound if none exists. Used by the auth feature for login.
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*User, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, params *query.ListParams) (*common.PageResponse[User], error)
	// SetPassword sets a new password for id, clearing MustChangePassword.
	// Admin-only: the route group requires PermissionManageUsers, so no
	// in-service permission check is needed here. For a caller changing their
	// own password, see SetOwnPassword.
	SetPassword(ctx context.Context, id uuid.UUID, in SetPasswordInput) (*User, error)
	// SetOwnPassword sets a new password for the caller's own account,
	// clearing MustChangePassword. The target id is always resolved from
	// authz.UserIDFromContext — never a path param or request field — so the
	// caller can only ever act on themselves.
	SetOwnPassword(ctx context.Context, in SetPasswordInput) (*User, error)
	// TransferMaster transfers the caller's own is_tenant_master flag to
	// targetID, an active user in the same tenant. The caller must currently
	// be the tenant master (errorz.Forbidden otherwise). Both loads and both
	// updates run inside a single transaction so a failure between them
	// rolls back instead of leaving a momentary zero-master state.
	TransferMaster(ctx context.Context, targetID uuid.UUID) (*User, error)
}

// userServiceImpl is the concrete implementation of UserService.
type userServiceImpl struct {
	repo     repository.Repository[User, uuid.UUID]
	roleRepo repository.Repository[roles.Role, uuid.UUID]
	db       *sqlkit.DB
	logger   logger.Logger
}

// NewUserService returns a UserService with the given dependencies. roleRepo
// is used to validate that a user's role_id references a system-scope role
// (see docs/STAFFING_RBAC.md ss3) — a cross-feature repository-type
// dependency, not a service-to-service one, mirroring the auth->users
// precedent (see docs/ARCHITECTURE.md "Feature slice = future service boundary").
// db backs TransferMaster's transactional two-row swap (go-sdk's
// (*sqlkit.DB).WithTransaction).
func NewUserService(
	logger logger.Logger, repo repository.Repository[User, uuid.UUID],
	roleRepo repository.Repository[roles.Role, uuid.UUID],
	db *sqlkit.DB,
) UserService {
	return &userServiceImpl{logger: logger, repo: repo, roleRepo: roleRepo, db: db}
}

// loadTenantScopedUser loads id and verifies it belongs to the caller's
// tenant (per the "tenant_id" JWT claim). Returns errorz.NotFound both when
// the user doesn't exist and when it belongs to a different tenant — the
// same not-found-not-forbidden convention as staffing's
// loadTenantScopedEvent, so a caller cannot probe for another tenant's user
// IDs.
func (s *userServiceImpl) loadTenantScopedUser(ctx context.Context, id uuid.UUID) (*User, error) {
	tenantID, ok := authz.TenantIDFromContext(ctx)
	if !ok {
		return nil, errorz.Unauthorized().WithMessage("no tenant claim present")
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorz.NotFound().WithMessage("user not found")
		}
		s.logger.ErrorWithContext(ctx, "user lookup failed", logger.F("id", id), logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to get user")
	}
	if entity.TenantID != tenantID {
		return nil, errorz.NotFound().WithMessage("user not found")
	}
	return entity, nil
}

// validateSystemScopeRole loads roleID and returns errorz.NotFound if it
// doesn't exist, or errorz.BadRequest if it exists but isn't a system-scope
// role — a user's role_id must reference a role assignable to a tenant-wide
// role (see docs/STAFFING_RBAC.md ss3), never an event-scoped one.
func (s *userServiceImpl) validateSystemScopeRole(ctx context.Context, roleID uuid.UUID) error {
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return errorz.NotFound().WithMessage("role not found")
		}
		s.logger.ErrorWithContext(ctx, "user role lookup failed", logger.F("role_id", roleID), logger.F("error", err))
		return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to look up role")
	}
	if role.Scope != roles.ScopeSystem {
		return errorz.BadRequest().WithMessage("role_id must reference a system-scope role")
	}
	return nil
}

// hashPassword returns the bcrypt hash of password, or an internal errorz on failure.
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to hash password")
	}
	return string(hash), nil
}

// Create creates a new user scoped to the caller's own tenant (resolved from
// the "tenant_id" JWT claim, never a request field). ID is generated by the
// service; the password is hashed before storage and never returned;
// MustChangePassword always starts true. Audit fields (created_at,
// updated_at) are set by the AuditableRepository.
func (s *userServiceImpl) Create(ctx context.Context, in CreateInput) (*User, error) {
	tenantID, ok := authz.TenantIDFromContext(ctx)
	if !ok {
		return nil, errorz.Unauthorized().WithMessage("no tenant claim present")
	}

	if err := s.validateSystemScopeRole(ctx, in.RoleID); err != nil {
		return nil, err
	}

	hash, err := hashPassword(in.Password)
	if err != nil {
		return nil, err
	}

	entity := &User{
		ID:                 uuid.New(),
		TenantID:           tenantID,
		Email:              in.Email,
		PasswordHash:       hash,
		RoleID:             in.RoleID,
		IsTenantMaster:     in.IsTenantMaster,
		MustChangePassword: true,
	}

	if err := s.repo.Create(ctx, entity); err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, errorz.Conflict().WithMessage("user already exists")
		}
		if errors.Is(err, repository.ErrInvalidEntity) {
			return nil, errorz.UnprocessableEntity().WithMessage("invalid user data")
		}
		s.logger.ErrorWithContext(ctx, "user create failed", logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to create user")
	}

	s.logger.InfoWithContext(ctx, "user created", logger.F("id", entity.ID))
	return entity, nil
}

// GetByID returns a user by ID, scoped to the caller's own tenant. Returns
// errorz.NotFound if not found, soft-deleted, or belonging to another tenant.
func (s *userServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.loadTenantScopedUser(ctx, id)
}

// GetByEmail returns a user by email, or errorz.NotFound if none exists.
// Email is unique across all tenants (migration 000012), so no tenant scope
// is needed to disambiguate — this is used only by the auth feature's login,
// which must look up a user before any tenant is known.
func (s *userServiceImpl) GetByEmail(ctx context.Context, email string) (*User, error) {
	opts := &repository.ListOptions{
		Filter: repository.Filter{Conditions: []repository.FilterCondition{
			{Field: "email", Operator: repository.FilterOperatorEq, Value: email},
		}},
		Pagination: repository.Pagination{Limit: 1},
	}
	items, _, err := s.repo.List(ctx, opts)
	if err != nil {
		s.logger.ErrorWithContext(ctx, "user get by email failed", logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to get user")
	}
	if len(items) == 0 {
		return nil, errorz.NotFound().WithMessage("user not found")
	}
	return items[0], nil
}

// applyUpdate mutates entity in place with every non-nil field of in,
// re-validating a provided role_id's scope. Split out of Update to keep that
// method's cognitive complexity within the lint budget.
func (s *userServiceImpl) applyUpdate(ctx context.Context, entity *User, in UpdateInput) error {
	if in.Email != nil {
		entity.Email = *in.Email
	}
	if in.RoleID != nil {
		if err := s.validateSystemScopeRole(ctx, *in.RoleID); err != nil {
			return err
		}
		entity.RoleID = *in.RoleID
	}
	if in.IsTenantMaster != nil {
		entity.IsTenantMaster = *in.IsTenantMaster
	}
	return nil
}

// Update updates a user scoped to the caller's own tenant. Only non-nil
// fields in UpdateInput are applied. Password is not settable here — see
// SetPassword. The updated_at field is set by the AuditableRepository.
func (s *userServiceImpl) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*User, error) {
	entity, err := s.loadTenantScopedUser(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.applyUpdate(ctx, entity, in); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, id, entity); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorz.NotFound().WithMessage("user not found")
		}
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, errorz.Conflict().WithMessage("user already exists")
		}
		s.logger.ErrorWithContext(ctx, "user update failed", logger.F("id", id), logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to update user")
	}

	s.logger.InfoWithContext(ctx, "user updated", logger.F("id", id))
	return entity, nil
}

// Delete soft-deletes a user scoped to the caller's own tenant. The
// AuditableRepository handles setting deleted_at and updated_at.
func (s *userServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.loadTenantScopedUser(ctx, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return errorz.NotFound().WithMessage("user not found")
		}
		s.logger.ErrorWithContext(ctx, "user delete failed", logger.F("id", id), logger.F("error", err))
		return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to delete user")
	}
	s.logger.InfoWithContext(ctx, "user deleted", logger.F("id", id))
	return nil
}

// List returns users belonging to the caller's own tenant, with filter,
// sort, and pagination from query.ListParams. tenant_id is always
// server-injected from the "tenant_id" JWT claim, never a caller-supplied
// filter (mirroring staffing.List injecting event_id).
func (s *userServiceImpl) List(ctx context.Context, params *query.ListParams) (*common.PageResponse[User], error) {
	if err := query.ValidateListParams(params, UserListConfig); err != nil {
		return nil, errorz.BadRequest().WithMessage(err.Error())
	}
	tenantID, ok := authz.TenantIDFromContext(ctx)
	if !ok {
		return nil, errorz.Unauthorized().WithMessage("no tenant claim present")
	}

	opts := query.ToListOptions(params)
	opts.Filter.Conditions = append(opts.Filter.Conditions, repository.FilterCondition{
		Field:    "tenant_id",
		Operator: repository.FilterOperatorEq,
		Value:    tenantID,
	})

	items, total, err := s.repo.List(ctx, opts)
	if err != nil {
		s.logger.ErrorWithContext(ctx, "user list failed", logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to list users")
	}
	return common.NewPageResponse(items, total, params.Page, params.Size), nil
}

// SetPassword sets a new password for id (an admin resetting another user's
// password) and sets MustChangePassword back to true — even if it was
// already false — since the target didn't choose the new password
// themselves. Admin-only: the caller reaches this method only via the
// manage_users route group, so no in-service permission check is needed. id
// is resolved through loadTenantScopedUser, so a cross-tenant target is
// errorz.NotFound.
func (s *userServiceImpl) SetPassword(ctx context.Context, id uuid.UUID, in SetPasswordInput) (*User, error) {
	return s.setPassword(ctx, id, in, true)
}

// SetOwnPassword sets a new password for the caller's own account and clears
// MustChangePassword to false, resolving the target id from
// authz.UserIDFromContext — never a path param or request field, so the
// caller can only ever act on themselves.
func (s *userServiceImpl) SetOwnPassword(ctx context.Context, in SetPasswordInput) (*User, error) {
	callerID, ok := authz.UserIDFromContext(ctx)
	if !ok {
		return nil, errorz.Unauthorized().WithMessage("no user claim present")
	}
	return s.setPassword(ctx, callerID, in, false)
}

// setPassword is the shared hash-and-update logic behind SetPassword and
// SetOwnPassword: it resolves id through loadTenantScopedUser (so a
// cross-tenant target is errorz.NotFound), hashes the new password, sets
// MustChangePassword to mustChangePassword, and persists. Callers decide the
// flag's value: SetPassword (admin reset) forces it true, SetOwnPassword
// (self-service) clears it to false.
func (s *userServiceImpl) setPassword(
	ctx context.Context, id uuid.UUID, in SetPasswordInput, mustChangePassword bool,
) (*User, error) {
	entity, err := s.loadTenantScopedUser(ctx, id)
	if err != nil {
		return nil, err
	}

	hash, err := hashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	entity.PasswordHash = hash
	entity.MustChangePassword = mustChangePassword

	if err := s.repo.Update(ctx, id, entity); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorz.NotFound().WithMessage("user not found")
		}
		s.logger.ErrorWithContext(ctx, "user set password failed", logger.F("id", id), logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to set password")
	}

	s.logger.InfoWithContext(ctx, "user password changed", logger.F("id", id))
	return entity, nil
}

// TransferMaster transfers the caller's is_tenant_master flag to targetID.
// The caller (authz.UserIDFromContext) must currently be the tenant master
// (errorz.Forbidden otherwise); targetID is resolved via
// loadTenantScopedUser, so a target in another tenant is errorz.NotFound.
// Both loads and both updates run inside a single transaction — the write
// order (clear caller, then set target) is forced by the existing partial
// unique index on (tenant_id) WHERE is_tenant_master = true, and the
// transaction ensures a failure between the two updates rolls back instead
// of leaving a momentary zero-master state.
func (s *userServiceImpl) TransferMaster(ctx context.Context, targetID uuid.UUID) (*User, error) {
	callerID, ok := authz.UserIDFromContext(ctx)
	if !ok {
		return nil, errorz.Unauthorized().WithMessage("no user claim present")
	}

	var target *User
	err := s.db.WithTransaction(ctx, func(txCtx context.Context) error {
		var txErr error
		target, txErr = s.transferMaster(txCtx, callerID, targetID)
		return txErr
	})
	if err != nil {
		return nil, err
	}

	s.logger.InfoWithContext(ctx, "tenant master transferred",
		logger.F("caller_id", callerID), logger.F("target_id", targetID))
	return target, nil
}

// transferMaster performs the actual two-row swap (clear caller's
// is_tenant_master, then set target's) against ctx, which TransferMaster
// always calls with the transaction-carrying context from
// (*sqlkit.DB).WithTransaction. Split out so this swap logic — permission
// check, tenant scoping, error translation — is unit-testable against a
// mocked repository without a live transaction; TransferMaster itself only
// adds the go-sdk transaction wrapper around it.
func (s *userServiceImpl) transferMaster(ctx context.Context, callerID, targetID uuid.UUID) (*User, error) {
	caller, err := s.loadTenantScopedUser(ctx, callerID)
	if err != nil {
		return nil, err
	}
	if !caller.IsTenantMaster {
		return nil, errorz.Forbidden().WithMessage("caller is not the tenant master")
	}

	target, err := s.loadTenantScopedUser(ctx, targetID)
	if err != nil {
		return nil, err
	}

	caller.IsTenantMaster = false
	if err := s.repo.Update(ctx, caller.ID, caller); err != nil {
		s.logger.ErrorWithContext(ctx, "tenant master transfer: clear caller failed",
			logger.F("id", caller.ID), logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to transfer tenant master")
	}

	target.IsTenantMaster = true
	if err := s.repo.Update(ctx, target.ID, target); err != nil {
		s.logger.ErrorWithContext(ctx, "tenant master transfer: set target failed",
			logger.F("id", target.ID), logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to transfer tenant master")
	}
	return target, nil
}
