package config

import "github.com/biairmal/go-sdk/lib/ratelimit"

// RateLimitConfig wraps go-sdk's ratelimit.Config with an app-level on/off
// switch. When disabled, main.go passes a nil ratelimit.Limiter to
// middleware.RateLimit, which then lets every request through unchanged.
type RateLimitConfig struct {
	Enabled   bool             `mapstructure:"enabled"`
	RateLimit ratelimit.Config `mapstructure:"ratelimit"`
}

// Validate validates the rate-limit config only when rate limiting is enabled.
func (c *RateLimitConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	return c.RateLimit.Validate()
}
