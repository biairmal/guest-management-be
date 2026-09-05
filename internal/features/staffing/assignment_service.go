package staffing

import (
	"context"
	"errors"

	common "github.com/biairmal/go-sdk/lib/common/dto"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/google/uuid"

	"github.com/biairmal/guest-management-be/internal/core/authz"
	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/roles"
	"github.com/biairmal/guest-management-be/internal/features/users"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../mocks/staffing/mock_service.go -package=mockstaffing github.com/biairmal/guest-management-be/internal/features/staffing StaffAssignmentService

// PermissionManageStaff is the permission code (see docs/STAFFING_RBAC.md
// ss3) required to assign/remove/update event staff. Gated on every route
// in assignment_routes.go via authz.RequirePermission.
const PermissionManageStaff = "manage_staff"

// StaffAssignmentService defines the application-level operations for
// event-scoped staff assignments. Every operation takes the owning event's
// ID so an assignment can never be read, changed, or deleted through a
// mismatched event in the URL, and every operation resolves the caller's
// tenant from the "tenant_id" JWT claim (authz.TenantIDFromContext) rather
// than trusting a request body field (see docs/FEATURES.md#staffing for the
// tenant-scoping-from-JWT rationale).
type StaffAssignmentService interface {
	Create(ctx context.Context, eventID uuid.UUID, in CreateAssignmentInput) (*EventStaffAssignment, error)
	GetByID(ctx context.Context, eventID, id uuid.UUID) (*EventStaffAssignment, error)
	Update(ctx context.Context, eventID, id uuid.UUID, in UpdateAssignmentInput) (*EventStaffAssignment, error)
	Delete(ctx context.Context, eventID, id uuid.UUID) error
	List(
		ctx context.Context, eventID uuid.UUID, params *query.ListParams,
	) (*common.PageResponse[EventStaffAssignment], error)
}

// staffAssignmentServiceImpl is the concrete implementation of StaffAssignmentService.
type staffAssignmentServiceImpl struct {
	repo      repository.Repository[EventStaffAssignment, uuid.UUID]
	eventRepo repository.Repository[events.Event, uuid.UUID]
	userRepo  repository.Repository[users.User, uuid.UUID]
	roleRepo  repository.Repository[roles.Role, uuid.UUID]
	logger    logger.Logger
}

// NewStaffAssignmentService returns a StaffAssignmentService with the given
// dependencies. eventRepo/userRepo/roleRepo are cross-feature
// repository-type dependencies — never another feature's service — the
// same direction as auth.Service depending on a users repository rather
// than users.UserService.
func NewStaffAssignmentService(
	logger logger.Logger,
	repo repository.Repository[EventStaffAssignment, uuid.UUID],
	eventRepo repository.Repository[events.Event, uuid.UUID],
	userRepo repository.Repository[users.User, uuid.UUID],
	roleRepo repository.Repository[roles.Role, uuid.UUID],
) StaffAssignmentService {
	return &staffAssignmentServiceImpl{
		repo: repo, eventRepo: eventRepo, userRepo: userRepo, roleRepo: roleRepo, logger: logger,
	}
}

// CreateAssignmentInput is the input for assigning a user to an event with
// an event-scoped role. event_id is taken from the URL and tenant_id from
// the caller's JWT claim; neither is part of this input (see
// docs/FEATURES.md#staffing).
//
// swagger:model CreateAssignmentInput
type CreateAssignmentInput struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
	RoleID uuid.UUID `json:"role_id" validate:"required"`
}

// UpdateAssignmentInput is the input for changing an existing assignment's
// role. event_id and user_id are immutable after creation — re-assigning a
// different user is a remove (Delete) plus a new assignment (Create), not
// an update (see docs/STAFFING_RBAC.md ss5).
//
// swagger:model UpdateAssignmentInput
type UpdateAssignmentInput struct {
	RoleID *uuid.UUID `json:"role_id,omitempty"`
}

// loadTenantScopedEvent loads eventID and verifies it belongs to the
// caller's tenant (per the "tenant_id" JWT claim). Returns errorz.NotFound
// both when the event doesn't exist and when it belongs to a different
// tenant — the same not-found-not-forbidden convention as
// WorkflowStepService, so a caller cannot probe for another tenant's event
// IDs (see docs/STAFFING_RBAC.md ss6).
func (s *staffAssignmentServiceImpl) loadTenantScopedEvent(
	ctx context.Context, eventID uuid.UUID,
) (*events.Event, error) {
	tenantID, ok := authz.TenantIDFromContext(ctx)
	if !ok {
		return nil, errorz.Unauthorized().WithMessage("no tenant claim present")
	}
	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorz.NotFound().WithMessage("event not found")
		}
		s.logger.ErrorWithContext(ctx, "staff assignment event lookup failed",
			logger.F("event_id", eventID), logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to look up event")
	}
	if event.TenantID != tenantID {
		return nil, errorz.NotFound().WithMessage("event not found")
	}
	return event, nil
}

// hasActiveAssignment reports whether userID currently has a non-deleted
// assignment on eventID. The audit repository decorator already injects
// "deleted_at IS NULL" into every List, so a match here means an active
// duplicate. go-sdk's Postgres error mapping does not translate a 23505
// unique_violation into repository.ErrAlreadyExists (see
// go-sdk/lib/repository/sql/helpers.go ConvertSQLError), so this
// pre-check is necessary — Create still keeps an errors.Is(ErrAlreadyExists)
// fallback for the race window between this check and the insert.
func (s *staffAssignmentServiceImpl) hasActiveAssignment(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	opts := &repository.ListOptions{
		Filter: repository.Filter{Conditions: []repository.FilterCondition{
			{Field: "event_id", Operator: repository.FilterOperatorEq, Value: eventID},
			{Field: "user_id", Operator: repository.FilterOperatorEq, Value: userID},
		}},
		Pagination: repository.Pagination{Limit: 1},
		SkipCount:  true,
	}
	items, _, err := s.repo.List(ctx, opts)
	if err != nil {
		s.logger.ErrorWithContext(ctx, "staff assignment duplicate check failed",
			logger.F("event_id", eventID), logger.F("user_id", userID), logger.F("error", err))
		return false, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to check existing staff assignment")
	}
	return len(items) > 0, nil
}

// Create assigns in.UserID to eventID with in.RoleID. Validates, in order:
// the event belongs to the caller's tenant; the target user exists and
// belongs to the same tenant; the role exists and is event-scoped; and no
// active assignment already exists for this (event, user) pair. ID is
// generated by the service; created_at/updated_at are stamped by the audit
// repository decorator.
func (s *staffAssignmentServiceImpl) Create(
	ctx context.Context, eventID uuid.UUID, in CreateAssignmentInput,
) (*EventStaffAssignment, error) {
	event, err := s.loadTenantScopedEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByID(ctx, in.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorz.NotFound().WithMessage("user not found")
		}
		s.logger.ErrorWithContext(ctx, "staff assignment user lookup failed",
			logger.F("user_id", in.UserID), logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to look up user")
	}
	if user.TenantID != event.TenantID {
		return nil, errorz.NotFound().WithMessage("user not found")
	}

	if err := s.validateEventScopeRole(ctx, in.RoleID); err != nil {
		return nil, err
	}

	active, err := s.hasActiveAssignment(ctx, eventID, in.UserID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, errorz.Conflict().WithMessage("user is already assigned to this event")
	}

	entity := &EventStaffAssignment{ID: uuid.New(), EventID: eventID, UserID: in.UserID, RoleID: in.RoleID}
	if err := s.repo.Create(ctx, entity); err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, errorz.Conflict().WithMessage("user is already assigned to this event")
		}
		if errors.Is(err, repository.ErrInvalidEntity) {
			return nil, errorz.UnprocessableEntity().WithMessage("invalid staff assignment data")
		}
		s.logger.ErrorWithContext(ctx, "staff assignment create failed", logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to create staff assignment")
	}

	s.logger.InfoWithContext(ctx, "staff assignment created", logger.F("id", entity.ID), logger.F("event_id", eventID))
	return entity, nil
}

// validateEventScopeRole loads roleID and returns errorz.NotFound if it
// doesn't exist, or errorz.BadRequest if it exists but isn't an
// event-scope role.
func (s *staffAssignmentServiceImpl) validateEventScopeRole(ctx context.Context, roleID uuid.UUID) error {
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return errorz.NotFound().WithMessage("role not found")
		}
		s.logger.ErrorWithContext(ctx, "staff assignment role lookup failed",
			logger.F("role_id", roleID), logger.F("error", err))
		return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to look up role")
	}
	if role.Scope != roles.ScopeEvent {
		return errorz.BadRequest().WithMessage("role_id must reference an event-scope role")
	}
	return nil
}

// GetByID returns a staff assignment by ID scoped to eventID. Returns
// errorz.NotFound if the event doesn't belong to the caller's tenant, or if
// the assignment doesn't exist, is soft-deleted, or belongs to a different
// event.
func (s *staffAssignmentServiceImpl) GetByID(
	ctx context.Context, eventID, id uuid.UUID,
) (*EventStaffAssignment, error) {
	if _, err := s.loadTenantScopedEvent(ctx, eventID); err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorz.NotFound().WithMessage("staff assignment not found")
		}
		s.logger.ErrorWithContext(ctx, "staff assignment get failed", logger.F("id", id), logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to get staff assignment")
	}
	if entity.EventID != eventID {
		return nil, errorz.NotFound().WithMessage("staff assignment not found")
	}
	return entity, nil
}

// Update changes an existing assignment's role. event_id/user_id are
// immutable and not part of UpdateAssignmentInput. When RoleID is provided
// it is re-validated as an event-scope role. The updated_at field is set by
// the audit repository decorator.
func (s *staffAssignmentServiceImpl) Update(
	ctx context.Context, eventID, id uuid.UUID, in UpdateAssignmentInput,
) (*EventStaffAssignment, error) {
	entity, err := s.GetByID(ctx, eventID, id)
	if err != nil {
		return nil, err
	}

	if in.RoleID != nil {
		if err := s.validateEventScopeRole(ctx, *in.RoleID); err != nil {
			return nil, err
		}
		entity.RoleID = *in.RoleID
	}

	if err := s.repo.Update(ctx, id, entity); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorz.NotFound().WithMessage("staff assignment not found")
		}
		s.logger.ErrorWithContext(ctx, "staff assignment update failed", logger.F("id", id), logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to update staff assignment")
	}

	s.logger.InfoWithContext(ctx, "staff assignment updated", logger.F("id", id))
	return entity, nil
}

// Delete soft-deletes a staff assignment scoped to eventID. The audit
// repository decorator handles setting deleted_at and updated_at.
// Removing and later re-assigning the same user to the same event is
// allowed (migration 000015's partial unique index) — it creates a new
// row, preserving the deleted one as staffing history.
func (s *staffAssignmentServiceImpl) Delete(ctx context.Context, eventID, id uuid.UUID) error {
	if _, err := s.GetByID(ctx, eventID, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return errorz.NotFound().WithMessage("staff assignment not found")
		}
		s.logger.ErrorWithContext(ctx, "staff assignment delete failed", logger.F("id", id), logger.F("error", err))
		return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to delete staff assignment")
	}
	s.logger.InfoWithContext(ctx, "staff assignment deleted", logger.F("id", id))
	return nil
}

// List returns the staff assignments for eventID with filter, sort, and
// pagination from query.ListParams. The event_id scope is always
// server-injected regardless of query filters — there is no cross-event
// staff listing (see docs/STAFFING_RBAC.md ss5).
func (s *staffAssignmentServiceImpl) List(
	ctx context.Context, eventID uuid.UUID, params *query.ListParams,
) (*common.PageResponse[EventStaffAssignment], error) {
	if _, err := s.loadTenantScopedEvent(ctx, eventID); err != nil {
		return nil, err
	}

	opts := listParamsToListOptions(params)
	opts.Filter.Conditions = append(opts.Filter.Conditions, repository.FilterCondition{
		Field:    "event_id",
		Operator: repository.FilterOperatorEq,
		Value:    eventID,
	})

	items, total, err := s.repo.List(ctx, opts)
	if err != nil {
		s.logger.ErrorWithContext(ctx, "staff assignment list failed", logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to list staff assignments")
	}
	return common.NewPageResponse(items, total, params.Page, params.Size), nil
}

// listParamsToListOptions converts query.ListParams to repository.ListOptions.
func listParamsToListOptions(params *query.ListParams) *repository.ListOptions {
	if params == nil {
		return &repository.ListOptions{}
	}

	page, size := params.Page, params.Size
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	offset := (page - 1) * size

	var conditions []repository.FilterCondition
	for field, value := range params.Filters {
		conditions = append(conditions, repository.FilterCondition{
			Field:    field,
			Operator: repository.FilterOperatorEq,
			Value:    value,
		})
	}

	var sorts []repository.Sort
	for _, s := range params.Sorts {
		dir := repository.SortAsc
		if s.Direction == common.SortDesc {
			dir = repository.SortDesc
		}
		sorts = append(sorts, repository.Sort{Field: s.Field, Direction: dir})
	}

	return &repository.ListOptions{
		Filter:     repository.Filter{Conditions: conditions},
		Pagination: repository.Pagination{Limit: size, Offset: offset},
		Sorts:      sorts,
	}
}
