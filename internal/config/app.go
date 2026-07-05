package config

import "github.com/biairmal/guest-management-be/internal/features/events"

// FeatureConfig aggregates configuration owned by individual features,
// nested under the "app" YAML section (app.<feature>.<config_name>).
// internal/app — the composition root, the only layer that knows every
// feature — reads from this when it wires each feature it registers. Adding
// a feature means adding a field here, not touching the root Config or
// cmd/api/main.go.
type FeatureConfig struct {
	Events events.Config `mapstructure:"events"`
}

// Validate validates every registered feature's configuration.
func (c *FeatureConfig) Validate() error {
	return c.Events.Validate()
}
