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

			checker := NewChecker(logger.NewNoOp(), resolver)
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

			checker := NewChecker(logger.NewNoOp(), resolver)
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
			checker := NewChecker(logger.NewNoOp(), resolver)

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
