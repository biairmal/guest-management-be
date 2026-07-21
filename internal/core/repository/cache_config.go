package repository

import (
	"fmt"
	"time"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/redis"
	"github.com/biairmal/go-sdk/lib/repository/cache"
)

// CacheConfig is the YAML/mapstructure-decodable shape for one repository's
// cache policy. A feature embeds one CacheConfig per repository it wants
// configurable caching for (see internal/features/events.Config), so
// different repositories can have different enabled/ttl/prefix/strategy
// settings instead of sharing one app-wide policy.
type CacheConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	TTL      time.Duration `mapstructure:"ttl"`
	Prefix   string        `mapstructure:"prefix"`
	Strategy string        `mapstructure:"strategy"` // write_around (default), write_through, write_behind
}

// DefaultCacheConfig returns a CacheConfig with caching enabled and sensible defaults.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled:  true,
		TTL:      5 * time.Minute,
		Prefix:   "guest-management",
		Strategy: "write_around",
	}
}

// Validate checks that the cache configuration is well-formed. It is a no-op
// when caching is disabled, since TTL/Strategy are meaningless in that case.
func (c *CacheConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.TTL <= 0 {
		return errorz.Internal().WithMessage("cache: ttl must be positive when enabled")
	}
	if _, err := c.ResolveStrategy(); err != nil {
		return err
	}
	return nil
}

// ResolveStrategy maps the configured strategy name to a go-sdk cache.CacheStrategy.
func (c *CacheConfig) ResolveStrategy() (cache.CacheStrategy, error) {
	switch c.Strategy {
	case "", "write_around":
		return cache.WriteAroundStrategy, nil
	case "write_through":
		return cache.WriteThroughStrategy, nil
	case "write_behind":
		return cache.WriteBehindStrategy, nil
	default:
		return 0, errorz.Internal().WithMessage(fmt.Sprintf("cache: unknown strategy %q", c.Strategy))
	}
}

// ToOptions resolves the config into runtime CacheOptions for NewRepository,
// binding it to the given Redis client. client may be nil (e.g. Redis not
// wired), in which case NewRepository treats caching as disabled regardless
// of c.Enabled.
func (c *CacheConfig) ToOptions(client redis.Client) (CacheOptions, error) {
	strategy, err := c.ResolveStrategy()
	if err != nil {
		return CacheOptions{}, err
	}
	return CacheOptions{
		Enabled:  c.Enabled,
		Client:   client,
		TTL:      c.TTL,
		Prefix:   c.Prefix,
		Strategy: strategy,
	}, nil
}
