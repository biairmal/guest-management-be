package app

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/redis"
	sdkrepository "github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	appconfig "github.com/biairmal/guest-management-be/internal/config"
	"github.com/biairmal/guest-management-be/internal/features/events/category"
	"github.com/biairmal/guest-management-be/internal/features/events/event"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowstep"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowsteptemplate"
	"github.com/biairmal/guest-management-be/internal/features/guests"
	"github.com/biairmal/guest-management-be/internal/features/roles"
	"github.com/biairmal/guest-management-be/internal/features/scans"
	"github.com/biairmal/guest-management-be/internal/features/staffing"
	"github.com/biairmal/guest-management-be/internal/features/templates"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/tickets"
	"github.com/biairmal/guest-management-be/internal/features/users"
	"github.com/google/uuid"
)

// repositories holds all feature repositories wired for the application.
type repositories struct {
	categoryRepository               sdkrepository.Repository[category.EventCategory, uuid.UUID]
	eventRepository                  sdkrepository.Repository[event.Event, uuid.UUID]
	workflowStepRepository           sdkrepository.Repository[workflowstep.WorkflowStep, uuid.UUID]
	workflowStepTemplateRepository   sdkrepository.Repository[workflowsteptemplate.WorkflowStepTemplate, uuid.UUID]
	tenantRepository                 sdkrepository.Repository[tenants.Tenant, uuid.UUID]
	userRepository                   sdkrepository.Repository[users.User, uuid.UUID]
	messageTemplateRepository        sdkrepository.Repository[templates.MessageTemplate, uuid.UUID]
	roleRepository                   sdkrepository.Repository[roles.Role, uuid.UUID]
	rolePermissionRepository         roles.RolePermissionRepository
	staffAssignmentRepository        sdkrepository.Repository[staffing.EventStaffAssignment, uuid.UUID]
	ticketTypeRepository             sdkrepository.Repository[tickets.TicketType, uuid.UUID]
	ticketTypeWorkflowStepRepository tickets.TicketTypeWorkflowStepRepository
	guestRepository                  sdkrepository.Repository[guests.Guest, uuid.UUID]
	ticketRepository                 sdkrepository.Repository[guests.Ticket, uuid.UUID]
	guestPIIEncryptor                guests.PIIEncryptor
	scanLogRepository                sdkrepository.Repository[scans.ScanLog, uuid.UUID]
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
	roleCacheOpts, err := featureConfig.Roles.Repository.RoleCache.ToOptions(redisClient)
	if err != nil {
		return nil, err
	}
	staffAssignmentCacheOpts, err := featureConfig.Staffing.Repository.StaffAssignmentCache.ToOptions(redisClient)
	if err != nil {
		return nil, err
	}
	ticketTypeCacheOpts, err := featureConfig.Tickets.Repository.TicketTypeCache.ToOptions(redisClient)
	if err != nil {
		return nil, err
	}
	guestCacheOpts, err := featureConfig.Guests.Repository.GuestCache.ToOptions(redisClient)
	if err != nil {
		return nil, err
	}
	ticketCacheOpts, err := featureConfig.Guests.Repository.TicketCache.ToOptions(redisClient)
	if err != nil {
		return nil, err
	}

	rolePermissionRepository := roles.NewCachedRolePermissionRepository(
		roles.NewRolePermissionRepository(log, db), redisClient, featureConfig.Roles.Repository.RolePermissionCache,
	)

	// guestPIIEncryptor is constructed once here and reused by both the guest
	// repository (encrypt/decrypt on write/read) and the guest service (the
	// blind-index filter rewrite in List) — see pii_encryptor.go.
	guestPIIEncryptor, err := newCryptoPIIEncryptor(*a.cryptoConfig)
	if err != nil {
		return nil, err
	}

	return &repositories{
		categoryRepository:     category.NewCategoryRepository(log, db, categoryCacheOpts),
		eventRepository:        event.NewEventRepository(log, db, eventCacheOpts),
		workflowStepRepository: workflowstep.NewWorkflowStepRepository(log, db, workflowStepCacheOpts),
		workflowStepTemplateRepository: workflowsteptemplate.NewWorkflowStepTemplateRepository(
			log, db, workflowStepTemplateCacheOpts,
		),
		tenantRepository:                 tenants.NewTenantRepository(log, db, tenantCacheOpts),
		userRepository:                   users.NewUserRepository(log, db, userCacheOpts),
		messageTemplateRepository:        templates.NewMessageTemplateRepository(log, db, messageTemplateCacheOpts),
		roleRepository:                   roles.NewRoleRepository(log, db, roleCacheOpts),
		rolePermissionRepository:         rolePermissionRepository,
		staffAssignmentRepository:        staffing.NewStaffAssignmentRepository(log, db, staffAssignmentCacheOpts),
		ticketTypeRepository:             tickets.NewTicketTypeRepository(log, db, ticketTypeCacheOpts),
		ticketTypeWorkflowStepRepository: tickets.NewTicketTypeWorkflowStepRepository(log, db),
		guestRepository:                  guests.NewGuestRepository(log, db, guestCacheOpts, guestPIIEncryptor),
		ticketRepository:                 guests.NewTicketRepository(log, db, ticketCacheOpts),
		guestPIIEncryptor:                guestPIIEncryptor,
		scanLogRepository:                scans.NewScanLogRepository(log, db),
	}, nil
}
