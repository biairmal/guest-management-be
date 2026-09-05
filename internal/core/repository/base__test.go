package repository

import (
	"testing"

	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository/cache"
	"github.com/biairmal/go-sdk/lib/repository/sql"
	mockredis "github.com/biairmal/go-sdk/mocks/redis"
	"go.uber.org/mock/gomock"
)

type testEntity struct {
	ID string `db:"id"`
}

func TestNewRepository_CacheWrapping(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mockredis.NewMockClient(ctrl)

	tests := []struct {
		name      string
		cacheOpts CacheOptions
		wantCache bool
	}{
		{name: "zero-value CacheOptions disables caching", cacheOpts: CacheOptions{}, wantCache: false},
		{name: "enabled without a client is not cached", cacheOpts: CacheOptions{Enabled: true}, wantCache: false},
		{
			name:      "disabled with a client is not cached",
			cacheOpts: CacheOptions{Enabled: false, Client: client},
			wantCache: false,
		},
		{
			name:      "enabled with a client wraps in the cache decorator",
			cacheOpts: CacheOptions{Enabled: true, Client: client, Strategy: cache.WriteAroundStrategy},
			wantCache: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewRepository[testEntity, string](
				logger.NewNoOp(), nil, "test_entities", []string{"id"}, tt.cacheOpts,
			)
			_, isCached := repo.(*cache.CachedRepository[testEntity, string])
			if isCached != tt.wantCache {
				t.Errorf("cached = %v, want %v", isCached, tt.wantCache)
			}
		})
	}
}

func TestNewRepositoryNoAudit_CacheWrapping(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mockredis.NewMockClient(ctrl)

	tests := []struct {
		name      string
		cacheOpts CacheOptions
		wantCache bool
	}{
		{name: "zero-value CacheOptions disables caching", cacheOpts: CacheOptions{}, wantCache: false},
		{name: "enabled without a client is not cached", cacheOpts: CacheOptions{Enabled: true}, wantCache: false},
		{
			name:      "enabled with a client wraps in the cache decorator",
			cacheOpts: CacheOptions{Enabled: true, Client: client, Strategy: cache.WriteAroundStrategy},
			wantCache: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewRepositoryNoAudit[testEntity, string](
				logger.NewNoOp(), nil, "test_entities_no_audit", []string{"id"}, tt.cacheOpts,
			)
			_, isCached := repo.(*cache.CachedRepository[testEntity, string])
			if isCached != tt.wantCache {
				t.Errorf("cached = %v, want %v", isCached, tt.wantCache)
			}
		})
	}
}

// TestNewRepositoryNoAudit_SkipsAuditDecorator asserts the returned
// repository is never an *audit.AuditableRepository, distinguishing it from
// NewRepository. AuditableRepository is unexported outside its package, so
// this is checked indirectly: without caching, NewRepositoryNoAudit must
// return the concrete *sql.SQLRepository type instead of any decorator.
func TestNewRepositoryNoAudit_SkipsAuditDecorator(t *testing.T) {
	repo := NewRepositoryNoAudit[testEntity, string](
		logger.NewNoOp(), nil, "test_entities_no_audit", []string{"id"}, CacheOptions{},
	)
	if _, isCached := repo.(*cache.CachedRepository[testEntity, string]); isCached {
		t.Fatal("expected an uncached repository")
	}
	if _, isSQLRepo := repo.(*sql.SQLRepository[testEntity, string]); !isSQLRepo {
		t.Errorf("expected *sql.SQLRepository, got %T (audit decorator was not skipped)", repo)
	}
}
