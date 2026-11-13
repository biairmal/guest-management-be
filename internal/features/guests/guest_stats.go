package guests

import (
	"context"
	"fmt"

	"github.com/biairmal/go-sdk/lib/logger"
	reposql "github.com/biairmal/go-sdk/lib/repository/sql"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/biairmal/guest-management-be/internal/features/events/event"
)

// GuestStats implements events/event's GuestStats: the aggregate guest counts
// the event list and the event setup save need. Wired in internal/app so events
// never imports guests.
//
// Direct SQL rather than the generic repository: these are GROUP BY aggregates
// over many events, and repository.Repository has no aggregate read. Counts are
// over live rows only (deleted_at IS NULL) — the generic repository's audit
// decorator adds that filter itself, and this bypasses it, so each query says so
// explicitly.
type GuestStats struct {
	base *reposql.BaseRepository
	log  logger.Logger
}

// NewGuestStats returns a GuestStats backed by direct SQL over guests.
func NewGuestStats(log logger.Logger, db *sqlkit.DB) *GuestStats {
	return &GuestStats{base: reposql.NewBaseRepository(db, guestsTable), log: log}
}

// CountByEvents returns the guest progress of each of eventIDs in one grouped
// query — one query per page of events, not one per event. An event with no
// guests is absent from the map, which reads as zeros.
func (r *GuestStats) CountByEvents(
	ctx context.Context, eventIDs []uuid.UUID,
) (map[uuid.UUID]event.GuestCounts, error) {
	counts := make(map[uuid.UUID]event.GuestCounts, len(eventIDs))
	if len(eventIDs) == 0 {
		return counts, nil
	}
	ids := make([]string, len(eventIDs))
	for i, id := range eventIDs {
		ids[i] = id.String()
	}

	// idx_guests_event_rsvp (event_id, rsvp_status) covers both aggregates.
	const q = `SELECT event_id, count(*), count(*) FILTER (WHERE rsvp_status = $2)
FROM guests WHERE event_id = ANY($1::uuid[]) AND deleted_at IS NULL GROUP BY event_id`
	if r.log != nil {
		r.log.DebugfWithContext(ctx, "query: %s args: %v", q, []any{ids, RsvpStatusConfirmed})
	}
	rows, err := r.base.GetReadConnection(ctx).QueryContext(ctx, q, pq.Array(ids), RsvpStatusConfirmed)
	if err != nil {
		return nil, fmt.Errorf("guests: count by events: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var eventID uuid.UUID
		var c event.GuestCounts
		if err := rows.Scan(&eventID, &c.Total, &c.Confirmed); err != nil {
			return nil, fmt.Errorf("guests: scan event counts: %w", err)
		}
		counts[eventID] = c
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("guests: read event counts: %w", err)
	}
	return counts, nil
}

// CountByTicketType returns how many of eventID's guests hold each ticket type,
// so the event's setup save can refuse to remove one that is still held. Guests
// with no ticket type assigned are not counted.
func (r *GuestStats) CountByTicketType(ctx context.Context, eventID uuid.UUID) (map[uuid.UUID]int, error) {
	const q = `SELECT ticket_type_id, count(*) FROM guests
WHERE event_id = $1 AND ticket_type_id IS NOT NULL AND deleted_at IS NULL GROUP BY ticket_type_id`
	if r.log != nil {
		r.log.DebugfWithContext(ctx, "query: %s args: %v", q, []any{eventID})
	}
	rows, err := r.base.GetReadConnection(ctx).QueryContext(ctx, q, eventID)
	if err != nil {
		return nil, fmt.Errorf("guests: count by ticket type: %w", err)
	}
	defer rows.Close()

	counts := make(map[uuid.UUID]int)
	for rows.Next() {
		var ticketTypeID uuid.UUID
		var n int
		if err := rows.Scan(&ticketTypeID, &n); err != nil {
			return nil, fmt.Errorf("guests: scan ticket type counts: %w", err)
		}
		counts[ticketTypeID] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("guests: read ticket type counts: %w", err)
	}
	return counts, nil
}
