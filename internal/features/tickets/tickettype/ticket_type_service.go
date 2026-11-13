package tickettype

import (
	"context"
	"encoding/json"
	"errors"

	common "github.com/biairmal/go-sdk/lib/common/dto"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/google/uuid"

	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/features/tickets/tickettypetemplate"
)

// emptyJSONObject is the default value stored for rules when the caller
// omits it, matching the ticket_types.rules column default of '{}'.
var emptyJSONObject = json.RawMessage("{}")

// TicketTypeListConfig declares the allow-listed sort/filter fields for
// ticket type list queries, enforced here in the service via
// query.ValidateListParams and reused by TicketTypeHandler.List for
// query.ParseListParams. event_id is always scoped from the URL, not a
// query filter.
var TicketTypeListConfig = query.ListParseConfig{
	AllowedSortFields:   []string{"id", "name", "created_at", "updated_at"},
	AllowedFilterFields: []string{"name"},
}

// TicketTypeService defines the application-level operations for
// event-scoped ticket types. Every operation takes the owning event's ID so a
// ticket type can never be read through a mismatched event in the URL.
//
// Reads plus the copy-on-create seed since B16: an event's ticket types are
// otherwise written only through the event's setup save, whose tickets-side
// half is SetupStore (event_setup_store.go). The per-ticket-type
// Create/Update/Delete/ReplaceWorkflowSteps operations, and their HTTP routes,
// are gone with it.
//
// No //go:generate mock is declared for this interface: nothing in this
// codebase mocks a feature's own service (no handler-level tests exist yet —
// see docs/TESTING.md), and generating one into the same mocks/tickets/tickettype
// package as TicketTypeWorkflowStepRepository's mock would create an import
// cycle back into this package from this package's own tests. Add one if a
// future consumer (e.g. a handler test) actually needs it. (events/event's
// TicketTypeSeeder, the interface this Service satisfies for the
// copy-on-create seeding, is a separate small interface owned by that
// package and mocked there instead — see event/ticket_type_seeder.go.)
//
// Moved as-is from the tickets package root (docs/DEVELOPMENT_PLAN.md B12);
// the tickettype.TicketTypeService stutter is a side effect of the split
// itself (package tickets never stuttered), not a fresh naming choice —
// internal/app and docs already reference it this way.
//
//nolint:revive // stutter is a side effect of the multi-entity split, not a fresh naming choice
type TicketTypeService interface {
	// GetByID returns a ticket type scoped to eventID with WorkflowStepIDs
	// populated (one extra lookup) — the only read operation that reports
	// workflow-step applicability.
	GetByID(ctx context.Context, eventID, id uuid.UUID) (*TicketType, error)
	List(ctx context.Context, eventID uuid.UUID, params *query.ListParams) (*common.PageResponse[TicketType], error)
	// SeedFromCategoryTemplates implements events/event's TicketTypeSeeder:
	// it copies categoryID's ticket_type_templates at version onto eventID
	// as new TicketTypes, each including exactly the event steps copied from
	// the step templates it included (stepIDs maps step template ID → event
	// step ID). Wired as the TicketTypeSeeder implementation in
	// internal/app — this package never imports events.
	SeedFromCategoryTemplates(
		ctx context.Context, eventID, categoryID uuid.UUID, version int, stepIDs map[uuid.UUID]uuid.UUID,
	) error
}

// ticketTypeServiceImpl is the concrete implementation of TicketTypeService.
type ticketTypeServiceImpl struct {
	repo             repository.Repository[TicketType, uuid.UUID]
	junctionRepo     TicketTypeWorkflowStepRepository
	templateRepo     repository.ReadRepository[tickettypetemplate.TicketTypeTemplate, uuid.UUID]
	templateStepRepo tickettypetemplate.WorkflowStepRepository
	logger           logger.Logger
}

// NewTicketTypeService returns a TicketTypeService with the given
// dependencies. templateRepo is a same-feature, sibling-entity dependency on
// tickettypetemplate's repository — backs SeedFromCategoryTemplates — mirroring
// events/event's existing direct dependency on workflowsteptemplate's repo;
// templateStepRepo is its step junction, read for each template's step access.
//
// The workflow step repository this took before B16 is gone with the two
// operations that needed it: the setup save validates step references against
// the event's own steps, so no cross-feature step lookup happens here any more.
func NewTicketTypeService(
	logger logger.Logger,
	repo repository.Repository[TicketType, uuid.UUID],
	junctionRepo TicketTypeWorkflowStepRepository,
	templateRepo repository.ReadRepository[tickettypetemplate.TicketTypeTemplate, uuid.UUID],
	templateStepRepo tickettypetemplate.WorkflowStepRepository,
) TicketTypeService {
	return &ticketTypeServiceImpl{
		repo: repo, junctionRepo: junctionRepo,
		templateRepo: templateRepo, templateStepRepo: templateStepRepo, logger: logger,
	}
}

// insertTicketType inserts a new ticket type under eventID with the given
// name/rules — shared by SeedFromCategoryTemplates and the setup store's
// ReplaceForEvent (see event_setup_store.go).
func (s *ticketTypeServiceImpl) insertTicketType(
	ctx context.Context, eventID uuid.UUID, name string, rules json.RawMessage,
) (*TicketType, error) {
	entity := &TicketType{ID: uuid.New(), EventID: eventID, Name: name, Rules: rules}
	if err := s.repo.Create(ctx, entity); err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, errorz.Conflict().WithMessage("ticket type already exists for this event")
		}
		if errors.Is(err, repository.ErrInvalidEntity) {
			return nil, errorz.UnprocessableEntity().WithMessage("invalid ticket type data")
		}
		s.logger.ErrorWithContext(ctx, "ticket type create failed", logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to create ticket type")
	}
	s.logger.InfoWithContext(ctx, "ticket type created", logger.F("id", entity.ID), logger.F("event_id", eventID))
	return entity, nil
}

// SeedFromCategoryTemplates lists categoryID's ticket_type_templates at
// version and creates a matching TicketType under eventID for each one,
// including exactly the event steps (via stepIDs) copied from the step
// templates the template included. Unlike Create's best-effort default
// step seed, every failure returns immediately: this runs inside
// EventService.Create's transaction, so a partial copy must roll back the
// whole event creation (B12 AC2).
func (s *ticketTypeServiceImpl) SeedFromCategoryTemplates(
	ctx context.Context, eventID, categoryID uuid.UUID, version int, stepIDs map[uuid.UUID]uuid.UUID,
) error {
	templates, _, err := s.templateRepo.List(ctx, &repository.ListOptions{
		Filter: repository.Filter{Conditions: []repository.FilterCondition{
			{Field: "category_id", Operator: repository.FilterOperatorEq, Value: categoryID},
			{Field: "version", Operator: repository.FilterOperatorEq, Value: version},
		}},
		Pagination: repository.Pagination{Limit: query.DefaultMaxSize},
	})
	if err != nil {
		return s.seedFailed(ctx, eventID, "failed to look up ticket type templates", err)
	}
	ids := make([]uuid.UUID, len(templates))
	for i, tmpl := range templates {
		ids[i] = tmpl.ID
	}
	templateSteps, err := s.templateStepRepo.WorkflowStepTemplateIDs(ctx, ids)
	if err != nil {
		return s.seedFailed(ctx, eventID, "failed to look up ticket type template steps", err)
	}

	for _, tmpl := range templates {
		entity, err := s.insertTicketType(ctx, eventID, tmpl.Name, tmpl.Rules)
		if err != nil {
			return err
		}
		eventStepIDs := make([]uuid.UUID, 0, len(templateSteps[tmpl.ID]))
		for _, stepTemplateID := range templateSteps[tmpl.ID] {
			if id, ok := stepIDs[stepTemplateID]; ok {
				eventStepIDs = append(eventStepIDs, id)
			}
		}
		if err := s.junctionRepo.SetWorkflowStepIDs(ctx, entity.ID, eventStepIDs); err != nil {
			return s.seedFailed(ctx, eventID, "failed to set seeded ticket type workflow steps", err)
		}
	}
	return nil
}

// seedFailed logs and wraps a SeedFromCategoryTemplates failure.
func (s *ticketTypeServiceImpl) seedFailed(ctx context.Context, eventID uuid.UUID, msg string, err error) error {
	s.logger.ErrorWithContext(ctx, msg, logger.F("event_id", eventID), logger.F("error", err))
	return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage(msg)
}

// loadScoped loads a ticket type by id, scoped to eventID, without
// populating WorkflowStepIDs.
func (s *ticketTypeServiceImpl) loadScoped(ctx context.Context, eventID, id uuid.UUID) (*TicketType, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorz.NotFound().WithMessage("ticket type not found")
		}
		s.logger.ErrorWithContext(ctx, "ticket type get failed", logger.F("id", id), logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to get ticket type")
	}
	if entity.EventID != eventID {
		return nil, errorz.NotFound().WithMessage("ticket type not found")
	}
	return entity, nil
}

// GetByID returns a ticket type by ID scoped to eventID, with
// WorkflowStepIDs populated from the ticket_type_workflow_steps junction.
// Returns errorz.NotFound if the ticket type doesn't exist, is
// soft-deleted, or belongs to a different event.
func (s *ticketTypeServiceImpl) GetByID(ctx context.Context, eventID, id uuid.UUID) (*TicketType, error) {
	entity, err := s.loadScoped(ctx, eventID, id)
	if err != nil {
		return nil, err
	}

	ids, err := s.junctionRepo.WorkflowStepIDsByTicketTypeID(ctx, id)
	if err != nil {
		s.logger.ErrorWithContext(ctx, "ticket type workflow step ids lookup failed",
			logger.F("id", id), logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to get ticket type")
	}
	entity.WorkflowStepIDs = ids
	return entity, nil
}

// List returns the ticket types for eventID with filter, sort, and
// pagination from query.ListParams. The event_id scope is enforced
// server-side regardless of query filters. WorkflowStepIDs is left
// unpopulated on every item (see TicketType doc).
func (s *ticketTypeServiceImpl) List(
	ctx context.Context, eventID uuid.UUID, params *query.ListParams,
) (*common.PageResponse[TicketType], error) {
	if err := query.ValidateListParams(params, TicketTypeListConfig); err != nil {
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
		s.logger.ErrorWithContext(ctx, "ticket type list failed", logger.F("error", err))
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to list ticket types")
	}
	return common.NewPageResponse(items, total, params.Page, params.Size), nil
}
