package users

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

const usersTable = "users"

// userColumns are the columns selected on reads (GetByID, List).
var userColumns = []string{
	"id", "tenant_id", "email", "password_hash", "role_id", "is_tenant_master", "created_at", "updated_at", "deleted_at",
}

// NewUserRepository returns a soft-delete-aware repository for users.
// TID is uuid.UUID — kept typed all the way through the service layer.
func NewUserRepository(
	log logger.Logger, db *sqlkit.DB, cacheOpts corerepository.CacheOptions,
) repository.Repository[User, uuid.UUID] {
	return corerepository.NewRepository[User, uuid.UUID](
		log, db, usersTable, userColumns, cacheOpts,
	)
}
