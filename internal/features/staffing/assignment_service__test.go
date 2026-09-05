package staffing

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

	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/roles"
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

// ctxWithTenant returns a context carrying tenantID as the "tenant_id" JWT
// claim, the mechanism authz.TenantIDFromContext reads.
func ctxWithTenant(tenantID uuid.UUID) context.Context {
	claims := sdkauth.NewClaims(map[string]any{"sub": uuid.NewString(), "tenant_id": tenantID.String()})
	return sdkauth.ContextWithClaims(context.Background(), claims)
}

func newService(
	assignmentRepo repository.Repository[EventStaffAssignment, uuid.UUID],
	eventRepo repository.Repository[events.Event, uuid.UUID],
	userRepo repository.Repository[users.User, uuid.UUID],
	roleRepo repository.Repository[roles.Role, uuid.UUID],
) StaffAssignmentService {
	return NewStaffAssignmentService(logger.NewNoOp(), assignmentRepo, eventRepo, userRepo, roleRepo)
}

func TestStaffAssignmentService_Create(t *testing.T) {
	tenantID := uuid.New()
	eventID := uuid.New()
	userID := uuid.New()
	roleID := uuid.New()

	t.Run("no tenant claim maps to 401", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		userRepo := mockrepository.NewMockRepository[users.User, uuid.UUID](ctrl)
		roleRepo := mockrepository.NewMockRepository[roles.Role, uuid.UUID](ctrl)
		svc := newService(assignmentRepo, eventRepo, userRepo, roleRepo)

		_, err := svc.Create(context.Background(), eventID, CreateAssignmentInput{UserID: userID, RoleID: roleID})
		assertErrorzCode(t, err, errorz.CodeUnauthorized)
	})

	t.Run("event not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		userRepo := mockrepository.NewMockRepository[users.User, uuid.UUID](ctrl)
		roleRepo := mockrepository.NewMockRepository[roles.Role, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(nil, repository.ErrNotFound)
		svc := newService(assignmentRepo, eventRepo, userRepo, roleRepo)

		_, err := svc.Create(ctxWithTenant(tenantID), eventID, CreateAssignmentInput{UserID: userID, RoleID: roleID})
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("event wrong tenant maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		userRepo := mockrepository.NewMockRepository[users.User, uuid.UUID](ctrl)
		roleRepo := mockrepository.NewMockRepository[roles.Role, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: uuid.New()}, nil)
		svc := newService(assignmentRepo, eventRepo, userRepo, roleRepo)

		_, err := svc.Create(ctxWithTenant(tenantID), eventID, CreateAssignmentInput{UserID: userID, RoleID: roleID})
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("user not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		userRepo := mockrepository.NewMockRepository[users.User, uuid.UUID](ctrl)
		roleRepo := mockrepository.NewMockRepository[roles.Role, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: tenantID}, nil)
		userRepo.EXPECT().GetByID(gomock.Any(), userID).Return(nil, repository.ErrNotFound)
		svc := newService(assignmentRepo, eventRepo, userRepo, roleRepo)

		_, err := svc.Create(ctxWithTenant(tenantID), eventID, CreateAssignmentInput{UserID: userID, RoleID: roleID})
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("user wrong tenant maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		userRepo := mockrepository.NewMockRepository[users.User, uuid.UUID](ctrl)
		roleRepo := mockrepository.NewMockRepository[roles.Role, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: tenantID}, nil)
		userRepo.EXPECT().GetByID(gomock.Any(), userID).Return(&users.User{ID: userID, TenantID: uuid.New()}, nil)
		svc := newService(assignmentRepo, eventRepo, userRepo, roleRepo)

		_, err := svc.Create(ctxWithTenant(tenantID), eventID, CreateAssignmentInput{UserID: userID, RoleID: roleID})
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("role not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		userRepo := mockrepository.NewMockRepository[users.User, uuid.UUID](ctrl)
		roleRepo := mockrepository.NewMockRepository[roles.Role, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: tenantID}, nil)
		userRepo.EXPECT().GetByID(gomock.Any(), userID).Return(&users.User{ID: userID, TenantID: tenantID}, nil)
		roleRepo.EXPECT().GetByID(gomock.Any(), roleID).Return(nil, repository.ErrNotFound)
		svc := newService(assignmentRepo, eventRepo, userRepo, roleRepo)

		_, err := svc.Create(ctxWithTenant(tenantID), eventID, CreateAssignmentInput{UserID: userID, RoleID: roleID})
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("role wrong scope maps to 400", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		userRepo := mockrepository.NewMockRepository[users.User, uuid.UUID](ctrl)
		roleRepo := mockrepository.NewMockRepository[roles.Role, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: tenantID}, nil)
		userRepo.EXPECT().GetByID(gomock.Any(), userID).Return(&users.User{ID: userID, TenantID: tenantID}, nil)
		roleRepo.EXPECT().GetByID(gomock.Any(), roleID).Return(&roles.Role{ID: roleID, Scope: roles.ScopeSystem}, nil)
		svc := newService(assignmentRepo, eventRepo, userRepo, roleRepo)

		_, err := svc.Create(ctxWithTenant(tenantID), eventID, CreateAssignmentInput{UserID: userID, RoleID: roleID})
		assertErrorzCode(t, err, errorz.CodeBadRequest)
	})

	t.Run("active duplicate maps to 409", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		userRepo := mockrepository.NewMockRepository[users.User, uuid.UUID](ctrl)
		roleRepo := mockrepository.NewMockRepository[roles.Role, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: tenantID}, nil)
		userRepo.EXPECT().GetByID(gomock.Any(), userID).Return(&users.User{ID: userID, TenantID: tenantID}, nil)
		roleRepo.EXPECT().GetByID(gomock.Any(), roleID).Return(&roles.Role{ID: roleID, Scope: roles.ScopeEvent}, nil)
		assignmentRepo.EXPECT().List(gomock.Any(), gomock.Any()).
			Return([]*EventStaffAssignment{{ID: uuid.New(), EventID: eventID, UserID: userID}}, int64(1), nil)
		svc := newService(assignmentRepo, eventRepo, userRepo, roleRepo)

		_, err := svc.Create(ctxWithTenant(tenantID), eventID, CreateAssignmentInput{UserID: userID, RoleID: roleID})
		assertErrorzCode(t, err, errorz.CodeConflict)
	})

	t.Run("duplicate check repo error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		userRepo := mockrepository.NewMockRepository[users.User, uuid.UUID](ctrl)
		roleRepo := mockrepository.NewMockRepository[roles.Role, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: tenantID}, nil)
		userRepo.EXPECT().GetByID(gomock.Any(), userID).Return(&users.User{ID: userID, TenantID: tenantID}, nil)
		roleRepo.EXPECT().GetByID(gomock.Any(), roleID).Return(&roles.Role{ID: roleID, Scope: roles.ScopeEvent}, nil)
		assignmentRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("boom"))
		svc := newService(assignmentRepo, eventRepo, userRepo, roleRepo)

		_, err := svc.Create(ctxWithTenant(tenantID), eventID, CreateAssignmentInput{UserID: userID, RoleID: roleID})
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	createHappyPathMocks := func(ctrl *gomock.Controller) (
		*mockrepository.MockRepository[EventStaffAssignment, uuid.UUID],
		*mockrepository.MockRepository[events.Event, uuid.UUID],
		*mockrepository.MockRepository[users.User, uuid.UUID],
		*mockrepository.MockRepository[roles.Role, uuid.UUID],
	) {
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		userRepo := mockrepository.NewMockRepository[users.User, uuid.UUID](ctrl)
		roleRepo := mockrepository.NewMockRepository[roles.Role, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: tenantID}, nil)
		userRepo.EXPECT().GetByID(gomock.Any(), userID).Return(&users.User{ID: userID, TenantID: tenantID}, nil)
		roleRepo.EXPECT().GetByID(gomock.Any(), roleID).Return(&roles.Role{ID: roleID, Scope: roles.ScopeEvent}, nil)
		assignmentRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		return assignmentRepo, eventRepo, userRepo, roleRepo
	}

	t.Run("create ErrAlreadyExists maps to 409 (race fallback)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo, eventRepo, userRepo, roleRepo := createHappyPathMocks(ctrl)
		assignmentRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrAlreadyExists)
		svc := newService(assignmentRepo, eventRepo, userRepo, roleRepo)

		_, err := svc.Create(ctxWithTenant(tenantID), eventID, CreateAssignmentInput{UserID: userID, RoleID: roleID})
		assertErrorzCode(t, err, errorz.CodeConflict)
	})

	t.Run("create ErrInvalidEntity maps to 422", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo, eventRepo, userRepo, roleRepo := createHappyPathMocks(ctrl)
		assignmentRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrInvalidEntity)
		svc := newService(assignmentRepo, eventRepo, userRepo, roleRepo)

		_, err := svc.Create(ctxWithTenant(tenantID), eventID, CreateAssignmentInput{UserID: userID, RoleID: roleID})
		assertErrorzCode(t, err, errorz.CodeUnprocessableEntity)
	})

	t.Run("create unexpected error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo, eventRepo, userRepo, roleRepo := createHappyPathMocks(ctrl)
		assignmentRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("boom"))
		svc := newService(assignmentRepo, eventRepo, userRepo, roleRepo)

		_, err := svc.Create(ctxWithTenant(tenantID), eventID, CreateAssignmentInput{UserID: userID, RoleID: roleID})
		assertErrorzCode(t, err, errorz.CodeInternal)
	})

	t.Run("happy path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo, eventRepo, userRepo, roleRepo := createHappyPathMocks(ctrl)
		assignmentRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		svc := newService(assignmentRepo, eventRepo, userRepo, roleRepo)

		got, err := svc.Create(ctxWithTenant(tenantID), eventID, CreateAssignmentInput{UserID: userID, RoleID: roleID})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if got.EventID != eventID || got.UserID != userID || got.RoleID != roleID {
			t.Errorf("got = %+v", got)
		}
	})
}

func TestStaffAssignmentService_GetByID(t *testing.T) {
	tenantID := uuid.New()
	eventID := uuid.New()
	id := uuid.New()

	t.Run("no tenant claim maps to 401", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		svc := newService(assignmentRepo, eventRepo, nil, nil)

		_, err := svc.GetByID(context.Background(), eventID, id)
		assertErrorzCode(t, err, errorz.CodeUnauthorized)
	})

	t.Run("event not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(nil, repository.ErrNotFound)
		svc := newService(assignmentRepo, eventRepo, nil, nil)

		_, err := svc.GetByID(ctxWithTenant(tenantID), eventID, id)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("assignment not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: tenantID}, nil)
		assignmentRepo.EXPECT().GetByID(gomock.Any(), id).Return(nil, repository.ErrNotFound)
		svc := newService(assignmentRepo, eventRepo, nil, nil)

		_, err := svc.GetByID(ctxWithTenant(tenantID), eventID, id)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("assignment belongs to different event maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: tenantID}, nil)
		assignmentRepo.EXPECT().GetByID(gomock.Any(), id).Return(&EventStaffAssignment{ID: id, EventID: uuid.New()}, nil)
		svc := newService(assignmentRepo, eventRepo, nil, nil)

		_, err := svc.GetByID(ctxWithTenant(tenantID), eventID, id)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("happy path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: tenantID}, nil)
		assignmentRepo.EXPECT().GetByID(gomock.Any(), id).Return(&EventStaffAssignment{ID: id, EventID: eventID}, nil)
		svc := newService(assignmentRepo, eventRepo, nil, nil)

		got, err := svc.GetByID(ctxWithTenant(tenantID), eventID, id)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.ID != id {
			t.Errorf("ID = %v, want %v", got.ID, id)
		}
	})
}

func TestStaffAssignmentService_Update(t *testing.T) {
	tenantID := uuid.New()
	eventID := uuid.New()
	id := uuid.New()
	newRoleID := uuid.New()

	baseGetMocks := func(ctrl *gomock.Controller) (
		*mockrepository.MockRepository[EventStaffAssignment, uuid.UUID],
		*mockrepository.MockRepository[events.Event, uuid.UUID],
	) {
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: tenantID}, nil)
		assignmentRepo.EXPECT().GetByID(gomock.Any(), id).Return(&EventStaffAssignment{ID: id, EventID: eventID}, nil)
		return assignmentRepo, eventRepo
	}

	t.Run("role not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo, eventRepo := baseGetMocks(ctrl)
		roleRepo := mockrepository.NewMockRepository[roles.Role, uuid.UUID](ctrl)
		roleRepo.EXPECT().GetByID(gomock.Any(), newRoleID).Return(nil, repository.ErrNotFound)
		svc := newService(assignmentRepo, eventRepo, nil, roleRepo)

		_, err := svc.Update(ctxWithTenant(tenantID), eventID, id, UpdateAssignmentInput{RoleID: &newRoleID})
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("role wrong scope maps to 400", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo, eventRepo := baseGetMocks(ctrl)
		roleRepo := mockrepository.NewMockRepository[roles.Role, uuid.UUID](ctrl)
		roleRepo.EXPECT().GetByID(gomock.Any(), newRoleID).Return(&roles.Role{ID: newRoleID, Scope: roles.ScopeSystem}, nil)
		svc := newService(assignmentRepo, eventRepo, nil, roleRepo)

		_, err := svc.Update(ctxWithTenant(tenantID), eventID, id, UpdateAssignmentInput{RoleID: &newRoleID})
		assertErrorzCode(t, err, errorz.CodeBadRequest)
	})

	t.Run("happy path role change", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo, eventRepo := baseGetMocks(ctrl)
		roleRepo := mockrepository.NewMockRepository[roles.Role, uuid.UUID](ctrl)
		roleRepo.EXPECT().GetByID(gomock.Any(), newRoleID).Return(&roles.Role{ID: newRoleID, Scope: roles.ScopeEvent}, nil)
		assignmentRepo.EXPECT().Update(gomock.Any(), id, gomock.Any()).Return(nil)
		svc := newService(assignmentRepo, eventRepo, nil, roleRepo)

		got, err := svc.Update(ctxWithTenant(tenantID), eventID, id, UpdateAssignmentInput{RoleID: &newRoleID})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if got.RoleID != newRoleID {
			t.Errorf("RoleID = %v, want %v", got.RoleID, newRoleID)
		}
	})

	t.Run("happy path without role change", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo, eventRepo := baseGetMocks(ctrl)
		assignmentRepo.EXPECT().Update(gomock.Any(), id, gomock.Any()).Return(nil)
		svc := newService(assignmentRepo, eventRepo, nil, nil)

		_, err := svc.Update(ctxWithTenant(tenantID), eventID, id, UpdateAssignmentInput{})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
	})

	t.Run("update repo not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo, eventRepo := baseGetMocks(ctrl)
		assignmentRepo.EXPECT().Update(gomock.Any(), id, gomock.Any()).Return(repository.ErrNotFound)
		svc := newService(assignmentRepo, eventRepo, nil, nil)

		_, err := svc.Update(ctxWithTenant(tenantID), eventID, id, UpdateAssignmentInput{})
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})
}

func TestStaffAssignmentService_Delete(t *testing.T) {
	tenantID := uuid.New()
	eventID := uuid.New()
	id := uuid.New()

	t.Run("get not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: tenantID}, nil)
		assignmentRepo.EXPECT().GetByID(gomock.Any(), id).Return(nil, repository.ErrNotFound)
		svc := newService(assignmentRepo, eventRepo, nil, nil)

		err := svc.Delete(ctxWithTenant(tenantID), eventID, id)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("happy path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: tenantID}, nil)
		assignmentRepo.EXPECT().GetByID(gomock.Any(), id).Return(&EventStaffAssignment{ID: id, EventID: eventID}, nil)
		assignmentRepo.EXPECT().Delete(gomock.Any(), id).Return(nil)
		svc := newService(assignmentRepo, eventRepo, nil, nil)

		if err := svc.Delete(ctxWithTenant(tenantID), eventID, id); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
	})
}

func TestStaffAssignmentService_List(t *testing.T) {
	tenantID := uuid.New()
	eventID := uuid.New()

	t.Run("no tenant claim maps to 401", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		svc := newService(assignmentRepo, eventRepo, nil, nil)

		params, err := query.ParseListParams(url.Values{}, query.ListParseConfig{})
		if err != nil {
			t.Fatalf("ParseListParams() error = %v", err)
		}
		_, err = svc.List(context.Background(), eventID, params)
		assertErrorzCode(t, err, errorz.CodeUnauthorized)
	})

	t.Run("event not found maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(nil, repository.ErrNotFound)
		svc := newService(assignmentRepo, eventRepo, nil, nil)

		params, err := query.ParseListParams(url.Values{}, query.ListParseConfig{})
		if err != nil {
			t.Fatalf("ParseListParams() error = %v", err)
		}
		_, err = svc.List(ctxWithTenant(tenantID), eventID, params)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("always injects event_id filter regardless of query params", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: tenantID}, nil)
		assignmentRepo.EXPECT().List(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, opts *repository.ListOptions) ([]*EventStaffAssignment, int64, error) {
				found := false
				for _, c := range opts.Filter.Conditions {
					if c.Field == "event_id" && c.Value == eventID {
						found = true
					}
				}
				if !found {
					t.Error("expected event_id filter condition to be present and match eventID")
				}
				return []*EventStaffAssignment{{ID: uuid.New(), EventID: eventID}}, int64(1), nil
			})
		svc := newService(assignmentRepo, eventRepo, nil, nil)

		// user_id is not in AllowedFilterFields here since we pass an empty
		// config; the assertion above focuses on the server-injected
		// event_id condition, which must be present regardless.
		params, err := query.ParseListParams(url.Values{"user_id": []string{uuid.NewString()}}, query.ListParseConfig{})
		if err != nil {
			t.Fatalf("ParseListParams() error = %v", err)
		}
		got, err := svc.List(ctxWithTenant(tenantID), eventID, params)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if got.Size != 20 {
			t.Errorf("Size = %d, want 20", got.Size)
		}
	})

	t.Run("repo error maps to 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		assignmentRepo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
		eventRepo := mockrepository.NewMockRepository[events.Event, uuid.UUID](ctrl)
		eventRepo.EXPECT().GetByID(gomock.Any(), eventID).Return(&events.Event{ID: eventID, TenantID: tenantID}, nil)
		assignmentRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("boom"))
		svc := newService(assignmentRepo, eventRepo, nil, nil)

		params, err := query.ParseListParams(url.Values{}, query.ListParseConfig{})
		if err != nil {
			t.Fatalf("ParseListParams() error = %v", err)
		}
		_, err = svc.List(ctxWithTenant(tenantID), eventID, params)
		assertErrorzCode(t, err, errorz.CodeInternal)
	})
}
