package config

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/sqlkit"
	"github.com/biairmal/guest-management-be/internal/app"
)

type Config struct {
	Logger   logger.Options
	Database sqlkit.Config
	App      app.Options
	Swagger  SwaggerConfig
}
