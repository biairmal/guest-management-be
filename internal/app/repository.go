package app

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/redis"
	sdkrepository "github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	appconfig "github.com/biairmal/guest-management-be/internal/config"
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/google/uuid"
)

// repositories holds all feature repositories wired for the application.
type repositories struct {
	categoryRepository sdkrepository.Repository[events.EventCategory, uuid.UUID]
	tenantRepository   sdkrepository.Repository[tenants.Tenant, uuid.UUID]
}

func (a *App) initializeRepository(
	log logger.Logger, db *sqlkit.DB, redisClient redis.Client, featureConfig *appconfig.FeatureConfig,
) (*repositories, error) {
	categoryCacheOpts, err := featureConfig.Events.Repository.CategoryCache.ToOptions(redisClient)
	if err != nil {
		return nil, err
	}
	tenantCacheOpts, err := featureConfig.Tenants.Repository.TenantCache.ToOptions(redisClient)
	if err != nil {
		return nil, err
	}
	return &repositories{
		categoryRepository: events.NewCategoryRepository(log, db, categoryCacheOpts),
		tenantRepository:   tenants.NewTenantRepository(log, db, tenantCacheOpts),
	}, nil
}
