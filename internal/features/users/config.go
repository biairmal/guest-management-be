package users

import (
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
)

// Config aggregates the users feature's own configuration, one field per
// layer (app.users.<layer> in config.yaml). Only Repository holds a real
// field today; Service/Handler get added here when they have a real setting
// to hold — no empty Options{} structs (see AGENTS.md).
type Config struct {
	Repository RepositoryConfig `mapstructure:"repository"`
}

// RepositoryConfig holds config for the users feature's repository layer:
// today, just the user repository's cache policy.
type RepositoryConfig struct {
	UserCache corerepository.CacheConfig `mapstructure:"user_cache"`
}

// DefaultConfig returns the users feature config with caching enabled by default.
func DefaultConfig() Config {
	return Config{Repository: RepositoryConfig{UserCache: corerepository.DefaultCacheConfig()}}
}

// Validate validates the users feature configuration.
func (c *Config) Validate() error {
	return c.Repository.Validate()
}

// Validate validates the users feature's repository-layer configuration.
func (c *RepositoryConfig) Validate() error {
	return c.UserCache.Validate()
}
