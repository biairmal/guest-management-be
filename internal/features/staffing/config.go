package staffing

import (
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
)

// Config aggregates the staffing feature's own configuration, one field per
// layer (app.staffing.<layer> in config.yaml). Only Repository holds a real
// field today; Service/Handler get added here when they have a real
// setting to hold — no empty Options{} structs (see AGENTS.md).
type Config struct {
	Repository RepositoryConfig `mapstructure:"repository"`
}

// RepositoryConfig holds config for the staffing feature's repository
// layer: today, just the staff assignment repository's cache policy.
type RepositoryConfig struct {
	StaffAssignmentCache corerepository.CacheConfig `mapstructure:"staff_assignment_cache"`
}

// DefaultConfig returns the staffing feature config with caching enabled by default.
func DefaultConfig() Config {
	return Config{Repository: RepositoryConfig{StaffAssignmentCache: corerepository.DefaultCacheConfig()}}
}

// Validate validates the staffing feature configuration.
func (c *Config) Validate() error {
	return c.Repository.Validate()
}

// Validate validates the staffing feature's repository-layer configuration.
func (c *RepositoryConfig) Validate() error {
	return c.StaffAssignmentCache.Validate()
}
