package app

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/redis"
	sdkrepository "github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	appconfig "github.com/biairmal/guest-management-be/internal/config"
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/templates"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/users"
	"github.com/google/uuid"
)

// repositories holds all feature repositories wired for the application.
type repositories struct {
	categoryRepository             sdkrepository.Repository[events.EventCategory, uuid.UUID]
	eventRepository                sdkrepository.Repository[events.Event, uuid.UUID]
	workflowStepRepository         sdkrepository.Repository[events.WorkflowStep, uuid.UUID]
	workflowStepTemplateRepository sdkrepository.Repository[events.WorkflowStepTemplate, uuid.UUID]
	tenantRepository               sdkrepository.Repository[tenants.Tenant, uuid.UUID]
	userRepository                 sdkrepository.Repository[users.User, uuid.UUID]
	messageTemplateRepository      sdkrepository.Repository[templates.MessageTemplate, uuid.UUID]
}

func (a *App) initializeRepository(
	log logger.Logger, db *sqlkit.DB, redisClient redis.Client, featureConfig *appconfig.FeatureConfig,
) (*repositories, error) {
	categoryCacheOpts, err := featureConfig.Events.Repository.CategoryCache.ToOptions(redisClient)
	if err != nil {
		return nil, err
	}
	eventCacheOpts, err := featureConfig.Events.Repository.EventCache.ToOptions(redisClient)
	if err != nil {
		return nil, err
	}
	workflowStepCacheOpts, err := featureConfig.Events.Repository.WorkflowStepCache.ToOptions(redisClient)
	if err != nil {
		return nil, err
	}
	workflowStepTemplateCacheOpts, err := featureConfig.Events.Repository.WorkflowStepTemplateCache.ToOptions(redisClient)
	if err != nil {
		return nil, err
	}
	tenantCacheOpts, err := featureConfig.Tenants.Repository.TenantCache.ToOptions(redisClient)
	if err != nil {
		return nil, err
	}
	userCacheOpts, err := featureConfig.Users.Repository.UserCache.ToOptions(redisClient)
	if err != nil {
		return nil, err
	}
	messageTemplateCacheOpts, err := featureConfig.Templates.Repository.MessageTemplateCache.ToOptions(redisClient)
	if err != nil {
		return nil, err
	}
	return &repositories{
		categoryRepository:             events.NewCategoryRepository(log, db, categoryCacheOpts),
		eventRepository:                events.NewEventRepository(log, db, eventCacheOpts),
		workflowStepRepository:         events.NewWorkflowStepRepository(log, db, workflowStepCacheOpts),
		workflowStepTemplateRepository: events.NewWorkflowStepTemplateRepository(log, db, workflowStepTemplateCacheOpts),
		tenantRepository:               tenants.NewTenantRepository(log, db, tenantCacheOpts),
		userRepository:                 users.NewUserRepository(log, db, userCacheOpts),
		messageTemplateRepository:      templates.NewMessageTemplateRepository(log, db, messageTemplateCacheOpts),
	}, nil
}
