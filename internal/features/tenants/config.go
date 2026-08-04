package tenants

import (
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
)

// Config aggregates the tenants feature's own configuration, one field per
// layer (app.tenants.<layer> in config.yaml). Only Repository holds a real
// field today; Service/Handler get added here when they have a real setting
// to hold — no empty Options{} structs (see AGENTS.md).
type Config struct {
	Repository RepositoryConfig `mapstructure:"repository"`
}

// RepositoryConfig holds config for the tenants feature's repository layer:
// today, just the tenant repository's cache policy.
type RepositoryConfig struct {
	TenantCache corerepository.CacheConfig `mapstructure:"tenant_cache"`
}

// DefaultConfig returns the tenants feature config with caching enabled by default.
func DefaultConfig() Config {
	return Config{Repository: RepositoryConfig{TenantCache: corerepository.DefaultCacheConfig()}}
}

// Validate validates the tenants feature configuration.
func (c *Config) Validate() error {
	return c.Repository.Validate()
}

// Validate validates the tenants feature's repository-layer configuration.
func (c *RepositoryConfig) Validate() error {
	return c.TenantCache.Validate()
}
