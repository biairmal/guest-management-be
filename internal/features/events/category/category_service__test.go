package category

import (
	"context"
	"errors"
	"net/url"
	"testing"

	sdkauth "github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	mockrepository "github.com/biairmal/go-sdk/mocks/repository"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowsteptemplate"
	mockcategory "github.com/biairmal/guest-management-be/mocks/events/category"
)

// assertErrorzCode fails unless err carries the wanted errorz code (or is nil when want == "").
func assertErrorzCode(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	var e *errorz.Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *errorz.Error, got %T: %v", err, err)
	}
	if e.Code != want {
		t.Errorf("code = %q, want %q", e.Code, want)
	}
}

func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }

// tenantCtx returns a context carrying a validated JWT for tenantID.
func tenantCtx(tenantID uuid.UUID) context.Context {
	claims := sdkauth.NewClaims(map[string]any{"sub": uuid.NewString(), "tenant_id": tenantID.String()})
	return sdkauth.ContextWithClaims(context.Background(), claims)
}

// stubStore is a minimal TicketTypeTemplateStore — see
// ticket_type_template_store.go for why it isn't a generated mock.
type stubStore struct {
	views       []TicketTypeTemplateView
	err         error
	gotVersion  int
	gotDrafts   []TicketTypeTemplateDraft
	replaceRuns int
}

func (s *stubStore) ListForCategory(_ context.Context, _ uuid.UUID, version int) ([]TicketTypeTemplateView, error) {
	s.gotVersion = version
	return s.views, s.err
}

func (s *stubStore) ReplaceForCategory(
	_ context.Context, _ uuid.UUID, version int, in []TicketTypeTemplateDraft,
) ([]TicketTypeTemplateView, error) {
	s.replaceRuns++
	s.gotVersion = version
	s.gotDrafts = in
	views := make([]TicketTypeTemplateView, len(in))
	for i, d := range in {
		views[i] = TicketTypeTemplateView{ID: uuid.New(), Name: d.Name, WorkflowStepTemplateIDs: d.WorkflowStepTemplateIDs}
	}
	return views, s.err
}

func TestValidateTemplateSet(t *testing.T) {
	steps := []WorkflowStepInput{{Name: "Check-in"}, {Name: "Dinner"}}
	tests := []struct {
		name       string
		tickets    []TicketTypeInput
		wantFields []string
	}{
		{name: "valid, empty steps allowed", tickets: []TicketTypeInput{{Name: "VIP", Steps: []int{0, 1}}, {Name: "Crew"}}},
		{
			name:       "names repeat case-insensitively",
			tickets:    []TicketTypeInput{{Name: "VIP"}, {Name: "Regular"}, {Name: " vip "}},
			wantFields: []string{"ticket_types[2].name"},
		},
		{
			name:       "step out of range",
			tickets:    []TicketTypeInput{{Name: "VIP", Steps: []int{-1, 2}}},
			wantFields: []string{"ticket_types[0].steps[0]", "ticket_types[0].steps[1]"},
		},
		{
			name:       "duplicate step",
			tickets:    []TicketTypeInput{{Name: "VIP", Steps: []int{1, 1}}},
			wantFields: []string{"ticket_types[0].steps[1]"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTemplateSet(steps, tt.tickets)
			if len(tt.wantFields) == 0 {
				assertErrorzCode(t, err, "")
				return
			}
			assertErrorzCode(t, err, errorz.CodeBadRequest)
			var e *errorz.Error
			if !errors.As(err, &e) {
				t.Fatal("expected *errorz.Error")
			}
			fields, ok := e.Meta["fields"].(map[string]string)
			if !ok || len(fields) != len(tt.wantFields) {
				t.Fatalf("fields = %v, want keys %v", e.Meta["fields"], tt.wantFields)
			}
			for _, k := range tt.wantFields {
				if _, ok := fields[k]; !ok {
					t.Errorf("missing field %q in %v", k, fields)
				}
			}
		})
	}
}

func TestNewCategory(t *testing.T) {
	own := uuid.New()
	other := uuid.New()
	tests := []struct {
		name       string
		ctx        context.Context
		in         CreateInput
		wantErr    string
		wantTenant *uuid.UUID
	}{
		{
			name: "no tenant claim", ctx: context.Background(),
			in: CreateInput{Source: SourceTenant}, wantErr: errorz.CodeUnauthorized,
		},
		{
			name: "tenant caller cannot create app category", ctx: tenantCtx(own),
			in: CreateInput{Source: SourceApp}, wantErr: errorz.CodeForbidden,
		},
		{name: "platform caller creates app category", ctx: tenantCtx(PlatformTenantID), in: CreateInput{Source: SourceApp}},
		{
			name:       "tenant caller's body tenant_id is ignored",
			ctx:        tenantCtx(own),
			in:         CreateInput{Source: SourceTenant, TenantID: ptrUUID(other)},
			wantTenant: &own,
		},
		{
			name:       "platform caller may create for another tenant",
			ctx:        tenantCtx(PlatformTenantID),
			in:         CreateInput{Source: SourceTenant, TenantID: ptrUUID(other)},
			wantTenant: &other,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newCategory(tt.ctx, tt.in)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr != "" {
				return
			}
			if got.TemplateVersion != 1 {
				t.Errorf("TemplateVersion = %d, want 1", got.TemplateVersion)
			}
			if (got.TenantID == nil) != (tt.wantTenant == nil) || (got.TenantID != nil && *got.TenantID != *tt.wantTenant) {
				t.Errorf("TenantID = %v, want %v", got.TenantID, tt.wantTenant)
			}
		})
	}
}

func TestCategoryService_GetByID(t *testing.T) {
	own := uuid.New()
	stepA, stepB := uuid.New(), uuid.New()

	tests := []struct {
		name    string
		repoRes *EventCategory
		repoErr error
		wantErr string
	}{
		{name: "not found maps to 404", repoErr: repository.ErrNotFound, wantErr: errorz.CodeNotFound},
		{name: "unexpected error maps to 500", repoErr: errors.New("boom"), wantErr: errorz.CodeInternal},
		{
			name:    "another tenant's category is 404",
			repoRes: &EventCategory{Source: SourceTenant, TenantID: ptrUUID(uuid.New())},
			wantErr: errorz.CodeNotFound,
		},
		{name: "app category is readable", repoRes: &EventCategory{Source: SourceApp, TemplateVersion: 3}},
		{
			name:    "own tenant category",
			repoRes: &EventCategory{Source: SourceTenant, TenantID: ptrUUID(own), TemplateVersion: 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[EventCategory, uuid.UUID](ctrl)
			stepRepo := mockrepository.NewMockRepository[workflowsteptemplate.WorkflowStepTemplate, uuid.UUID](ctrl)
			repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(tt.repoRes, tt.repoErr)
			store := &stubStore{views: []TicketTypeTemplateView{
				{Name: "VIP", WorkflowStepTemplateIDs: []uuid.UUID{stepB, stepA}},
			}}
			if tt.wantErr == "" {
				stepRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(
					[]*workflowsteptemplate.WorkflowStepTemplate{{ID: stepA, Name: "a"}, {ID: stepB, Name: "b"}}, int64(2), nil,
				)
			}

			svc := NewService(logger.NewNoOp(), repo, nil, stepRepo, store, nil)
			got, err := svc.GetByID(tenantCtx(own), uuid.New())
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr != "" {
				return
			}
			if store.gotVersion != 3 {
				t.Errorf("store read version %d, want 3", store.gotVersion)
			}
			if len(got.WorkflowSteps) != 2 || got.WorkflowSteps[1].ID != stepB {
				t.Errorf("WorkflowSteps = %+v", got.WorkflowSteps)
			}
			if steps := got.TicketTypes[0].Steps; len(steps) != 2 || steps[0] != 0 || steps[1] != 1 {
				t.Errorf("ticket type Steps = %v, want [0 1]", steps)
			}
		})
	}
}

// TestCategoryService_writeGuards covers what Replace/Delete reject before any
// transaction starts (db is nil here).
func TestCategoryService_writeGuards(t *testing.T) {
	own := uuid.New()
	appCategory := &EventCategory{Source: SourceApp}
	ownCategory := &EventCategory{Source: SourceTenant, TenantID: ptrUUID(own), TemplateVersion: 1}

	t.Run("tenant caller cannot replace an app category", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[EventCategory, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(appCategory, nil)
		svc := NewService(logger.NewNoOp(), repo, nil, nil, nil, nil)
		_, err := svc.Replace(tenantCtx(own), uuid.New(), ReplaceInput{Name: "x", TemplateVersion: 1})
		assertErrorzCode(t, err, errorz.CodeForbidden)
	})

	t.Run("tenant caller cannot delete an app category", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[EventCategory, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(appCategory, nil)
		svc := NewService(logger.NewNoOp(), repo, nil, nil, nil, nil)
		assertErrorzCode(t, svc.Delete(tenantCtx(own), uuid.New()), errorz.CodeForbidden)
	})

	t.Run("invalid template set is 400 before the transaction", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[EventCategory, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(ownCategory, nil)
		svc := NewService(logger.NewNoOp(), repo, nil, nil, nil, nil)
		_, err := svc.Replace(tenantCtx(own), uuid.New(), ReplaceInput{
			Name: "x", TemplateVersion: 1, TicketTypes: []TicketTypeInput{{Name: "VIP", Steps: []int{0}}},
		})
		assertErrorzCode(t, err, errorz.CodeBadRequest)
	})

	t.Run("platform caller deletes an app category", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[EventCategory, uuid.UUID](ctrl)
		repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(appCategory, nil)
		repo.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil)
		svc := NewService(logger.NewNoOp(), repo, nil, nil, nil, nil)
		assertErrorzCode(t, svc.Delete(tenantCtx(PlatformTenantID), uuid.New()), "")
	})
}

func TestCategoryService_replaceTemplateSet(t *testing.T) {
	in := ReplaceInput{
		Name:            "Wedding",
		TemplateVersion: 4,
		WorkflowSteps:   []WorkflowStepInput{{Name: "Check-in"}, {Name: "Dinner", AllowsMultiple: true}},
		TicketTypes:     []TicketTypeInput{{Name: "VIP", Steps: []int{1, 0}}, {Name: "Crew", Steps: []int{}}},
	}

	t.Run("missing category maps to 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		versionRepo := mockcategory.NewMockTemplateVersionRepository(ctrl)
		versionRepo.EXPECT().TemplateVersionForUpdate(gomock.Any(), gomock.Any()).Return(0, repository.ErrNotFound)
		svc := &categoryServiceImpl{versionRepo: versionRepo, logger: logger.NewNoOp()}
		_, err := svc.replaceTemplateSet(context.Background(), &EventCategory{ID: uuid.New()}, in)
		assertErrorzCode(t, err, errorz.CodeNotFound)
	})

	t.Run("stale template_version is 409 and changes nothing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		versionRepo := mockcategory.NewMockTemplateVersionRepository(ctrl)
		versionRepo.EXPECT().TemplateVersionForUpdate(gomock.Any(), gomock.Any()).Return(5, nil)
		store := &stubStore{}
		// No repo/stepRepo expectations: any write would fail the test.
		svc := &categoryServiceImpl{versionRepo: versionRepo, ticketTypes: store, logger: logger.NewNoOp()}
		_, err := svc.replaceTemplateSet(context.Background(), &EventCategory{ID: uuid.New()}, in)
		assertErrorzCode(t, err, errorz.CodeConflict)
		if store.replaceRuns != 0 {
			t.Error("ticket type templates must not be replaced on a stale version")
		}
	})

	t.Run("bumps the version, retires old steps, writes the new set", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mockrepository.NewMockRepository[EventCategory, uuid.UUID](ctrl)
		versionRepo := mockcategory.NewMockTemplateVersionRepository(ctrl)
		stepRepo := mockrepository.NewMockRepository[workflowsteptemplate.WorkflowStepTemplate, uuid.UUID](ctrl)
		oldStep := uuid.New()

		versionRepo.EXPECT().TemplateVersionForUpdate(gomock.Any(), gomock.Any()).Return(4, nil)
		repo.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uuid.UUID, e *EventCategory) error {
				if e.TemplateVersion != 5 || e.Name != "Wedding" {
					t.Errorf("updated category = %+v, want version 5 name Wedding", e)
				}
				return nil
			})
		stepRepo.EXPECT().List(gomock.Any(), gomock.Any()).
			Return([]*workflowsteptemplate.WorkflowStepTemplate{{ID: oldStep}}, int64(1), nil)
		stepRepo.EXPECT().Delete(gomock.Any(), oldStep).Return(nil)
		var created []*workflowsteptemplate.WorkflowStepTemplate
		stepRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Times(2).
			DoAndReturn(func(_ context.Context, s *workflowsteptemplate.WorkflowStepTemplate) error {
				created = append(created, s)
				return nil
			})
		store := &stubStore{}
		svc := &categoryServiceImpl{
			repo: repo, versionRepo: versionRepo, stepRepo: stepRepo, ticketTypes: store, logger: logger.NewNoOp(),
		}

		got, err := svc.replaceTemplateSet(context.Background(), &EventCategory{ID: uuid.New()}, in)
		assertErrorzCode(t, err, "")
		if got.TemplateVersion != 5 || store.gotVersion != 5 {
			t.Errorf("version = %d (store %d), want 5", got.TemplateVersion, store.gotVersion)
		}
		if created[1].OrderIndex != 1 || created[1].Version != 5 || !created[1].AllowsMultiple {
			t.Errorf("second step = %+v", created[1])
		}
		vipSteps := store.gotDrafts[0].WorkflowStepTemplateIDs
		if len(vipSteps) != 2 || vipSteps[0] != created[1].ID || vipSteps[1] != created[0].ID {
			t.Errorf("VIP draft steps = %v, want indexes resolved to new step ids", vipSteps)
		}
		if s := got.TicketTypes[0].Steps; len(s) != 2 || s[0] != 0 || s[1] != 1 {
			t.Errorf("VIP view steps = %v, want [0 1]", s)
		}
	})
}

func TestCategoryService_createCategory(t *testing.T) {
	tests := []struct {
		name    string
		repoErr error
		wantErr string
	}{
		{name: "already exists maps to 409", repoErr: repository.ErrAlreadyExists, wantErr: errorz.CodeConflict},
		{name: "invalid entity maps to 422", repoErr: repository.ErrInvalidEntity, wantErr: errorz.CodeUnprocessableEntity},
		{name: "unexpected repo error maps to 500", repoErr: errors.New("boom"), wantErr: errorz.CodeInternal},
		{name: "happy path writes the template set at version 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[EventCategory, uuid.UUID](ctrl)
			stepRepo := mockrepository.NewMockRepository[workflowsteptemplate.WorkflowStepTemplate, uuid.UUID](ctrl)
			repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tt.repoErr)
			if tt.wantErr == "" {
				stepRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			}
			store := &stubStore{}
			svc := &categoryServiceImpl{repo: repo, stepRepo: stepRepo, ticketTypes: store, logger: logger.NewNoOp()}

			_, err := svc.createCategory(context.Background(), &EventCategory{ID: uuid.New(), TemplateVersion: 1}, CreateInput{
				WorkflowSteps: []WorkflowStepInput{{Name: "Check-in"}},
				TicketTypes:   []TicketTypeInput{{Name: "VIP", Steps: []int{0}}},
			})
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && (store.gotVersion != 1 || len(store.gotDrafts) != 1) {
				t.Errorf("store got version %d drafts %v", store.gotVersion, store.gotDrafts)
			}
		})
	}
}

func TestCategoryService_List(t *testing.T) {
	tests := []struct {
		name     string
		repoErr  error
		wantErr  string
		wantSize int
	}{
		{name: "repo error maps to 500", repoErr: errors.New("boom"), wantErr: errorz.CodeInternal},
		{name: "happy path", wantSize: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mockrepository.NewMockRepository[EventCategory, uuid.UUID](ctrl)
			repo.EXPECT().
				List(gomock.Any(), gomock.Any()).
				Return([]*EventCategory{{Name: "x"}}, int64(1), tt.repoErr)

			svc := NewService(logger.NewNoOp(), repo, nil, nil, nil, nil)
			params, err := query.ParseListParams(url.Values{}, query.ListParseConfig{})
			if err != nil {
				t.Fatalf("ParseListParams() error = %v", err)
			}
			got, err := svc.List(context.Background(), params)
			assertErrorzCode(t, err, tt.wantErr)
			if tt.wantErr == "" && got.Size != tt.wantSize {
				t.Errorf("Size = %d, want %d", got.Size, tt.wantSize)
			}
		})
	}
}
