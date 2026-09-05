package roles

import (
	"context"
	"fmt"
	"time"

	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/redis"
	"github.com/biairmal/go-sdk/lib/repository/cache"
	"github.com/biairmal/go-sdk/lib/serializer"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../mocks/roles/mock_role_permission_repository.go -package=mockroles github.com/biairmal/guest-management-be/internal/features/roles RolePermissionRepository

// RolePermissionRepository resolves the permission codes granted to a role
// via role_permissions. role_permissions has a composite primary key
// (role_id, permission_id) and this is a read-only join, not CRUD over a
// single entity, so it deliberately does not implement the generic
// repository.Repository[T, TID] pattern (a single-column WHERE keyed by
// one typed ID) — it gets a small, purpose-built interface instead.
type RolePermissionRepository interface {
	// PermissionCodesByRoleID returns the permission codes granted to
	// roleID. Returns an empty slice (not an error) if the role has no
	// granted permissions or roleID doesn't exist — the caller (authz.Checker)
	// treats "no permissions" and "unknown role" identically: the check fails.
	PermissionCodesByRoleID(ctx context.Context, roleID uuid.UUID) ([]string, error)
}

// sqlRolePermissionRepository implements RolePermissionRepository with a
// direct SQL join over role_permissions/permissions, bypassing the generic
// repository/sql machinery (which is built around a single-entity table).
type sqlRolePermissionRepository struct {
	db  *sqlkit.DB
	log logger.Logger
}

// NewRolePermissionRepository returns a RolePermissionRepository backed by a
// direct SQL join. Reads use the follower connection (db.Follower()) since
// this is a read-only lookup.
func NewRolePermissionRepository(log logger.Logger, db *sqlkit.DB) RolePermissionRepository {
	return &sqlRolePermissionRepository{db: db, log: log}
}

// PermissionCodesByRoleID joins role_permissions to permissions and returns
// the granted codes for roleID.
func (r *sqlRolePermissionRepository) PermissionCodesByRoleID(
	ctx context.Context, roleID uuid.UUID,
) ([]string, error) {
	const query = `
		SELECT p.code
		FROM role_permissions rp
		JOIN permissions p ON p.id = rp.permission_id
		WHERE rp.role_id = $1`

	if r.log != nil {
		r.log.DebugfWithContext(ctx, "query: %s args: %v", query, []any{roleID})
	}

	rows, err := r.db.Follower().QueryContext(ctx, query, roleID)
	if err != nil {
		return nil, fmt.Errorf("role_permissions: query permission codes: %w", err)
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var code string
		if scanErr := rows.Scan(&code); scanErr != nil {
			return nil, fmt.Errorf("role_permissions: scan permission code: %w", scanErr)
		}
		codes = append(codes, code)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("role_permissions: read permission codes: %w", err)
	}
	return codes, nil
}

// rolePermissionsCacheNamespace roots the cache key namespace, mirroring
// corerepository.NewRepository's table-name-derived namespace even though
// this repository has no single backing table.
const rolePermissionsCacheNamespace = "role_permissions"

// cachedRolePermissionRepository is a read-through Redis cache decorator
// over a RolePermissionRepository. Unlike the generic repository/cache
// decorator, there is no write path to invalidate here — this interface is
// read-only — so every entry is simply cached for cfg.TTL and left to expire.
type cachedRolePermissionRepository struct {
	inner  RolePermissionRepository
	client redis.Client
	keyGen cache.KeyGenerator
	ttl    time.Duration
}

// NewCachedRolePermissionRepository wraps inner in a read-through Redis
// cache keyed by role ID, JSON-encoding the []string permission-code slice.
// When cfg.Enabled is false or client is nil, inner is returned unwrapped —
// mirroring corerepository.NewRepository's caching-disabled behavior.
func NewCachedRolePermissionRepository(
	inner RolePermissionRepository, client redis.Client, cfg corerepository.CacheConfig,
) RolePermissionRepository {
	if !cfg.Enabled || client == nil {
		return inner
	}
	namespace := rolePermissionsCacheNamespace
	if cfg.Prefix != "" {
		namespace = cfg.Prefix + ":" + rolePermissionsCacheNamespace
	}
	return &cachedRolePermissionRepository{
		inner: inner, client: client,
		keyGen: cache.NewDefaultKeyGenerator(namespace), ttl: cfg.TTL,
	}
}

// PermissionCodesByRoleID returns the cached permission codes for roleID
// when present; otherwise it delegates to inner and populates the cache.
// Any Redis error (including a cache miss) falls through to inner so a
// Redis outage never blocks a permission check.
func (r *cachedRolePermissionRepository) PermissionCodesByRoleID(
	ctx context.Context, roleID uuid.UUID,
) ([]string, error) {
	key := r.keyGen.Generate("role_id", roleID)

	if cached, err := r.client.Get(ctx, key); err == nil {
		var codes []string
		if parseErr := serializer.ParseJSON([]byte(cached), &codes); parseErr == nil {
			return codes, nil
		}
		// corrupt cache entry — fall through to inner and overwrite below
	}

	codes, err := r.inner.PermissionCodesByRoleID(ctx, roleID)
	if err != nil {
		return nil, err
	}

	if data, jsonErr := serializer.ToJSON(codes); jsonErr == nil {
		//nolint:errcheck // best-effort cache write; a Redis outage must not fail the read
		_ = r.client.Set(ctx, key, data, r.ttl)
	}
	return codes, nil
}
