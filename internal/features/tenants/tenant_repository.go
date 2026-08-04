package tenants

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

const tenantsTable = "tenants"

// tenantColumns are the columns selected on reads (GetByID, List).
var tenantColumns = []string{
	"id", "name", "type", "settings", "branding", "created_at", "updated_at", "deleted_at",
}

// NewTenantRepository returns a soft-delete-aware repository for tenants.
// TID is uuid.UUID — kept typed all the way through the service layer.
func NewTenantRepository(
	log logger.Logger, db *sqlkit.DB, cacheOpts corerepository.CacheOptions,
) repository.Repository[Tenant, uuid.UUID] {
	return corerepository.NewRepository[Tenant, uuid.UUID](
		log, db, tenantsTable, tenantColumns, cacheOpts,
	)
}
