package workflowstep

import (
	"context"
	"errors"

	common "github.com/biairmal/go-sdk/lib/common/dto"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/google/uuid"

	"github.com/biairmal/guest-management-be/internal/core/query"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../../mocks/events/workflowstep/mock_service.go -package=mockworkflowstep github.com/biairmal/guest-management-be/internal/features/events/workflowstep Service

// WorkflowStepListConfig declares the allow-listed sort/filter fields for
// workflow step list queries, enforced here in the service via
// query.ValidateListParams and reused by Handler.List for
// query.ParseListParams. event_id is always scoped from the URL, not a
// query filter.
var WorkflowStepListConfig = query.ListParseConfig{
	AllowedSortFields:   []string{"id", "name", "order_index", "allows_multiple", "created_at", "updated_at"},
	AllowedFilterFields: []string{"name"},
}

// Service defines the application-level operations for
// event-scoped workflow steps. Every operation takes the owning event's ID
// so a step can never be read or changed through a mismatched event in the URL.
//
// Since B16 the only write is Sync, and its only caller is the event's setup
// save (events/event's ReplaceSetup, a sibling package of the same feature
// slice): one write path instead of two that could drift. The per-step
// Create/Update/Delete operations, and their HTTP routes, are gone with it.
type Service interface {
	GetByID(ctx context.Context, eventID, id uuid.UUID) (*WorkflowStep, error)
	List(ctx context.Context, eventID uuid.UUID, params *query.ListParams) (*common.PageResponse[WorkflowStep], error)
	// Sync reconciles eventID's workflow steps to exactly match in: entries
	// with an ID are updated in place, entries without an ID are created,
	// and any existing step not referenced by in is deleted. Runs on ctx, so
	// it joins the caller's transaction when there is one.
	Sync(ctx context.Context, eventID uuid.UUID, in []SyncWorkflowStepInput) ([]*WorkflowStep, error)
}

// workflowStepServiceImpl is the concrete implementation of Service.
type workflowStepServiceImpl struct {
	repo   repository.Repository[WorkflowStep, uuid.UUID]
	logger logger.Logger
}

// NewService returns a Service with the given dependencies.
func NewService(
	logger logger.Logger, repo repository.Repository[WorkflowStep, uuid.UUID],
) Service {
	return &workflowStepServiceImpl{logger: logger, repo: repo}
}

// GetByID returns a workflow step by ID scoped to eventID. Returns
// errorz.NotFound if the step doesn't exist, is soft-deleted, or belongs to
// a different event.
func (s *workflowStepServiceImpl) GetByID(ctx context.Context, eventID, id uuid.UUID) (*WorkflowStep, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorz.NotFound().WithMessage("workflow step not found")
		}
		s.logger.ErrorWithContext(ctx, "workflow step get failed", logger.F("id", id), logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to get workflow step")
	}
	if entity.EventID != eventID {
		return nil, errorz.NotFound().WithMessage("workflow step not found")
	}
	return entity, nil
}

// List returns the workflow steps for eventID with filter, sort, and
// pagination from query.ListParams. The event_id scope is enforced
// server-side regardless of query filters.
func (s *workflowStepServiceImpl) List(
	ctx context.Context, eventID uuid.UUID, params *query.ListParams,
) (*common.PageResponse[WorkflowStep], error) {
	if err := query.ValidateListParams(params, WorkflowStepListConfig); err != nil {
		return nil, errorz.BadRequest().WithMessage(err.Error())
	}
	opts := query.ToListOptions(params)
	opts.Filter.Conditions = append(opts.Filter.Conditions, repository.FilterCondition{
		Field:    "event_id",
		Operator: repository.FilterOperatorEq,
		Value:    eventID,
	})

	items, total, err := s.repo.List(ctx, opts)
	if err != nil {
		s.logger.ErrorWithContext(ctx, "workflow step list failed", logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to list workflow steps")
	}
	return common.NewPageResponse(items, total, params.Page, params.Size), nil
}

// Sync reconciles eventID's workflow steps to exactly match in: entries
// with an ID are updated in place, entries without an ID are created, and
// any existing step not referenced by in is deleted.
//
// Reordering is applied in two passes: every updated step first moves to a
// unique negative order_index (phase 1), then to its requested final value
// only once every create/delete/temp-move has landed (phase 2). Without
// this, swapping two steps' order_index (e.g. drag-reorder in a UI) would
// hit the (event_id, order_index) unique constraint on whichever update
// runs first, since its target value is still held by the other row.
func (s *workflowStepServiceImpl) Sync(
	ctx context.Context, eventID uuid.UUID, in []SyncWorkflowStepInput,
) ([]*WorkflowStep, error) {
	if err := validateUniqueOrderIndexes(in); err != nil {
		return nil, err
	}

	existingByID, err := s.reconcileExisting(ctx, eventID, in)
	if err != nil {
		return nil, err
	}

	if err := s.shiftUpdatedStepsToTempOrder(ctx, in, existingByID); err != nil {
		return nil, err
	}

	result := make([]*WorkflowStep, len(in))
	if err := s.createNewSteps(ctx, eventID, in, result); err != nil {
		return nil, err
	}
	if err := s.applyFinalOrder(ctx, in, existingByID, result); err != nil {
		return nil, err
	}

	s.logger.InfoWithContext(ctx, "workflow steps synced", logger.F("event_id", eventID), logger.F("count", len(result)))
	return result, nil
}

// reconcileExisting loads eventID's current steps, validates that every ID
// referenced in in belongs to this event, deletes every existing step not
// referenced by in, and returns the surviving steps indexed by ID for the
// remaining Sync phases.
func (s *workflowStepServiceImpl) reconcileExisting(
	ctx context.Context, eventID uuid.UUID, in []SyncWorkflowStepInput,
) (map[uuid.UUID]*WorkflowStep, error) {
	existing, _, err := s.repo.List(ctx, &repository.ListOptions{
		Filter: repository.Filter{Conditions: []repository.FilterCondition{
			{Field: "event_id", Operator: repository.FilterOperatorEq, Value: eventID},
		}},
		Pagination: repository.Pagination{Limit: query.DefaultMaxSize},
	})
	if err != nil {
		s.logger.ErrorWithContext(ctx, "workflow step sync list failed",
			logger.F("event_id", eventID), logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to list workflow steps")
	}

	existingByID := make(map[uuid.UUID]*WorkflowStep, len(existing))
	for _, e := range existing {
		existingByID[e.ID] = e
	}

	referenced := make(map[uuid.UUID]bool, len(in))
	for _, item := range in {
		if item.ID == nil {
			continue
		}
		if _, ok := existingByID[*item.ID]; !ok {
			return nil, errorz.NotFound().WithMessage("workflow step " + item.ID.String() + " not found")
		}
		referenced[*item.ID] = true
	}

	for _, e := range existing {
		if referenced[e.ID] {
			continue
		}
		if err := s.repo.Delete(ctx, e.ID); err != nil {
			s.logger.ErrorWithContext(ctx, "workflow step sync delete failed", logger.F("id", e.ID), logger.F("error", err))
			return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to delete workflow step")
		}
	}

	return existingByID, nil
}

// shiftUpdatedStepsToTempOrder is Sync's phase 1: every entry that
// references an existing step is updated with its new name/allows_multiple
// and a unique negative order_index, freeing its old slot without
// colliding with any other step's current or future order_index.
func (s *workflowStepServiceImpl) shiftUpdatedStepsToTempOrder(
	ctx context.Context, in []SyncWorkflowStepInput, existingByID map[uuid.UUID]*WorkflowStep,
) error {
	for i, item := range in {
		if item.ID == nil {
			continue
		}
		cur := existingByID[*item.ID]
		cur.Name = item.Name
		cur.AllowsMultiple = item.AllowsMultiple
		cur.OrderIndex = -(i + 1)
		if err := s.repo.Update(ctx, cur.ID, cur); err != nil {
			s.logger.ErrorWithContext(ctx, "workflow step sync update (temp order) failed",
				logger.F("id", cur.ID), logger.F("error", err))
			return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to update workflow step")
		}
	}
	return nil
}

// createNewSteps is Sync's phase 2: entries without an ID are inserted with
// their requested (final) order_index. By this point every pre-existing
// step occupying a real order_index has either been deleted or moved to a
// temporary negative one, so no collision is possible here.
func (s *workflowStepServiceImpl) createNewSteps(
	ctx context.Context, eventID uuid.UUID, in []SyncWorkflowStepInput, result []*WorkflowStep,
) error {
	for i, item := range in {
		if item.ID != nil {
			continue
		}
		entity := &WorkflowStep{
			ID:             uuid.New(),
			EventID:        eventID,
			Name:           item.Name,
			OrderIndex:     item.OrderIndex,
			AllowsMultiple: item.AllowsMultiple,
		}
		if err := s.repo.Create(ctx, entity); err != nil {
			if errors.Is(err, repository.ErrInvalidEntity) {
				return errorz.UnprocessableEntity().WithMessage("invalid workflow step data")
			}
			s.logger.ErrorWithContext(ctx, "workflow step sync create failed", logger.F("error", err))
			return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to create workflow step")
		}
		result[i] = entity
	}
	return nil
}

// applyFinalOrder is Sync's phase 3: updated steps move from their
// temporary negative order_index (phase 1) to the requested final value,
// now that the real order_index space is free of collisions.
func (s *workflowStepServiceImpl) applyFinalOrder(
	ctx context.Context, in []SyncWorkflowStepInput, existingByID map[uuid.UUID]*WorkflowStep, result []*WorkflowStep,
) error {
	for i, item := range in {
		if item.ID == nil {
			continue
		}
		cur := existingByID[*item.ID]
		cur.OrderIndex = item.OrderIndex
		if err := s.repo.Update(ctx, cur.ID, cur); err != nil {
			s.logger.ErrorWithContext(ctx, "workflow step sync update failed", logger.F("id", cur.ID), logger.F("error", err))
			return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to update workflow step")
		}
		result[i] = cur
	}
	return nil
}

// validateUniqueOrderIndexes rejects a Sync payload where two entries share
// the same order_index — the (event_id, order_index) DB constraint would
// otherwise reject one of them mid-reconciliation instead of surfacing a
// clean 400 up front.
func validateUniqueOrderIndexes(in []SyncWorkflowStepInput) error {
	seen := make(map[int]bool, len(in))
	for _, item := range in {
		if seen[item.OrderIndex] {
			return errorz.BadRequest().WithMessage("duplicate order_index in workflow step list")
		}
		seen[item.OrderIndex] = true
	}
	return nil
}
