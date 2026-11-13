package event

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/google/uuid"

	"github.com/biairmal/guest-management-be/internal/core/authz"
	"github.com/biairmal/guest-management-be/internal/core/query"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowstep"
)

// GetSetup returns eventID's ordered workflow steps and its ticket types, each
// listing the steps it includes as ascending indexes into the step array — the
// one read an event-setup screen needs.
func (s *eventServiceImpl) GetSetup(ctx context.Context, eventID uuid.UUID) (*Setup, error) {
	if _, err := s.loadScoped(ctx, eventID); err != nil {
		return nil, err
	}
	steps, err := s.eventSteps(ctx, eventID)
	if err != nil {
		return nil, err
	}
	ticketTypes, err := s.ticketTypes.ListForEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	return buildSetup(steps, ticketTypes), nil
}

// ReplaceSetup replaces eventID's whole setup — its workflow steps, their
// order, its ticket types, and each ticket type's step access — in one
// transaction. If any part fails, nothing changes.
//
// Unlike a category's template set (which versions, keeping old rows as
// history), this reconciles: guests, tickets and scan logs point at step and
// ticket type ids, so an item sent with its id is updated in place and keeps
// that id. Items without an id are created; live items left out are deleted.
func (s *eventServiceImpl) ReplaceSetup(ctx context.Context, eventID uuid.UUID, in SetupInput) (*Setup, error) {
	var setup *Setup
	err := s.db.WithTransaction(ctx, func(txCtx context.Context) error {
		var txErr error
		setup, txErr = s.replaceSetup(txCtx, eventID, in)
		return txErr
	})
	if err != nil {
		return nil, err
	}
	s.logger.InfoWithContext(ctx, "event setup replaced", logger.F("event_id", eventID),
		logger.F("workflow_steps", len(setup.WorkflowSteps)), logger.F("ticket_types", len(setup.TicketTypes)))
	return setup, nil
}

// replaceSetup is ReplaceSetup's transaction body, split out so it can be
// tested against mocks without a live transaction (like createEvent).
//
// The event row is locked first, so two setup saves on one event run one after
// the other rather than interleaving their step reorders; there is no version
// compare, so the second save wins (see the B16 non-goals).
func (s *eventServiceImpl) replaceSetup(ctx context.Context, eventID uuid.UUID, in SetupInput) (*Setup, error) {
	if err := s.lockEventInTenant(ctx, eventID); err != nil {
		return nil, err
	}
	steps, err := s.eventSteps(ctx, eventID)
	if err != nil {
		return nil, err
	}
	ticketTypes, err := s.ticketTypes.ListForEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if err := validateSetup(in, steps, ticketTypes); err != nil {
		return nil, err
	}
	if err := s.guardHeldTicketTypes(ctx, eventID, in, ticketTypes); err != nil {
		return nil, err
	}

	// Steps first: the ticket type drafts below reference the saved step ids.
	// Sync already deletes left-out steps and survives a reorder without
	// tripping the (event_id, order_index) unique constraint, and it runs on
	// ctx, so it joins this transaction.
	saved, err := s.steps.Sync(ctx, eventID, syncStepInput(in.WorkflowSteps))
	if err != nil {
		return nil, err
	}
	written, err := s.ticketTypes.ReplaceForEvent(ctx, eventID, ticketTypeDrafts(in.TicketTypes, saved))
	if err != nil {
		return nil, err
	}
	return buildSetup(saved, written), nil
}

// lockEventInTenant locks eventID's row for the rest of ctx's transaction and
// verifies it belongs to the caller's tenant, reporting another tenant's event
// (and a missing one) as not found. Reads the database directly, not the event
// cache, so the tenant it checks is the committed one.
func (s *eventServiceImpl) lockEventInTenant(ctx context.Context, eventID uuid.UUID) error {
	callerTenant, ok := authz.TenantIDFromContext(ctx)
	if !ok {
		return errorz.Unauthorized().WithMessage("no tenant claim present")
	}
	owner, found, err := s.tenantRepo.TenantIDForEventUpdate(ctx, eventID)
	if err != nil {
		return s.internal(ctx, "failed to lock event", err)
	}
	if !found || owner != callerTenant {
		return errorz.NotFound().WithMessage("event not found")
	}
	return nil
}

// guardHeldTicketTypes rejects the save when it would remove a ticket type that
// guests still hold — their tickets reference it, so dropping it would orphan
// them. The guest count is only queried when something is actually being
// removed. The first offender is enough to reject the whole save; nothing has
// been written yet.
func (s *eventServiceImpl) guardHeldTicketTypes(
	ctx context.Context, eventID uuid.UUID, in SetupInput, existing []TicketTypeStoreView,
) error {
	kept := keptIDs(in.TicketTypes)
	removing := false
	for _, e := range existing {
		if !kept[e.ID] {
			removing = true
			break
		}
	}
	if !removing {
		return nil
	}

	held, err := s.guestStats.CountByTicketType(ctx, eventID)
	if err != nil {
		return s.internal(ctx, "failed to count guests by ticket type", err)
	}
	for _, e := range existing {
		if kept[e.ID] {
			continue
		}
		if n := held[e.ID]; n > 0 {
			return errorz.BadRequest().WithMessage("validation failed").WithMeta("fields", map[string]string{
				"ticket_types": fmt.Sprintf("%s is held by %d guests; reassign them first", e.Name, n),
			})
		}
	}
	return nil
}

// eventSteps returns eventID's live workflow steps in step order.
func (s *eventServiceImpl) eventSteps(
	ctx context.Context, eventID uuid.UUID,
) ([]*workflowstep.WorkflowStep, error) {
	steps, _, err := s.workflowStepRepo.List(ctx, &repository.ListOptions{
		Filter: repository.Filter{Conditions: []repository.FilterCondition{
			{Field: "event_id", Operator: repository.FilterOperatorEq, Value: eventID},
		}},
		Pagination: repository.Pagination{Limit: query.DefaultMaxSize},
		Sorts:      []repository.Sort{{Field: "order_index", Direction: repository.SortAsc}},
	})
	if err != nil {
		return nil, s.internal(ctx, "failed to list workflow steps", err)
	}
	return steps, nil
}

// internal logs err and wraps it as an internal errorz error.
func (s *eventServiceImpl) internal(ctx context.Context, msg string, err error) error {
	s.logger.ErrorWithContext(ctx, msg, logger.F("error", err))
	return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage(msg)
}

// keptIDs collects the ids a setup payload's ticket types reference — the ones
// being updated rather than replaced.
func keptIDs(ticketTypes []TicketTypeInput) map[uuid.UUID]bool {
	kept := make(map[uuid.UUID]bool, len(ticketTypes))
	for _, tt := range ticketTypes {
		if tt.ID != nil {
			kept[*tt.ID] = true
		}
	}
	return kept
}

// syncStepInput turns the payload's steps into a workflowstep.Sync request,
// using each step's array position as its order_index — array order is step
// order, so the indexes are unique by construction.
func syncStepInput(steps []WorkflowStepInput) []workflowstep.SyncWorkflowStepInput {
	in := make([]workflowstep.SyncWorkflowStepInput, len(steps))
	for i, st := range steps {
		in[i] = workflowstep.SyncWorkflowStepInput{
			ID: st.ID, Name: st.Name, OrderIndex: i, AllowsMultiple: st.AllowsMultiple,
		}
	}
	return in
}

// ticketTypeDrafts turns the payload's ticket types into store drafts,
// resolving each step index to the id of the step just saved at that position.
// Indexes are already validated in range by validateSetup.
func ticketTypeDrafts(ticketTypes []TicketTypeInput, saved []*workflowstep.WorkflowStep) []TicketTypeDraft {
	drafts := make([]TicketTypeDraft, len(ticketTypes))
	for i, tt := range ticketTypes {
		ids := make([]uuid.UUID, len(tt.Steps))
		for j, idx := range tt.Steps {
			ids[j] = saved[idx].ID
		}
		drafts[i] = TicketTypeDraft{ID: tt.ID, Name: tt.Name, Rules: tt.Rules, WorkflowStepIDs: ids}
	}
	return drafts
}

// validateSetup checks the business rules the boundary validator can't: every
// id sent belongs to this event, ticket type names are unique case-insensitively,
// and every step reference is in range with no duplicates. Errors are keyed by
// JSON path under the "fields" meta, the shape go-sdk's validator emits — the
// same rules as a category's template set save, plus the id-ownership check
// (this save reconciles rather than recreating). An empty payload is valid: it
// clears the event's setup.
func validateSetup(in SetupInput, steps []*workflowstep.WorkflowStep, ticketTypes []TicketTypeStoreView) error {
	fields := map[string]string{}

	liveSteps := make(map[uuid.UUID]bool, len(steps))
	for _, st := range steps {
		liveSteps[st.ID] = true
	}
	for i, st := range in.WorkflowSteps {
		if st.ID != nil && !liveSteps[*st.ID] {
			fields[fmt.Sprintf("workflow_steps[%d].id", i)] = "does not belong to this event"
		}
	}

	liveTicketTypes := make(map[uuid.UUID]bool, len(ticketTypes))
	for _, tt := range ticketTypes {
		liveTicketTypes[tt.ID] = true
	}
	firstByName := map[string]int{}
	for i, tt := range in.TicketTypes {
		if tt.ID != nil && !liveTicketTypes[*tt.ID] {
			fields[fmt.Sprintf("ticket_types[%d].id", i)] = "does not belong to this event"
		}
		key := strings.ToLower(strings.TrimSpace(tt.Name))
		if first, dup := firstByName[key]; dup {
			fields[fmt.Sprintf("ticket_types[%d].name", i)] = fmt.Sprintf("duplicates ticket_types[%d].name", first)
		} else {
			firstByName[key] = i
		}
		validateStepRefs(fields, i, tt.Steps, len(in.WorkflowSteps))
	}

	if len(fields) > 0 {
		return errorz.BadRequest().WithMessage("validation failed").WithMeta("fields", fields)
	}
	return nil
}

// validateStepRefs records a field error for every step reference of ticket
// type i that is out of range or repeated.
func validateStepRefs(fields map[string]string, i int, refs []int, stepCount int) {
	seen := map[int]bool{}
	for j, idx := range refs {
		path := fmt.Sprintf("ticket_types[%d].steps[%d]", i, j)
		switch {
		case idx < 0 || idx >= stepCount:
			fields[path] = fmt.Sprintf("must reference a workflow step (0..%d)", stepCount-1)
		case seen[idx]:
			fields[path] = "duplicate step reference"
		}
		seen[idx] = true
	}
}

// buildSetup assembles the setup response: steps in order, and each ticket
// type's step ids mapped back to ascending step indexes.
func buildSetup(steps []*workflowstep.WorkflowStep, ticketTypes []TicketTypeStoreView) *Setup {
	setup := &Setup{
		WorkflowSteps: make([]WorkflowStepView, len(steps)),
		TicketTypes:   make([]TicketTypeView, len(ticketTypes)),
	}
	indexByID := make(map[uuid.UUID]int, len(steps))
	for i, st := range steps {
		indexByID[st.ID] = i
		setup.WorkflowSteps[i] = WorkflowStepView{ID: st.ID, Name: st.Name, AllowsMultiple: st.AllowsMultiple}
	}
	for i, tt := range ticketTypes {
		idxs := make([]int, 0, len(tt.WorkflowStepIDs))
		for _, id := range tt.WorkflowStepIDs {
			if idx, ok := indexByID[id]; ok {
				idxs = append(idxs, idx)
			}
		}
		sort.Ints(idxs)
		setup.TicketTypes[i] = TicketTypeView{ID: tt.ID, Name: tt.Name, Rules: tt.Rules, Steps: idxs}
	}
	return setup
}
