# Feature behaviour

How the app behaves, feature by feature — intent, invariants, endpoints, and lifecycle. This satisfies the "document how the app works" goal and is required for every feature (see [../AGENTS.md](../AGENTS.md#documentation)). For the data model, see [DATABASE.md](DATABASE.md); for the roadmap of unbuilt features, see [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md).

Each section follows the same template:

- **Intent** — what the feature is for.
- **Invariants** — rules that always hold (enforced in the service).
- **Endpoints** — HTTP surface (also in Swagger).
- **States & lifecycle** — how records are created, changed, and removed.

---

## events

Source: `internal/features/events`. Tables: `event_categories`, `events`, `workflow_steps`, `workflow_step_templates` (see [DATABASE.md](DATABASE.md)). One vertical slice covering event categories (the taxonomy), events themselves, their per-event workflow steps, and the per-category templates those steps are seeded from.

### Event categories

#### Intent

Manages **event categories** — the taxonomy events are classified under. Categories are either **app-defined** (available to every tenant) or **tenant-defined** (private to one tenant).

#### Invariants

- `source` is one of `"app"` or `"tenant"`.
- When `source == "app"`, `tenant_id` **must be null** (a system category belongs to no tenant).
- When `source == "tenant"`, `tenant_id` **is required**.
- `name` is required and non-empty.
- On update (partial), only provided fields change; the same source/tenant rules re-apply to the resulting record.

> Field-presence/format checks (`required`, `oneof`) are enforced at the HTTP boundary via `validate:"..."` tags on `CreateInput`/`UpdateInput` (see [PATTERNS.md](PATTERNS.md#request-validation-boundary)); the cross-field source/tenant rule above stays in the service as a business invariant.

#### Endpoints

Base path `/api/v1/event-categories`:

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List (paginated, filtered, sorted) | 200 | 400 invalid query |
| `GET` | `/{id}` | Get one by UUID | 200 | 400 bad UUID · 404 not found |
| `POST` | `/` | Create | 201 | 400 invalid body · 409 conflict · 422 invalid entity |
| `PUT` | `/{id}` | Partial update | 200 | 400 · 404 not found |
| `DELETE` | `/{id}` | Soft delete | 204 | 400 · 404 not found |

**List query:** `?page=1&size=20&sort=name,ASC&sort=id,DESC&name=Gala&source=app`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, source, tenant_id, name, created_at, updated_at`); unknown field → 400.
- Filters: `name`, `source`, `tenant_id` (exact match); unknown keys ignored.

#### States & lifecycle

- **Create** — service generates the `id` (UUID); `created_at`/`updated_at` are stamped by the audit repository decorator.
- **Update** — partial; `updated_at` re-stamped by the decorator.
- **Delete** — **soft**: `deleted_at` is set; the row remains. All reads/lists automatically exclude soft-deleted rows (`deleted_at IS NULL`, injected by the decorator).
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrAlreadyExists`→409, `ErrInvalidEntity`→422); unexpected errors become 500 and are logged with context.

### Events

#### Intent

Manages **events** — a tenant's scheduled occasion, classified under an event category, with a start/end date range. Events are the parent of workflow steps, ticket types, guests, and staff assignments (see [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md) B5–B9).

#### Invariants

- `tenant_id`, `category_id`, `name`, `start_date`, and `end_date` are required on create.
- `tenant_id` is **immutable** after creation — an event cannot move tenants; it is not part of `UpdateEventInput`.
- `end_date` must not be before `start_date`, both on create and on the resulting record after a partial update.
- `is_multi_day` is **derived**, not caller-supplied: `true` when `start_date` and `end_date` fall on different calendar days (each evaluated in its own timestamp's location), recomputed whenever either date changes.
- `rsvp_required` defaults to `true` when omitted on create and can be toggled via a partial update; it decides whether [guests](#guests) issue a ticket only on RSVP confirm (`true`, the default) or immediately at invitation time (`false`) — see [guests](#guests) for the consuming behavior.
- **Create seeds default workflow steps from the category's templates**: right after the event row is inserted, `EventService` looks up `workflow_step_templates` for `category_id` (ordered by `order_index`) and copies each one into a `workflow_steps` row for the new event — the customizable starting point a UI shows immediately after event creation. A category with no templates copies nothing. This is **best-effort**: a template lookup or copy failure is logged but does not fail event creation. Templates themselves are managed via [Workflow step templates](#workflow-step-templates). The caller can always add/edit/remove the resulting steps via [Workflow steps → Sync](#workflow-steps).

> Field-presence/format checks (`required`) are enforced at the HTTP boundary via `validate:"..."` tags on `CreateEventInput`/`UpdateEventInput` (see [PATTERNS.md](PATTERNS.md#request-validation-boundary)); the date-ordering rule and `is_multi_day` derivation stay in the service as business invariants.

#### Endpoints

Base path `/api/v1/events`:

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List (paginated, filtered, sorted) | 200 | 400 invalid query |
| `GET` | `/{id}` | Get one by UUID | 200 | 400 bad UUID · 404 not found |
| `POST` | `/` | Create | 201 | 400 invalid body/date order · 409 conflict · 422 invalid entity |
| `PUT` | `/{id}` | Partial update | 200 | 400 · 404 not found |
| `DELETE` | `/{id}` | Soft delete | 204 | 400 · 404 not found |

**List query:** `?page=1&size=20&sort=start_date,ASC&tenant_id=...&category_id=...&name=Gala`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, tenant_id, category_id, name, start_date, end_date, is_multi_day, created_at, updated_at`); unknown field → 400.
- Filters: `tenant_id`, `category_id`, `name` (exact match); unknown keys ignored.

#### States & lifecycle

- **Create** — service generates the `id` (UUID) and derives `is_multi_day`; `created_at`/`updated_at` are stamped by the audit repository decorator.
- **Update** — partial; `is_multi_day` is re-derived whenever a date changes; `tenant_id` cannot change; `updated_at` re-stamped by the decorator.
- **Delete** — **soft**: `deleted_at` is set; the row remains. All reads/lists automatically exclude soft-deleted rows (`deleted_at IS NULL`, injected by the decorator).
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrAlreadyExists`→409, `ErrInvalidEntity`→422); unexpected errors become 500 and are logged with context.

### Workflow steps

#### Intent

Manages an event's **workflow steps** — the lifecycle stages a ticket moves through at an event (e.g. Check-in, Photo booth). Always scoped to a parent event via the URL; there is no top-level workflow-step listing. New events start with steps copied from their category's `workflow_step_templates` (see [Events](#events)); from there, a UI reconciles the whole list — add, rename, reorder, remove — in one call via `PUT` (**Sync**) rather than issuing separate create/update/delete requests per step.

#### Invariants

- `name` and `order_index` are required on create; `order_index` must be `>= 0`.
- `(event_id, order_index)` is unique — DB-enforced; a conflicting `order_index` on create or update surfaces as `errorz.Conflict` (409).
- `event_id` is **immutable** and always taken from the URL, never the body — it is not part of `CreateWorkflowStepInput`/`UpdateWorkflowStepInput`/`SyncWorkflowStepInput`.
- Every read/update/delete is scoped to the `{event_id}` in the URL: a step that exists but belongs to a different event resolves as `errorz.NotFound` (404), not a cross-event leak.
- **Sync** (`PUT /workflow-steps`) treats the request body as the *entire* desired list for the event: entries with an `id` are updated, entries without one are created, and any existing step **not** present in the list is deleted. Two entries sharing an `order_index` are rejected up front (400) rather than left to race against the DB constraint. An `id` that doesn't belong to this event is rejected (404).

> Field-presence/format checks (`required`, `gte=0`) are enforced at the HTTP boundary via `validate:"..."` tags (see [PATTERNS.md](PATTERNS.md#request-validation-boundary)); the event-scope check and Sync's reconciliation rules are business invariants enforced in the service since no struct tag or DB constraint can express them.

#### Endpoints

Base path `/api/v1/events/{event_id}/workflow-steps`:

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List for the event (paginated, filtered, sorted) | 200 | 400 invalid event id/query |
| `PUT` | `/` | **Sync** — replace the entire list (add/update/delete/reorder in one call) | 200 | 400 duplicate order_index/unknown id · 422 invalid entity |
| `GET` | `/{id}` | Get one by UUID, scoped to the event | 200 | 400 bad UUID · 404 not found |
| `POST` | `/` | Create a single step under the event | 201 | 400 invalid body · 409 order_index conflict · 422 invalid entity |
| `PUT` | `/{id}` | Partial update of a single step | 200 | 400 · 404 not found · 409 order_index conflict |
| `DELETE` | `/{id}` | Soft delete a single step | 204 | 400 · 404 not found |

The single-step `POST`/`PUT {id}`/`DELETE {id}` endpoints remain available (e.g. for scripts or a future mobile client that reorders one step at a time); a "one screen, one save button" UI is expected to call **Sync** exclusively.

**List query:** `?page=1&size=20&sort=order_index,ASC&name=Check-in`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, name, order_index, allows_multiple, created_at, updated_at`); unknown field → 400.
- Filters: `name` (exact match); `event_id` is always forced from the URL and is not a query filter.

**Sync body:** a JSON array of `{ id?, name, order_index, allows_multiple? }`. Example — update step `A`, add a new step, and (implicitly) delete every other existing step:
```json
[
  { "id": "11111111-...", "name": "Check-in", "order_index": 0 },
  { "name": "Photo booth", "order_index": 1, "allows_multiple": true }
]
```

#### States & lifecycle

- **Create** (single-step) — service generates the `id` (UUID) and sets `event_id` from the URL; `created_at`/`updated_at` are stamped by the audit repository decorator.
- **Update** (single-step) — partial; `event_id` cannot change; `updated_at` re-stamped by the decorator.
- **Delete** (single-step) — **soft**: `deleted_at` is set; the row remains. All reads/lists automatically exclude soft-deleted rows (`deleted_at IS NULL`, injected by the decorator).
- **Sync** — reconciles in three passes: (1) delete every existing step not referenced by the payload, (2) move every referenced step to a unique negative `order_index` and apply its new `name`/`allows_multiple`, (3) create new entries and move the referenced steps to their final `order_index`. The two-phase reorder (negative, then final) exists so that swapping two steps' `order_index` — the common drag-reorder case — never trips the `(event_id, order_index)` unique constraint on an intermediate state. Sync is **not wrapped in a database transaction** (no feature in this codebase uses one yet); a failure partway through can leave a partial result, in which case re-submitting the same Sync call is the recovery path.
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrAlreadyExists`→409, `ErrInvalidEntity`→422); a step belonging to a different event is also reported as 404; unexpected errors become 500 and are logged with context.

### Workflow step templates

#### Intent

Manages a category's **workflow step templates** — the default lifecycle stages `EventService.Create` copies onto every new event in that category (see [Events](#events)). Always scoped to a parent event category via the URL; there is no top-level listing. Unlike event-level workflow steps, templates have no **Sync** endpoint — they are administered as individual records (add/rename/reorder one at a time) since they change far less often than an event's own steps.

#### Invariants

- `name` and `order_index` are required on create; `order_index` must be `>= 0`.
- `(category_id, order_index)` is unique — DB-enforced; a conflicting `order_index` on create or update surfaces as `errorz.Conflict` (409).
- `category_id` is **immutable** and always taken from the URL, never the body — it is not part of `CreateWorkflowStepTemplateInput`/`UpdateWorkflowStepTemplateInput`.
- Every read/update/delete is scoped to the `{category_id}` in the URL: a template that exists but belongs to a different category resolves as `errorz.NotFound` (404), not a cross-category leak.
- `ticket_type_applicability` is an opaque, optional JSON document — the service passes it through unvalidated.

> Field-presence/format checks (`required`, `gte=0`) are enforced at the HTTP boundary via `validate:"..."` tags (see [PATTERNS.md](PATTERNS.md#request-validation-boundary)); the category-scope check is a business invariant enforced in the service since no struct tag or DB constraint can express it.

#### Endpoints

Base path `/api/v1/event-categories/{category_id}/workflow-step-templates`:

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List for the category (paginated, filtered, sorted) | 200 | 400 invalid category id/query |
| `GET` | `/{id}` | Get one by UUID, scoped to the category | 200 | 400 bad UUID · 404 not found |
| `POST` | `/` | Create a template under the category | 201 | 400 invalid body · 409 order_index conflict · 422 invalid entity |
| `PUT` | `/{id}` | Partial update of a single template | 200 | 400 · 404 not found · 409 order_index conflict |
| `DELETE` | `/{id}` | Soft delete a single template | 204 | 400 · 404 not found |

**List query:** `?page=1&size=20&sort=order_index,ASC&name=Check-in`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, name, order_index, allows_multiple, created_at, updated_at`); unknown field → 400.
- Filters: `name` (exact match); `category_id` is always forced from the URL and is not a query filter.

#### States & lifecycle

- **Create** — service generates the `id` (UUID) and sets `category_id` from the URL; `created_at`/`updated_at` are stamped by the audit repository decorator.
- **Update** — partial; `category_id` cannot change; `updated_at` re-stamped by the decorator.
- **Delete** — **soft**: `deleted_at` is set; the row remains. All reads/lists automatically exclude soft-deleted rows (`deleted_at IS NULL`, injected by the decorator).
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrAlreadyExists`→409, `ErrInvalidEntity`→422); a template belonging to a different category is also reported as 404; unexpected errors become 500 and are logged with context.

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

Manages **users** — the members of a tenant who log in and act on its behalf (staff, admins). Each user belongs to exactly one tenant and one role; roles determine permissions (see [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md) B3/B6). At most one user per tenant is the **tenant master** — the default user who can create other users for that tenant.

### Invariants

- `tenant_id`, `email`, `password`, and `role_id` are required on create; `email` must be a valid address.
- `tenant_id` is **immutable** after creation — a user cannot move tenants; it is not part of `UpdateInput`.
- `role_id` must reference a **system-scope** role (`roles.scope = 'system'` — see [roles](#roles) and [docs/STAFFING_RBAC.md](STAFFING_RBAC.md) §3): the service loads the role by ID on create and on any update that changes `role_id`, returning `errorz.NotFound` (404) if the role doesn't exist and `errorz.BadRequest` (400) if it exists but is `event`-scoped. An event-scoped role (e.g. Usher) can never become a user's tenant-wide role.
- `password` is never stored or returned in the clear: the service hashes it with bcrypt before persisting (`password_hash`), and the model tags `PasswordHash` `json:"-"` so it is never serialized in any response.
- `email` is unique **across all tenants** (migration 000012 — see [DATABASE.md](DATABASE.md#35-users)), so [B3 `auth`](#auth) can look a user up at login by email alone. At most one user per tenant may have `is_tenant_master = true`, also DB-enforced. Exactly one user in the whole system may hold the Super Admin role — DB-enforced by a partial unique index on `users.role_id` (migration 000014; see [DATABASE.md](DATABASE.md)).
- On update (partial), only provided fields change; a provided `password` is re-hashed; a provided `role_id` is re-validated as system-scope.

> Field-presence/format checks (`required`, `email`, `min`) are enforced at the HTTP boundary via `validate:"..."` tags on `CreateInput`/`UpdateInput` (see [PATTERNS.md](PATTERNS.md#request-validation-boundary)). `tenant_id` scoping stays explicit in the request body — [B3 `auth`](#auth) puts `user_id`/`tenant_id` in the authenticated request context (`auth.ClaimsFromContext`), but users' own endpoints don't yet consume it in place of the body field.

### Endpoints

Base path `/api/v1/users`:

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List (paginated, filtered, sorted) | 200 | 400 invalid query |
| `GET` | `/{id}` | Get one by UUID | 200 | 400 bad UUID · 404 not found |
| `POST` | `/` | Create | 201 | 400 invalid body · 409 conflict · 422 invalid entity |
| `PUT` | `/{id}` | Partial update | 200 | 400 · 404 not found · 409 conflict |
| `DELETE` | `/{id}` | Soft delete | 204 | 400 · 404 not found |

**List query:** `?page=1&size=20&sort=email,ASC&sort=id,DESC&tenant_id=...&email=...`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, tenant_id, email, role_id, is_tenant_master, created_at, updated_at`); unknown field → 400.
- Filters: `tenant_id`, `email`, `role_id`, `is_tenant_master` (exact match); unknown keys ignored.

### States & lifecycle

- **Create** — service loads and validates `role_id`'s scope first (404/400 short-circuit before any write), then generates the `id` (UUID) and hashes the plaintext `password` with bcrypt; `created_at`/`updated_at` are stamped by the audit repository decorator.
- **Update** — partial; a provided `password` is re-hashed, a provided `role_id` is re-validated as system-scope, `tenant_id` cannot change; `updated_at` re-stamped by the decorator.
- **Delete** — **soft**: `deleted_at` is set; the row remains. All reads/lists automatically exclude soft-deleted rows (`deleted_at IS NULL`, injected by the decorator).
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrAlreadyExists`→409, `ErrInvalidEntity`→422); a missing `role_id`→404, a `role_id` that isn't system-scope→400; unexpected errors (including a bcrypt hashing failure) become 500 and are logged with context.

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

Assigns tenant users to events with an **event-scoped role** (e.g. Usher, Photobooth Staff) that governs what they can do at that one event, independent of their tenant-wide role (see [docs/STAFFING_RBAC.md](STAFFING_RBAC.md) §1–§2, §5). Every endpoint requires the caller's **system-level role** to hold the `manage_staff` permission (Tenant Admin/Super Admin by the seeded catalog — see [roles](#roles)); an event-level role never grants staff-management rights on its own.

### Invariants

- **Tenant scoping comes from the JWT, not the request body** — every endpoint resolves the caller's tenant via `authz.TenantIDFromContext` (the `tenant_id` claim), and `CreateAssignmentInput`/`UpdateAssignmentInput` carry **no `tenant_id` field**. This is a deliberate, narrow divergence from the [`users`](#users)/[`events`](#events) precedent (which still take `tenant_id` explicitly in the body — a known, separately-tracked gap): permission enforcement is meaningless if a caller can simply name a different tenant's `event_id`/`user_id` in the body while the permission check passes on their own role.
- Every request is authorized against the caller's **system-level role's** permissions (the `manage_staff` permission), checked by `authz.RequirePermission` middleware before the handler runs — see [docs/STAFFING_RBAC.md](STAFFING_RBAC.md) §6.
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

Source: `internal/features/tickets`. Tables: `ticket_types`, `ticket_type_workflow_steps` (see [DATABASE.md](DATABASE.md)). This phase ships **`TicketType` only** — the `Ticket` QR artifact issued to a guest is B8, which depends on `guests` existing first.

### Intent

Manages an event's **ticket types** (e.g. Regular, VIP) and which of that event's workflow steps each type applies to, so guests receive the right ticket and check-in later enforces only the steps that type is entitled to (REQUIREMENT.md §3.8, §4.4). Always scoped to a parent event via the URL; there is no top-level ticket-type listing.

### Invariants

- `name` is required on create; `(event_id, name)` is unique — DB-enforced; a conflicting name on create or update surfaces as `errorz.Conflict` (409).
- `event_id` is **immutable** and always taken from the URL, never the body — it is not part of `CreateTicketTypeInput`/`UpdateTicketTypeInput`.
- Every read/update/delete/workflow-step-replace is scoped to the `{event_id}` in the URL: a ticket type that exists but belongs to a different event resolves as `errorz.NotFound` (404), not a cross-event leak — the same convention as [workflow steps](#workflow-steps).
- `rules` is an opaque, optional JSON document (default `{}`), passed through unvalidated — same treatment as `workflow_step_templates.ticket_type_applicability`.
- **Every route requires the `manage_events` permission** (`authz.RequirePermission`, same mechanism [staffing](#staffing) uses for `manage_staff`) — a deliberate divergence from `events`/`workflow-steps`, which stay auth-only/ungated in this phase.
- `workflow_step_ids` (the set of the event's workflow steps this ticket type currently applies to) is populated only on `GetByID` (one extra lookup) and on the workflow-step-replace response; it is left empty on `List`/`Update` to avoid an N+1 join across every row of a list.
- **`PUT .../workflow-steps`** replaces the *entire* set of workflow steps a ticket type applies to (full-replace, no incremental add/remove — mirroring [workflow steps](#workflow-steps)' Sync design). Every submitted ID is validated to belong to the ticket type's own event; an ID that doesn't exist or belongs to a different event is rejected as `errorz.BadRequest` (400) before anything is written.

> Field-presence/format checks (`required`) are enforced at the HTTP boundary via `validate:"..."` tags on `CreateTicketTypeInput`/`UpdateTicketTypeInput` (see [PATTERNS.md](PATTERNS.md#request-validation-boundary)); the event-scope check and workflow-step cross-event validation are business invariants enforced in the service since no struct tag or DB constraint can express them.

### Endpoints

Base path `/api/v1/events/{event_id}/ticket-types`. Every route requires a valid access token **and** the `manage_events` permission (`authz.RequirePermission`):

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List for the event (paginated, filtered, sorted) | 200 | 400 invalid event id/query · 401 · 403 |
| `POST` | `/` | Create a ticket type under the event | 201 | 400 invalid body · 401 · 403 · 409 name conflict · 422 invalid entity |
| `GET` | `/{id}` | Get one, scoped to the event — response includes `workflow_step_ids` | 200 | 400 bad UUID · 401 · 403 · 404 not found |
| `PUT` | `/{id}` | Partial update (`name`, `rules`) | 200 | 400 · 401 · 403 · 404 not found · 409 name conflict |
| `DELETE` | `/{id}` | Soft delete | 204 | 400 · 401 · 403 · 404 not found |
| `PUT` | `/{id}/workflow-steps` | **Replace** the full set of workflow steps this ticket type applies to | 200 | 400 a step id not on this event · 401 · 403 · 404 ticket type not found |

**List query:** `?page=1&size=20&sort=name,ASC&name=VIP`.
- `page` 1-based; `size` default 20, clamped to 100.
- `sort` repeatable, `field,DIR` — only fields in the allow-list (`id, name, created_at, updated_at`); unknown field → 400.
- Filters: `name` (exact match); `event_id` is always forced from the URL and is not a query filter.

**Workflow-step replace body:** `{ "workflow_step_ids": ["11111111-...", "22222222-..."] }` — an empty array clears applicability entirely.

### States & lifecycle

- **Create** — service generates the `id` (UUID) and sets `event_id` from the URL; `rules` defaults to `{}` when omitted; `created_at`/`updated_at` are stamped by the audit repository decorator. Right after the row insert, the new ticket type's workflow-step applicability is seeded to **ALL** of the event's current workflow steps (fail open — an organizer forgetting to configure a new type should default to full admission, not silently block guests at every check-in step) via `TicketTypeWorkflowStepRepository.SetWorkflowStepIDs`. This is **best-effort**, the same non-blocking treatment as `EventService.Create`'s template-copy (see [Events](#events)): a workflow-step lookup or assign failure is logged but does not fail ticket type creation.
- **Update** — partial (`name`/`rules` only); `event_id` cannot change; `workflow_step_ids` is left unpopulated on the response; `updated_at` re-stamped by the decorator.
- **Delete** — **soft**: `deleted_at` is set; the row remains. All reads/lists automatically exclude soft-deleted rows (`deleted_at IS NULL`, injected by the decorator).
- **Replace workflow steps** — full-replace via `TicketTypeWorkflowStepRepository.SetWorkflowStepIDs` (delete-then-reinsert every association for the ticket type); **not wrapped in a database transaction** (no feature in this codebase uses one yet, same as [workflow steps](#workflow-steps)' Sync) — a failure partway through is recovered by resubmitting the same call.
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrAlreadyExists`→409, `ErrInvalidEntity`→422); a ticket type or workflow step belonging to a different event is also reported as 404/400 respectively (see Invariants); a missing `manage_events` permission →403; unexpected errors become 500 and are logged with context.

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
- Every read/update/delete/invitation-send is scoped to the `{event_id}` in the URL: a guest that exists but
  belongs to a different event resolves as `errorz.NotFound` (404), not a cross-event leak — the same convention
  as [ticket-types](#tickets)/[workflow-steps](#workflow-steps).
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
- **The public RSVP endpoint is looked up by token alone, with no event/auth scoping** — the caller is an
  unauthenticated guest with no session. An unknown token is `errorz.NotFound` (404). Confirming issues the
  guest's `Ticket` if one hasn't already been issued (idempotent — a no-op if `rsvp_required` was `false` and one
  was already issued at invitation time); **declining never touches an already-issued ticket** — a per-product
  decision to keep this simple rather than voiding it.
- **Invitation delivery is a publish, not a real send** — `GuestService.SendInvitation` calls an app-local
  `InvitationPublisher` interface (`internal/features/guests/invitation_publisher.go`) best-effort (a failure is
  logged but doesn't fail the call, same treatment as `EventService.Create`'s template-copy). The only
  implementation today, `LoggingInvitationPublisher`, just logs the message — go-sdk has no Kafka/queue package
  yet to wire a real one to.

> Field-presence/format checks (`required`, `email`) are enforced at the HTTP boundary via `validate:"..."` tags
> on `CreateGuestInput`/`UpdateGuestInput`/`RSVPInput` (see [PATTERNS.md](PATTERNS.md#request-validation-boundary));
> the event/ticket-type scoping, PII encryption, and RSVP/ticket-issuance rules stay in the service as business
> invariants no struct tag or DB constraint can express.

### Endpoints

Base path `/api/v1/events/{event_id}/guests`. Every route requires a valid access token **and** the
`manage_guests` permission (`authz.RequirePermission`, seeded in B6's catalog, previously unused by any slice):

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List for the event (paginated, filtered, sorted) | 200 | 400 · 401 · 403 |
| `POST` | `/` | Create a guest under the event | 201 | 400 invalid body/ticket type cross-event · 401 · 403 · 422 |
| `GET` | `/{id}` | Get one, scoped to the event — includes the issued ticket/QR summary once one exists | 200 | 400 bad UUID · 401 · 403 · 404 |
| `PUT` | `/{id}` | Partial update (`name`, `email`, `phone`, `ticket_type_id`) | 200 | 400 · 401 · 403 · 404 |
| `DELETE` | `/{id}` | Soft delete | 204 | 400 · 401 · 403 · 404 |
| `POST` | `/{id}/invitation` | Send invitation — requires `ticket_type_id` already set | 200 | 400 no ticket type assigned · 401 · 403 · 404 |

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
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrInvalidEntity`→422); a ticket type belonging to a different event is `errorz.BadRequest` (400); a guest belonging to a different event is `errorz.NotFound` (404); a missing `manage_guests` permission →403; unexpected errors become 500 and are logged with context.

---

## scans

Source: `internal/features/scans`. Table: `scan_logs` (see [DATABASE.md](DATABASE.md)). Flat layout, one entity —
`ScanLog` is a permanent, append-only audit row; `Ticket`, `WorkflowStep`, and `TicketType` are read/written
through already-exported cross-feature repositories (`guests.NewTicketRepository`,
`events/workflowstep.NewWorkflowStepRepository`, `tickets.TicketTypeWorkflowStepRepository`), never owned here.

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
