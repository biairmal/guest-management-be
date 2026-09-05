package staffing

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
			name: "invalid repository staff assignment cache strategy is rejected",
			cfg: func() Config {
				c := DefaultConfig()
				c.Repository.StaffAssignmentCache.Strategy = "bogus"
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
			name: "default staff assignment cache is valid",
			cfg:  RepositoryConfig{StaffAssignmentCache: corerepository.DefaultCacheConfig()},
		},
		{
			name:    "invalid staff assignment cache strategy is rejected",
			cfg:     RepositoryConfig{StaffAssignmentCache: corerepository.CacheConfig{Enabled: true, Strategy: "bogus"}},
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
