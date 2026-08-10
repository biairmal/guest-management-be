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
- `password` is never stored or returned in the clear: the service hashes it with bcrypt before persisting (`password_hash`), and the model tags `PasswordHash` `json:"-"` so it is never serialized in any response.
- `email` is unique **across all tenants** (migration 000012 — see [DATABASE.md](DATABASE.md#35-users)), so [B3 `auth`](#auth) can look a user up at login by email alone. At most one user per tenant may have `is_tenant_master = true`, also DB-enforced.
- On update (partial), only provided fields change; a provided `password` is re-hashed.

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

- **Create** — service generates the `id` (UUID) and hashes the plaintext `password` with bcrypt; `created_at`/`updated_at` are stamped by the audit repository decorator.
- **Update** — partial; a provided `password` is re-hashed, `tenant_id` cannot change; `updated_at` re-stamped by the decorator.
- **Delete** — **soft**: `deleted_at` is set; the row remains. All reads/lists automatically exclude soft-deleted rows (`deleted_at IS NULL`, injected by the decorator).
- **Errors** — repository sentinels are translated to `errorz` codes (`ErrNotFound`→404, `ErrAlreadyExists`→409, `ErrInvalidEntity`→422); unexpected errors (including a bcrypt hashing failure) become 500 and are logged with context.

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
- Every issued token carries a `type` claim (`access` or `refresh`) and a `tenant_id` claim. The protected-route
  middleware is wired with a validator wrapped in `AccessOnlyValidator`, which rejects any token whose `type`
  isn't `access` — so a refresh token, despite being signed by the same issuer, cannot be used to call a
  protected endpoint.
- `POST /auth/refresh` validates the refresh token with the *unwrapped* validator, checks `type == "refresh"`
  itself, and re-loads the user by the token's subject so a deleted account cannot refresh past its removal.
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
