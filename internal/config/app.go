package config

import (
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/guests"
	"github.com/biairmal/guest-management-be/internal/features/roles"
	"github.com/biairmal/guest-management-be/internal/features/staffing"
	"github.com/biairmal/guest-management-be/internal/features/templates"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/tickets"
	"github.com/biairmal/guest-management-be/internal/features/users"
)

// FeatureConfig aggregates configuration owned by individual features,
// nested under the "app" YAML section (app.<feature>.<config_name>).
// internal/app — the composition root, the only layer that knows every
// feature — reads from this when it wires each feature it registers. Adding
// a feature means adding a field here, not touching the root Config or
// cmd/api/main.go.
type FeatureConfig struct {
	Events    events.Config    `mapstructure:"events"`
	Tenants   tenants.Config   `mapstructure:"tenants"`
	Users     users.Config     `mapstructure:"users"`
	Templates templates.Config `mapstructure:"templates"`
	Roles     roles.Config     `mapstructure:"roles"`
	Staffing  staffing.Config  `mapstructure:"staffing"`
	Tickets   tickets.Config   `mapstructure:"tickets"`
	Guests    guests.Config    `mapstructure:"guests"`
}

// Validate validates every registered feature's configuration.
func (c *FeatureConfig) Validate() error {
	if err := c.Events.Validate(); err != nil {
		return err
	}
	if err := c.Tenants.Validate(); err != nil {
		return err
	}
	if err := c.Users.Validate(); err != nil {
		return err
	}
	if err := c.Templates.Validate(); err != nil {
		return err
	}
	if err := c.Roles.Validate(); err != nil {
		return err
	}
	if err := c.Staffing.Validate(); err != nil {
		return err
	}
	if err := c.Tickets.Validate(); err != nil {
		return err
	}
	return c.Guests.Validate()
}
