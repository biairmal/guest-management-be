package config

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/redis"
	"github.com/biairmal/go-sdk/sqlkit"
	"github.com/biairmal/go-sdk/validator"
)

// Config is the root configuration tree for the application. It embeds go-sdk
// config structs for each infrastructure concern, app-specific sections, and
// App (FeatureConfig) — the aggregate of every registered feature's own
// config, nested under the "app" YAML section.
type Config struct {
	Logger    logger.Options
	Server    ServerConfig
	Database  sqlkit.Config
	Redis     redis.Config
	Validator validator.Config
	Swagger   SwaggerConfig
	App       FeatureConfig
}
