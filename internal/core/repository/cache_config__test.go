package repository

import (
	"testing"
	"time"

	mockredis "github.com/biairmal/go-sdk/mocks/redis"
	"github.com/biairmal/go-sdk/repository/cache"
	"go.uber.org/mock/gomock"
)

func TestCacheConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     CacheConfig
		wantErr bool
	}{
		{name: "disabled skips all checks", cfg: CacheConfig{Enabled: false}},
		{name: "default config is valid", cfg: DefaultCacheConfig()},
		{
			name:    "enabled with zero ttl is invalid",
			cfg:     CacheConfig{Enabled: true, TTL: 0, Strategy: "write_around"},
			wantErr: true,
		},
		{
			name:    "enabled with negative ttl is invalid",
			cfg:     CacheConfig{Enabled: true, TTL: -time.Second},
			wantErr: true,
		},
		{
			name:    "enabled with unknown strategy is invalid",
			cfg:     CacheConfig{Enabled: true, TTL: time.Minute, Strategy: "bogus"},
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

func TestCacheConfigResolveStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		want     cache.CacheStrategy
		wantErr  bool
	}{
		{name: "empty defaults to write-around", strategy: "", want: cache.WriteAroundStrategy},
		{name: "write_around", strategy: "write_around", want: cache.WriteAroundStrategy},
		{name: "write_through", strategy: "write_through", want: cache.WriteThroughStrategy},
		{name: "write_behind", strategy: "write_behind", want: cache.WriteBehindStrategy},
		{name: "unknown strategy errors", strategy: "bogus", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := CacheConfig{Strategy: tt.strategy}
			got, err := cfg.ResolveStrategy()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveStrategy() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("ResolveStrategy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheConfigToOptions(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mockredis.NewMockClient(ctrl)

	t.Run("valid config resolves options bound to the client", func(t *testing.T) {
		cfg := CacheConfig{Enabled: true, TTL: time.Minute, Prefix: "p", Strategy: "write_through"}
		got, err := cfg.ToOptions(client)
		if err != nil {
			t.Fatalf("ToOptions() error = %v, want nil", err)
		}
		want := CacheOptions{
			Enabled: true, Client: client, TTL: time.Minute, Prefix: "p", Strategy: cache.WriteThroughStrategy,
		}
		if got != want {
			t.Errorf("ToOptions() = %+v, want %+v", got, want)
		}
	})

	t.Run("unknown strategy errors instead of silently defaulting", func(t *testing.T) {
		cfg := CacheConfig{Enabled: true, TTL: time.Minute, Strategy: "bogus"}
		if _, err := cfg.ToOptions(client); err == nil {
			t.Error("ToOptions() error = nil, want error for unknown strategy")
		}
	})
}
