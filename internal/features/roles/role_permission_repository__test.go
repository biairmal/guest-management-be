package roles

import (
	"context"
	"errors"
	"testing"
	"time"

	mockredis "github.com/biairmal/go-sdk/mocks/redis"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	mockroles "github.com/biairmal/guest-management-be/mocks/roles"
)

func TestNewCachedRolePermissionRepository_Disabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	inner := mockroles.NewMockRolePermissionRepository(ctrl)

	tests := []struct {
		name   string
		cfg    corerepository.CacheConfig
		client bool
	}{
		{name: "disabled cfg returns inner unwrapped", cfg: corerepository.CacheConfig{Enabled: false}, client: true},
		{
			name: "enabled without client returns inner unwrapped",
			cfg:  corerepository.CacheConfig{Enabled: true}, client: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var client *mockredis.MockClient
			if tt.client {
				client = mockredis.NewMockClient(ctrl)
			}
			var got RolePermissionRepository
			if client != nil {
				got = NewCachedRolePermissionRepository(inner, client, tt.cfg)
			} else {
				got = NewCachedRolePermissionRepository(inner, nil, tt.cfg)
			}
			if got != inner {
				t.Errorf("expected NewCachedRolePermissionRepository to return inner unwrapped, got %T", got)
			}
		})
	}
}

func TestCachedRolePermissionRepository_PermissionCodesByRoleID(t *testing.T) {
	roleID := uuid.New()
	cfg := corerepository.CacheConfig{Enabled: true, TTL: 5 * time.Minute, Prefix: "guest-management"}

	t.Run("cache hit returns cached codes without calling inner", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := mockroles.NewMockRolePermissionRepository(ctrl)
		client := mockredis.NewMockClient(ctrl)
		client.EXPECT().Get(gomock.Any(), gomock.Any()).Return(`["check_in","manage_guests"]`, nil)

		repo := NewCachedRolePermissionRepository(inner, client, cfg)
		got, err := repo.PermissionCodesByRoleID(context.Background(), roleID)
		if err != nil {
			t.Fatalf("PermissionCodesByRoleID() error = %v", err)
		}
		if len(got) != 2 || got[0] != "check_in" || got[1] != "manage_guests" {
			t.Errorf("got = %v", got)
		}
	})

	t.Run("cache miss falls through to inner and populates cache", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := mockroles.NewMockRolePermissionRepository(ctrl)
		client := mockredis.NewMockClient(ctrl)
		client.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", errors.New("cache miss"))
		inner.EXPECT().PermissionCodesByRoleID(gomock.Any(), roleID).Return([]string{"check_in"}, nil)
		client.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), cfg.TTL).Return(nil)

		repo := NewCachedRolePermissionRepository(inner, client, cfg)
		got, err := repo.PermissionCodesByRoleID(context.Background(), roleID)
		if err != nil {
			t.Fatalf("PermissionCodesByRoleID() error = %v", err)
		}
		if len(got) != 1 || got[0] != "check_in" {
			t.Errorf("got = %v", got)
		}
	})

	t.Run("corrupt cache entry falls through to inner", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := mockroles.NewMockRolePermissionRepository(ctrl)
		client := mockredis.NewMockClient(ctrl)
		client.EXPECT().Get(gomock.Any(), gomock.Any()).Return("not-json", nil)
		inner.EXPECT().PermissionCodesByRoleID(gomock.Any(), roleID).Return([]string{"check_in"}, nil)
		client.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), cfg.TTL).Return(nil)

		repo := NewCachedRolePermissionRepository(inner, client, cfg)
		got, err := repo.PermissionCodesByRoleID(context.Background(), roleID)
		if err != nil {
			t.Fatalf("PermissionCodesByRoleID() error = %v", err)
		}
		if len(got) != 1 || got[0] != "check_in" {
			t.Errorf("got = %v", got)
		}
	})

	t.Run("inner error propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := mockroles.NewMockRolePermissionRepository(ctrl)
		client := mockredis.NewMockClient(ctrl)
		client.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", errors.New("cache miss"))
		inner.EXPECT().PermissionCodesByRoleID(gomock.Any(), roleID).Return(nil, errors.New("boom"))

		repo := NewCachedRolePermissionRepository(inner, client, cfg)
		_, err := repo.PermissionCodesByRoleID(context.Background(), roleID)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("cache write failure does not fail the read", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := mockroles.NewMockRolePermissionRepository(ctrl)
		client := mockredis.NewMockClient(ctrl)
		client.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", errors.New("cache miss"))
		inner.EXPECT().PermissionCodesByRoleID(gomock.Any(), roleID).Return([]string{"check_in"}, nil)
		client.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), cfg.TTL).Return(errors.New("redis down"))

		repo := NewCachedRolePermissionRepository(inner, client, cfg)
		got, err := repo.PermissionCodesByRoleID(context.Background(), roleID)
		if err != nil {
			t.Fatalf("PermissionCodesByRoleID() error = %v", err)
		}
		if len(got) != 1 || got[0] != "check_in" {
			t.Errorf("got = %v", got)
		}
	})
}
