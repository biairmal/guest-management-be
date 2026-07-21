package config

import (
	"fmt"
	"time"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// ServerConfig holds HTTP server networking and timeout settings.
type ServerConfig struct {
	Host              string        `mapstructure:"host"`
	Port              int           `mapstructure:"port"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout"`
}

// DefaultServerConfig returns a ServerConfig with sensible defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Host:              "127.0.0.1",
		Port:              8080,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		ShutdownTimeout:   30 * time.Second,
	}
}

// Addr returns the "host:port" address the server should listen on.
func (c ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Validate validates the server configuration.
func (c *ServerConfig) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return errorz.Internal().WithMessage(fmt.Sprintf("server: port must be between 1 and 65535, got %d", c.Port))
	}
	return nil
}
