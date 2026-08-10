package templates

import (
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
)

// Config aggregates the templates feature's own configuration, one field per
// layer (app.templates.<layer> in config.yaml). Only Repository holds a real
// field today; Service/Handler get added here when they have a real setting
// to hold — no empty Options{} structs (see AGENTS.md).
type Config struct {
	Repository RepositoryConfig `mapstructure:"repository"`
}

// RepositoryConfig holds config for the templates feature's repository
// layer: today, just the message template repository's cache policy.
type RepositoryConfig struct {
	MessageTemplateCache corerepository.CacheConfig `mapstructure:"message_template_cache"`
}

// DefaultConfig returns the templates feature config with caching enabled by default.
func DefaultConfig() Config {
	return Config{Repository: RepositoryConfig{MessageTemplateCache: corerepository.DefaultCacheConfig()}}
}

// Validate validates the templates feature configuration.
func (c *Config) Validate() error {
	return c.Repository.Validate()
}

// Validate validates the templates feature's repository-layer configuration.
func (c *RepositoryConfig) Validate() error {
	return c.MessageTemplateCache.Validate()
}
