package event

import (
	"context"
	"errors"
	"testing"

	sdkauth "github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/google/uuid"
)

// callerTenantID is the tenant every test's caller belongs to — the
// "tenant_id" claim tenantCtx puts in the context.
var callerTenantID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

// otherTenantID is a second tenant, used to assert that its events read as
// not found rather than forbidden.
var otherTenantID = uuid.MustParse("22222222-2222-2222-2222-222222222222")

// tenantCtx returns a context carrying validated claims for a caller of
// callerTenantID — what httpkit/middleware.Auth puts there in production, and
// what every tenant-scoped service operation reads via authz.TenantIDFromContext.
func tenantCtx() context.Context {
	return sdkauth.ContextWithClaims(context.Background(), sdkauth.NewClaims(map[string]any{
		"sub": uuid.NewString(), "tenant_id": callerTenantID.String(),
	}))
}

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
