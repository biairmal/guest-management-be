# Feature behaviour

How the app behaves, feature by feature — intent, invariants, endpoints, and lifecycle. This satisfies the "document how the app works" goal and is required for every feature (see [../AGENTS.md](../AGENTS.md#documentation)). For the data model, see [DATABASE.md](DATABASE.md); for the roadmap of unbuilt features, see [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md).

Each section follows the same template:

- **Intent** — what the feature is for.
- **Invariants** — rules that always hold (enforced in the service).
- **Endpoints** — HTTP surface (also in Swagger).
- **States & lifecycle** — how records are created, changed, and removed.

---

## events

Source: `internal/features/events`. Tables: `event_categories`, `events`, `workflow_steps`, `workflow_step_templates` (see [DATABASE.md](DATABASE.md)). One vertical slice covering event categories (the taxonomy), events themselves, their per-event workflow steps, and each category's versioned template set (workflow step templates here; ticket type templates in [tickets](#tickets)).

### Event categories

#### Intent

Manages **event categories** — the taxonomy events are classified under — together with each category's **template set**: the ordered workflow steps, the ticket types, and which steps each ticket type includes, that every new event in the category starts from (REQUIREMENT.md §3.5, §3.12, §4.11; B15). Categories are either **app-defined** (available to every tenant) or **tenant-defined** (private to one tenant). The category is the aggregate root of its template set: it is read, created, and replaced as one unit, in one transaction. The rows live in `workflow_step_templates` (this slice) and `ticket_type_templates` + `ticket_type_template_workflow_steps` (written through `tickettypetemplate.Store`, which `internal/app` wires in as the category's `TicketTypeTemplateStore`, so `events` never imports `tickets`).

#### Invariants

- `source` is one of `"app"` or `"tenant"`; app categories have no `tenant_id`, tenant categories always have one.
- **Tenant scope comes from the JWT** (`tenant_id` claim) on every single-category call. A tenant category of another tenant is **404** (not 403, so ids can't be probed). App categories are readable by everyone.
- **Only the platform tenant** (`00000000-0000-0000-0000-000000000001`, seeded by migration 000014) may create, replace, or delete app categories; any other tenant gets **403**. On create, `tenant_id` in the body is used only when the platform tenant creates a tenant category; everyone else's tenant category belongs to their own tenant.
- **Writes require `manage_events`** (`POST`/`PUT`/`DELETE`, via `authz.RequirePermission`).
- **Versioned template sets.** `template_version` starts at 1; every successful `PUT` increments it. The previous version's template rows are soft-deleted and kept as history; the new version gets new rows. Because uniqueness is per `(category_id, version, …)`, removing a step or ticket type and adding one back with the same name or position in a later save never collides with old rows.
- **Optimistic concurrency.** `PUT` must carry the `template_version` it was based on. The service reads the version `FOR UPDATE` from the database (never the cache) inside the save transaction; a mismatch is **409** with `meta.template_version` = the current version, and nothing changes. Concurrent saves serialize on that row lock; the later one fails the compare.
- **Template set rules** (400, `meta.fields` keyed by JSON path, same shape as the boundary validator): ticket type names unique within the payload, case-insensitively (`ticket_types[2].name`); every `steps` entry is an index into `workflow_steps` (`ticket_types[0].steps[1]`), in range and not repeated. `steps: []` is valid. Missing `name`s and arrays over 100 items are rejected at the boundary (the go-sdk validator keys nested errors by field name, not path).
- Saving a template set **never changes events already created** from the category — events copy the template rows once, at creation (see [Events](#events)).

#### Endpoints

Base path `/api/v1/event-categories`:

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List categories (paginated, filtered, sorted; no template sets) | 200 | 400 invalid query |
| `GET` | `/{id}` | Category + current template set | 200 | 400 bad UUID · 404 not found / other tenant |
| `POST` | `/` | Create a category with its template set (`manage_events`) | 201 | 400 · 403 · 409 · 422 |
| `PUT` | `/{id}` | Replace name + whole template set (`manage_events`) | 200 | 400 · 403 · 404 · 409 stale `template_version` |
| `DELETE` | `/{id}` | Soft delete (`manage_events`) | 204 | 400 · 403 · 404 |

**Single-category shape** (GET/POST/PUT response):
`{id, source, tenant_id, name, template_version, created_at, updated_at, workflow_steps: [{id, name, allows_multiple}], ticket_types: [{id, name, rules, steps: [int]}]}` — `workflow_steps` array order is step order; `steps` are ascending indexes into `workflow_steps`.

**Bodies:** `POST {source, tenant_id?, name, workflow_steps: [{name, allows_multiple}], ticket_types: [{name, rules?, steps: [int]}]}`; `PUT {name, template_version, workflow_steps, ticket_types}` (full replacement; `source`/`tenant_id` immutable). `rules` is opaque JSON, default `{}`.

**List query:** `?page=1&size=20&sort=name,ASC&sort=id,DESC&name=Gala&source=app`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, source, tenant_id, name, created_at, updated_at`); unknown field → 400.
- Filters: `name`, `source`, `tenant_id` (exact match); unknown keys ignored. The list is **not** tenant-scoped yet (PRD-001 Q4 stays open for it).

#### States & lifecycle

- **Create** — one transaction: category row at `template_version = 1`, then the step templates (`order_index` = array index), then the ticket type templates and their step links. Any failure creates nothing.
- **Replace** — one transaction: lock + compare version, write the category row (new name, version + 1) through the cached repository so reads see it, soft-delete the live step and ticket type templates, insert the new set at the new version. Any failure changes nothing.
- **Delete** — **soft**: `deleted_at` is set; template rows are left as they are. Events can no longer be created in a deleted category (422).
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrAlreadyExists`→409, `ErrInvalidEntity`→422); unexpected errors become 500 and are logged with context.

### Events

#### Intent

Manages **events** — a tenant's scheduled occasion, classified under an event category, with a start/end date range. Events are the parent of workflow steps, ticket types, guests, and staff assignments (see [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md) B5–B9). Since B16 an event is also the **aggregate root of its setup**: its workflow steps, its ticket types, and each ticket type's step access are read and written as one unit, the same way a [category](#event-categories) owns its template set.

#### Invariants

- `category_id`, `name`, `start_date`, and `end_date` are required on create.
- **`tenant_id` is never caller-supplied.** On create it comes from the caller's `tenant_id` JWT claim (401 without it) — it is not part of `CreateEventInput`, so one tenant cannot create an event under another. It is **immutable** afterwards: an event cannot move tenants.
- **Every operation is scoped to the caller's tenant.** `GetByID`/`Update`/`Delete`/`GetSetup`/`ReplaceSetup` report another tenant's event as `errorz.NotFound` (404), never `Forbidden` — so its ids cannot be probed (the same not-found-not-forbidden convention as [categories](#event-categories) and [staffing](#staffing)). `List` appends a `tenant_id = <caller>` condition after `query.ToListOptions`, and `tenant_id` is neither a filter nor a sort field.
- **Every `{event_id}`-scoped route of every feature enforces the same rule, once, in middleware.** `authz.RequireEventInTenant` reads the event's owning tenant (one primary-key read, straight from the database via `event.EventTenantRepository`, never the event cache) and writes 404 when it is missing or differs from the caller's. It is `r.Use`'d **before** `authz.RequirePermission` on the setup, [workflow-steps](#workflow-steps), [ticket-types](#ticket-types), [guests](#guests), [staffing](#staffing) and [scans](#scans) route groups (not the public RSVP route), so another tenant's event reads as absent rather than forbidden. Before B16 `authz.Checker` never looked at tenant and only `staffing` checked it itself, so a Tenant Admin of tenant B could read and change tenant A's guests (including decrypted email/phone), ticket types, steps and scans by knowing an event id.
- `end_date` must not be before `start_date`, both on create and on the resulting record after a partial update.
- `is_multi_day` is **derived**, not caller-supplied: `true` when `start_date` and `end_date` fall on different calendar days (each evaluated in its own timestamp's location), recomputed whenever either date changes.
- `rsvp_required` defaults to `true` when omitted on create and can be toggled via a partial update; it decides whether [guests](#guests) issue a ticket only on RSVP confirm (`true`, the default) or immediately at invitation time (`false`) — see [guests](#guests) for the consuming behavior. **Turning it off does not issue tickets retroactively**: a guest invited while it was on who confirms afterwards gets no ticket until someone issues one via [guests' manual ticket route](#guests) (a deliberate product decision, B16 R2 — a UI should say so next to the switch).
- **Writes require the `manage_events` permission** (`authz.RequirePermission`, the same code [categories](#event-categories) gates on, declared locally in `event_constants.go` since permission codes are feature-owned): `POST`/`PUT`/`DELETE /events[/{id}]` and `PUT /events/{event_id}/setup`. Reads stay open to any authenticated caller — the tenant scope above is what protects them.
- **Create runs inside one transaction** (`(*sqlkit.DB).WithTransaction`, the same mechanism [users](#users)' `TransferMaster` uses): the event insert, the workflow-step-template copy, and the ticket-type-template copy (below) all run against the same transaction-carrying context, so a failure in any of the three rolls back the whole event creation instead of committing a partially-seeded event.
- **Create records the category's template version and seeds default workflow steps from it** (B15): first the category's `template_version` is read `FOR SHARE` inside the transaction (a concurrent template save waits until the event commits, so the version recorded as `events.category_template_version` is exactly the rows copied; a missing or deleted category is 422). Then `workflow_step_templates` for `category_id` at that version (ordered by `order_index`) are copied into `workflow_steps` for the new event — the customizable starting point a UI shows immediately after event creation. A category with no step templates copies nothing. A lookup or per-template copy failure **aborts event creation** (rolled back). Templates themselves are edited through [Event categories](#event-categories). The caller then edits the resulting steps through the **setup save** below.
- **Create seeds default ticket types from the same template version**: right after the step copy, `EventService` calls `SeedFromCategoryTemplates(event_id, category_id, version, stepIDs)` on the `TicketTypeSeeder` interface `event` declares and `internal/app` wires to `tickets/tickettype` (`events` never imports `tickets`). `stepIDs` maps each step template to the event step just copied from it, so every seeded ticket type includes **exactly** the event steps its template included. A category with no ticket-type templates leaves the event with zero ticket types. A seed failure aborts event creation (rolled back). Later template saves never change an already-created event.
- **The setup save (`PUT /events/{event_id}/setup`) is the only way to change an event's steps or ticket types.** It replaces the whole setup in one request and one transaction; if any part fails, nothing changes. The per-item write routes on `/workflow-steps` and `/ticket-types` were removed with it (B16 R1) — two write paths would drift.
- **The setup save reconciles rather than versions.** Unlike a category's template set, guests, tickets and scan logs point at step and ticket type ids, so an item sent **with** its `id` is updated in place and keeps that id (everything referencing it is unaffected); an item **without** one is created; a live item left out is deleted. The array position of each entry in `workflow_steps` is its `order_index`; each ticket type's `steps` are indexes into that same array.
- **The setup save refuses to remove a ticket type guests still hold** (400, nothing changed): the error names the type and how many guests hold it under `meta.fields["ticket_types"]`. The count comes from the `GuestStats` interface `event` declares and `internal/app` wires to `guests`, and is only queried when the payload actually removes something.
- **The setup save locks the event row** (`SELECT tenant_id ... FOR UPDATE`, via `EventTenantRepository.TenantIDForEventUpdate`) before validating, so two concurrent saves on one event run one after the other instead of interleaving their step reorders. There is **no version compare**: the second save wins (see [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md) B16's non-goals).
- **Setup validation** runs in the service, keyed by JSON path under `meta.fields` — the same shape and rules as a category's template set save, plus the id-ownership check: ticket type names unique case-insensitively (`ticket_types[i].name`), every `steps` index in range with no duplicates (`ticket_types[i].steps[j]`), and every `id` sent belonging to this event (`workflow_steps[i].id`, `ticket_types[i].id`). An empty payload is valid — it clears the setup.

> Field-presence/format checks (`required`, array `max=100`) are enforced at the HTTP boundary via `validate:"..."` tags on `CreateEventInput`/`UpdateEventInput`/`SetupInput` (see [PATTERNS.md](PATTERNS.md#request-validation-boundary)); the date-ordering rule, `is_multi_day` derivation, the tenant scope and every setup rule above stay in the service as business invariants.

#### Endpoints

Base path `/api/v1/events`. Reads need a valid access token; writes also need the `manage_events` permission:

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List the caller's tenant's events (paginated, filtered, sorted), each with `guest_count`/`confirmed_count` | 200 | 400 invalid query · 401 no tenant claim |
| `GET` | `/{id}` | Get one by UUID, scoped to the tenant | 200 | 400 bad UUID · 401 · 404 not found / another tenant |
| `POST` | `/` | Create under the caller's tenant | 201 | 400 invalid body/date order · 401 · 403 · 409 · 422 |
| `PUT` | `/{id}` | Partial update | 200 | 400 · 401 · 403 · 404 |
| `DELETE` | `/{id}` | Soft delete | 204 | 400 · 401 · 403 · 404 |

Base path `/api/v1/events/{event_id}/setup` (behind `authz.RequireEventInTenant`):

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | The event's ordered workflow steps and its ticket types with their step access | 200 | 400 bad UUID · 401 · 404 not found / another tenant |
| `PUT` | `/` | **Replace** the whole setup in one transaction | 200 | 400 validation/`id` not on this event/held ticket type removed · 401 · 403 · 404 · 409 name or order clash with a removed item |

**List query:** `?page=1&size=20&sort=start_date,ASC&category_id=...&name=Gala&start_date=2026-09-22T10:00:00Z;gt`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, category_id, name, start_date, end_date, is_multi_day, created_at, updated_at`); unknown field → 400.
- Filters: `category_id`, `name` (exact match), `start_date`, `end_date`. The date fields accept a range operator suffix — `;gt`, `;gte`, `;lt`, `;lte` (see [PATTERNS.md](PATTERNS.md#list-query--allow-list-parsing-and-enforcement) for the general `value;operator` syntax) — so a UI's status tabs are each one request: **Upcoming** `start_date=<now>;gt`, **Live** `start_date=<now>;lte&end_date=<now>;gte`, **Ended** `end_date=<now>;lt`. One filter per field, so there is no "between" on a single field.
- `tenant_id` is **not** a filter or sort field: the scope is forced from the JWT.
- **Implementation note:** a hand-built raw query string must percent-encode a literal `;` (`%3B`) in a filter value — any real HTTP client library does this automatically.

**Setup body and response:**
```json
{
  "workflow_steps": [
    { "id": "11111111-...", "name": "Check-in", "allows_multiple": false },
    { "name": "Dinner", "allows_multiple": true }
  ],
  "ticket_types": [
    { "id": "22222222-...", "name": "VIP", "rules": {}, "steps": [0, 1] },
    { "name": "Crew", "steps": [1] }
  ]
}
```
Array order of `workflow_steps` is step order; `steps` are indexes into it (ascending in the response). Omitting an existing item deletes it; both arrays are capped at 100 entries, matching the limit the setup read pages with.

#### States & lifecycle

- **Create** — service generates the `id` (UUID), takes `tenant_id` from the JWT and derives `is_multi_day`; `created_at`/`updated_at` are stamped by the audit repository decorator.
- **Update** — partial; `is_multi_day` is re-derived whenever a date changes; `tenant_id` cannot change; `updated_at` re-stamped by the decorator.
- **Delete** — **soft**: `deleted_at` is set; the row remains. All reads/lists automatically exclude soft-deleted rows (`deleted_at IS NULL`, injected by the decorator).
- **Get setup** — one step list (ordered by `order_index`) plus `TicketTypeStore.ListForEvent`; each ticket type's step ids are mapped back to ascending indexes into the step array.
- **Replace setup** — one `WithTransaction`: lock the event row and check its tenant → read the current steps and ticket types → validate → guard held ticket types → reconcile steps via `workflowstep.Service.Sync` (which already survives a reorder without tripping the `(event_id, order_index)` unique constraint) → `TicketTypeStore.ReplaceForEvent` with the step indexes resolved to the ids `Sync` just saved → build the response from what was written. Steps are written before ticket types because the ticket type drafts reference the saved step ids; both run on the transaction-carrying context, so a ticket-type failure rolls the step changes back (covered by `internal/app/event_setup_integration_test.go`).
- **Guest counts** — `List` collects the page's event ids and calls `GuestStats.CountByEvents` **once** (one grouped query over `guests`, covered by `idx_guests_event_rsvp`), not one query per event. An event with no guests reads as zeros. `GET /{id}` does not report counts.
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrAlreadyExists`→409, `ErrInvalidEntity`→422); another tenant's event is also 404; a missing `manage_events` permission on a write →403; unexpected errors become 500 and are logged with context.

### Workflow steps

#### Intent

Manages an event's **workflow steps** — the lifecycle stages a ticket moves through at an event (e.g. Check-in, Photo booth). Always scoped to a parent event via the URL; there is no top-level workflow-step listing. New events start with steps copied from their category's `workflow_step_templates` (see [Events](#events)); from there they are edited **only** through the event's [setup save](#events), which reconciles the whole list — add, rename, reorder, remove — in one transaction.

Since B16 this slice is **read-only over HTTP**. Its write path is `Service.Sync`, called by `events/event`'s `ReplaceSetup` (a sibling package of the same feature slice), and the per-step `POST`/`PUT`/`DELETE` routes, handlers and service methods are gone with the `PUT /` sync route. The reads stay because the guest ticket-type picker and the scan step picker need the step list, and ushers need it without `manage_events`.

#### Invariants

- `(event_id, order_index)` is unique — DB-enforced. The setup save assigns `order_index` from array position, so a payload can't collide with itself.
- `event_id` is **immutable** and always taken from the URL, never the body.
- Every read is scoped to the `{event_id}` in the URL: a step that exists but belongs to a different event resolves as `errorz.NotFound` (404), not a cross-event leak. A step of **another tenant's** event is 404 before the handler runs, via `authz.RequireEventInTenant` (see [Events](#events)).
- **Sync** treats its argument as the *entire* desired list for the event: entries with an `id` are updated, entries without one are created, and any existing step **not** present is deleted. Two entries sharing an `order_index` are rejected up front (400) rather than left to race against the DB constraint. An `id` that doesn't belong to this event is rejected (404 from `Sync`; the setup save catches it first as a 400 keyed by JSON path).

#### Endpoints

Base path `/api/v1/events/{event_id}/workflow-steps`, all behind `authz.RequireEventInTenant`:

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List for the event (paginated, filtered, sorted) | 200 | 400 invalid event id/query · 401 · 404 another tenant's event |
| `GET` | `/{id}` | Get one by UUID, scoped to the event | 200 | 400 bad UUID · 401 · 404 not found |

Writes go through [`PUT /api/v1/events/{event_id}/setup`](#events).

**List query:** `?page=1&size=20&sort=order_index,ASC&name=Check-in`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, name, order_index, allows_multiple, created_at, updated_at`); unknown field → 400.
- Filters: `name` (exact match); `event_id` is always forced from the URL and is not a query filter.

#### States & lifecycle

- **Sync** — reconciles in three passes: (1) delete every existing step not referenced by the payload, (2) move every referenced step to a unique negative `order_index` and apply its new `name`/`allows_multiple`, (3) create new entries and move the referenced steps to their final `order_index`. The two-phase reorder (negative, then final) exists so that swapping two steps' `order_index` — the common drag-reorder case — never trips the `(event_id, order_index)` unique constraint on an intermediate state. Since B16 its only caller is the setup save, so it always runs **inside that transaction**: a later failure rolls every pass back.
- **Delete** — **soft**: `deleted_at` is set; the row remains, so scan history pointing at the step survives. All reads/lists exclude soft-deleted rows (`deleted_at IS NULL`, injected by the decorator). A soft-deleted row still holds its `order_index`, so it counts against the unique constraint; the setup save's negative-then-final reorder keeps that out of the way for the ordinary cases, and a fresh step landing on a removed step's exact index surfaces as 409.
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrAlreadyExists`→409, `ErrInvalidEntity`→422); a step belonging to a different event is also reported as 404; unexpected errors become 500 and are logged with context.

### Workflow step templates

`events/workflowsteptemplate` is **model + repository only** since B15: one row per default workflow step of a category, per template version (`version`, `order_index` unique within it). There is no HTTP surface of its own — the rows are written and read as part of the category's template set ([Event categories](#event-categories)) and copied onto new events by [Events](#events)' `Create`. A save soft-deletes the previous version's rows (history); nothing reads old versions yet.

---

## templates

Source: `internal/features/templates`. Table: `message_templates` (see [DATABASE.md](DATABASE.md)).

### Intent

Manages **message templates** — reusable email/WhatsApp message bodies (invitation, ticket delivery, thank you) at three scopes: **app** (global default), **tenant** (override), and **event** (instance-level override). Resolving which template applies to a given event prefers event, then tenant, then app (see [REQUIREMENT.md](REQUIREMENT.md) ss2.2) — that resolution is left to the caller/future sender feature; this slice only manages the records themselves.

### Invariants

- `source` is one of `"app"`, `"tenant"`, or `"event"`; `channel` is one of `"email"` or `"whatsapp"`.
- When `source == "app"`, `tenant_id` and `event_id` **must both be null**.
- When `source == "tenant"`, `tenant_id` **is required** and `event_id` **must be null**.
- When `source == "event"`, `tenant_id` **and** `event_id` **are both required**.
- When `channel == "email"`, `subject` **is required** (non-empty).
- When `channel == "whatsapp"`, `subject` **must be empty** — WhatsApp messages have no subject line.
- `name` and `body` are required. `variables` is an opaque, optional JSON document (a list of placeholder names for UI/validation use).
- On update (partial), only provided fields change; the source/tenant/event and channel/subject rules re-apply to the resulting record.
- Uniqueness per scope is DB-enforced via partial unique indexes: app — `(name, channel)`; tenant — `(tenant_id, name, channel)`; event — `(event_id, name, channel)`.

> Field-presence/format checks (`required`, `oneof`) are enforced at the HTTP boundary via `validate:"..."` tags on `CreateInput`/`UpdateInput` (see [PATTERNS.md](PATTERNS.md#request-validation-boundary)); the cross-field source/tenant/event and channel/subject rules stay in the service as business invariants.

### Endpoints

Base path `/api/v1/message-templates`:

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List (paginated, filtered, sorted) | 200 | 400 invalid query |
| `GET` | `/{id}` | Get one by UUID | 200 | 400 bad UUID · 404 not found |
| `POST` | `/` | Create | 201 | 400 invalid body/scope/channel · 409 conflict · 422 invalid entity |
| `PUT` | `/{id}` | Partial update | 200 | 400 · 404 not found · 409 conflict |
| `DELETE` | `/{id}` | Soft delete | 204 | 400 · 404 not found |

**List query:** `?page=1&size=20&sort=name,ASC&name=Invitation&source=tenant&channel=email`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, source, tenant_id, event_id, name, channel, created_at, updated_at`); unknown field → 400.
- Filters: `name`, `source`, `tenant_id`, `event_id`, `channel` (exact match); unknown keys ignored.

### States & lifecycle

- **Create** — service generates the `id` (UUID); `created_at`/`updated_at` are stamped by the audit repository decorator.
- **Update** — partial; the resulting record is re-validated against the same scope/channel invariants; `updated_at` re-stamped by the decorator.
- **Delete** — **soft**: `deleted_at` is set; the row remains. All reads/lists automatically exclude soft-deleted rows (`deleted_at IS NULL`, injected by the decorator).
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrAlreadyExists`→409, `ErrInvalidEntity`→422); unexpected errors become 500 and are logged with context.

---

## tenants

Source: `internal/features/tenants`. Table: `tenants` (see [DATABASE.md](DATABASE.md)).

### Intent

Manages **tenants** — the customer organizations that own staff users, events, categories, and templates. Almost every other table carries a `tenant_id`; this is the foundational, top-of-hierarchy slice everything else scopes to.

### Invariants

- `name` is required and non-empty.
- `type` is free-form and optional (e.g. `default`, `enterprise`); no enum is enforced.
- `settings` and `branding` are opaque JSON documents; when omitted on create, each defaults to an empty object (`{}`), matching the column default.
- On update (partial), only provided fields change; omitted `settings`/`branding` leave the existing value untouched (they are not reset to `{}`).

> Field-presence/format checks (`required`) are enforced at the HTTP boundary via `validate:"..."` tags on `CreateInput`/`UpdateInput` (see [PATTERNS.md](PATTERNS.md#request-validation-boundary)).

### Endpoints

Base path `/api/v1/tenants`:

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List (paginated, filtered, sorted) | 200 | 400 invalid query |
| `GET` | `/{id}` | Get one by UUID | 200 | 400 bad UUID · 404 not found |
| `POST` | `/` | Create | 201 | 400 invalid body · 409 conflict · 422 invalid entity |
| `PUT` | `/{id}` | Partial update | 200 | 400 · 404 not found |
| `DELETE` | `/{id}` | Soft delete | 204 | 400 · 404 not found |

**List query:** `?page=1&size=20&sort=name,ASC&sort=id,DESC&name=Acme&type=enterprise`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, name, type, created_at, updated_at`); unknown field → 400.
- Filters: `name`, `type` (exact match); unknown keys ignored.

### States & lifecycle

- **Create** — service generates the `id` (UUID); `settings`/`branding` default to `{}` when omitted; `created_at`/`updated_at` are stamped by the audit repository decorator.
- **Update** — partial; `updated_at` re-stamped by the decorator.
- **Delete** — **soft**: `deleted_at` is set; the row remains. All reads/lists automatically exclude soft-deleted rows (`deleted_at IS NULL`, injected by the decorator).
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrAlreadyExists`→409, `ErrInvalidEntity`→422); unexpected errors become 500 and are logged with context.

---

## users

Source: `internal/features/users`. Table: `users` (see [DATABASE.md](DATABASE.md)).

### Intent

Manages **users** — the members of a tenant who log in and act on its behalf (staff, admins). Each user belongs to exactly one tenant and one role; roles determine permissions (see [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md) B3/B6). At most one user per tenant is the **tenant master** — the default user who can create other users for that tenant, and who can transfer that status to another active user in the same tenant (B11).

### Invariants

- **Every `/api/v1/users` endpoint requires the `manage_users` permission** (STAFFING_RBAC.md §3), checked by `authz.RequirePermission` middleware — a caller without it gets 403, no/invalid token gets 401. The one exception is `POST /me/password` (see below), a self-service route registered outside that permission group — a valid access token is its only requirement.
- **Tenant scoping comes from the JWT, not the request body** (B11, mirroring [`staffing`](#staffing)) — every endpoint resolves the caller's tenant via `authz.TenantIDFromContext`; `CreateInput` carries no `tenant_id` field, and `GetByID`/`Update`/`Delete`/`List` treat a target belonging to a different tenant as `errorz.NotFound` (404), never `errorz.Forbidden` — the same not-found-not-forbidden convention as `staffing`/`tickets`, so a caller can't distinguish "doesn't exist" from "belongs to someone else." `tenant_id` is no longer a caller-supplied list filter/sort field.
- `email`, `password`, and `role_id` are required on create; `email` must be a valid address.
- `tenant_id` is **immutable** after creation — a user cannot move tenants; it is not part of `UpdateInput` and is not a request field on `CreateInput` either (resolved from the JWT instead).
- `role_id` must reference a **system-scope** role (`roles.scope = 'system'` — see [roles](#roles) and [docs/STAFFING_RBAC.md](STAFFING_RBAC.md) §3): the service loads the role by ID on create and on any update that changes `role_id`, returning `errorz.NotFound` (404) if the role doesn't exist and `errorz.BadRequest` (400) if it exists but is `event`-scoped. An event-scoped role (e.g. Usher) can never become a user's tenant-wide role.
- `password` is never stored or returned in the clear: the service hashes it with bcrypt before persisting (`password_hash`), and the model tags `PasswordHash` `json:"-"` so it is never serialized in any response. `UpdateInput` has no `password` field — password changes go only through `POST /{id}/password` (admin) or `POST /me/password` (self-service), never both a `PUT` and a dedicated endpoint for the same field.
- `email` is unique **across all tenants** (migration 000012 — see [DATABASE.md](DATABASE.md#35-users)), so [B3 `auth`](#auth) can look a user up at login by email alone. At most one user per tenant may have `is_tenant_master = true`, also DB-enforced. Exactly one user in the whole system may hold the Super Admin role — DB-enforced by a partial unique index on `users.role_id` (migration 000014; see [DATABASE.md](DATABASE.md)).
- **`must_change_password`** (migration 000018, B11) starts `true` on every newly created user — the system surfaces this on login (`auth.TokenPair.must_change_password`, see [auth](#auth)) but never enforces it (no lockout). A successful self-service `POST /me/password` clears it to `false`. A successful admin-only `POST /{id}/password` sets it back to `true` — even if it was already `false` — since the target didn't choose the new password themselves (B11 amendment, REQUIREMENT.md §3.2/§4.8 AC4/AC5).
- On update (partial), only provided fields change; a provided `role_id` is re-validated as system-scope; `tenant_id`/`password`/`must_change_password` are not settable via `PUT`.

> Field-presence/format checks (`required`, `email`, `min`) are enforced at the HTTP boundary via `validate:"..."` tags on `CreateInput`/`UpdateInput`/`SetPasswordInput` (see [PATTERNS.md](PATTERNS.md#request-validation-boundary)).

### Endpoints

Base path `/api/v1/users`. Every route requires a valid access token **and** the `manage_users` permission, except `POST /me/password` (self-service — see below):

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List (paginated, filtered, sorted), scoped to the caller's tenant | 200 | 400 invalid query · 401 · 403 |
| `GET` | `/{id}` | Get one by UUID, scoped to the caller's tenant | 200 | 400 bad UUID · 401 · 403 · 404 not found |
| `POST` | `/` | Create, in the caller's own tenant | 201 | 400 invalid body · 401 · 403 · 409 conflict · 422 invalid entity |
| `PUT` | `/{id}` | Partial update (no password field), scoped to the caller's tenant | 200 | 400 · 401 · 403 · 404 not found · 409 conflict |
| `DELETE` | `/{id}` | Soft delete, scoped to the caller's tenant | 204 | 400 · 401 · 403 · 404 not found |
| `POST` | `/{id}/password` | **Admin-only.** Set a new password for `{id}`, forcing `must_change_password` back to `true` | 200 | 400 · 401 · 403 (missing `manage_users`) · 404 |
| `POST` | `/me/password` | **Self-service-only.** Set a new password for the caller's own account, clearing `must_change_password` to `false` | 200 | 400 · 401 |
| `POST` | `/{id}/transfer-master` | Transfer the caller's `is_tenant_master` to `{id}` | 200 | 400 · 401 · 403 (missing permission, or caller isn't the current master) · 404 |

**`POST /{id}/password` vs. `POST /me/password`** (B11 amendment — see [PATTERNS.md's `/me` sub-resource pattern](PATTERNS.md#self-service-vs-admin-routes-the-me-sub-resource-pattern)): these are two separate routes, handlers, and service methods, not one route with a permission branch. `POST /{id}/password` lives inside the same `manage_users` route group as `List`/`GetByID`/`Create`/`Update`/`Delete`/`TransferMaster` — `UserService.SetPassword` has no in-service permission check, since the route middleware is the sole authorization gate. `POST /me/password` is registered outside that group; `UserService.SetOwnPassword` resolves the target id from `authz.UserIDFromContext` (the JWT subject) — never a path param or request field — so the caller can only ever act on themselves and needs only a valid access token. Both methods share the actual hash-and-update logic via an unexported `setPassword` helper, but each passes the `must_change_password` value appropriate to its route: `SetPassword` (admin reset) passes `true`, `SetOwnPassword` (self-service) passes `false` — a further B11 amendment splitting what was originally a single unconditional `false` (REQUIREMENT.md §3.2/§4.8 AC4/AC5). Either way the target id still resolves through the same tenant-scoped lookup, so a cross-tenant target is `errorz.NotFound` (404).

**`POST /{id}/transfer-master`** is gated like the rest of the group (`manage_users`) **plus** a service-level check that the caller is currently `is_tenant_master` for their own tenant (`errorz.Forbidden` otherwise — REQUIREMENT.md §4.8 frames this as self-referential, "transfer *their* status," not delegable to any `manage_users` holder). `{id}` is the target (new master), resolved through the same tenant-scoped lookup (cross-tenant target → 404). Both the caller's and target's row loads/updates run inside a single `(*sqlkit.DB).WithTransaction` — write order is clear-caller-then-set-target, forced by the existing partial unique index on `(tenant_id) WHERE is_tenant_master = true` — so a failure between the two `Update` calls rolls back instead of leaving a tenant with zero masters.

**List query:** `?page=1&size=20&sort=email,ASC&sort=id,DESC&email=...`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, email, role_id, is_tenant_master, created_at, updated_at`); unknown field → 400. `tenant_id` is not sortable/filterable — every list is already scoped to the caller's own tenant.
- Filters: `email`, `role_id`, `is_tenant_master` (exact match); unknown keys ignored.

### States & lifecycle

- **Create** — service resolves `tenant_id` from the JWT claim, loads and validates `role_id`'s scope (404/400 short-circuit before any write), then generates the `id` (UUID), hashes the plaintext `password` with bcrypt, and always sets `must_change_password = true`; `created_at`/`updated_at` are stamped by the audit repository decorator.
- **Update** — partial; a provided `role_id` is re-validated as system-scope, `tenant_id` cannot change; `updated_at` re-stamped by the decorator. Password is not settable here.
- **SetPassword / SetOwnPassword** — both hash the new password and persist via the same `repo.Update` path (shared unexported `setPassword` helper), differing in how the target id is resolved (path param + `manage_users`, vs. JWT subject only) **and** in the resulting `must_change_password`: `SetPassword` (admin reset) forces it `true`, `SetOwnPassword` (self-service) clears it `false`.
- **TransferMaster** — inside one transaction: loads the caller (403 if not currently master), loads the target (tenant-scoped), clears the caller's `is_tenant_master`, then sets the target's.
- **Delete** — **soft**: `deleted_at` is set; the row remains. All reads/lists automatically exclude soft-deleted rows (`deleted_at IS NULL`, injected by the decorator).
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrAlreadyExists`→409, `ErrInvalidEntity`→422); a missing `role_id`→404, a `role_id` that isn't system-scope→400; a cross-tenant target→404; a missing/insufficient permission→401/403; unexpected errors (including a bcrypt hashing failure) become 500 and are logged with context.

---

## auth

Source: `internal/features/auth`. No table of its own — reads `users` rows directly off the shared `users`
repository (see [ARCHITECTURE.md](ARCHITECTURE.md#feature-slice--future-service-boundary)).

### Intent

Verifies a user's email/password and issues the tokens every other protected endpoint requires: a
stateless access/refresh JWT pair, built on `go-sdk`'s `auth` package (HS256 today; RS256/JWKS/remote available
via config with no code change). This is also the seam that plugs `httpkit/middleware.Auth` into the app-wide
middleware chain (`cmd/api/main.go`), so from B3 onward every route is protected unless explicitly marked public
in the route policy (`configs/config.yaml` `auth.token.rules`).

### Invariants

- Login always returns the same generic `401 invalid email or password` for both an unknown email and a
  wrong password — a caller cannot enumerate valid emails from the error alone.
- Every issued token carries a `type` claim (`access` or `refresh`), a `tenant_id` claim, and (since B6) a
  `role_id` claim — the user's current system-level role, read by
  [`internal/core/authz`](STAFFING_RBAC.md) to authorize permission-gated endpoints (e.g. [staffing](#staffing)).
  The protected-route middleware is wired with a validator wrapped in `AccessOnlyValidator`, which rejects any
  token whose `type` isn't `access` — so a refresh token, despite being signed by the same issuer, cannot be
  used to call a protected endpoint.
- `POST /auth/refresh` validates the refresh token with the *unwrapped* validator, checks `type == "refresh"`
  itself, and re-loads the user by the token's subject so a deleted account cannot refresh past its removal.
  `role_id` on the newly-issued pair is read from that freshly-reloaded user, **never** carried over from the
  presented refresh token's own claims — so a role change (or downgrade) takes effect on the very next refresh
  rather than persisting for the remainder of the old token's TTL (see [docs/STAFFING_RBAC.md](STAFFING_RBAC.md) §6).
- Access/refresh tokens are **stateless** — nothing is persisted server-side, so there is no logout/revocation
  endpoint; a compromised token is only invalidated by its `exp`.
- `email` is unique across all tenants (migration 000012 — see [users](#users) above), so login needs only an
  email, not a tenant identifier.
- `TokenPair.must_change_password` (B11) mirrors the subject user's `must_change_password` flag at the moment
  of issuance, populated on both `Login` and `Refresh` from the `*users.User` each already loads — free reuse
  of data already on hand, not an extra query. It only surfaces the flag; nothing else in `auth` reads or
  enforces it (see [users](#users) for what clears it).

### Endpoints

Base path `/api/v1/auth`:

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `POST` | `/login` | Verify credentials, issue a token pair | 200 | 400 invalid body · 401 invalid email or password |
| `POST` | `/refresh` | Exchange a refresh token for a new pair | 200 | 400 invalid body · 401 invalid/expired/non-refresh token |

Both routes are marked `public: true` in the route policy — otherwise the protected-route middleware would
require a valid access token to reach the very endpoints that issue one.

### States & lifecycle

- **Login** — looks up the user by email (`errorz.NotFound` → generic 401, same as a password mismatch),
  verifies the bcrypt hash, then issues a fresh access token (`auth.token.issuer.default_ttl`, default 15m)
  and refresh token (`auth.refresh_ttl`, default 7d).
- **Refresh** — validates the refresh token, re-loads the user, and issues a brand-new pair (rotation; the
  presented refresh token is not itself invalidated, since nothing is stored server-side).
- **Errors** — token validation failures resolve through `go-sdk`'s `auth` sentinels (all wrap
  `errorz.ErrUnauthorized` → 401); unexpected repository/signing errors become 500 and are logged with context.

---

## roles

Source: `internal/features/roles`. Tables: `roles`, `permissions`, `role_permissions` (see [DATABASE.md](DATABASE.md), [docs/STAFFING_RBAC.md](STAFFING_RBAC.md)).

### Intent

The shared roles/permissions engine every other feature's authorization checks build on: a **role** (`roles`) has a name, description, and a **scope** (`system` or `event`) and is granted a set of **permissions** (`permissions`) via `role_permissions`. [`users`](#users) validates that a user's `role_id` is `system`-scope; [`staffing`](#staffing) validates that an assignment's `role_id` is `event`-scope; [`internal/core/authz`](STAFFING_RBAC.md) resolves a role's permission codes to authorize a request. This phase ships **model + repository only** — no HTTP surface (mirrors `workflow_step_templates` shipping model+repository-only in B4 before handler/routes landed in B5). The starter role/permission catalog (Super Admin, Tenant Admin, Tenant Staff, Usher, Photobooth Staff; `manage_tenants`/`manage_users`/`manage_events`/`manage_staff`/`manage_guests`/`manage_workflows`/`check_in`) is seeded by migration `000014` — see [docs/STAFFING_RBAC.md](STAFFING_RBAC.md) §3 for the full grant table and §7 for what's explicitly out of scope this phase (no admin CRUD endpoints for roles/permissions, no per-tenant custom roles).

**Event-scoped permission fallback (`authz.Checker.RequireForEvent`):** on any route whose path carries `{event_id}`, `authz.RequirePermission` middleware runs `Checker.RequireForEvent` instead of the plain system-role `Require`. It tries the caller's system-level role first (unchanged, and returned immediately on success or on an internal resolver error); only when that fails with a 401/403 does it fall back to looking up the caller's active `event_staff_assignments` row on that `event_id` (via `staffing.NewEventRoleResolver`, the same `event_id`+`user_id`, `Limit:1` lookup `staffing`'s own duplicate-assignment check uses) and re-checks *that* row's `role_id` against the same permission code. No assignment found, or one found whose role also lacks the code, falls through to the original system-check error unchanged. This is how an event-scoped role (e.g. Usher) can satisfy a permission-gated endpoint (e.g. `scans`' `check_in`) without the user holding an elevated system-wide role — see [docs/DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md) item A9. `Checker.Has` and every non-`{event_id}` route (e.g. `users`' `manage_users` group) are unaffected — they keep using the plain system-role-only `Require`.

### Invariants

- `roles.scope` is `system` or `event` (DB `CHECK`, migration 000013) — a role is one or the other, never both.
- **`roles` has no `deleted_at` column** (system/reference data, not user content) — its repository is built with `internal/core/repository.NewRepositoryNoAudit`, not the usual `NewRepository`, since the audit decorator assumes `deleted_at` exists and would break `List`/`Count` and silently no-op `Delete` against this table.
- `role_permissions` has a composite primary key `(role_id, permission_id)` and is a read-only join for this phase — modeled as a small purpose-built `RolePermissionRepository` interface (`PermissionCodesByRoleID`), not the generic `repository.Repository[T,TID]` pattern, and given its own hand-rolled Redis read-through cache decorator (`NewCachedRolePermissionRepository`).
- Exactly one user in the system may hold the seeded Super Admin role, enforced by a partial unique index on `users.role_id` (migration 000014) — see [users](#users).

### Endpoints

None this phase — internal-only, consumed by `users`, `staffing`, and `internal/core/authz`. A future phase may add admin CRUD for roles/permissions (see [docs/STAFFING_RBAC.md](STAFFING_RBAC.md) §7).

### States & lifecycle

- Roles/permissions/role_permissions are **seeded once** (migration `000014`) and otherwise static in this phase — no create/update/delete path exists yet.
- `PermissionCodesByRoleID` reads are cached (read-through, JSON-encoded `[]string`, keyed by role ID) when `app.roles.repository.role_permission_cache.enabled` is true; a Redis outage or cache-miss falls through to the DB join, never fails the read.

---

## staffing

Source: `internal/features/staffing`. Table: `event_staff_assignments` (see [DATABASE.md](DATABASE.md), [docs/STAFFING_RBAC.md](STAFFING_RBAC.md)).

### Intent

Assigns tenant users to events with an **event-scoped role** (e.g. Usher, Photobooth Staff) that governs what they can do at that one event, independent of their tenant-wide role (see [docs/STAFFING_RBAC.md](STAFFING_RBAC.md) §1–§2, §5). Every endpoint requires the `manage_staff` permission, normally held by the caller's **system-level role** (Tenant Admin/Super Admin by the seeded catalog — see [roles](#roles)); since this route also carries `{event_id}`, an event-scoped role that happens to grant `manage_staff` would satisfy it too via the same fallback every `{event_id}` route gets (see [roles' event-scoped permission fallback](#roles)) — the seeded catalog just doesn't grant `manage_staff` to any event-scope role today.

### Invariants

- **Tenant scoping comes from the JWT, not the request body** — every endpoint resolves the caller's tenant via `authz.TenantIDFromContext` (the `tenant_id` claim), and `CreateAssignmentInput`/`UpdateAssignmentInput` carry **no `tenant_id` field**. This was a deliberate, narrow divergence when staffing shipped; [`events`](#events) followed it in B16 (its create no longer takes `tenant_id`), and [`users`](#users) remains the last slice that takes one explicitly — a known, separately-tracked gap. Permission enforcement is meaningless if a caller can simply name a different tenant's `event_id`/`user_id` in the body while the permission check passes on their own role.
- Every request is authorized against the `manage_staff` permission, checked by `authz.RequirePermission` middleware before the handler runs — the caller's **system-level role** first, falling back to an event-scoped assignment on this route's `{event_id}` (see [roles' event-scoped permission fallback](#roles)) — see [docs/STAFFING_RBAC.md](STAFFING_RBAC.md) §6.
- Another tenant's event is 404 before the handler runs, via `authz.RequireEventInTenant` (`r.Use`'d ahead of the permission check — see [Events](#events)). The service's own `loadTenantScopedEvent` keeps checking the tenant as well: it also returns the event, and the service must stay correct for any transport. The middleware is what makes the rule uniform across every `{event_id}` route rather than something each feature could forget.
- `event_id` is always taken from the URL, and every operation first loads the event and verifies `event.tenant_id` matches the caller's tenant claim. A mismatch (or a nonexistent event) resolves as `errorz.NotFound` (404), **never** `errorz.Forbidden` — the same not-found-not-forbidden convention as `WorkflowStepService`, so a caller can't distinguish "doesn't exist" from "belongs to someone else."
- **Create** validates, in order: the event belongs to the caller's tenant (404 otherwise); the target user exists and belongs to the *same* tenant as the event (404 otherwise — same not-found convention); the role exists (404) and is **event-scope** (`errorz.BadRequest`/400 if it's `system`-scope instead); and no other **active** assignment already exists for this `(event_id, user_id)` pair (`errorz.Conflict`/409).
- A user can hold **at most one active assignment per event** at a time — enforced by a partial unique index `(event_id, user_id) WHERE deleted_at IS NULL` (migration 000015, replacing the original plain `UNIQUE (event_id, user_id)` from migration 000007, which would have permanently blocked re-assignment after a removal). **Removing and later re-assigning the same user to the same event is allowed** and creates a brand-new row — never a reactivation of the deleted one — so an event's staffing history stays an accurate log of who was ever staffed (see [docs/STAFFING_RBAC.md](STAFFING_RBAC.md) §5).
- **`event_id` and `user_id` are immutable** after creation — `UpdateAssignmentInput` carries only `role_id`. Re-assigning a different user (or moving an assignment to a different event) is a `DELETE` + `POST`, not a `PUT`. A provided `role_id` on update is re-validated as event-scope.
- **List is always scoped to one event** — `event_id` is server-injected into the query regardless of caller-supplied filters; there is no cross-event staff listing in this phase.

### Endpoints

Base path `/api/v1/events/{event_id}/staff`. Every route requires a valid access token **and** the `manage_staff` permission (`authz.RequirePermission`):

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List staff for the event (paginated, filtered, sorted) | 200 | 400 invalid event id/query · 401 · 403 · 404 event not found |
| `GET` | `/{id}` | Get one assignment by UUID, scoped to the event | 200 | 400 bad UUID · 401 · 403 · 404 not found |
| `POST` | `/` | Assign a user to the event with an event-scope role | 201 | 400 invalid body/role not event-scope · 401 · 403 · 404 event/user/role not found · 409 already assigned · 422 invalid entity |
| `PUT` | `/{id}` | Change an assignment's role | 200 | 400 invalid body/role not event-scope · 401 · 403 · 404 not found |
| `DELETE` | `/{id}` | Remove a user from the event (soft delete) | 204 | 400 · 401 · 403 · 404 not found |

**List query:** `?page=1&size=20&sort=created_at,ASC&user_id=...&role_id=...`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, user_id, role_id, created_at, updated_at`); unknown field → 400.
- Filters: `user_id`, `role_id` (exact match); `event_id` is always forced from the URL and is not a query filter.

### States & lifecycle

- **Create** — service generates the `id` (UUID) and sets `event_id`/`user_id`/`role_id` from the URL/validated input; `created_at`/`updated_at` are stamped by the audit repository decorator. The active-duplicate check is a `List`-based pre-check (necessary, not optional — `go-sdk`'s Postgres error mapping does not yet translate a `23505 unique_violation` into `repository.ErrAlreadyExists`, see `go-sdk/lib/repository/sql/helpers.go`); a residual `errors.Is(err, repository.ErrAlreadyExists)` translation on the `Create` call itself is kept as a race-condition fallback.
- **Update** — only `role_id` may change, re-validated as event-scope; `updated_at` re-stamped by the decorator.
- **Delete** — **soft**: `deleted_at` is set; the row remains as staffing history. All reads/lists automatically exclude soft-deleted rows (`deleted_at IS NULL`, injected by the decorator) — including the active-duplicate check, so removing and re-adding the same user to the same event is allowed (see Invariants).
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrAlreadyExists`→409, `ErrInvalidEntity`→422); a missing `tenant_id`/`role_id` JWT claim →401; a missing `manage_staff` permission →403; an assignment/event resolved as belonging to a different tenant or event →404; unexpected errors become 500 and are logged with context.

---

## tickets

Source: `internal/features/tickets`. Tables: `ticket_types`, `ticket_type_workflow_steps`, `ticket_type_templates`, `ticket_type_template_workflow_steps` (see [DATABASE.md](DATABASE.md)). `config.go`/`tickets_constants.go` (holding `PermissionManageEvents`) stay at the feature root; the feature splits into two per-entity subpackages — `tickettype` (ticket types + the workflow-step junction) and `tickettypetemplate` (B12) — per AGENTS.md's multi-entity rule, mirroring `events/category` + `events/workflowsteptemplate`. The `Ticket` QR artifact issued to a guest is a separate feature ([guests](#guests)).

### Ticket types

#### Intent

Manages an event's **ticket types** (e.g. Regular, VIP) and which of that event's workflow steps each type applies to, so guests receive the right ticket and check-in later enforces only the steps that type is entitled to (REQUIREMENT.md §3.8, §4.4). Always scoped to a parent event via the URL; there is no top-level ticket-type listing.

Since B16 this entity is **read-only over HTTP**. Its writes are the event's [setup save](#events) (via `SetupStore`, which implements `events/event`'s `TicketTypeStore`) and the copy-on-create seed from the category's templates; the per-ticket-type `POST`/`PUT`/`DELETE` routes and the `PUT /{id}/workflow-steps` route are gone with them, so one event's ticket types have a single write path.

#### Invariants

- `name` is required; `(event_id, name)` is unique — DB-enforced; a conflicting name surfaces as `errorz.Conflict` (409).
- `event_id` is **immutable** and always taken from the URL, never the body.
- Every read is scoped to the `{event_id}` in the URL: a ticket type that exists but belongs to a different event resolves as `errorz.NotFound` (404), not a cross-event leak — the same convention as [workflow steps](#workflow-steps). A ticket type of **another tenant's** event is 404 before the handler runs, via `authz.RequireEventInTenant` (see [Events](#events)).
- `rules` is an opaque, optional JSON document (default `{}`), passed through unvalidated — same treatment as `ticket_type_templates.rules`.
- **The reads need only a valid access token** (plus the tenant check above) — they lost the `manage_events` gate in B16: a caller who may register guests must be able to see the ticket types to pick from, and an usher needs the list to scan. Writes are gated where they now live, on the event's setup save.
- `workflow_step_ids` (the set of the event's workflow steps this ticket type currently applies to) is populated only on `GetByID` (one extra lookup); it is left empty on `List` to avoid an N+1 join across every row of a list. The [setup read](#events) reports the same information for every type at once, as indexes into its step array.
- **`SetupStore.ReplaceForEvent` reconciles, it does not recreate**: a draft carrying an `id` updates that row in place, so the tickets already issued against it stay valid; a draft without one is created; any live type not in the payload is soft-deleted. Deletes run before the writes, so a name freed by a removal is available to a draft in the same save. The step access of each written type is set through `TicketTypeWorkflowStepRepository.SetWorkflowStepIDs`, which runs on the caller's transaction.

> Field-presence/format checks (`required`) are enforced at the HTTP boundary on the event's `SetupInput` (see [PATTERNS.md](PATTERNS.md#request-validation-boundary)); the event-scope check and the step-index/name/id rules are business invariants enforced in `events/event`'s `ReplaceSetup` since no struct tag or DB constraint can express them.

#### Endpoints

Base path `/api/v1/events/{event_id}/ticket-types`, all behind `authz.RequireEventInTenant` and requiring a valid access token:

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List for the event (paginated, filtered, sorted) | 200 | 400 invalid event id/query · 401 · 404 another tenant's event |
| `GET` | `/{id}` | Get one, scoped to the event — response includes `workflow_step_ids` | 200 | 400 bad UUID · 401 · 404 not found |

Writes go through [`PUT /api/v1/events/{event_id}/setup`](#events).

**List query:** `?page=1&size=20&sort=name,ASC&name=VIP`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, name, created_at, updated_at`); unknown field → 400.
- Filters: `name` (exact match); `event_id` is always forced from the URL and is not a query filter.

#### States & lifecycle

- **SeedFromCategoryTemplates** — implements `events/event`'s `TicketTypeSeeder` (wired in `internal/app`); called by `EventService.Create` inside its transaction with the category template version and the step-template → event-step map. Lists `ticket_type_templates` for the category at that version plus their step links (`ticket_type_template_workflow_steps`, one query), then for each template inserts a matching `TicketType` (same name/rules) and sets its steps to the mapped event step ids. Any failure returns immediately and rolls back the whole event creation. The junction repository runs on the transaction in `ctx` (go-sdk `BaseRepository.GetConnection`); before B15 it always used the leader/follower directly, so these inserts ran outside the event transaction and failed their FKs. There is no FK from `ticket_types` back to the templates; it is a one-time copy.
- **ReplaceForEvent** (the setup save's tickets half) — reads the event's live types once, soft-deletes the ones the payload drops, then per draft either updates the named row or inserts a new one, and sets its step access. `rules` defaults to `{}` on a create that omits it, matching the column default. Runs entirely on the setup save's transaction, so a failure anywhere rolls back the step changes that preceded it too.
- **Delete** — **soft**: `deleted_at` is set; the row remains, so tickets and scan logs pointing at the type survive. All reads/lists automatically exclude soft-deleted rows (`deleted_at IS NULL`, injected by the decorator). Because a soft-deleted row keeps its `name` and `UNIQUE (event_id, name)` counts it, removing a type and adding a *different* type with the same name in one save surfaces as 409 (marked as a deliberate ceiling in `event_setup_store.go`; the fix is a partial unique index).
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrAlreadyExists`→409, `ErrInvalidEntity`→422); a ticket type belonging to a different event is also reported as 404; unexpected errors become 500 and are logged with context.

### Ticket type templates

`tickets/tickettypetemplate` has **no HTTP surface** since B15. It owns the per-category default ticket types (`ticket_type_templates`, one set per template version, `(category_id, version, name)` unique) and which step templates each includes (`ticket_type_template_workflow_steps`, write-once per version). `Store` implements `events/category`'s `TicketTypeTemplateStore`: `ListForCategory(category, version)` and `ReplaceForCategory(category, version, drafts)` (soft-delete the live rows, insert the drafts with their step links), both running in the category save's transaction. The rows are edited only through [Event categories](#event-categories) and copied onto new events by [Ticket types → SeedFromCategoryTemplates](#ticket-types). `rules` is opaque JSON, default `{}`.

---

## guests

Source: `internal/features/guests`. Tables: `guests`, `tickets` (see [DATABASE.md](DATABASE.md)). Flat layout —
`Ticket` ships **model + repository only** (no HTTP surface, mirroring [roles](#roles)); it's created internally
by `GuestService` and surfaced only through `GET .../guests/{id}`.

### Intent

Manages an event's **guest list**: CRUD, assigning a ticket type, sending an invitation, and the public guest-facing
RSVP flow that (depending on the event's `rsvp_required`) issues a QR-coded `Ticket` either on RSVP confirm or
immediately at invitation time (REQUIREMENT.md §3.10, §4.1, §4.2). Always scoped to a parent event via the URL
(except the public RSVP endpoint); there is no top-level guest listing.

### Invariants

- `name` and `email` are required on create; `phone` and `ticket_type_id` are optional. `rsvp_status` starts at
  `none`.
- `event_id` is **immutable** and always taken from the URL, never the body — it is not part of
  `CreateGuestInput`/`UpdateGuestInput`.
- Every read/update/delete/invitation-send/ticket-issue is scoped to the `{event_id}` in the URL: a guest that
  exists but belongs to a different event resolves as `errorz.NotFound` (404), not a cross-event leak — the same
  convention as [ticket-types](#tickets)/[workflow-steps](#workflow-steps).
- **A guest of another tenant's event is 404 before any handler runs**, via `authz.RequireEventInTenant`, which is
  `r.Use`'d ahead of the `manage_guests` check (see [Events](#events)). This matters most here: before B16 a
  Tenant Admin of one tenant could read another tenant's guest list — decrypted `email` and `phone` included —
  by knowing an event id, because their system role satisfied `manage_guests` for *any* `{event_id}`. Ordering is
  load-bearing: the tenant check must answer 404 before the permission check could answer 403.
- A provided `ticket_type_id` (on create or update) must belong to the guest's own event — validated the same way
  `tickets` validates a workflow-step ID (400 otherwise).
- **`email`/`phone` are encrypted at rest; `name` is not.** `name` stays plaintext specifically so it supports
  partial-match search (`?name=Ali;like` — see **List query** below). Encryption sits behind an
  app-local `PIIEncryptor` interface (`internal/features/guests/pii_encryptor.go`), so the scheme can change
  without touching `guest_repository.go`/`guest_service.go`; the concrete implementation
  (`internal/app/pii_encryptor.go`'s `cryptoPIIEncryptor`) wraps go-sdk's `crypto.Encryptor` (Phase 9, shipped) —
  AES-256-GCM for `Encrypt`/`Decrypt`, HMAC-SHA256 for `BlindIndex`. Keys come from `Config.Crypto`
  (`GUEST_PII_ENCRYPTION_KEY`/`GUEST_PII_BLIND_INDEX_KEY`, both standard-base64, no safe default — see
  `.env.example`).
- **Exact-match search on `email`/`phone` via a blind index, not ciphertext comparison** — `email_hash`/
  `phone_hash` columns hold a deterministic digest (`PIIEncryptor.BlindIndex`) of the normalized value
  (lowercased/trimmed for email, digits-only for phone). A list filter on `email`/`phone` is rewritten by the
  service to match against the hash column; a `;like` (or any non-`eq`) operator on either is rejected (400) —
  a hash has no notion of partial match, so honoring one would be dishonest.
- **`POST .../invitation` requires `ticket_type_id` already set** (400 otherwise). It generates a unique,
  unguessable `invitation_token` (returned only in this response — there's no real email/SMS channel yet) and
  sets `rsvp_status` to `invited`. If the event's `rsvp_required` is `false`, it also issues the guest's `Ticket`
  immediately; otherwise the ticket waits for RSVP confirm.
- **`POST .../ticket` issues a guest's `Ticket` manually, whatever their `rsvp_status`** (B16 R2) — the escape
  hatch for a guest the RSVP flow never produced one for: invited while the event's `rsvp_required` was on, then
  confirming after it was turned off (turning the switch off is not retroactive — see [Events](#events)), or never
  invited at all. It is **idempotent**: a guest who already holds a ticket is returned unchanged, with no second
  ticket issued. It requires a `ticket_type_id` (400 otherwise, the same rule `SendInvitation`/`RSVP` enforce) and
  it **never changes `rsvp_status`** — issuing a ticket is not a claim about whether the guest replied.
- **The public RSVP endpoint is looked up by token alone, with no event/auth scoping** — the caller is an
  unauthenticated guest with no session. An unknown token is `errorz.NotFound` (404). Confirming issues the
  guest's `Ticket` if one hasn't already been issued (idempotent — a no-op if `rsvp_required` was `false` and one
  was already issued at invitation time); **declining never touches an already-issued ticket** — a per-product
  decision to keep this simple rather than voiding it.
- **Invitation delivery is a publish, not a real send** — `GuestService.SendInvitation` calls an app-local
  `InvitationPublisher` interface (`internal/features/guests/invitation_publisher.go`) best-effort (a failure is
  logged but doesn't fail the call, same treatment as `EventService.Create`'s template-copy). The concrete
  implementation (`internal/app/invitation_publisher.go`'s `queueInvitationPublisher`) is a thin adapter over
  go-sdk's `queue.Publisher` — JSON-encodes `InvitationMessage` and publishes it to the `guests.invitation` topic,
  keyed by `guest_id`. Backend (`noop`/`logging`/`kafka`) is selected by `Config.Queue`
  (`QUEUE_BACKEND`, default `noop` — see `.env.example`); switching to a real Kafka cluster is a config change,
  not a code change. The consumer that actually sends an email/WhatsApp message from this (using the
  `message_templates` feature's invitation template) still doesn't exist — out of scope for this phase.

> Field-presence/format checks (`required`, `email`) are enforced at the HTTP boundary via `validate:"..."` tags
> on `CreateGuestInput`/`UpdateGuestInput`/`RSVPInput` (see [PATTERNS.md](PATTERNS.md#request-validation-boundary));
> the event/ticket-type scoping, PII encryption, and RSVP/ticket-issuance rules stay in the service as business
> invariants no struct tag or DB constraint can express.

### Endpoints

Base path `/api/v1/events/{event_id}/guests`. Every route sits behind `authz.RequireEventInTenant` and requires a
valid access token **and** the `manage_guests` permission (`authz.RequirePermission`, seeded in B6's catalog) —
since every route carries `{event_id}`, this falls back to an event-scoped assignment on this event when the
caller's system role doesn't grant it (see [roles' event-scoped permission fallback](#roles)):

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List for the event (paginated, filtered, sorted) | 200 | 400 · 401 · 403 |
| `POST` | `/` | Create a guest under the event | 201 | 400 invalid body/ticket type cross-event · 401 · 403 · 422 |
| `GET` | `/{id}` | Get one, scoped to the event — includes the issued ticket/QR summary once one exists | 200 | 400 bad UUID · 401 · 403 · 404 |
| `PUT` | `/{id}` | Partial update (`name`, `email`, `phone`, `ticket_type_id`) | 200 | 400 · 401 · 403 · 404 |
| `DELETE` | `/{id}` | Soft delete | 204 | 400 · 401 · 403 · 404 |
| `POST` | `/{id}/invitation` | Send invitation — requires `ticket_type_id` already set | 200 | 400 no ticket type assigned · 401 · 403 · 404 |
| `POST` | `/{id}/ticket` | Issue the ticket manually, whatever the RSVP status — idempotent | 200 | 400 no ticket type assigned · 401 · 403 · 404 |

Public, unauthenticated (route-policy `public: true`, same mechanism as `auth`'s `/login`/`/refresh`):

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `POST` | `/api/v1/guests/rsvp/{token}` | Guest confirms/declines (`{"status":"confirmed"\|"declined"}`) | 200 | 400 invalid status · 404 unknown token |

**List query:** `?page=1&size=20&sort=name,ASC&name=Ali;like&rsvp_status=invited`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, name, rsvp_status, created_at, updated_at`); unknown field → 400.
- Filters: `name` (exact by default, or partial via a `;like` suffix — see [PATTERNS.md](PATTERNS.md#list-query--allow-list-parsing-and-enforcement) for the general `value;operator` syntax), `email`, `phone` (exact only), `rsvp_status` (exact); `event_id` is always forced from the URL and is not a query filter.
- **Implementation note:** a hand-built raw query string must percent-encode a literal `;` (`%3B`) in a filter value — any real HTTP client library does this automatically.

### States & lifecycle

- **Create** — service generates the `id` (UUID); a provided `ticket_type_id` is validated against the event first; `rsvp_status` starts at `none`; `created_at`/`updated_at` are stamped by the audit repository decorator. `email`/`phone` are encrypted and their blind-index hashes computed by `guest_repository.go` transparently — the service never sees ciphertext.
- **Update** — partial; a provided `ticket_type_id` is re-validated; `event_id` cannot change; `updated_at` re-stamped by the decorator.
- **Delete** — **soft**: `deleted_at` is set; the row remains. All reads/lists automatically exclude soft-deleted rows (`deleted_at IS NULL`, injected by the decorator).
- **Send invitation** — sets `rsvp_status=invited`, generates `invitation_token`, calls `InvitationPublisher` (best-effort); issues the `Ticket` immediately only when the event's `rsvp_required` is `false`.
- **RSVP** — looked up by `invitation_token`; confirming sets `rsvp_status=confirmed` and issues the `Ticket` if one doesn't already exist; declining sets `rsvp_status=declined` and never touches an existing ticket.
- **Issue ticket** — `loadScoped` (with its ticket, if any) → return unchanged when one exists → otherwise the same `issueTicketIfNeeded` `SendInvitation`/`RSVP` use, then one `repo.Update`. `rsvp_status` is left exactly as it was.
- **Guest counts for event screens** — `guests.GuestStats` implements `events/event`'s `GuestStats` with two direct grouped queries over live rows: `CountByEvents` (one call per page of events, `count(*)` plus `count(*) FILTER (WHERE rsvp_status = 'confirmed')` grouped by `event_id`, covered by `idx_guests_event_rsvp`) and `CountByTicketType` (per event, used by the event's setup save to refuse removing a held ticket type). Wired in `internal/app` so `events` never imports `guests`; direct SQL because `repository.Repository` has no aggregate read, which also means each query spells out `deleted_at IS NULL` itself rather than relying on the audit decorator.
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrInvalidEntity`→422); a ticket type belonging to a different event is `errorz.BadRequest` (400); a guest belonging to a different event is `errorz.NotFound` (404); a missing `manage_guests` permission →403; unexpected errors become 500 and are logged with context.

---

## scans

Source: `internal/features/scans`. Table: `scan_logs` (see [DATABASE.md](DATABASE.md)). Flat layout, one entity —
`ScanLog` is a permanent, append-only audit row; `Ticket`, `WorkflowStep`, and `TicketType` are read/written
through already-exported cross-feature repositories (`guests.NewTicketRepository`,
`events/workflowstep.NewWorkflowStepRepository`, `tickets/tickettype.TicketTypeWorkflowStepRepository`), never owned here.

### Intent

Records a guest's ticket being scanned against a specific workflow step during check-in, enforcing that step's
one-time-vs-repeatable rule (`WorkflowStep.AllowsMultiple`) so a single-use step (e.g. souvenir pickup) can't be
claimed twice while a repeatable step (e.g. photo booth) accumulates an accurate, permanent count of every visit
(REQUIREMENT.md §4.3, §4.5). Always scoped to a parent event via the URL; there is no top-level scan listing.

### Invariants

- `ScanLogService` exposes only `RecordScan` and `ListScans` — no `Update`/`Delete`/`GetByID` endpoint. `scan_logs`
  has no `deleted_at` column and is never edited once written; `ScanLogRepository` is built via
  `internal/core/repository.NewRepositoryNoAudit`, the same call [roles](#roles) makes for its own no-`deleted_at`
  table.
- **No cache decorator on `ScanLogRepository`** — a deliberate deviation from every other feature repository in
  this codebase. `RecordScan`'s non-repeatable-step duplicate check calls `Count` on every scan and must see every
  prior write immediately; go-sdk's cache decorator caches `List`/`Count` results by filter key, which could let a
  stale count pass a step that was already completed.
- **The client identifies the ticket by `qr_code`, never a resolved `ticket_id`** — the server resolves `qr_code`
  to a `Ticket` internally, scoped to the `event_id` in the URL. A ticket that exists but belongs to a different
  event is indistinguishable from an unknown QR code — both resolve as `errorz.NotFound` (404), the same
  no-cross-event-leak convention as [ticket-types](#tickets)/[guests](#guests).
- **`RecordScan` validates in a fixed order** (see States & lifecycle) before writing anything: ticket resolution
  → ticket not invalidated → workflow step belongs to this event → ticket type entitled to that step →
  non-repeatable-step duplicate check → insert → one-way ticket status flip. Not wrapped in a database
  transaction — no feature in this codebase uses one yet (same accepted gap as [tickets](#tickets)' workflow-step
  replace); a narrow race between two concurrent scans of the same non-repeatable step could both pass the
  duplicate check before either inserts.
- **Every route requires the `check_in` permission** (`authz.RequirePermission`), not a new `record_scans` code —
  reusing the existing seeded permission (migration `000014`) every relevant role (Usher, Photobooth Staff,
  Tenant Staff, Tenant Admin, Super Admin) already holds. Both recording a scan and reading scan history share the
  same gate; no acceptance criterion asks for a narrower read-only tier.
- **A scan against another tenant's event is 404 before the handler runs**, via `authz.RequireEventInTenant`,
  `r.Use`'d ahead of the permission check (see [Events](#events)). That costs one primary-key read per scan, which
  is accepted for now; caching the event→tenant mapping is deferred until scan latency (PRD-001 G3) shows it
  matters.
- **A caller with only an event-scoped assignment on this event (e.g. Usher, no elevated system-level role) can
  still pass this gate** — `authz.RequirePermission` falls back from the system-role check to that assignment's
  `role_id` on this `{event_id}` (see [roles' event-scoped permission fallback](#roles), [docs/DEVELOPMENT_PLAN.md
  item A9](DEVELOPMENT_PLAN.md)).
- **`operator_user_id` is always left `NULL`** this phase — no acceptance criterion asks for "who scanned"
  reporting, and nothing in this codebase yet resolves a caller's `user_id` from context (only `role_id`/
  `tenant_id`, via `internal/core/authz`).
- `GET /` returns raw, paginated `scan_logs` rows, not an aggregated/counted history — the completion count of a
  repeatable step, or the single completion of a single-use step, is independently verifiable by reading `total`
  on a filtered page.

> Field-presence checks (`required` on `qr_code`/`workflow_step_id`) are enforced at the HTTP boundary via
> `validate:"..."` tags on `RecordScanInput` (see [PATTERNS.md](PATTERNS.md#request-validation-boundary)); every
> other rule above is a business invariant enforced in the service.

### Endpoints

Base path `/api/v1/events/{event_id}/scans`. Every route requires a valid access token **and** the `check_in`
permission (`authz.RequirePermission`):

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `POST` | `/` | Record a scan — body is `qr_code` + `workflow_step_id` | 201 | 400 workflow step not on this event · 400 ticket type not entitled to this step · 401 · 403 · 404 ticket or workflow step not found for this event · 409 ticket invalidated · 409 step already completed |
| `GET` | `/` | Scan history for the event (paginated, filtered, sorted) | 200 | 400 invalid event id/query · 401 · 403 |

**List query:** `?page=1&size=20&sort=scanned_at,DESC&ticket_id=...&workflow_step_id=...`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, scanned_at`); unknown field → 400.
- Filters: `ticket_id`, `workflow_step_id` (both exact match only — no `;like` use case for UUID equality);
  `event_id` is always forced from the URL and is not a query filter.

### States & lifecycle

`RecordScan`'s validation order, all inside the service (transport-agnostic; the handler only decodes/validates
request shape):

1. Resolve `qr_code` to a `Ticket` filtered on both `qr_code` and the URL `event_id` → 404 if none found.
2. `ticket.status == invalidated` → 409.
3. Load the `workflow_steps` row by `workflow_step_id` → 404 if it doesn't exist at all; a step belonging to a
   different event → 400.
4. Confirm `workflow_step_id` is among the ticket's ticket type's entitled steps
   (`TicketTypeWorkflowStepRepository.WorkflowStepIDsByTicketTypeID`) → 400 if absent.
5. If the step's `allows_multiple` is `false`, `ScanLogRepository.Count` filtered on `ticket_id` +
   `workflow_step_id` — a non-zero count → 409 "already completed". Skipped entirely when `allows_multiple` is
   `true`, so every call reaches step 6 and inserts its own row with no cap.
6. Insert the `scan_logs` row (`id`, `event_id`, `ticket_id`, `workflow_step_id`, `scanned_at`;
   `operator_user_id` stays `NULL`).
7. If `ticket.status == active`, update it to `used` — a one-way, first-successful-scan-of-any-step transition
   that runs at most once per ticket. Once `used`, step 6 keeps inserting new `scan_logs` rows on every later call
   (including further repeatable-step scans) without ever touching `status` again. A failure updating `status` is
   logged but does not fail the scan response — the `scan_logs` row from step 6 is already persisted and is the
   record of what actually happened regardless.
- **No ticket-status rollback** — once a ticket flips to `used`, nothing in this pass ever reverts it to `active`.
- **Errors** — repository sentinels are translated to `errorz` codes; a ticket or workflow step not resolvable for
  this event is 404/400 (see Invariants); a missing `check_in` permission → 403; unexpected errors become 500 and
  are logged with context.

---

## Template for new features

Copy this when adding a slice (and add a [feature-map](../AGENTS.md#feature-map) row):

```markdown
## <feature>

Source: `internal/features/<feature>`. Table(s): `<table>`.

### Intent
<one paragraph>

### Invariants
- <rule enforced in the service>

### Endpoints
| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| ... | ... | ... | ... | ... |

### States & lifecycle
<creation, transitions, soft-delete behaviour, error mapping>
```
