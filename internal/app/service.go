package app

import (
	sdkauth "github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/logger"
	appconfig "github.com/biairmal/guest-management-be/internal/config"
	appauth "github.com/biairmal/guest-management-be/internal/features/auth"
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/users"
)

type service struct {
	categoryService events.CategoryService
	tenantService   tenants.TenantService
	userService     users.UserService
	authService     appauth.Service
}

func (a *App) initializeService(
	logger logger.Logger, repositories *repositories,
	authIssuer sdkauth.Issuer, authValidator sdkauth.Validator, authConfig *appconfig.AuthConfig,
) *service {
	return &service{
		categoryService: events.NewCategoryService(logger, repositories.categoryRepository),
		tenantService:   tenants.NewTenantService(logger, repositories.tenantRepository),
		userService:     users.NewUserService(logger, repositories.userRepository),
		authService: appauth.NewService(
			logger, repositories.userRepository, authIssuer, authValidator,
			authConfig.Token.Issuer.DefaultTTL, authConfig.RefreshTTL,
		),
	}
}
