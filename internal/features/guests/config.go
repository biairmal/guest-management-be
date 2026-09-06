package guests

import (
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
)

// Config aggregates the guests feature's own configuration, one field per
// layer (app.guests.<layer> in config.yaml). Only Repository holds a real
// field today; Service/Handler get added here when they have a real setting
// to hold — no empty Options{} structs (see AGENTS.md).
type Config struct {
	Repository RepositoryConfig `mapstructure:"repository"`
}

// RepositoryConfig holds config for the guests feature's repository layer:
// the guest and ticket repositories' cache policies.
type RepositoryConfig struct {
	GuestCache  corerepository.CacheConfig `mapstructure:"guest_cache"`
	TicketCache corerepository.CacheConfig `mapstructure:"ticket_cache"`
}

// DefaultConfig returns the guests feature config with caching enabled by default.
func DefaultConfig() Config {
	return Config{Repository: RepositoryConfig{
		GuestCache:  corerepository.DefaultCacheConfig(),
		TicketCache: corerepository.DefaultCacheConfig(),
	}}
}

// Validate validates the guests feature configuration.
func (c *Config) Validate() error {
	return c.Repository.Validate()
}

// Validate validates the guests feature's repository-layer configuration.
func (c *RepositoryConfig) Validate() error {
	if err := c.GuestCache.Validate(); err != nil {
		return err
	}
	return c.TicketCache.Validate()
}
