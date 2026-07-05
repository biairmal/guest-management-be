package repository

import (
	"testing"

	"github.com/biairmal/go-sdk/logger"
	mockredis "github.com/biairmal/go-sdk/mocks/redis"
	"github.com/biairmal/go-sdk/repository/cache"
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
