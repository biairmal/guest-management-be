# Database Schema Documentation — Guest Management (PostgreSQL)

This document describes the PostgreSQL schema for the Guest Management System: tables, columns, relationships, and design decisions. The schema supports multi-tenancy, role-based permissions, three-layer configuration (app → tenant → event), QR tickets, workflow steps, message templates (email/WhatsApp), and soft delete.

---

## 1. Overview

- **Database:** PostgreSQL (13+ recommended for `gen_random_uuid()`).
- **Migrations:** [golang-migrate](https://github.com/golang-migrate/migrate); migration files live in `./migrations`.
- **Default database name:** `guest_management` (override via `DATABASE_URL`).

**Design principles:**

- **Multi-tenant:** All tenant-scoped data carries `tenant_id`; isolation is enforced in the application layer (filter by `tenant_id`).
- **Three-layer configuration:** App defaults, tenant overrides, and event-level customisation for categories, workflow templates, and message templates.
- **Soft delete:** Most entity tables use `deleted_at TIMESTAMPTZ NULL`; `NULL` means active. List queries should use `WHERE deleted_at IS NULL` unless including deleted rows.
- **Role-based permissions:** Users have a `role_id` (FK to `roles`); roles have many permissions via `role_permissions`. Permissions and roles are configurable by the system administrator.
- **Enum-like columns:** Values such as `rsvp_status`, ticket `status`, and `source` are stored as `VARCHAR(32)` with `CHECK` constraints; the canonical set of values is defined in the Go application.

---

## 2. Entity and Table Summary

| Domain Entity           | PostgreSQL Table             | Purpose |
| ----------------------- | ----------------------------- | ------- |
| Tenant                  | `tenants`                     | Organisation; settings and branding as JSONB. |
| Permission              | `permissions`                 | Permission codes (e.g. check_in, create_users); system table. |
| Role                    | `roles`                       | Role names (e.g. Tenant Admin, Event Staff); system table. |
| RolePermission          | `role_permissions`            | Many-to-many: role ↔ permission. |
| User                    | `users`                       | Tenant member; `role_id`, `is_tenant_master` (default user per tenant). |
| EventStaffAssignment    | `event_staff_assignments`     | User assigned to event with a role (permissions from role). |
| Event                   | `events`                      | Event; `tenant_id`, `category_id`, dates, `is_multi_day`. |
| Event Category          | `event_categories`            | Category (app or tenant); single table with `source`. |
| WorkflowStepTemplate    | `workflow_step_templates`     | Template steps per category. |
| WorkflowStep            | `workflow_steps`              | Event-level workflow steps (from templates + custom). |
| TicketType              | `ticket_types`                | Ticket type per event (e.g. Regular, VIP); rules in JSONB. |
| TicketType ↔ WorkflowStep | `ticket_type_workflow_steps` | Many-to-many: ticket type ↔ workflow step. |
| Ticket                  | `tickets`                     | QR ticket; `guest_id`, `event_id`, `ticket_type_id`, `status`. |
| Guest                   | `guests`                      | Guest per event; `rsvp_status`, optional `ticket_id`. |
| ScanLog                 | `scan_logs`                   | Log of QR scan (ticket + workflow step); audit trail. |
| MessageTemplate         | `message_templates`           | Email/WhatsApp templates (app, tenant, or event scope). |

---

## 3. Tables and Columns

All tables use `UUID` primary keys with `DEFAULT gen_random_uuid()`. Timestamps use `TIMESTAMPTZ`. Tables that support soft delete include `deleted_at`; listing should filter by `deleted_at IS NULL` unless including deleted rows.

### 3.1 tenants

Represents a tenant (organisation). Holds settings and branding as JSONB.

| Column      | Type         | Nullable | Description |
| ----------- | ------------ | -------- | ----------- |
| id          | UUID         | No       | Primary key. |
| name        | TEXT         | No       | Display name of the tenant (organisation). |
| type        | TEXT         | Yes      | Tenant type (e.g. default, enterprise). |
| settings    | JSONB        | No       | Tenant-level configuration (default `{}`). |
| branding    | JSONB        | No       | Branding assets and options (default `{}`). |
| created_at  | TIMESTAMPTZ  | No       | When the row was created. |
| updated_at  | TIMESTAMPTZ  | No       | When the row was last updated. |
| deleted_at  | TIMESTAMPTZ  | Yes      | When the row was soft-deleted; NULL if active. |

---

### 3.2 permissions

System table of permission codes. Configurable by the system administrator; no soft delete.

| Column      | Type         | Nullable | Description |
| ----------- | ------------ | -------- | ----------- |
| id          | UUID         | No       | Primary key. |
| code        | VARCHAR(32)  | No       | Permission code (e.g. check_in, manage_guests, create_users); unique. |
| name        | TEXT         | No       | Display name. |
| description | TEXT         | Yes      | Optional description. |
| created_at  | TIMESTAMPTZ  | No       | When the row was created. |
| updated_at  | TIMESTAMPTZ  | No       | When the row was last updated. |

---

### 3.3 roles

System table of role names. Permissions are assigned via `role_permissions`; no soft delete.

| Column      | Type         | Nullable | Description |
| ----------- | ------------ | -------- | ----------- |
| id          | UUID         | No       | Primary key. |
| name        | VARCHAR(128) | No       | Role name (e.g. Tenant Admin, Event Staff). |
| description | TEXT         | Yes      | Optional description. |
| created_at  | TIMESTAMPTZ  | No       | When the row was created. |
| updated_at  | TIMESTAMPTZ  | No       | When the row was last updated. |

---

### 3.4 role_permissions

Junction table: many-to-many between roles and permissions. No soft delete.

| Column         | Type | Nullable | Description |
| -------------- | ---- | -------- | ----------- |
| role_id        | UUID | No       | Role (FK to roles.id); part of primary key. |
| permission_id  | UUID | No       | Permission (FK to permissions.id); part of primary key. |

---

### 3.5 users

Users belonging to a tenant. Each user has one role (`role_id`); at most one user per tenant has `is_tenant_master = true` (default/master user who can create other users for that tenant).

| Column           | Type        | Nullable | Description |
| ----------------- | ----------- | -------- | ----------- |
| id                | UUID        | No       | Primary key. |
| tenant_id         | UUID        | No       | Tenant this user belongs to (FK to tenants.id). |
| email             | TEXT        | No       | Login email; unique per tenant. |
| password_hash     | TEXT        | No       | Hashed password. |
| role_id           | UUID        | No       | Role (FK to roles.id); determines permissions. |
| is_tenant_master  | BOOLEAN     | No       | True if this user is the default/master for the tenant; at most one per tenant. |
| created_at        | TIMESTAMPTZ | No       | When the row was created. |
| updated_at        | TIMESTAMPTZ | No       | When the row was last updated. |
| deleted_at        | TIMESTAMPTZ | Yes      | When the row was soft-deleted; NULL if active. |

**Constraints:** `UNIQUE (tenant_id, email)`; partial unique index on `(tenant_id) WHERE is_tenant_master = true`.

---

### 3.6 event_categories

Event categories with a single table and a `source` discriminator: `app` (global) or `tenant` (tenant override). When `source = 'app'`, `tenant_id` is NULL; when `source = 'tenant'`, `tenant_id` is set. Events reference one category via `category_id`.

| Column      | Type         | Nullable | Description |
| ----------- | ------------ | -------- | ----------- |
| id          | UUID         | No       | Primary key. |
| source      | VARCHAR(32)  | No       | One of: app, tenant (CHECK; managed in Go). App = global template; tenant = tenant override. |
| tenant_id   | UUID         | Yes      | Tenant that owns this category when source = tenant; NULL when source = app. |
| name        | TEXT         | No       | Category display name. |
| created_at  | TIMESTAMPTZ  | No       | When the row was created. |
| updated_at  | TIMESTAMPTZ  | No       | When the row was last updated. |
| deleted_at  | TIMESTAMPTZ  | Yes      | When the row was soft-deleted; NULL if active. |

**Constraint:** `(source = 'app' AND tenant_id IS NULL) OR (source = 'tenant' AND tenant_id IS NOT NULL)`.

---

### 3.7 workflow_step_templates

Template workflow steps attached to an event category. When an event is created or configured, the application copies these into `workflow_steps` (no FK back to template).

| Column                    | Type        | Nullable | Description |
| ------------------------- | ----------- | -------- | ----------- |
| id                        | UUID        | No       | Primary key. |
| category_id               | UUID        | No       | Category this template belongs to (FK to event_categories.id). |
| name                      | TEXT        | No       | Step name (e.g. Check-in, Photo booth). |
| order_index               | INT         | No       | Order within the category (unique per category). |
| allows_multiple           | BOOLEAN     | No       | Whether this step can be completed more than once per ticket. |
| ticket_type_applicability | JSONB       | Yes      | Optional rules for which ticket types this step applies to. |
| created_at                | TIMESTAMPTZ | No       | When the row was created. |
| updated_at                | TIMESTAMPTZ | No       | When the row was last updated. |
| deleted_at                | TIMESTAMPTZ | Yes      | When the row was soft-deleted; NULL if active. |

**Constraint:** `UNIQUE (category_id, order_index)`.

---

### 3.8 events

Events belong to a tenant and an event category. Each event has workflow steps, ticket types, guests, and optional staff assignments and message templates.

| Column       | Type        | Nullable | Description |
| ------------ | ----------- | -------- | ----------- |
| id           | UUID        | No       | Primary key. |
| tenant_id    | UUID        | No       | Tenant that owns this event (FK to tenants.id). |
| category_id  | UUID        | No       | Event category used for this event (FK to event_categories.id). |
| name         | TEXT        | No       | Event display name. |
| description  | TEXT        | Yes      | Event description. |
| start_date   | TIMESTAMPTZ | No       | Event start (with timezone). |
| end_date     | TIMESTAMPTZ | No       | Event end (with timezone). |
| is_multi_day | BOOLEAN     | No       | Whether the event spans multiple days. |
| created_at   | TIMESTAMPTZ | No       | When the row was created. |
| updated_at   | TIMESTAMPTZ | No       | When the row was last updated. |
| deleted_at   | TIMESTAMPTZ | Yes      | When the row was soft-deleted; NULL if active. |

---

### 3.9 workflow_steps

Event-level workflow steps (e.g. Check-in, Photo booth). Derived from category templates or customised per event; no FK back to template.

| Column          | Type        | Nullable | Description |
| --------------- | ----------- | -------- | ----------- |
| id              | UUID        | No       | Primary key. |
| event_id        | UUID        | No       | Event this step belongs to (FK to events.id). |
| name            | TEXT        | No       | Step name (e.g. Check-in). |
| order_index     | INT         | No       | Order within the event (unique per event). |
| allows_multiple | BOOLEAN     | No       | Whether this step can be completed more than once per ticket. |
| created_at      | TIMESTAMPTZ | No       | When the row was created. |
| updated_at      | TIMESTAMPTZ | No       | When the row was last updated. |
| deleted_at      | TIMESTAMPTZ | Yes      | When the row was soft-deleted; NULL if active. |

**Constraint:** `UNIQUE (event_id, order_index)`.

---

### 3.10 event_staff_assignments

Links a user to an event with a specific role. Permissions for that assignment are those of the role (from `role_permissions`).

| Column      | Type        | Nullable | Description |
| ----------- | ----------- | -------- | ----------- |
| id          | UUID        | No       | Primary key. |
| event_id    | UUID        | No       | Event (FK to events.id). |
| user_id     | UUID        | No       | User assigned (FK to users.id). |
| role_id     | UUID        | No       | Role for this event (FK to roles.id); permissions come from this role. |
| created_at  | TIMESTAMPTZ | No       | When the row was created. |
| updated_at  | TIMESTAMPTZ | No       | When the row was last updated. |
| deleted_at  | TIMESTAMPTZ | Yes      | When the row was soft-deleted; NULL if active. |

**Constraint:** `UNIQUE (event_id, user_id)`.

---

### 3.11 ticket_types

Ticket types per event (e.g. Regular, VIP). Rules (e.g. single_entry, multi_entry) are stored in JSONB.

| Column      | Type        | Nullable | Description |
| ----------- | ----------- | -------- | ----------- |
| id          | UUID        | No       | Primary key. |
| event_id    | UUID        | No       | Event this ticket type belongs to (FK to events.id). |
| name        | TEXT        | No       | Ticket type name (e.g. Regular, VIP); unique per event. |
| rules       | JSONB       | No       | Entry rules and other config (default `{}`). |
| created_at  | TIMESTAMPTZ | No       | When the row was created. |
| updated_at  | TIMESTAMPTZ | No       | When the row was last updated. |
| deleted_at  | TIMESTAMPTZ | Yes      | When the row was soft-deleted; NULL if active. |

**Constraint:** `UNIQUE (event_id, name)`.

---

### 3.12 ticket_type_workflow_steps

Junction table: many-to-many between ticket types and workflow steps. Determines which workflow steps apply to which ticket type. No soft delete.

| Column            | Type | Nullable | Description |
| ----------------- | ---- | -------- | ----------- |
| ticket_type_id    | UUID | No       | Ticket type (FK to ticket_types.id); part of primary key. |
| workflow_step_id  | UUID | No       | Workflow step (FK to workflow_steps.id); part of primary key. |

---

### 3.13 guests

Guests belong to an event. They have an RSVP status and may have an assigned ticket.

| Column      | Type        | Nullable | Description |
| ----------- | ----------- | -------- | ----------- |
| id          | UUID        | No       | Primary key. |
| event_id    | UUID        | No       | Event this guest belongs to (FK to events.id). |
| name        | TEXT        | No       | Guest display name. |
| email       | TEXT        | No       | Guest email. |
| phone       | TEXT        | Yes      | Guest phone. |
| rsvp_status | VARCHAR(32) | No       | One of: none, invited, confirmed, declined (CHECK; managed in Go). |
| ticket_id   | UUID        | Yes      | Assigned ticket (FK to tickets.id) if any. |
| created_at  | TIMESTAMPTZ | No       | When the row was created. |
| updated_at  | TIMESTAMPTZ | No       | When the row was last updated. |
| deleted_at  | TIMESTAMPTZ | Yes      | When the row was soft-deleted; NULL if active. |

---

### 3.14 tickets

QR-based admission artifact. Each ticket is for one guest, one event, and one ticket type. Status is managed in Go (e.g. active, used, invalidated).

| Column         | Type        | Nullable | Description |
| -------------- | ----------- | -------- | ----------- |
| id             | UUID        | No       | Primary key. |
| guest_id       | UUID        | No       | Guest who holds this ticket (FK to guests.id). |
| event_id       | UUID        | No       | Event (FK to events.id). |
| ticket_type_id | UUID        | No       | Ticket type (FK to ticket_types.id). |
| qr_code        | TEXT        | No       | QR code value; unique per event. |
| status         | VARCHAR(32) | No       | One of: active, used, invalidated (CHECK; managed in Go). |
| created_at     | TIMESTAMPTZ | No       | When the row was created. |
| updated_at     | TIMESTAMPTZ | No       | When the row was last updated. |
| deleted_at     | TIMESTAMPTZ | Yes      | When the row was soft-deleted; NULL if active. |

**Constraint:** `UNIQUE (event_id, qr_code)`.

---

### 3.15 scan_logs

Audit log of a QR scan: which ticket, which workflow step, when, and optionally which operator. No soft delete.

| Column            | Type        | Nullable | Description |
| ----------------- | ----------- | -------- | ----------- |
| id                | UUID        | No       | Primary key. |
| event_id          | UUID        | No       | Event (FK to events.id). |
| ticket_id         | UUID        | No       | Ticket scanned (FK to tickets.id). |
| workflow_step_id  | UUID        | No       | Step completed (FK to workflow_steps.id). |
| scanned_at        | TIMESTAMPTZ | No       | When the scan occurred. |
| operator_user_id  | UUID        | Yes      | Staff user who performed the scan (FK to users.id), if recorded. |

---

### 3.16 message_templates

Templates for sending messages (e.g. invitation, ticket delivery, thank-you) via email or WhatsApp. Three scopes: **app** (global), **tenant** (tenant-level), **event** (event-specific). Resolve template for an event: prefer event-level, then tenant-level, then app.

| Column      | Type         | Nullable | Description |
| ----------- | ------------ | -------- | ----------- |
| id          | UUID         | No       | Primary key. |
| source      | VARCHAR(32)  | No       | One of: app, tenant, event (CHECK; managed in Go). |
| tenant_id   | UUID         | Yes      | Tenant that owns this template when source = tenant or event; NULL when source = app. |
| event_id    | UUID         | Yes      | Event this template belongs to when source = event; NULL when source = app or tenant. |
| name        | VARCHAR(128) | No       | Template name (e.g. Invitation, Ticket delivery, Thank you). |
| channel     | VARCHAR(32)  | No       | One of: email, whatsapp (CHECK; managed in Go). |
| subject     | TEXT         | Yes      | Email subject line; NULL for WhatsApp. |
| body        | TEXT         | No       | Message body; may contain placeholders (e.g. {{guest_name}}, {{event_name}}). |
| variables   | JSONB        | Yes      | Optional list of variable names for validation/UI. |
| created_at  | TIMESTAMPTZ  | No       | When the row was created. |
| updated_at  | TIMESTAMPTZ  | No       | When the row was last updated. |
| deleted_at  | TIMESTAMPTZ  | Yes      | When the row was soft-deleted; NULL if active. |

**Constraints:**  
- `(source = 'app' AND tenant_id IS NULL AND event_id IS NULL) OR (source = 'tenant' AND tenant_id IS NOT NULL AND event_id IS NULL) OR (source = 'event' AND tenant_id IS NOT NULL AND event_id IS NOT NULL)`.  
- Uniqueness per scope: app — `(name, channel)`; tenant — `(tenant_id, name, channel)`; event — `(event_id, name, channel)` (enforced via partial unique indexes).

---

## 4. Relationship Diagram (Mermaid)

The diagram below shows tables and their relationships. Cardinality: `||--o{` means one-to-many; `}o--||` means many-to-one; `||--o|` means one-to-zero-or-one. Entities with soft delete include a `deleted_at` column; `scan_logs` and `ticket_type_workflow_steps` do not use soft delete.

```mermaid
erDiagram
    tenants ||--o{ users : "has"
    tenants ||--o{ event_categories : "has"
    tenants ||--o{ events : "has"
    tenants ||--o{ message_templates : "has"
    events ||--o{ message_templates : "has"

    roles ||--o{ users : "role"
    roles ||--o{ role_permissions : "has"
    permissions ||--o{ role_permissions : "in"
    roles ||--o{ event_staff_assignments : "role"

    event_categories ||--o{ workflow_step_templates : "has"
    events }o--|| event_categories : "category"
    events ||--o{ workflow_steps : "has"
    events ||--o{ ticket_types : "has"
    events ||--o{ guests : "has"
    events ||--o{ event_staff_assignments : "has"

    users ||--o{ event_staff_assignments : "assigned"
    ticket_types ||--o{ ticket_type_workflow_steps : ""
    workflow_steps ||--o{ ticket_type_workflow_steps : ""
    guests ||--o| tickets : "has"
    tickets }o--|| ticket_types : "type"
    tickets ||--o{ scan_logs : "scanned"
    workflow_steps ||--o{ scan_logs : "step"
    events ||--o{ scan_logs : "event"

    tenants { uuid id string name jsonb settings jsonb branding timestamptz deleted_at }
    permissions { uuid id varchar32 code string name }
    roles { uuid id varchar128 name }
    role_permissions { uuid role_id uuid permission_id }
    users { uuid id uuid tenant_id string email uuid role_id bool is_tenant_master timestamptz deleted_at }
    event_categories { uuid id varchar32 source uuid tenant_id_nullable string name timestamptz deleted_at }
    workflow_step_templates { uuid id uuid category_id int order_index bool allows_multiple timestamptz deleted_at }
    events { uuid id uuid tenant_id uuid category_id timestamptz start_date timestamptz end_date timestamptz deleted_at }
    workflow_steps { uuid id uuid event_id int order_index bool allows_multiple timestamptz deleted_at }
    ticket_types { uuid id uuid event_id string name jsonb rules timestamptz deleted_at }
    ticket_type_workflow_steps { uuid ticket_type_id uuid workflow_step_id }
    guests { uuid id uuid event_id varchar32 rsvp_status uuid ticket_id_nullable timestamptz deleted_at }
    tickets { uuid id uuid guest_id uuid event_id uuid ticket_type_id string qr_code varchar32 status timestamptz deleted_at }
    scan_logs { uuid id uuid event_id uuid ticket_id uuid workflow_step_id timestamptz scanned_at }
    event_staff_assignments { uuid id uuid event_id uuid user_id uuid role_id timestamptz deleted_at }
    message_templates { uuid id varchar32 source uuid tenant_id_nullable uuid event_id_nullable varchar128 name varchar32 channel text subject text body jsonb variables timestamptz deleted_at }
```

**Relationship summary:**

- **Tenants** have many users, event_categories (tenant scope), events, and message_templates (tenant/event scope).
- **Roles** are assigned to users (tenant-level) and to event_staff_assignments (event-level); roles have many permissions via role_permissions.
- **Events** belong to one tenant and one event_category; they have workflow_steps, ticket_types, guests, event_staff_assignments, scan_logs, and optionally message_templates.
- **Event categories** have many workflow_step_templates.
- **Ticket types** and **workflow_steps** are linked by ticket_type_workflow_steps (many-to-many).
- **Guests** have zero or one ticket; **tickets** reference guest, event, and ticket_type.
- **Scan_logs** record a scan of a ticket at a workflow step (and optionally the operator user).

---

## 5. Soft Delete and System Tables

**Tables with soft delete:**  
tenants, users, event_categories, workflow_step_templates, events, workflow_steps, event_staff_assignments, ticket_types, guests, tickets, message_templates.

For these tables, `deleted_at IS NULL` means the row is active. List queries should use `WHERE deleted_at IS NULL` unless deleted rows are explicitly needed. Partial indexes `(deleted_at) WHERE deleted_at IS NULL` support these queries.

**Tables without soft delete:**  
permissions, roles, role_permissions (system/reference data), scan_logs (audit trail), ticket_type_workflow_steps (junction).

---

## 6. Migrations

Migrations are applied in order from `./migrations` using golang-migrate. Sequence: 000001 (tenants) → 000002 (permissions, roles, role_permissions) → 000003 (users) → 000004 (event_categories, workflow_step_templates) → 000005 (events, workflow_steps) → 000006 (message_templates) → 000007 (event_staff_assignments) → 000008 (ticket_types, ticket_type_workflow_steps) → 000009 (guests, tickets) → 000010 (scan_logs) → 000011 (indexes).

To apply all pending migrations:

```bash
make migration-up
```

Override the database URL if needed:

```bash
make migration-up DATABASE_URL=postgres://user:pass@host:5432/guest_management?sslmode=disable
```
