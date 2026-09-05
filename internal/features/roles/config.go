package roles

import (
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
)

// Config aggregates the roles feature's own configuration, one field per
// layer (app.roles.<layer> in config.yaml). Only Repository holds a real
// field today — this slice ships model + repository only in this phase (no
// service/handler; see docs/DEVELOPMENT_PLAN.md B6) — no empty Options{}
// structs (see AGENTS.md).
type Config struct {
	Repository RepositoryConfig `mapstructure:"repository"`
}

// RepositoryConfig holds config for the roles feature's repository layer:
// one CacheConfig for the role repository and one for the role-permission
// join repository.
type RepositoryConfig struct {
	RoleCache           corerepository.CacheConfig `mapstructure:"role_cache"`
	RolePermissionCache corerepository.CacheConfig `mapstructure:"role_permission_cache"`
}

// DefaultConfig returns the roles feature config with caching enabled by default.
func DefaultConfig() Config {
	return Config{Repository: RepositoryConfig{
		RoleCache:           corerepository.DefaultCacheConfig(),
		RolePermissionCache: corerepository.DefaultCacheConfig(),
	}}
}

// Validate validates the roles feature configuration.
func (c *Config) Validate() error {
	return c.Repository.Validate()
}

// Validate validates the roles feature's repository-layer configuration.
func (c *RepositoryConfig) Validate() error {
	if err := c.RoleCache.Validate(); err != nil {
		return err
	}
	return c.RolePermissionCache.Validate()
}
