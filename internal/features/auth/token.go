// Package auth implements login and token refresh on top of go-sdk's auth
// package: it looks up credentials via the users feature's published
// UserService interface (see docs/ARCHITECTURE.md "Feature slice = future
// service boundary") and issues a stateless access/refresh JWT pair.
package auth

import (
	"context"

	sdkauth "github.com/biairmal/go-sdk/lib/auth"
)

// Token "type" claim values distinguishing the two JWTs issued together by
// Login (see issueTokenPair): access tokens authenticate API calls; refresh
// tokens exist only to mint a new pair via POST /api/v1/auth/refresh.
const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

// AccessOnlyValidator wraps base so that a token whose "type" claim isn't
// "access" (i.e. a refresh token) is rejected. Without this, a refresh
// token — signed by the same issuer — would validate successfully on any
// protected route, since go-sdk's Validator has no notion of token "type".
// Build the Validator passed to httpkit/middleware.Auth with this; Refresh,
// by contrast, validates with the unwrapped base Validator and checks for
// "type"=="refresh" itself.
func AccessOnlyValidator(base sdkauth.Validator) sdkauth.Validator {
	return sdkauth.ValidatorFunc(func(ctx context.Context, token string) (sdkauth.Claims, error) {
		claims, err := base.Validate(ctx, token)
		if err != nil {
			return nil, err
		}
		if typ, _ := claims.Get("type"); typ != tokenTypeAccess {
			return nil, sdkauth.ErrInvalidToken
		}
		return claims, nil
	})
}
