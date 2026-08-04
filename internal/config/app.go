package config

import (
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
)

// FeatureConfig aggregates configuration owned by individual features,
// nested under the "app" YAML section (app.<feature>.<config_name>).
// internal/app — the composition root, the only layer that knows every
// feature — reads from this when it wires each feature it registers. Adding
// a feature means adding a field here, not touching the root Config or
// cmd/api/main.go.
type FeatureConfig struct {
	Events  events.Config  `mapstructure:"events"`
	Tenants tenants.Config `mapstructure:"tenants"`
}

// Validate validates every registered feature's configuration.
func (c *FeatureConfig) Validate() error {
	if err := c.Events.Validate(); err != nil {
		return err
	}
	return c.Tenants.Validate()
}
