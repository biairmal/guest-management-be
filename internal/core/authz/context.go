package authz

import (
	"context"

	sdkauth "github.com/biairmal/go-sdk/lib/auth"
	"github.com/google/uuid"
)

// RoleIDFromContext returns the caller's system-level role ID from the
// "role_id" JWT claim (set by internal/features/auth on every issued
// access/refresh token) stored in ctx, and whether it was present and a
// valid UUID.
func RoleIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	return uuidClaim(ctx, "role_id")
}

// TenantIDFromContext returns the caller's tenant ID from the "tenant_id"
// JWT claim stored in ctx, and whether it was present and a valid UUID.
func TenantIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	return uuidClaim(ctx, "tenant_id")
}

// uuidClaim reads key from the validated auth.Claims stored in ctx (set by
// httpkit/middleware.Auth via sdkauth.ContextWithClaims) and parses it as a
// UUID. Returns (uuid.Nil, false) when Claims are absent, the key is
// missing, the value isn't a string, or it doesn't parse as a UUID.
func uuidClaim(ctx context.Context, key string) (uuid.UUID, bool) {
	claims, ok := sdkauth.ClaimsFromContext(ctx)
	if !ok {
		return uuid.Nil, false
	}
	raw, ok := claims.Get(key)
	if !ok {
		return uuid.Nil, false
	}
	s, ok := raw.(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}
