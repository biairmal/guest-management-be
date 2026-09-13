package authz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkauth "github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	mockauthz "github.com/biairmal/guest-management-be/mocks/authz"
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

func ctxWithClaim(key, value string) context.Context {
	claims := sdkauth.NewClaims(map[string]any{"sub": uuid.NewString(), key: value})
	return sdkauth.ContextWithClaims(context.Background(), claims)
}

func TestRoleIDFromContext(t *testing.T) {
	roleID := uuid.New()

	tests := []struct {
		name   string
		ctx    context.Context //nolint:containedctx // table-driven fixture, not a stored context
		wantID uuid.UUID
		wantOK bool
	}{
		{name: "absent claims", ctx: context.Background(), wantOK: false},
		{name: "present and valid", ctx: ctxWithClaim("role_id", roleID.String()), wantID: roleID, wantOK: true},
		{name: "present but malformed", ctx: ctxWithClaim("role_id", "not-a-uuid"), wantOK: false},
		{name: "different claim present", ctx: ctxWithClaim("tenant_id", roleID.String()), wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := RoleIDFromContext(tt.ctx)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.wantID {
				t.Errorf("id = %v, want %v", got, tt.wantID)
			}
		})
	}
}

func TestTenantIDFromContext(t *testing.T) {
	tenantID := uuid.New()

	tests := []struct {
		name   string
		ctx    context.Context //nolint:containedctx // table-driven fixture, not a stored context
		wantID uuid.UUID
		wantOK bool
	}{
		{name: "absent claims", ctx: context.Background(), wantOK: false},
		{name: "present and valid", ctx: ctxWithClaim("tenant_id", tenantID.String()), wantID: tenantID, wantOK: true},
		{name: "present but malformed", ctx: ctxWithClaim("tenant_id", "not-a-uuid"), wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := TenantIDFromContext(tt.ctx)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.wantID {
				t.Errorf("id = %v, want %v", got, tt.wantID)
			}
		})
	}
}

func TestUserIDFromContext(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name   string
		ctx    context.Context //nolint:containedctx // table-driven fixture, not a stored context
		wantID uuid.UUID
		wantOK bool
	}{
		{name: "absent claims", ctx: context.Background(), wantOK: false},
		{
			name: "present and valid", wantID: userID, wantOK: true,
			ctx: sdkauth.ContextWithClaims(context.Background(), sdkauth.NewClaims(map[string]any{"sub": userID.String()})),
		},
		{
			name: "present but malformed", wantOK: false,
			ctx: sdkauth.ContextWithClaims(context.Background(), sdkauth.NewClaims(map[string]any{"sub": "not-a-uuid"})),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := UserIDFromContext(tt.ctx)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.wantID {
				t.Errorf("id = %v, want %v", got, tt.wantID)
			}
		})
	}
}

func TestChecker_Require(t *testing.T) {
	roleID := uuid.New()

	tests := []struct {
		name        string
		ctx         context.Context //nolint:containedctx // table-driven fixture, not a stored context
		resolverRes []string
		resolverErr error
		expectsCall bool
		wantErr     string
	}{
		{name: "no role claim maps to 401", ctx: context.Background(), wantErr: errorz.CodeUnauthorized},
		{
			name: "resolver error maps to 500", ctx: ctxWithClaim("role_id", roleID.String()),
			resolverErr: errors.New("boom"), expectsCall: true, wantErr: errorz.CodeInternal,
		},
		{
			name: "role lacks permission maps to 403", ctx: ctxWithClaim("role_id", roleID.String()),
			resolverRes: []string{"check_in"}, expectsCall: true, wantErr: errorz.CodeForbidden,
		},
		{
			name: "role holds permission", ctx: ctxWithClaim("role_id", roleID.String()),
			resolverRes: []string{"manage_staff"}, expectsCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			resolver := mockauthz.NewMockPermissionResolver(ctrl)
			if tt.expectsCall {
				resolver.EXPECT().ResolvePermissions(gomock.Any(), roleID).Return(tt.resolverRes, tt.resolverErr)
			}

			checker := NewChecker(logger.NewNoOp(), resolver, nil)
			err := checker.Require(tt.ctx, "manage_staff")
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestChecker_Has(t *testing.T) {
	roleID := uuid.New()

	tests := []struct {
		name        string
		ctx         context.Context //nolint:containedctx // table-driven fixture, not a stored context
		resolverRes []string
		resolverErr error
		expectsCall bool
		want        bool
		wantErr     bool
	}{
		{name: "no role claim returns false, no error", ctx: context.Background(), want: false},
		{
			name: "resolver error propagates", ctx: ctxWithClaim("role_id", roleID.String()),
			resolverErr: errors.New("boom"), expectsCall: true, wantErr: true,
		},
		{
			name: "role lacks permission returns false", ctx: ctxWithClaim("role_id", roleID.String()),
			resolverRes: []string{"check_in"}, expectsCall: true, want: false,
		},
		{
			name: "role holds permission returns true", ctx: ctxWithClaim("role_id", roleID.String()),
			resolverRes: []string{"manage_staff"}, expectsCall: true, want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			resolver := mockauthz.NewMockPermissionResolver(ctrl)
			if tt.expectsCall {
				resolver.EXPECT().ResolvePermissions(gomock.Any(), roleID).Return(tt.resolverRes, tt.resolverErr)
			}

			checker := NewChecker(logger.NewNoOp(), resolver, nil)
			got, err := checker.Has(tt.ctx, "manage_staff")
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

// ctxWithRoleAndUser builds a context carrying both a "role_id" and a
// subject ("sub") claim, the shape RequireForEvent needs to run its
// event-scoped fallback (it needs the caller's user ID in addition to their
// system role_id).
func ctxWithRoleAndUser(roleID, userID uuid.UUID) context.Context {
	claims := sdkauth.NewClaims(map[string]any{"sub": userID.String(), "role_id": roleID.String()})
	return sdkauth.ContextWithClaims(context.Background(), claims)
}

func TestChecker_RequireForEvent(t *testing.T) {
	systemRoleID := uuid.New()
	eventRoleID := uuid.New()
	userID := uuid.New()
	eventID := uuid.New()

	tests := []struct {
		name string
		ctx  context.Context //nolint:containedctx // table-driven fixture, not a stored context

		systemPermissions []string
		systemResolverErr error

		expectEventLookup   bool
		eventFound          bool
		eventResolverRoleID uuid.UUID
		eventResolverErr    error

		expectEventPermissionResolve bool
		eventPermissions             []string
		eventPermissionResolveErr    error

		wantErr string
	}{
		{
			name: "system role already grants permission - no event fallback (AC4)",
			ctx:  ctxWithRoleAndUser(systemRoleID, userID), systemPermissions: []string{"manage_staff"},
		},
		{
			name: "resolver error on system check maps to 500, no event fallback",
			ctx:  ctxWithRoleAndUser(systemRoleID, userID), systemResolverErr: errors.New("boom"),
			wantErr: errorz.CodeInternal,
		},
		{
			name: "no role claim, no sub claim - system 401 with no fallback possible",
			ctx:  context.Background(), wantErr: errorz.CodeUnauthorized,
		},
		{
			name: "system check fails, no assignment on event - lookup miss returns original 403 (AC2/AC3)",
			ctx:  ctxWithRoleAndUser(systemRoleID, userID), systemPermissions: []string{"check_in"},
			expectEventLookup: true, eventFound: false, wantErr: errorz.CodeForbidden,
		},
		{
			name: "system check fails, active assignment grants permission via event role (AC1)",
			ctx:  ctxWithRoleAndUser(systemRoleID, userID), systemPermissions: []string{"check_in"},
			expectEventLookup: true, eventFound: true, eventResolverRoleID: eventRoleID,
			expectEventPermissionResolve: true, eventPermissions: []string{"manage_staff"},
		},
		{
			name: "system check fails, assignment found but its role lacks permission - explicit deny, not a miss",
			ctx:  ctxWithRoleAndUser(systemRoleID, userID), systemPermissions: []string{"check_in"},
			expectEventLookup: true, eventFound: true, eventResolverRoleID: eventRoleID,
			expectEventPermissionResolve: true, eventPermissions: []string{"check_in"},
			wantErr: errorz.CodeForbidden,
		},
		{
			name: "event role resolver failure maps to 500",
			ctx:  ctxWithRoleAndUser(systemRoleID, userID), systemPermissions: []string{"check_in"},
			expectEventLookup: true, eventResolverErr: errors.New("boom"), wantErr: errorz.CodeInternal,
		},
		{
			name: "resolving the event role's permissions fails maps to 500",
			ctx:  ctxWithRoleAndUser(systemRoleID, userID), systemPermissions: []string{"check_in"},
			expectEventLookup: true, eventFound: true, eventResolverRoleID: eventRoleID,
			expectEventPermissionResolve: true, eventPermissionResolveErr: errors.New("boom"), wantErr: errorz.CodeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			resolver := mockauthz.NewMockPermissionResolver(ctrl)
			eventResolver := mockauthz.NewMockEventRoleResolver(ctrl)

			if _, ok := RoleIDFromContext(tt.ctx); ok {
				resolver.EXPECT().ResolvePermissions(gomock.Any(), systemRoleID).
					Return(tt.systemPermissions, tt.systemResolverErr)
			}
			if tt.expectEventLookup {
				eventResolver.EXPECT().ResolveEventRoleID(gomock.Any(), eventID, userID).
					Return(tt.eventResolverRoleID, tt.eventFound, tt.eventResolverErr)
			}
			if tt.expectEventPermissionResolve {
				resolver.EXPECT().ResolvePermissions(gomock.Any(), tt.eventResolverRoleID).
					Return(tt.eventPermissions, tt.eventPermissionResolveErr)
			}

			checker := NewChecker(logger.NewNoOp(), resolver, eventResolver)
			err := checker.RequireForEvent(tt.ctx, eventID, "manage_staff")
			assertErrorzCode(t, err, tt.wantErr)
		})
	}
}

func TestRequirePermission(t *testing.T) {
	roleID := uuid.New()

	tests := []struct {
		name        string
		ctx         context.Context //nolint:containedctx // table-driven fixture, not a stored context
		resolverRes []string
		resolverErr error
		expectsCall bool
		wantStatus  int
		wantNext    bool
	}{
		{
			name: "no role claim -> 401", ctx: context.Background(),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "resolver error -> 500", ctx: ctxWithClaim("role_id", roleID.String()),
			resolverErr: errors.New("boom"), expectsCall: true, wantStatus: http.StatusInternalServerError,
		},
		{
			name: "missing permission -> 403", ctx: ctxWithClaim("role_id", roleID.String()),
			resolverRes: []string{"check_in"}, expectsCall: true, wantStatus: http.StatusForbidden,
		},
		{
			name: "has permission -> next handler called", ctx: ctxWithClaim("role_id", roleID.String()),
			resolverRes: []string{"manage_staff"}, expectsCall: true, wantStatus: http.StatusOK, wantNext: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			resolver := mockauthz.NewMockPermissionResolver(ctrl)
			if tt.expectsCall {
				resolver.EXPECT().ResolvePermissions(gomock.Any(), roleID).Return(tt.resolverRes, tt.resolverErr)
			}
			checker := NewChecker(logger.NewNoOp(), resolver, nil)

			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			mw := RequirePermission(checker, "manage_staff")
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody).WithContext(tt.ctx)
			mw(next).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if called != tt.wantNext {
				t.Errorf("next called = %v, want %v", called, tt.wantNext)
			}
		})
	}
}

// requestWithEventIDParam builds a request carrying ctx and a chi
// {event_id} URL param set to raw (skipped when raw is empty), the shape
// RequirePermission reads to decide whether to run RequireForEvent.
func requestWithEventIDParam(ctx context.Context, raw string) *http.Request {
	rctx := chi.NewRouteContext()
	if raw != "" {
		rctx.URLParams.Add("event_id", raw)
	}
	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody).WithContext(ctx)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestRequirePermission_EventScoped(t *testing.T) {
	systemRoleID := uuid.New()
	eventRoleID := uuid.New()
	userID := uuid.New()
	eventID := uuid.New()

	tests := []struct {
		name string

		eventIDParam      string
		ctx               context.Context //nolint:containedctx // table-driven fixture, not a stored context
		systemPermissions []string

		expectEventLookup bool
		eventFound        bool
		eventPermissions  []string

		wantStatus int
		wantNext   bool
	}{
		{
			name: "no event_id param uses unscoped Require unchanged",
			ctx:  ctxWithClaim("role_id", systemRoleID.String()), systemPermissions: []string{"manage_staff"},
			wantStatus: http.StatusOK, wantNext: true,
		},
		{
			name:         "malformed event_id param falls back to unscoped Require",
			eventIDParam: "not-a-uuid",
			ctx:          ctxWithClaim("role_id", systemRoleID.String()), systemPermissions: []string{"manage_staff"},
			wantStatus: http.StatusOK, wantNext: true,
		},
		{
			name:         "system role already grants permission - no event fallback (AC4)",
			eventIDParam: eventID.String(),
			ctx:          ctxWithRoleAndUser(systemRoleID, userID), systemPermissions: []string{"manage_staff"},
			wantStatus: http.StatusOK, wantNext: true,
		},
		{
			name:         "event-scoped assignment alone authorizes (AC1)",
			eventIDParam: eventID.String(),
			ctx:          ctxWithRoleAndUser(systemRoleID, userID), systemPermissions: []string{"check_in"},
			expectEventLookup: true, eventFound: true, eventPermissions: []string{"manage_staff"},
			wantStatus: http.StatusOK, wantNext: true,
		},
		{
			name:         "no assignment on this event -> 403 (AC2/AC3)",
			eventIDParam: eventID.String(),
			ctx:          ctxWithRoleAndUser(systemRoleID, userID), systemPermissions: []string{"check_in"},
			expectEventLookup: true, eventFound: false,
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			resolver := mockauthz.NewMockPermissionResolver(ctrl)
			eventResolver := mockauthz.NewMockEventRoleResolver(ctrl)

			resolver.EXPECT().ResolvePermissions(gomock.Any(), systemRoleID).Return(tt.systemPermissions, nil)
			if tt.expectEventLookup {
				eventResolver.EXPECT().ResolveEventRoleID(gomock.Any(), eventID, userID).
					Return(eventRoleID, tt.eventFound, nil)
			}
			if tt.eventFound {
				resolver.EXPECT().ResolvePermissions(gomock.Any(), eventRoleID).Return(tt.eventPermissions, nil)
			}

			checker := NewChecker(logger.NewNoOp(), resolver, eventResolver)

			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			mw := RequirePermission(checker, "manage_staff")
			rec := httptest.NewRecorder()
			mw(next).ServeHTTP(rec, requestWithEventIDParam(tt.ctx, tt.eventIDParam))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if called != tt.wantNext {
				t.Errorf("next called = %v, want %v", called, tt.wantNext)
			}
		})
	}
}
