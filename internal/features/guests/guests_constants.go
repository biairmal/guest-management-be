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

// TicketStatusActive is the status a Ticket is issued with. Transitions to
// used/invalidated are B9 (scans/check-in) concerns, out of scope here.
const TicketStatusActive = "active"
