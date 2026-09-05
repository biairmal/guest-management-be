package roles

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	mockroles "github.com/biairmal/guest-management-be/mocks/roles"
)

func TestPermissionResolverAdapter_ResolvePermissions(t *testing.T) {
	roleID := uuid.New()

	tests := []struct {
		name    string
		repoRes []string
		repoErr error
	}{
		{name: "delegates success", repoRes: []string{"manage_staff", "check_in"}},
		{name: "delegates error", repoErr: errors.New("boom")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockroles.NewMockRolePermissionRepository(ctrl)
			repo.EXPECT().PermissionCodesByRoleID(gomock.Any(), roleID).Return(tt.repoRes, tt.repoErr)

			resolver := NewPermissionResolver(repo)
			got, err := resolver.ResolvePermissions(context.Background(), roleID)
			if !errors.Is(err, tt.repoErr) && tt.repoErr != nil {
				t.Fatalf("ResolvePermissions() error = %v, want %v", err, tt.repoErr)
			}
			if tt.repoErr == nil {
				if len(got) != len(tt.repoRes) {
					t.Errorf("got = %v, want %v", got, tt.repoRes)
				}
			}
		})
	}
}
