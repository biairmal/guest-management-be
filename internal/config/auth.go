package config

import (
	"time"

	sdkauth "github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/errorz"
)

// AuthConfig wraps go-sdk's auth.Config (token issuing/validation + route
// policy) with RefreshTTL. The stateless dual-JWT scheme in
// internal/features/auth issues a second, longer-lived refresh token
// alongside every access token; go-sdk's Issuer only carries one default TTL
// (Token.Issuer.DefaultTTL, used here for access tokens), so the refresh
// token's lifetime is this app-specific addition.
type AuthConfig struct {
	Token      sdkauth.Config `mapstructure:"token"`
	RefreshTTL time.Duration  `mapstructure:"refresh_ttl"`
}

// DefaultAuthConfig returns go-sdk's local-HS256 auth default plus a 7-day
// refresh token lifetime.
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{Token: sdkauth.DefaultConfig(), RefreshTTL: 7 * 24 * time.Hour}
}

// Validate validates the token config and the refresh TTL.
func (c *AuthConfig) Validate() error {
	if err := c.Token.Validate(); err != nil {
		return err
	}
	if c.RefreshTTL <= 0 {
		return errorz.BadRequest().WithMessage("auth: refresh_ttl must be positive")
	}
	return nil
}
