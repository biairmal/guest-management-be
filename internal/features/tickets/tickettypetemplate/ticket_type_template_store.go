package tickettypetemplate

import (
	"context"
	"encoding/json"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/google/uuid"

	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/features/events/category"
)

// emptyJSONObject is the default value stored for rules when the caller
// omits it, matching the ticket_type_templates.rules column default of '{}'.
var emptyJSONObject = json.RawMessage("{}")

// Store implements events/category's TicketTypeTemplateStore: the tickets
// half of a category's template set save (B15). Wired in internal/app so
// events never imports tickets.
type Store struct {
	repo     repository.Repository[TicketTypeTemplate, uuid.UUID]
	stepRepo WorkflowStepRepository
	logger   logger.Logger
}

// NewStore returns a Store over the ticket type template repository and its
// step junction repository.
func NewStore(
	logger logger.Logger, repo repository.Repository[TicketTypeTemplate, uuid.UUID], stepRepo WorkflowStepRepository,
) *Store {
	return &Store{logger: logger, repo: repo, stepRepo: stepRepo}
}

// ListForCategory returns categoryID's live ticket type templates at version
// with the step templates each one includes.
func (s *Store) ListForCategory(
	ctx context.Context, categoryID uuid.UUID, version int,
) ([]category.TicketTypeTemplateView, error) {
	templates, err := s.list(ctx, categoryID, version)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(templates))
	for i, t := range templates {
		ids[i] = t.ID
	}
	stepIDs, err := s.stepRepo.WorkflowStepTemplateIDs(ctx, ids)
	if err != nil {
		return nil, s.internal(ctx, "failed to look up ticket type template steps", err)
	}

	views := make([]category.TicketTypeTemplateView, len(templates))
	for i, t := range templates {
		views[i] = category.TicketTypeTemplateView{
			ID: t.ID, Name: t.Name, Rules: t.Rules, WorkflowStepTemplateIDs: stepIDs[t.ID],
		}
	}
	return views, nil
}

// ReplaceForCategory soft-deletes categoryID's live ticket type templates
// (the previous version, kept as history) and inserts drafts at version with
// their step links. Runs inside the caller's transaction.
func (s *Store) ReplaceForCategory(
	ctx context.Context, categoryID uuid.UUID, version int, drafts []category.TicketTypeTemplateDraft,
) ([]category.TicketTypeTemplateView, error) {
	old, err := s.list(ctx, categoryID, 0)
	if err != nil {
		return nil, err
	}
	for _, t := range old {
		if err := s.repo.Delete(ctx, t.ID); err != nil {
			return nil, s.internal(ctx, "failed to delete ticket type template", err)
		}
	}

	views := make([]category.TicketTypeTemplateView, len(drafts))
	for i, d := range drafts {
		rules := d.Rules
		if len(rules) == 0 {
			rules = emptyJSONObject
		}
		t := &TicketTypeTemplate{ID: uuid.New(), CategoryID: categoryID, Version: version, Name: d.Name, Rules: rules}
		if err := s.repo.Create(ctx, t); err != nil {
			return nil, s.internal(ctx, "failed to create ticket type template", err)
		}
		if err := s.stepRepo.Insert(ctx, t.ID, d.WorkflowStepTemplateIDs); err != nil {
			return nil, s.internal(ctx, "failed to link ticket type template steps", err)
		}
		views[i] = category.TicketTypeTemplateView{
			ID: t.ID, Name: t.Name, Rules: t.Rules, WorkflowStepTemplateIDs: d.WorkflowStepTemplateIDs,
		}
	}
	return views, nil
}

// list returns categoryID's live ticket type templates, at version when it
// is non-zero, else at any version.
func (s *Store) list(ctx context.Context, categoryID uuid.UUID, version int) ([]*TicketTypeTemplate, error) {
	conds := []repository.FilterCondition{
		{Field: "category_id", Operator: repository.FilterOperatorEq, Value: categoryID},
	}
	if version != 0 {
		conds = append(conds, repository.FilterCondition{
			Field: "version", Operator: repository.FilterOperatorEq, Value: version,
		})
	}
	templates, _, err := s.repo.List(ctx, &repository.ListOptions{
		Filter:     repository.Filter{Conditions: conds},
		Pagination: repository.Pagination{Limit: query.DefaultMaxSize},
		// ponytail: insertion order via created_at (id breaks ties); add an
		// order_index column if payload order must survive coarse clocks.
		Sorts: []repository.Sort{
			{Field: "created_at", Direction: repository.SortAsc}, {Field: "id", Direction: repository.SortAsc},
		},
	})
	if err != nil {
		return nil, s.internal(ctx, "failed to list ticket type templates", err)
	}
	return templates, nil
}

// internal logs err and wraps it as an internal errorz error.
func (s *Store) internal(ctx context.Context, msg string, err error) error {
	s.logger.ErrorWithContext(ctx, msg, logger.F("error", err))
	return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage(msg)
}
