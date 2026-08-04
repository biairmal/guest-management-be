package config

import (
	"testing"

	"github.com/biairmal/go-sdk/lib/metrics"
)

func TestMetricsConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     MetricsConfig
		wantErr bool
	}{
		{name: "disabled skips validation of a malformed namespace", cfg: MetricsConfig{
			Enabled: false,
			Metrics: metrics.Config{Namespace: "not valid!"},
		}},
		{name: "enabled with default metrics config is valid", cfg: MetricsConfig{
			Enabled: true,
			Metrics: metrics.DefaultConfig(),
		}},
		{name: "enabled with malformed namespace is rejected", cfg: MetricsConfig{
			Enabled: true,
			Metrics: metrics.Config{Namespace: "not valid!"},
		}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
