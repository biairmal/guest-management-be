package config

import (
	"testing"

	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/roles"
	"github.com/biairmal/guest-management-be/internal/features/staffing"
	"github.com/biairmal/guest-management-be/internal/features/templates"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/tickets"
	"github.com/biairmal/guest-management-be/internal/features/users"
)

// defaultFeatureConfig returns a FeatureConfig with every registered
// feature's default configuration, for tests that only want to flex one
// feature's Validate() failure at a time.
func defaultFeatureConfig() FeatureConfig {
	return FeatureConfig{
		Events: events.DefaultConfig(), Tenants: tenants.DefaultConfig(),
		Users: users.DefaultConfig(), Templates: templates.DefaultConfig(),
		Roles: roles.DefaultConfig(), Staffing: staffing.DefaultConfig(),
		Tickets: tickets.DefaultConfig(),
	}
}

func TestFeatureConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     FeatureConfig
		wantErr bool
	}{
		{
			name: "default events, tenants, users, templates, roles and staffing config is valid",
			cfg:  defaultFeatureConfig(),
		},
		{
			name: "invalid events config is rejected",
			cfg: func() FeatureConfig {
				c := defaultFeatureConfig()
				c.Events.Repository.CategoryCache.Strategy = "bogus"
				return c
			}(),
			wantErr: true,
		},
		{
			name: "invalid tenants config is rejected",
			cfg: func() FeatureConfig {
				c := defaultFeatureConfig()
				c.Tenants.Repository.TenantCache.Strategy = "bogus"
				return c
			}(),
			wantErr: true,
		},
		{
			name: "invalid users config is rejected",
			cfg: func() FeatureConfig {
				c := defaultFeatureConfig()
				c.Users.Repository.UserCache.Strategy = "bogus"
				return c
			}(),
			wantErr: true,
		},
		{
			name: "invalid templates config is rejected",
			cfg: func() FeatureConfig {
				c := defaultFeatureConfig()
				c.Templates.Repository.MessageTemplateCache.Strategy = "bogus"
				return c
			}(),
			wantErr: true,
		},
		{
			name: "invalid roles config is rejected",
			cfg: func() FeatureConfig {
				c := defaultFeatureConfig()
				c.Roles.Repository.RoleCache.Strategy = "bogus"
				return c
			}(),
			wantErr: true,
		},
		{
			name: "invalid staffing config is rejected",
			cfg: func() FeatureConfig {
				c := defaultFeatureConfig()
				c.Staffing.Repository.StaffAssignmentCache.Strategy = "bogus"
				return c
			}(),
			wantErr: true,
		},
		{
			name: "invalid tickets config is rejected",
			cfg: func() FeatureConfig {
				c := defaultFeatureConfig()
				c.Tickets.Repository.TicketTypeCache.Strategy = "bogus"
				return c
			}(),
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
