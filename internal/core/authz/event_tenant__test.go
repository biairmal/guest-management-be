package authz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkauth "github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	mockauthz "github.com/biairmal/guest-management-be/mocks/authz"
)

// TestRequireEventInTenant covers B16 AC11: an {event_id} route for an event in
// another tenant must answer 404 — never 403, never the handler's data — so
// another tenant's event ids cannot be probed.
func TestRequireEventInTenant(t *testing.T) {
	callerTenant := uuid.New()
	otherTenant := uuid.New()
	eventID := uuid.New()

	tests := []struct {
		name string

		eventIDParam string
		ctx          context.Context //nolint:containedctx // table-driven fixture, not a stored context
		resolverRes  uuid.UUID
		resolverOK   bool
		resolverErr  error
		expectsCall  bool
		wantStatus   int
		wantNext     bool
	}{
		{
			name:         "no event_id param -> 400",
			ctx:          ctxWithClaim("tenant_id", callerTenant.String()),
			eventIDParam: "",
			wantStatus:   http.StatusBadRequest,
		},
		{
			name:         "malformed event_id -> 400",
			ctx:          ctxWithClaim("tenant_id", callerTenant.String()),
			eventIDParam: "not-a-uuid",
			wantStatus:   http.StatusBadRequest,
		},
		{
			name:         "no tenant claim -> 401, resolver untouched",
			ctx:          context.Background(),
			eventIDParam: eventID.String(),
			wantStatus:   http.StatusUnauthorized,
		},
		{
			name:         "resolver error -> 500",
			ctx:          ctxWithClaim("tenant_id", callerTenant.String()),
			eventIDParam: eventID.String(),
			resolverErr:  errors.New("boom"),
			expectsCall:  true,
			wantStatus:   http.StatusInternalServerError,
		},
		{
			name:         "unknown event -> 404",
			ctx:          ctxWithClaim("tenant_id", callerTenant.String()),
			eventIDParam: eventID.String(),
			resolverOK:   false,
			expectsCall:  true,
			wantStatus:   http.StatusNotFound,
		},
		{
			name:         "event in another tenant -> 404, not 403",
			ctx:          ctxWithClaim("tenant_id", callerTenant.String()),
			eventIDParam: eventID.String(),
			resolverRes:  otherTenant,
			resolverOK:   true,
			expectsCall:  true,
			wantStatus:   http.StatusNotFound,
		},
		{
			name:         "event in the caller's tenant -> next handler called",
			ctx:          ctxWithClaim("tenant_id", callerTenant.String()),
			eventIDParam: eventID.String(),
			resolverRes:  callerTenant,
			resolverOK:   true,
			expectsCall:  true,
			wantStatus:   http.StatusOK,
			wantNext:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			resolver := mockauthz.NewMockEventTenantResolver(ctrl)
			if tt.expectsCall {
				resolver.EXPECT().EventTenantID(gomock.Any(), eventID).
					Return(tt.resolverRes, tt.resolverOK, tt.resolverErr)
			}

			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			mw := RequireEventInTenant(logger.NewNoOp(), resolver)
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

// TestRequireEventInTenant_RunsBeforeRequirePermission pins the ordering AC11
// depends on: chained the way the route setups chain them, an event outside the
// caller's tenant answers 404 even though the caller also lacks the permission
// that would otherwise produce 403.
func TestRequireEventInTenant_RunsBeforeRequirePermission(t *testing.T) {
	ctrl := gomock.NewController(t)
	eventID := uuid.New()
	roleID := uuid.New()

	ctx := sdkauth.ContextWithClaims(context.Background(), sdkauth.NewClaims(map[string]any{
		"sub": uuid.NewString(), "tenant_id": uuid.NewString(), "role_id": roleID.String(),
	}))

	tenantResolver := mockauthz.NewMockEventTenantResolver(ctrl)
	tenantResolver.EXPECT().EventTenantID(gomock.Any(), eventID).Return(uuid.New(), true, nil)
	// The permission resolver must never be reached: the tenant check short-circuits.
	permissions := mockauthz.NewMockPermissionResolver(ctrl)
	checker := NewChecker(logger.NewNoOp(), permissions, nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	chain := RequireEventInTenant(logger.NewNoOp(), tenantResolver)(
		RequirePermission(checker, "manage_guests")(next),
	)

	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, requestWithEventIDParam(ctx, eventID.String()))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (the tenant check must win over the 403)", rec.Code)
	}
}
