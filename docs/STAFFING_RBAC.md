# Roles, Permissions & Event Staffing — Requirements

> Supersedes the role/permission pseudo-code in REQUIREMENT.md §3.2 (User) and §3.3
> (EventStaffAssignment), which predates this design. Schema reference: DATABASE.md §3.2–3.5, §3.10.

## 1. Overview

Every user in the system has exactly one **system-level role** (e.g. Super Admin, Tenant Admin, Tenant
Staff) that governs what they can do across their tenant. Separately, a user may be assigned to
specific **events** with an **event-level role** (e.g. Usher, Photobooth Staff) that governs what they
can do within that one event only. Both kinds of role are built on one shared roles/permissions engine,
kept apart by an explicit scope so a tenant-wide role can never be assigned to an event and vice versa.

## 2. User hierarchy

- **Super Admin** — exactly one user, platform-wide, not scoped to any real tenant. Can manage tenants
  and everything within them.
- **Tenant Admin** — full administrator within one tenant. A tenant may have multiple Tenant Admins.
- **Tenant Master User** — at most one designated user per tenant (already an existing concept,
  `users.is_tenant_master`), independent of role — this is an ownership/default-contact flag, not a
  permission level.
- **Tenant Staff** — baseline staff member of a tenant; day-to-day operational access, not
  administrative.
- **Event staff** — any tenant user (Tenant Admin or Tenant Staff) may additionally be assigned to one
  or more events with an event-scoped role. Event roles are a separate, smaller set of roles meant to
  describe what someone does *at* an event (check people in, run a photobooth, ...), not their standing
  within the tenant.

Only a user holding a permission that allows staff management may assign other (lower-level) users to
an event.

## 3. Roles & permissions model

- A **role** has a name, description, and a **scope**: `system` (assignable to a user's tenant-wide
  role) or `event` (assignable to a staff member's role on one specific event). A role is one or the
  other, never both.
- A **permission** is a discrete, named capability (e.g. "manage guests", "check in guests"). Roles are
  granted a set of permissions; a user's effective permissions are exactly the permissions of their
  role.
- Roles and permissions are global, reusable definitions — not duplicated per tenant. (Tenant-specific
  customization of roles/permissions is out of scope for this phase; see §7.)
- Starter role set:

  | Role | Scope | Grants |
  |---|---|---|
  | Super Admin | system | every permission |
  | Tenant Admin | system | manage users, manage events, manage staff, manage guests, manage workflows, check in |
  | Tenant Staff | system | manage guests, check in |
  | Usher | event | check in |
  | Photobooth Staff | event | check in |

- Starter permission catalog:

  | Permission | Meaning |
  |---|---|
  | `manage_tenants` | create/update/delete tenants (platform-wide) |
  | `manage_users` | create/update/delete users within a tenant |
  | `manage_events` | create/update/delete events and event categories |
  | `manage_staff` | assign/remove staff on an event |
  | `manage_guests` | create/update/delete guests and tickets |
  | `manage_workflows` | create/update/delete an event's workflow steps |
  | `check_in` | scan tickets / record guest check-ins |

  This catalog is expected to grow (e.g. future photobooth-specific permissions) without requiring a
  new mechanism — new permissions and roles are additive.

## 4. Super Admin

- There is exactly one Super Admin in the whole system, ever.
- Every user belongs to a tenant (no exceptions), so the Super Admin is modeled as belonging to one
  reserved, non-customer-facing tenant created specifically to hold platform-level users. This keeps
  "every user has a tenant" true without adding a special case elsewhere in the system.
- The system must prevent more than one user from ever holding the Super Admin role.

## 5. Event staff assignment

- A user can be assigned to a given event at most once at a time, with exactly one event-level role.
- Assigning a user to an event requires: the user belongs to the same tenant that owns the event, and
  the role being assigned is an event-scoped role (not a system-scoped one).
- Removing a user from an event and later re-assigning them to the same event is allowed — this is
  treated as a new assignment, not a reactivation, so the full history of who was ever staffed on an
  event is preserved.
- Changing an existing assignment's role is allowed; changing which user or which event an assignment
  refers to is not — that is a removal plus a new assignment instead, so an event's staffing history
  stays an accurate log of who did what.
- Listing staff always reflects one specific event; there is no cross-event staff listing in this
  phase.

## 6. Access control rules

- A request is authenticated (who is making it) separately from being authorized (what they're allowed
  to do) — this document concerns authorization.
- Every action a user takes against staffing is checked against their **system-level role's**
  permissions (their event-level role, if any, does not itself grant staffing-management rights — only
  `manage_staff` does, which is a system-level permission held by Tenant Admin/Super Admin).
- A user may only see or affect data belonging to their own tenant. Attempting to act on another
  tenant's event or user must fail the same way a nonexistent resource would (not reveal that the
  resource exists but belongs to someone else).
- A role change for a user takes effect promptly — a user should not be able to retain old permissions
  indefinitely after their role is downgraded or changed.

## 7. Out of scope for this phase

- Administrative endpoints for creating/editing custom roles or permissions (the starter set in §3 is
  fixed via initial setup; extending it currently requires a deploy, not an in-app action).
- Per-tenant custom roles (all tenants share the same role/permission catalog).
- Additional event roles beyond Usher and Photobooth Staff (the model supports adding more later
  without any structural change).
