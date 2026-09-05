package tickets

import (
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
)

// Config aggregates the tickets feature's own configuration, one field per
// layer (app.tickets.<layer> in config.yaml). Only Repository holds a real
// field today; Service/Handler get added here when they have a real
// setting to hold — no empty Options{} structs (see AGENTS.md).
type Config struct {
	Repository RepositoryConfig `mapstructure:"repository"`
}

// RepositoryConfig holds config for the tickets feature's repository layer:
// today, just the ticket type repository's cache policy. The
// ticket_type_workflow_steps junction repository has no cache decorator —
// not requested by the Technical Design for this phase.
type RepositoryConfig struct {
	TicketTypeCache corerepository.CacheConfig `mapstructure:"ticket_type_cache"`
}

// DefaultConfig returns the tickets feature config with caching enabled by default.
func DefaultConfig() Config {
	return Config{Repository: RepositoryConfig{TicketTypeCache: corerepository.DefaultCacheConfig()}}
}

// Validate validates the tickets feature configuration.
func (c *Config) Validate() error {
	return c.Repository.Validate()
}

// Validate validates the tickets feature's repository-layer configuration.
func (c *RepositoryConfig) Validate() error {
	return c.TicketTypeCache.Validate()
}
