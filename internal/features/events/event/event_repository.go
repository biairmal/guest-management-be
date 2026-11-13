package event

import (
	"context"
	"fmt"

	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	reposql "github.com/biairmal/go-sdk/lib/repository/sql"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../../mocks/events/eventtenant/mock_event_tenant_repository.go -package=mockeventtenant github.com/biairmal/guest-management-be/internal/features/events/event EventTenantRepository

const eventsTable = "events"

// eventColumns are the columns selected on reads (GetByID, List).
var eventColumns = []string{
	"id", "tenant_id", "category_id", "category_template_version", "name", "description",
	"start_date", "end_date", "is_multi_day", "rsvp_required", "created_at", "updated_at", "deleted_at",
}

// NewEventRepository returns a soft-delete-aware repository for events.
// TID is uuid.UUID — kept typed all the way through the service layer.
func NewEventRepository(
	log logger.Logger, db *sqlkit.DB, cacheOpts corerepository.CacheOptions,
) repository.Repository[Event, uuid.UUID] {
	return corerepository.NewRepository[Event, uuid.UUID](
		log, db, eventsTable, eventColumns, cacheOpts,
	)
}

// EventTenantRepository reads which tenant owns an event, straight from the
// database rather than the event cache, so the answer is the committed one.
// Deliberately narrow: a tenant check on every {event_id} route must not pay
// for (or be fooled by) a full cached entity read.
//
// The stutter is unavoidable — this is the events/event package's repository
// of event tenants, and the consumer-side interface it satisfies lives in
// internal/core/authz.
//
// Its generated mock goes to mocks/events/eventtenant rather than
// mocks/events/event: that package holds Service's mock, which imports this
// package, so importing it from this package's own tests would cycle (the same
// trap TicketTypeSeeder documents). Split out, the mock's package references
// only uuid, so event's tests can use it.
//
//nolint:revive // stutter: package event's repository, named for what it reads
type EventTenantRepository interface {
	// EventTenantID returns the tenant owning eventID and whether a live
	// (not soft-deleted) event with that ID exists. It satisfies
	// authz.EventTenantResolver, which internal/app wires this to — core
	// never imports this feature.
	EventTenantID(ctx context.Context, eventID uuid.UUID) (tenantID uuid.UUID, found bool, err error)
	// TenantIDForEventUpdate is EventTenantID with the event row locked
	// exclusively (FOR UPDATE) until ctx's transaction ends, so two setup
	// saves on one event run one after the other instead of interleaving
	// their step reorders. Same idiom as category's TemplateVersionForUpdate.
	TenantIDForEventUpdate(ctx context.Context, eventID uuid.UUID) (tenantID uuid.UUID, found bool, err error)
}

// sqlEventTenantRepository implements EventTenantRepository with direct SQL on
// the transaction in ctx (leader when there is none).
type sqlEventTenantRepository struct {
	base *reposql.BaseRepository
	log  logger.Logger
}

// NewEventTenantRepository returns an EventTenantRepository backed by direct SQL.
func NewEventTenantRepository(log logger.Logger, db *sqlkit.DB) EventTenantRepository {
	return &sqlEventTenantRepository{base: reposql.NewBaseRepository(db, eventsTable), log: log}
}

// EventTenantID implements EventTenantRepository.
func (r *sqlEventTenantRepository) EventTenantID(ctx context.Context, eventID uuid.UUID) (uuid.UUID, bool, error) {
	return r.tenantID(ctx, eventID, "")
}

// TenantIDForEventUpdate implements EventTenantRepository.
func (r *sqlEventTenantRepository) TenantIDForEventUpdate(
	ctx context.Context, eventID uuid.UUID,
) (uuid.UUID, bool, error) {
	return r.tenantID(ctx, eventID, " FOR UPDATE")
}

// tenantID reads eventID's tenant, optionally under lock.
func (r *sqlEventTenantRepository) tenantID(
	ctx context.Context, eventID uuid.UUID, lock string,
) (uuid.UUID, bool, error) {
	q := `SELECT tenant_id FROM events WHERE id = $1 AND deleted_at IS NULL` + lock
	if r.log != nil {
		r.log.DebugfWithContext(ctx, "query: %s args: %v", q, []any{eventID})
	}
	var tenantID uuid.UUID
	err := r.base.GetConnection(ctx).QueryRowContext(ctx, q, eventID).Scan(&tenantID)
	if sqlkit.IsNoRows(err) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("events: read tenant id: %w", err)
	}
	return tenantID, true, nil
}
