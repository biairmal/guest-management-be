package config

import "github.com/biairmal/go-sdk/tracer"

// TracingConfig wraps go-sdk's tracer.Config with an app-level on/off switch.
// tracer.Config has no zero-value way to fully disable tracing (Validate
// requires service_name/endpoint), so Enabled selects NewOTel vs NewNoOp in main.go.
type TracingConfig struct {
	Enabled bool          `mapstructure:"enabled"`
	Tracer  tracer.Config `mapstructure:"tracer"`
}

// Validate validates the tracer config only when tracing is enabled.
func (c *TracingConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	return c.Tracer.Validate()
}
