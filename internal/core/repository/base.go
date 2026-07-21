// Package repository provides a shared, typed constructor that composes the
// go-sdk generic SQL repository with the audit decorator, so a feature
// repository is a single call instead of hand-wiring both per feature.
package repository

import (
	"time"

	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/redis"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/repository/cache"
	"github.com/biairmal/go-sdk/lib/repository/sql"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	"github.com/biairmal/guest-management-be/internal/core/audit"
)

// CacheOptions configures the optional caching decorator NewRepository applies
// on top of the audit-wrapped repository. Client is nil when Redis isn't
// wired; NewRepository treats a nil Client the same as Enabled == false, so
// callers can pass a zero-value CacheOptions to disable caching entirely.
type CacheOptions struct {
	Enabled  bool
	Client   redis.Client
	TTL      time.Duration
	Prefix   string
	Strategy cache.CacheStrategy
}

// NewRepository returns a soft-delete-aware repository for TEntity: a
// go-sdk SQL repository over table, wrapped in the audit decorator, with
// selectColumns used for reads (GetByID, List). TID is kept typed all the
// way through the service layer — never widen it to `any`.
//
// When cacheOpts.Enabled is true and cacheOpts.Client is non-nil, the result
// is additionally wrapped in the go-sdk cache decorator, keyed by table name
// (optionally namespaced under cacheOpts.Prefix).
func NewRepository[TEntity any, TID comparable](
	log logger.Logger,
	db *sqlkit.DB,
	table string,
	selectColumns []string,
	cacheOpts CacheOptions,
) repository.Repository[TEntity, TID] {
	sqlRepo := sql.NewSQLRepository[TEntity, TID](
		log,
		db,
		table,
		sql.WithSelectColumns[TEntity, TID](selectColumns),
	)
	auditRepo := audit.NewAuditableRepository[TEntity, TID](sqlRepo)

	if !cacheOpts.Enabled || cacheOpts.Client == nil {
		return auditRepo
	}

	namespace := table
	if cacheOpts.Prefix != "" {
		namespace = cacheOpts.Prefix + ":" + table
	}

	return cache.NewCachedRepository[TEntity, TID](
		auditRepo,
		cacheOpts.Client,
		cache.WithKeyGenerator(cache.NewDefaultKeyGenerator(namespace)),
		cache.WithTTL(cacheOpts.TTL),
		cache.WithStrategy(cacheOpts.Strategy),
	)
}
