// Package repository provides a shared, typed constructor that composes the
// go-sdk generic SQL repository with the audit decorator, so a feature
// repository is a single call instead of hand-wiring both per feature.
package repository

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/repository"
	"github.com/biairmal/go-sdk/repository/sql"
	"github.com/biairmal/go-sdk/sqlkit"
	"github.com/biairmal/guest-management-be/internal/core/audit"
)

// NewRepository returns a soft-delete-aware repository for TEntity: a
// go-sdk SQL repository over table, wrapped in the audit decorator, with
// selectColumns used for reads (GetByID, List). TID is kept typed all the
// way through the service layer — never widen it to `any`.
func NewRepository[TEntity any, TID comparable](
	log logger.Logger,
	db *sqlkit.DB,
	table string,
	selectColumns []string,
) repository.Repository[TEntity, TID] {
	sqlRepo := sql.NewSQLRepository[TEntity, TID](
		log,
		db,
		table,
		sql.WithSelectColumns[TEntity, TID](selectColumns),
	)
	return audit.NewAuditableRepository(sqlRepo)
}
