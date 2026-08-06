package config

import (
	"testing"
	"time"

	sdkauth "github.com/biairmal/go-sdk/lib/auth"
)

func TestAuthConfigValidate(t *testing.T) {
	validTokenCfg := sdkauth.Config{
		Mode:  sdkauth.ModeLocal,
		Local: sdkauth.LocalConfig{Algorithm: sdkauth.AlgorithmHS256, HS256Secret: "s"},
	}

	tests := []struct {
		name    string
		cfg     AuthConfig
		wantErr bool
	}{
		{name: "default config with a secret is valid", cfg: func() AuthConfig {
			c := DefaultAuthConfig()
			c.Token.Local.HS256Secret = "s"
			return c
		}()},
		{name: "zero refresh_ttl is rejected", cfg: AuthConfig{
			Token: validTokenCfg,
		}, wantErr: true},
		{name: "negative refresh_ttl is rejected", cfg: AuthConfig{
			Token:      validTokenCfg,
			RefreshTTL: -time.Hour,
		}, wantErr: true},
		{name: "invalid token config is rejected", cfg: AuthConfig{
			Token:      sdkauth.Config{Mode: "bogus"},
			RefreshTTL: time.Hour,
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
