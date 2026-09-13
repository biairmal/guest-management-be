package staffing

import (
	"context"
	"errors"
	"testing"

	"github.com/biairmal/go-sdk/lib/repository"
	mockrepository "github.com/biairmal/go-sdk/mocks/repository"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestEventRoleResolverAdapter_ResolveEventRoleID(t *testing.T) {
	eventID := uuid.New()
	userID := uuid.New()
	roleID := uuid.New()

	tests := []struct {
		name      string
		listRes   []*EventStaffAssignment
		listErr   error
		wantFound bool
		wantRole  uuid.UUID
		wantErr   bool
	}{
		{name: "no active assignment", listRes: nil, wantFound: false},
		{
			name: "active assignment found", listRes: []*EventStaffAssignment{{RoleID: roleID}},
			wantFound: true, wantRole: roleID,
		},
		{name: "repo error propagates", listErr: errors.New("boom"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[EventStaffAssignment, uuid.UUID](ctrl)
			repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, opts *repository.ListOptions) ([]*EventStaffAssignment, int64, error) {
					if opts.Pagination.Limit != 1 {
						t.Errorf("Limit = %d, want 1", opts.Pagination.Limit)
					}
					want := repository.Filter{Conditions: []repository.FilterCondition{
						{Field: "event_id", Operator: repository.FilterOperatorEq, Value: eventID},
						{Field: "user_id", Operator: repository.FilterOperatorEq, Value: userID},
					}}
					if len(opts.Filter.Conditions) != len(want.Conditions) {
						t.Errorf("Filter.Conditions = %v, want %v", opts.Filter.Conditions, want.Conditions)
					}
					return tt.listRes, 0, tt.listErr
				},
			)

			resolver := NewEventRoleResolver(repo)
			gotRole, gotFound, err := resolver.ResolveEventRoleID(context.Background(), eventID, userID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if gotFound != tt.wantFound {
				t.Errorf("found = %v, want %v", gotFound, tt.wantFound)
			}
			if tt.wantFound && gotRole != tt.wantRole {
				t.Errorf("roleID = %v, want %v", gotRole, tt.wantRole)
			}
		})
	}
}
