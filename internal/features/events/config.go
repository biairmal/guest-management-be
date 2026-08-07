package events

import (
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
)

// Config aggregates the events feature's own configuration, one field per
// layer (app.events.<layer> in config.yaml). Only Repository holds a real
// field today; Service/Handler get added here when they have a real setting
// to hold — no empty Options{} structs (see AGENTS.md).
type Config struct {
	Repository RepositoryConfig `mapstructure:"repository"`
}

// RepositoryConfig holds config for the events feature's repository layer:
// one CacheConfig per repository (category, event, workflow step, workflow step template).
type RepositoryConfig struct {
	CategoryCache             corerepository.CacheConfig `mapstructure:"category_cache"`
	EventCache                corerepository.CacheConfig `mapstructure:"event_cache"`
	WorkflowStepCache         corerepository.CacheConfig `mapstructure:"workflow_step_cache"`
	WorkflowStepTemplateCache corerepository.CacheConfig `mapstructure:"workflow_step_template_cache"`
}

// DefaultConfig returns the events feature config with caching enabled by default.
func DefaultConfig() Config {
	return Config{Repository: RepositoryConfig{
		CategoryCache:             corerepository.DefaultCacheConfig(),
		EventCache:                corerepository.DefaultCacheConfig(),
		WorkflowStepCache:         corerepository.DefaultCacheConfig(),
		WorkflowStepTemplateCache: corerepository.DefaultCacheConfig(),
	}}
}

// Validate validates the events feature configuration.
func (c *Config) Validate() error {
	return c.Repository.Validate()
}

// Validate validates the events feature's repository-layer configuration.
func (c *RepositoryConfig) Validate() error {
	if err := c.CategoryCache.Validate(); err != nil {
		return err
	}
	if err := c.EventCache.Validate(); err != nil {
		return err
	}
	if err := c.WorkflowStepCache.Validate(); err != nil {
		return err
	}
	return c.WorkflowStepTemplateCache.Validate()
}
