package config

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/redis"
	"github.com/biairmal/go-sdk/sqlkit"
)

// Config is the root configuration tree for the application. It embeds go-sdk
// config structs for each infrastructure concern plus app-specific sections.
type Config struct {
	Logger   logger.Options
	Server   ServerConfig
	Database sqlkit.Config
	Redis    redis.Config
	Swagger  SwaggerConfig
}
