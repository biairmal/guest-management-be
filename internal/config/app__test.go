package config

import (
	"testing"

	"github.com/biairmal/guest-management-be/internal/features/events"
)

func TestFeatureConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     FeatureConfig
		wantErr bool
	}{
		{name: "default events config is valid", cfg: FeatureConfig{Events: events.DefaultConfig()}},
		{
			name: "invalid events config is rejected",
			cfg: FeatureConfig{Events: func() events.Config {
				c := events.DefaultConfig()
				c.Repository.CategoryCache.Strategy = "bogus"
				return c
			}()},
			wantErr: true,
		},
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
