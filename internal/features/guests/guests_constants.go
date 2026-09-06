package guests

// PermissionManageGuests is the permission code (seeded in B6's catalog,
// migration 000014, previously unused by any slice) required for every
// staff-facing guest endpoint. Gated in guest_routes.go via
// authz.RequirePermission. The public RSVP endpoint is the one deliberate
// exception — gated by invitation token possession instead.
const PermissionManageGuests = "manage_guests"

// RSVP status values (guests.rsvp_status), matching REQUIREMENT.md ss3.10.
const (
	RsvpStatusNone      = "none"
	RsvpStatusInvited   = "invited"
	RsvpStatusConfirmed = "confirmed"
	RsvpStatusDeclined  = "declined"
)

// Ticket.Status values. TicketStatusActive is the status a Ticket is issued
// with. TicketStatusUsed/TicketStatusInvalidated are B9 (scans/check-in)
// concerns: scans.ScanLogService.RecordScan flips a ticket from active to
// used on its first successful scan of any workflow step (a one-way
// transition, never reverted); nothing in this codebase sets
// TicketStatusInvalidated yet (no B9 acceptance criterion requires an
// invalidation trigger — the check is present so RecordScan can reject an
// already-invalidated ticket per AC4 whenever one exists).
const (
	TicketStatusActive      = "active"
	TicketStatusUsed        = "used"
	TicketStatusInvalidated = "invalidated"
)
