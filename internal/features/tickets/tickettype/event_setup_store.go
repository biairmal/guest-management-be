package tickettype

import (
	"context"
	"errors"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/google/uuid"

	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/features/events/event"
)

// SetupStore implements events/event's TicketTypeStore: the tickets half of an
// event's setup save (B16 R1). Wired in internal/app so events never imports
// tickets, the same shape as tickettypetemplate.Store for a category's template
// set.
type SetupStore struct {
	repo         repository.Repository[TicketType, uuid.UUID]
	junctionRepo TicketTypeWorkflowStepRepository
	logger       logger.Logger
}

// NewSetupStore returns a SetupStore over the ticket type repository and its
// workflow-step junction repository.
func NewSetupStore(
	logger logger.Logger,
	repo repository.Repository[TicketType, uuid.UUID],
	junctionRepo TicketTypeWorkflowStepRepository,
) *SetupStore {
	return &SetupStore{logger: logger, repo: repo, junctionRepo: junctionRepo}
}

// ListForEvent returns eventID's live ticket types with the workflow steps each
// one includes.
func (s *SetupStore) ListForEvent(
	ctx context.Context, eventID uuid.UUID,
) ([]event.TicketTypeStoreView, error) {
	types, err := s.list(ctx, eventID)
	if err != nil {
		return nil, err
	}
	views := make([]event.TicketTypeStoreView, len(types))
	for i, t := range types {
		stepIDs, err := s.junctionRepo.WorkflowStepIDsByTicketTypeID(ctx, t.ID)
		if err != nil {
			return nil, s.internal(ctx, "failed to look up ticket type workflow steps", err)
		}
		views[i] = event.TicketTypeStoreView{ID: t.ID, Name: t.Name, Rules: t.Rules, WorkflowStepIDs: stepIDs}
	}
	return views, nil
}

// ReplaceForEvent reconciles eventID's ticket types to exactly match drafts: a
// draft with an ID is updated in place and keeps that id (so the tickets already
// issued against it stay valid), one without is created, and any live ticket
// type not in drafts is soft-deleted. Runs inside the caller's transaction.
//
// Deletes run first, so a name freed by a removal is available to a draft in the
// same save.
//
// ponytail: soft delete keeps ticket_types.name, and UNIQUE (event_id, name)
// counts deleted rows, so removing a ticket type and adding a *new* one with the
// same name in one save hits the constraint and surfaces as 409. Upgrade path:
// make the constraint partial on deleted_at IS NULL (a migration), as B15 did
// for the templates by versioning them. Add when re-using a removed name in one
// save actually comes up.
func (s *SetupStore) ReplaceForEvent(
	ctx context.Context, eventID uuid.UUID, drafts []event.TicketTypeDraft,
) ([]event.TicketTypeStoreView, error) {
	existing, err := s.list(ctx, eventID)
	if err != nil {
		return nil, err
	}
	existingByID := make(map[uuid.UUID]*TicketType, len(existing))
	for _, t := range existing {
		existingByID[t.ID] = t
	}

	kept := make(map[uuid.UUID]bool, len(drafts))
	for _, d := range drafts {
		if d.ID != nil {
			kept[*d.ID] = true
		}
	}
	for _, t := range existing {
		if kept[t.ID] {
			continue
		}
		if err := s.repo.Delete(ctx, t.ID); err != nil {
			return nil, s.internal(ctx, "failed to delete ticket type", err)
		}
	}

	views := make([]event.TicketTypeStoreView, len(drafts))
	for i, d := range drafts {
		entity, err := s.write(ctx, eventID, d, existingByID)
		if err != nil {
			return nil, err
		}
		if err := s.junctionRepo.SetWorkflowStepIDs(ctx, entity.ID, d.WorkflowStepIDs); err != nil {
			return nil, s.internal(ctx, "failed to set ticket type workflow steps", err)
		}
		views[i] = event.TicketTypeStoreView{
			ID: entity.ID, Name: entity.Name, Rules: entity.Rules, WorkflowStepIDs: d.WorkflowStepIDs,
		}
	}
	return views, nil
}

// write updates the ticket type d names, or inserts a new one when d carries no
// ID. The caller (events/event's ReplaceSetup) has already validated that a
// non-nil ID belongs to this event; the lookup below is the belt-and-braces
// version of that, so a caller that skipped it gets a 400 rather than a panic.
func (s *SetupStore) write(
	ctx context.Context, eventID uuid.UUID, d event.TicketTypeDraft, existingByID map[uuid.UUID]*TicketType,
) (*TicketType, error) {
	rules := d.Rules
	if len(rules) == 0 {
		rules = emptyJSONObject
	}
	if d.ID == nil {
		entity := &TicketType{ID: uuid.New(), EventID: eventID, Name: d.Name, Rules: rules}
		if err := s.repo.Create(ctx, entity); err != nil {
			return nil, s.writeFailed(ctx, "failed to create ticket type", err)
		}
		return entity, nil
	}

	entity, ok := existingByID[*d.ID]
	if !ok {
		return nil, errorz.BadRequest().WithMessage("ticket type " + d.ID.String() + " does not belong to this event")
	}
	entity.Name, entity.Rules = d.Name, rules
	if err := s.repo.Update(ctx, entity.ID, entity); err != nil {
		return nil, s.writeFailed(ctx, "failed to update ticket type", err)
	}
	return entity, nil
}

// list returns eventID's live ticket types in insertion order.
func (s *SetupStore) list(ctx context.Context, eventID uuid.UUID) ([]*TicketType, error) {
	types, _, err := s.repo.List(ctx, &repository.ListOptions{
		Filter: repository.Filter{Conditions: []repository.FilterCondition{
			{Field: "event_id", Operator: repository.FilterOperatorEq, Value: eventID},
		}},
		Pagination: repository.Pagination{Limit: query.DefaultMaxSize},
		// ponytail: insertion order via created_at (id breaks ties); add an
		// order_index column if payload order must survive coarse clocks. Same
		// call as tickettypetemplate.Store.list.
		Sorts: []repository.Sort{
			{Field: "created_at", Direction: repository.SortAsc}, {Field: "id", Direction: repository.SortAsc},
		},
	})
	if err != nil {
		return nil, s.internal(ctx, "failed to list ticket types", err)
	}
	return types, nil
}

// writeFailed translates a repository sentinel from a ticket type insert or
// update, so a name clash reads as 409 rather than 500.
func (s *SetupStore) writeFailed(ctx context.Context, msg string, err error) error {
	switch {
	case errors.Is(err, repository.ErrAlreadyExists):
		return errorz.Conflict().WithMessage("ticket type already exists for this event")
	case errors.Is(err, repository.ErrNotFound):
		return errorz.NotFound().WithMessage("ticket type not found")
	case errors.Is(err, repository.ErrInvalidEntity):
		return errorz.UnprocessableEntity().WithMessage("invalid ticket type data")
	}
	return s.internal(ctx, msg, err)
}

// internal logs err and wraps it as an internal errorz error.
func (s *SetupStore) internal(ctx context.Context, msg string, err error) error {
	s.logger.ErrorWithContext(ctx, msg, logger.F("error", err))
	return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage(msg)
}
