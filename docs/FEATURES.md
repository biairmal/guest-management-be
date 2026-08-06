# Feature behaviour

How the app behaves, feature by feature — intent, invariants, endpoints, and lifecycle. This satisfies the "document how the app works" goal and is required for every feature (see [../AGENTS.md](../AGENTS.md#documentation)). For the data model, see [DATABASE.md](DATABASE.md); for the roadmap of unbuilt features, see [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md).

Each section follows the same template:

- **Intent** — what the feature is for.
- **Invariants** — rules that always hold (enforced in the service).
- **Endpoints** — HTTP surface (also in Swagger).
- **States & lifecycle** — how records are created, changed, and removed.

---

## events

Source: `internal/features/events`. Table: `event_categories` (see [DATABASE.md](DATABASE.md)).

### Intent

Manages **event categories** — the taxonomy events are classified under. Categories are either **app-defined** (available to every tenant) or **tenant-defined** (private to one tenant).

### Invariants

- `source` is one of `"app"` or `"tenant"`.
- When `source == "app"`, `tenant_id` **must be null** (a system category belongs to no tenant).
- When `source == "tenant"`, `tenant_id` **is required**.
- `name` is required and non-empty.
- On update (partial), only provided fields change; the same source/tenant rules re-apply to the resulting record.

> Field-presence/format checks (`required`, `oneof`) are enforced at the HTTP boundary via `validate:"..."` tags on `CreateInput`/`UpdateInput` (see [PATTERNS.md](PATTERNS.md#request-validation-boundary)); the cross-field source/tenant rule above stays in the service as a business invariant.

### Endpoints

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

### States & lifecycle

- **Create** — service generates the `id` (UUID); `created_at`/`updated_at` are stamped by the audit repository decorator.
- **Update** — partial; `updated_at` re-stamped by the decorator.
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
- `(tenant_id, email)` is unique, and at most one user per tenant may have `is_tenant_master = true` — both enforced by database constraints (see [DATABASE.md](DATABASE.md#35-users)).
- On update (partial), only provided fields change; a provided `password` is re-hashed.

> Field-presence/format checks (`required`, `email`, `min`) are enforced at the HTTP boundary via `validate:"..."` tags on `CreateInput`/`UpdateInput` (see [PATTERNS.md](PATTERNS.md#request-validation-boundary)); tenant-id scoping is explicit in the request body until [B3 `auth`](DEVELOPMENT_PLAN.md) lands and it moves to the auth context.

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
