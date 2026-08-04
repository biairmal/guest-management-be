package config

import "github.com/biairmal/go-sdk/lib/metrics"

// MetricsConfig wraps go-sdk's metrics.Config with an app-level on/off
// switch. When disabled, main.go wires a NewNoOp Recorder and skips
// exposing the /metrics scrape endpoint.
type MetricsConfig struct {
	Enabled bool           `mapstructure:"enabled"`
	Metrics metrics.Config `mapstructure:"metrics"`
}

// Validate validates the metrics config only when metrics are enabled.
func (c *MetricsConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	return c.Metrics.Validate()
}
