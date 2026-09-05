package roles

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

const rolesTable = "roles"

// roleColumns are the columns selected on reads (GetByID, List).
var roleColumns = []string{"id", "name", "description", "scope", "created_at", "updated_at"}

// NewRoleRepository returns a repository for roles. roles has no deleted_at
// column (system/reference data — see docs/DATABASE.md ss5), so this uses
// corerepository.NewRepositoryNoAudit rather than NewRepository: the audit
// decorator assumes deleted_at exists and would break List/Count and
// silently no-op Delete against this table. TID is uuid.UUID — kept typed
// all the way through the service layer.
func NewRoleRepository(
	log logger.Logger, db *sqlkit.DB, cacheOpts corerepository.CacheOptions,
) repository.Repository[Role, uuid.UUID] {
	return corerepository.NewRepositoryNoAudit[Role, uuid.UUID](
		log, db, rolesTable, roleColumns, cacheOpts,
	)
}
