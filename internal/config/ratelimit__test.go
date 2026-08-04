package config

import (
	"testing"

	"github.com/biairmal/go-sdk/lib/ratelimit"
)

func TestRateLimitConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     RateLimitConfig
		wantErr bool
	}{
		{name: "disabled skips validation of a bogus backend", cfg: RateLimitConfig{
			Enabled:   false,
			RateLimit: ratelimit.Config{Backend: "bogus"},
		}},
		{name: "enabled with default ratelimit config is valid", cfg: RateLimitConfig{
			Enabled:   true,
			RateLimit: ratelimit.DefaultConfig(),
		}},
		{name: "enabled with bogus backend is rejected", cfg: RateLimitConfig{
			Enabled:   true,
			RateLimit: ratelimit.Config{Backend: "bogus", Burst: 1},
		}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
