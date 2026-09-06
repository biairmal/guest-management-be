# 📄 Guest Management System — Requirements & Domain Specification
### Format Used:
This document uses a structured System Requirements Specification (SRS) format, which includes:

- Functional Requirements
- Non-functional Requirements
- Domain Model Definitions
- Entity Relationship Structure
- Workflow Definitions
- Multi-tenancy model

All diagrams are written in pseudo-UML text, and domain definitions follow Domain-Driven Design (DDD) terminology.

---


# 🏛 1. System Overview
A multi-tenant guest management platform supporting:

- Multi-day events
- Invitations & RSVP
- Ticket distribution (QR)
- Check-in & workflow processing
- Staff-level access & role permissions
- VIP workflows
- Customizable workflow steps (default templates + tenant overrides)

System supports:

- App-wide templates/configurations (global defaults)
- Tenant-level templates/configurations (tenant-specific overrides)
- Event-level customizations

---


# 🏢 2. Multi-Tenant Architecture Requirements

## 2.1 Tenants
- Each tenant represents a customer organization.
- A tenant has:
  - Staff users
  - Tenants’ own event categories
  - Tenants’ workflow templates
  - Branding, settings, and communication templates
  - Their own guests & events

## 2.2 Template/Config Layers
Three layers of configuration:

### 1. Application Default (Global)
- Default event categories  
- Default workflow templates per category  
- Default ticket rules  
- Available workflows  

### 2. Tenant Templates (Overrides / Extend)
- Tenant can:
  - Override app workflow templates
  - Create new categories
  - Customize ticket types
  - Customize email/SMS templates

Tenant defaults apply to all future events of that tenant.

### 3. Event Customization (Instance level)
The organizer can:
- Add/remove workflow steps
- Reorder steps
- Assign steps to ticket types
- Add VIP-only workflows  
- Add event-specific staff users  

---


# 🧩 3. Domain Concepts & Entities

Below are the main domain entities with concise definitions.

## 3.1 Tenant

    Tenant
    - id
    - name
    - type
    - settings
    - branding
    - staffUsers[] (tenant-level)
    - categories[] (tenant-level overrides)
    - workflowStepTemplates[] (tenant-level overrides)

## 3.2 User

A person who can log in to the system.\
Two types:

-   Tenant-level staff -- can manage multiple events
-   Event-level staff -- only manages a specific event

```
    User
    - id
    - tenantId
    - email
    - passwordHash
    - role (TenantAdmin, TenantStaff, EventStaff)
```

## 3.3 EventStaffAssignment

Links a user to a specific event.

    EventStaffAssignment
    - id
    - eventId
    - userId
    - permissions (CheckIn, ManageGuests, ManageWorkflows)

## 3.4 Event

    Event
    - id
    - tenantId
    - categoryId
    - name
    - description
    - startDate
    - endDate
    - isMultiDay
    - rsvpRequired (default true — when false, a guest's ticket is issued at invitation time instead of waiting on RSVP confirmation)
    - staffAssignments[]
    - workflows[] (generated from templates + customized)
    - ticketTypes[]
    - guests[]

## 3.5 Event Category

Events belong to a category that defines default behavior.

Three layers:

    EventCategory (app default)
    TenantEventCategory (tenant override)
    EventEventCategory (assigned to event)

Unified structure:

    EventCategoryBase
    - id
    - source (APP or TENANT)
    - name
    - workflowStepTemplates[]

## 3.6 WorkflowStepTemplate

Defines default steps for a category.

    WorkflowStepTemplate
    - id
    - categoryId
    - name
    - orderIndex
    - allowsMultiple
    - ticketTypeApplicability (optional)

## 3.7 WorkflowStep (Event-level)

Customized version used by an event.

    WorkflowStep
    - id
    - eventId
    - name
    - orderIndex
    - allowsMultiple
    - appliesToTicketTypes[]

## 3.8 TicketType

Tickets define what workflows a guest can perform.

    TicketType
    - id
    - eventId
    - name (Regular, VIP, etc.)
    - rules (single entry / multi entry)
    - workflowStepIds[]

## 3.9 Ticket

QR-based admission artifact.

    Ticket
    - id
    - guestId
    - eventId
    - ticketTypeId
    - qrCode
    - status (Active, Used, Invalidated)

## 3.10 Guest

    Guest
    - id
    - eventId
    - name
    - email
    - phone
    - ticketTypeId
    - invitationToken (opaque, unguessable — lets an unauthenticated guest reach their own RSVP link)
    - rsvpStatus (None, Invited, Confirmed, Declined)
    - ticketId

## 3.11 ScanLog

Represents scanning a QR for workflow processing.

    ScanLog
    - id
    - eventId
    - ticketId
    - workflowStepId
    - timestamp

------------------------------------------------------------------------



# 🚀 4. Functional Requirements

## 4.1 Invitation & RSVP

-   System sends invitation emails.
-   Each invited guest gets a unique `invitationToken` embedded in their invitation link; the guest
    confirms/declines through that link **without logging in** (guests are not `User` accounts).
-   Guest can confirm/decline RSVP.
-   Tenant can customize invitation templates.
-   RSVP must update guest status.
-   Confirming RSVP triggers ticket issuance for guests who already have a ticket type assigned (see §4.2) —
    **only when the event requires RSVP** (`rsvpRequired = true`). When an event doesn't require RSVP, the
    guest's ticket is issued as soon as the invitation is sent, not gated on any RSVP response.

## 4.2 Ticket Distribution

-   Tickets can be sent:
    -   Only to RSVP-confirmed guests
    -   To all invited guests
-   Tickets are QR-based.
-   Multi-use or single-use rules based on ticket type.
-   Distribution can be triggered automatically or manually.

## 4.3 Workflow Handling

-   Workflow represents event processes:
    -   Check-in\
    -   Souvenir pickup\
    -   Photo booth\
    -   VIP lounge\
    -   Checkout\
-   Steps come from:
    -   App default templates
    -   Tenant overrides
    -   Event customizations

### Behavior:

-   Each QR scan maps to a workflow step.
-   Steps may only be performed once unless allowsMultiple = true.

## 4.4 Customized Workflows for Ticket Types

-   VIP ticket includes more steps.
-   Regular ticket includes fewer steps.
-   Each ticket type can override applicability of steps.

## 4.5 Event Day Operations

-   Staff app scans QR.
-   System validates:
    -   Ticket ownership
    -   Ticket usage rules
    -   Workflow step eligibility
-   Logs the scan.

## 4.6 Event Staff Access

-   Event owner can add existing users as staff or invite new staff.
-   Staff must log in.
-   Permissions:
    -   Guest management
    -   Only scanning
    -   Workflow management
    -   Full event admin

## 4.7 Thank You Messages

-   After event completion:
    -   System can automatically send thank-you messages.
    -   Template is customizable at tenant level.

------------------------------------------------------------------------



# 🧱 5. Non-functional Requirements

## 5.1 Multi-Tenant Isolation

-   No shared guests, events, or staff between tenants unless explicitly
    assigned.

## 5.2 Customization

Hierarchy: 1. App default\
2. Tenant template\
3. Event-level customization

## 5.3 Scalability

-   Support large events, high scanning volume, bulk email operations.

## 5.4 Security

-   Password authentication
-   Role-based authorization
-   Tenant data isolation
-   Guest PII — `email` and `phone` — is encrypted at rest (`name` stays plaintext, so it remains searchable
    by partial match). Authorized staff still see plaintext values through the API; only storage is opaque.
    Exact-match search on `email`/`phone` works via a deterministic blind index alongside the ciphertext, so a
    search must supply the complete value — no partial match on encrypted fields.

------------------------------------------------------------------------



# 🔗 6. Domain Relationship Diagram (Text UML)

    Tenant
    ├── Users (TenantStaff, TenantAdmin)
    ├── TenantEventCategory
    │       └── WorkflowStepTemplate
    └── Events
            ├── EventStaffAssignment → User
            ├── WorkflowStep (from templates + custom)
            ├── TicketType
            │       └── WorkflowSteps (many-to-many)
            └── Guests
                └── Ticket
                        └── ScanLog

------------------------------------------------------------------------



# 📦 7. Workflow Template Logic Summary

1.  App default templates store the baseline workflows.
2.  Tenant templates override them (add/remove/modify).
3.  Event creation copies tenant templates.
4.  Event-level customization allows final adjustments.

------------------------------------------------------------------------
