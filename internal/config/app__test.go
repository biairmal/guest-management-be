package config

import (
	"testing"

	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/templates"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/users"
)

func TestFeatureConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     FeatureConfig
		wantErr bool
	}{
		{
			name: "default events, tenants, users and templates config is valid",
			cfg: FeatureConfig{
				Events: events.DefaultConfig(), Tenants: tenants.DefaultConfig(),
				Users: users.DefaultConfig(), Templates: templates.DefaultConfig(),
			},
		},
		{
			name: "invalid events config is rejected",
			cfg: FeatureConfig{Events: func() events.Config {
				c := events.DefaultConfig()
				c.Repository.CategoryCache.Strategy = "bogus"
				return c
			}(), Tenants: tenants.DefaultConfig(), Users: users.DefaultConfig(), Templates: templates.DefaultConfig()},
			wantErr: true,
		},
		{
			name: "invalid tenants config is rejected",
			cfg: FeatureConfig{Events: events.DefaultConfig(), Tenants: func() tenants.Config {
				c := tenants.DefaultConfig()
				c.Repository.TenantCache.Strategy = "bogus"
				return c
			}(), Users: users.DefaultConfig(), Templates: templates.DefaultConfig()},
			wantErr: true,
		},
		{
			name: "invalid users config is rejected",
			cfg: FeatureConfig{
				Events: events.DefaultConfig(), Tenants: tenants.DefaultConfig(), Users: func() users.Config {
					c := users.DefaultConfig()
					c.Repository.UserCache.Strategy = "bogus"
					return c
				}(), Templates: templates.DefaultConfig(),
			},
			wantErr: true,
		},
		{
			name: "invalid templates config is rejected",
			cfg: FeatureConfig{
				Events: events.DefaultConfig(), Tenants: tenants.DefaultConfig(), Users: users.DefaultConfig(),
				Templates: func() templates.Config {
					c := templates.DefaultConfig()
					c.Repository.MessageTemplateCache.Strategy = "bogus"
					return c
				}(),
			},
			wantErr: true,
		},
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
