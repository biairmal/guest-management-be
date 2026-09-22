package category

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	common "github.com/biairmal/go-sdk/lib/common/dto"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	"github.com/google/uuid"

	"github.com/biairmal/guest-management-be/internal/core/authz"
	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowsteptemplate"
)

// Service defines the application-level operations for event categories.
// A category is the aggregate root of its template set: Create and Replace
// write the category, its workflow step templates, and its ticket type
// templates (with their step access) as one version in one transaction.
//
// No //go:generate mock is declared for this interface: its mock would share
// mocks/events/category with TemplateVersionRepository's mock, which this
// package's own tests import — a cycle (same reason as tickettype's
// TicketTypeService). Nothing mocks a feature service today.
type Service interface {
	Create(ctx context.Context, in CreateInput) (*Detail, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Detail, error)
	Replace(ctx context.Context, id uuid.UUID, in ReplaceInput) (*Detail, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, params *query.ListParams) (*common.PageResponse[EventCategory], error)
}

// categoryServiceImpl is the concrete implementation of Service.
type categoryServiceImpl struct {
	repo        repository.Repository[EventCategory, uuid.UUID]
	versionRepo TemplateVersionRepository
	stepRepo    repository.Repository[workflowsteptemplate.WorkflowStepTemplate, uuid.UUID]
	ticketTypes TicketTypeTemplateStore
	db          *sqlkit.DB
	logger      logger.Logger
}

// NewService returns a Service with the given dependencies. stepRepo is the
// sibling workflowsteptemplate repository (same feature); ticketTypes is the
// tickets-side store wired in internal/app; db backs the transactional save.
func NewService(
	logger logger.Logger,
	repo repository.Repository[EventCategory, uuid.UUID],
	versionRepo TemplateVersionRepository,
	stepRepo repository.Repository[workflowsteptemplate.WorkflowStepTemplate, uuid.UUID],
	ticketTypes TicketTypeTemplateStore,
	db *sqlkit.DB,
) Service {
	return &categoryServiceImpl{
		logger: logger, repo: repo, versionRepo: versionRepo, stepRepo: stepRepo, ticketTypes: ticketTypes, db: db,
	}
}

// Create creates a category at template version 1 together with its
// template set. The category's tenant comes from the JWT; only the platform
// tenant may create app categories. If any part fails, nothing is created.
//
//nolint:gocritic // DTO passed by value to match the by-value CreateInput convention across feature services
func (s *categoryServiceImpl) Create(ctx context.Context, in CreateInput) (*Detail, error) {
	entity, err := newCategory(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := validateTemplateSet(in.WorkflowSteps, in.TicketTypes); err != nil {
		return nil, err
	}

	var detail *Detail
	err = s.db.WithTransaction(ctx, func(txCtx context.Context) error {
		detail, err = s.createCategory(txCtx, entity, in)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.logger.InfoWithContext(ctx, "event category created", logger.F("id", entity.ID))
	return detail, nil
}

// newCategory builds the category row for in, applying the caller's scope.
//
//nolint:gocritic // DTO passed by value, see Create
func newCategory(ctx context.Context, in CreateInput) (*EventCategory, error) {
	callerTenant, ok := authz.TenantIDFromContext(ctx)
	if !ok {
		return nil, errorz.Unauthorized().WithMessage("no tenant claim present")
	}
	entity := &EventCategory{ID: uuid.New(), Source: in.Source, Name: in.Name, TemplateVersion: 1}
	switch in.Source {
	case SourceApp:
		if callerTenant != PlatformTenantID {
			return nil, errorz.Forbidden().WithMessage("only the platform tenant may manage app categories")
		}
	case SourceTenant:
		tenantID := callerTenant
		if callerTenant == PlatformTenantID && in.TenantID != nil {
			tenantID = *in.TenantID
		}
		entity.TenantID = &tenantID
	}
	return entity, nil
}

// createCategory is Create's transaction body, split out so it can be tested
// against mocks without a live transaction (like event's createEvent).
//
//nolint:gocritic // DTO passed by value, see Create
func (s *categoryServiceImpl) createCategory(
	ctx context.Context, entity *EventCategory, in CreateInput,
) (*Detail, error) {
	if err := s.repo.Create(ctx, entity); err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, errorz.Conflict().WithMessage("event category already exists")
		}
		if errors.Is(err, repository.ErrInvalidEntity) {
			return nil, errorz.UnprocessableEntity().WithMessage("invalid event category data")
		}
		return nil, s.internal(ctx, "failed to create event category", err)
	}
	return s.writeTemplateSet(ctx, entity, in.WorkflowSteps, in.TicketTypes)
}

// GetByID returns a category with its current template set. Another
// tenant's category resolves as not found; app categories are readable by
// every caller.
func (s *categoryServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*Detail, error) {
	entity, _, err := s.loadScoped(ctx, id)
	if err != nil {
		return nil, err
	}
	steps, err := s.stepTemplates(ctx, id, entity.TemplateVersion)
	if err != nil {
		return nil, err
	}
	ticketTypes, err := s.ticketTypes.ListForCategory(ctx, id, entity.TemplateVersion)
	if err != nil {
		return nil, err
	}
	return buildDetail(entity, steps, ticketTypes), nil
}

// Replace replaces the category's name and whole template set as the next
// template version, in one transaction. in.TemplateVersion must equal the
// current version (409 otherwise); the previous version's rows are
// soft-deleted and kept as history. Existing events are never touched.
func (s *categoryServiceImpl) Replace(ctx context.Context, id uuid.UUID, in ReplaceInput) (*Detail, error) {
	entity, err := s.loadWritable(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := validateTemplateSet(in.WorkflowSteps, in.TicketTypes); err != nil {
		return nil, err
	}

	var detail *Detail
	err = s.db.WithTransaction(ctx, func(txCtx context.Context) error {
		detail, err = s.replaceTemplateSet(txCtx, entity, in)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.logger.InfoWithContext(ctx, "event category template set replaced",
		logger.F("id", id), logger.F("template_version", detail.TemplateVersion))
	return detail, nil
}

// replaceTemplateSet is Replace's transaction body. The version is read
// FOR UPDATE from the database (not the category cache), so concurrent saves
// serialize and the later one fails the compare; the category row itself is
// then written through the cached repository so reads see the new version.
func (s *categoryServiceImpl) replaceTemplateSet(
	ctx context.Context, entity *EventCategory, in ReplaceInput,
) (*Detail, error) {
	current, err := s.versionRepo.TemplateVersionForUpdate(ctx, entity.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorz.NotFound().WithMessage("event category not found")
		}
		return nil, s.internal(ctx, "failed to read event category template version", err)
	}
	if current != in.TemplateVersion {
		return nil, errorz.Conflict().
			WithMessage("event category was changed by someone else; reload and try again").
			WithMeta("template_version", current)
	}

	entity.Name = in.Name
	entity.TemplateVersion = current + 1
	if err := s.repo.Update(ctx, entity.ID, entity); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorz.NotFound().WithMessage("event category not found")
		}
		return nil, s.internal(ctx, "failed to update event category", err)
	}

	old, err := s.stepTemplates(ctx, entity.ID, current)
	if err != nil {
		return nil, err
	}
	for _, step := range old {
		if err := s.stepRepo.Delete(ctx, step.ID); err != nil {
			return nil, s.internal(ctx, "failed to delete workflow step template", err)
		}
	}
	return s.writeTemplateSet(ctx, entity, in.WorkflowSteps, in.TicketTypes)
}

// writeTemplateSet inserts steps and ticketTypes at entity.TemplateVersion
// and returns the resulting detail. Step indexes in ticketTypes are already
// validated by validateTemplateSet.
func (s *categoryServiceImpl) writeTemplateSet(
	ctx context.Context, entity *EventCategory, steps []WorkflowStepInput, ticketTypes []TicketTypeInput,
) (*Detail, error) {
	written := make([]*workflowsteptemplate.WorkflowStepTemplate, len(steps))
	for i, step := range steps {
		written[i] = &workflowsteptemplate.WorkflowStepTemplate{
			ID:             uuid.New(),
			CategoryID:     entity.ID,
			Version:        entity.TemplateVersion,
			Name:           step.Name,
			OrderIndex:     i,
			AllowsMultiple: step.AllowsMultiple,
		}
		if err := s.stepRepo.Create(ctx, written[i]); err != nil {
			return nil, s.internal(ctx, "failed to create workflow step template", err)
		}
	}

	drafts := make([]TicketTypeTemplateDraft, len(ticketTypes))
	for i, tt := range ticketTypes {
		ids := make([]uuid.UUID, len(tt.Steps))
		for j, idx := range tt.Steps {
			ids[j] = written[idx].ID
		}
		drafts[i] = TicketTypeTemplateDraft{Name: tt.Name, Rules: tt.Rules, WorkflowStepTemplateIDs: ids}
	}
	views, err := s.ticketTypes.ReplaceForCategory(ctx, entity.ID, entity.TemplateVersion, drafts)
	if err != nil {
		return nil, err
	}
	return buildDetail(entity, written, views), nil
}

// Delete soft-deletes an event category. The AuditableRepository handles
// setting deleted_at and updated_at.
func (s *categoryServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.loadWritable(ctx, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return errorz.NotFound().WithMessage("event category not found")
		}
		return s.internal(ctx, "failed to delete event category", err)
	}
	s.logger.InfoWithContext(ctx, "event category deleted", logger.F("id", id))
	return nil
}

// loadScoped loads id and applies the read scope: a tenant category of
// another tenant is not found (not forbidden, so ids can't be probed).
// Returns the caller's tenant too.
func (s *categoryServiceImpl) loadScoped(ctx context.Context, id uuid.UUID) (*EventCategory, uuid.UUID, error) {
	callerTenant, ok := authz.TenantIDFromContext(ctx)
	if !ok {
		return nil, uuid.Nil, errorz.Unauthorized().WithMessage("no tenant claim present")
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, uuid.Nil, errorz.NotFound().WithMessage("event category not found")
		}
		return nil, uuid.Nil, s.internal(ctx, "failed to get event category", err)
	}
	if entity.Source == SourceTenant && (entity.TenantID == nil || *entity.TenantID != callerTenant) {
		return nil, uuid.Nil, errorz.NotFound().WithMessage("event category not found")
	}
	return entity, callerTenant, nil
}

// loadWritable is loadScoped plus the write rule: app categories can be
// changed only by the platform tenant.
func (s *categoryServiceImpl) loadWritable(ctx context.Context, id uuid.UUID) (*EventCategory, error) {
	entity, callerTenant, err := s.loadScoped(ctx, id)
	if err != nil {
		return nil, err
	}
	if entity.Source == SourceApp && callerTenant != PlatformTenantID {
		return nil, errorz.Forbidden().WithMessage("only the platform tenant may manage app categories")
	}
	return entity, nil
}

// stepTemplates returns categoryID's live workflow step templates at
// version, in step order.
func (s *categoryServiceImpl) stepTemplates(
	ctx context.Context, categoryID uuid.UUID, version int,
) ([]*workflowsteptemplate.WorkflowStepTemplate, error) {
	steps, _, err := s.stepRepo.List(ctx, &repository.ListOptions{
		Filter: repository.Filter{Conditions: []repository.FilterCondition{
			{Field: "category_id", Operator: repository.FilterOperatorEq, Value: categoryID},
			{Field: "version", Operator: repository.FilterOperatorEq, Value: version},
		}},
		Pagination: repository.Pagination{Limit: query.DefaultMaxSize},
		Sorts:      []repository.Sort{{Field: "order_index", Direction: repository.SortAsc}},
	})
	if err != nil {
		return nil, s.internal(ctx, "failed to list workflow step templates", err)
	}
	return steps, nil
}

// internal logs err and wraps it as an internal errorz error.
func (s *categoryServiceImpl) internal(ctx context.Context, msg string, err error) error {
	s.logger.ErrorWithContext(ctx, msg, logger.F("error", err))
	return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage(msg)
}

// validateTemplateSet checks the business rules the boundary validator
// can't: ticket type names unique case-insensitively, and every step
// reference in range with no duplicates. Errors are keyed by JSON path under
// the "fields" meta, the shape go-sdk's validator emits. steps: [] is valid.
func validateTemplateSet(steps []WorkflowStepInput, ticketTypes []TicketTypeInput) error {
	fields := map[string]string{}
	firstByName := map[string]int{}
	for i, tt := range ticketTypes {
		key := strings.ToLower(strings.TrimSpace(tt.Name))
		if first, dup := firstByName[key]; dup {
			fields[fmt.Sprintf("ticket_types[%d].name", i)] = fmt.Sprintf("duplicates ticket_types[%d].name", first)
		} else {
			firstByName[key] = i
		}

		seen := map[int]bool{}
		for j, idx := range tt.Steps {
			path := fmt.Sprintf("ticket_types[%d].steps[%d]", i, j)
			switch {
			case idx < 0 || idx >= len(steps):
				fields[path] = fmt.Sprintf("must reference a workflow step (0..%d)", len(steps)-1)
			case seen[idx]:
				fields[path] = "duplicate step reference"
			}
			seen[idx] = true
		}
	}
	if len(fields) > 0 {
		return errorz.BadRequest().WithMessage("validation failed").WithMeta("fields", fields)
	}
	return nil
}

// buildDetail assembles the GET shape: steps in order, and each ticket
// type's step template IDs mapped back to ascending step indexes.
func buildDetail(
	entity *EventCategory, steps []*workflowsteptemplate.WorkflowStepTemplate, ticketTypes []TicketTypeTemplateView,
) *Detail {
	detail := &Detail{
		EventCategory: *entity,
		WorkflowSteps: make([]WorkflowStepView, len(steps)),
		TicketTypes:   make([]TicketTypeView, len(ticketTypes)),
	}
	indexByID := make(map[uuid.UUID]int, len(steps))
	for i, step := range steps {
		indexByID[step.ID] = i
		detail.WorkflowSteps[i] = WorkflowStepView{ID: step.ID, Name: step.Name, AllowsMultiple: step.AllowsMultiple}
	}
	for i, tt := range ticketTypes {
		idxs := make([]int, 0, len(tt.WorkflowStepTemplateIDs))
		for _, id := range tt.WorkflowStepTemplateIDs {
			if idx, ok := indexByID[id]; ok {
				idxs = append(idxs, idx)
			}
		}
		sort.Ints(idxs)
		detail.TicketTypes[i] = TicketTypeView{ID: tt.ID, Name: tt.Name, Rules: tt.Rules, Steps: idxs}
	}
	return detail
}

// EventCategoryListConfig declares the allow-listed sort/filter fields for
// event category list queries, enforced here in the service (the
// transport-agnostic authority) via query.ValidateListParams and reused by
// Handler.List for query.ParseListParams. Pagination (page/size/max
// size) is not set here, so it falls back to the shared defaults in
// internal/core/query.
var EventCategoryListConfig = query.ListParseConfig{
	AllowedSortFields:   []string{"id", "source", "tenant_id", "name", "created_at", "updated_at"},
	AllowedFilterFields: []string{"name", "source", "tenant_id"},
}

// List returns event categories with filter, sort, and pagination from query.ListParams.
func (s *categoryServiceImpl) List(
	ctx context.Context, params *query.ListParams,
) (*common.PageResponse[EventCategory], error) {
	if err := query.ValidateListParams(params, EventCategoryListConfig); err != nil {
		return nil, errorz.BadRequest().WithMessage(err.Error())
	}
	items, total, err := s.repo.List(ctx, query.ToListOptions(params))
	if err != nil {
		return nil, s.internal(ctx, "failed to list event categories", err)
	}
	return common.NewPageResponse(items, total, params.Page, params.Size), nil
}
