package roles

import (
	"testing"

	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "default config is valid", cfg: DefaultConfig()},
		{
			name: "invalid role cache strategy is rejected",
			cfg: func() Config {
				c := DefaultConfig()
				c.Repository.RoleCache.Strategy = "bogus"
				return c
			}(),
			wantErr: true,
		},
		{
			name: "invalid role permission cache strategy is rejected",
			cfg: func() Config {
				c := DefaultConfig()
				c.Repository.RolePermissionCache.Strategy = "bogus"
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

func TestRepositoryConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     RepositoryConfig
		wantErr bool
	}{
		{
			name: "default caches are valid",
			cfg: RepositoryConfig{
				RoleCache:           corerepository.DefaultCacheConfig(),
				RolePermissionCache: corerepository.DefaultCacheConfig(),
			},
		},
		{
			name: "invalid role cache strategy is rejected",
			cfg: RepositoryConfig{
				RoleCache:           corerepository.CacheConfig{Enabled: true, Strategy: "bogus"},
				RolePermissionCache: corerepository.DefaultCacheConfig(),
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
