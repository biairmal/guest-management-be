# API Contract

Condensed, hand-maintained summary of the live HTTP surface — generated from the Swagger annotations in the
handler files (`make swagger-generate` → `api/swagger/{swagger.json,swagger.yaml,docs.go}`) and cross-checked
against the service/DTO code. Frontend and Tester should integrate against **this file**, not handler code
directly; if an endpoint you need isn't here, flag it back to `backend-developer` rather than reading Go
source. See [FEATURES.md](FEATURES.md) for the *why* behind each invariant, [DATABASE.md](DATABASE.md) for the
schema, and the live Swagger UI (`/swagger/index.html`, Basic-Auth protected) for the full generated spec.

Last regenerated: 2026-09-01 (B6 `staffing` — adds `roles`(internal-only)/`staffing`, `role_id` JWT claim,
`users.role_id` scope validation).

---

## 1. Conventions

### Base URL & versioning

All endpoints are prefixed `/api/v1`, except `/health`, `/ready`, `/metrics`, and `/swagger/*`.

### Auth

- `POST /api/v1/auth/login` and `POST /api/v1/auth/refresh` are the only **public** routes under `/api/v1`.
  Every other route requires `Authorization: Bearer <access_token>` (per `auth.token.rules` in
  `configs/config.yaml`, `default_protected: true`).
- A missing/invalid/expired token → `401`. A valid *refresh* token presented on a protected route also → `401`
  (access-only validator).
- Since B6, every issued token (access and refresh) carries three claims beyond the standard JWT ones:
  `type` (`"access"` | `"refresh"`), `tenant_id` (UUID string), `role_id` (UUID string — the user's current
  **system-level** role). `role_id`/`tenant_id` are not returned to the client in the login/refresh response
  body — they only exist as JWT claims; decode the access token client-side if the UI needs them (e.g. to hide
  admin-only nav items).
- Routes gated on a specific permission (currently only `staffing`'s) additionally return `403` when the
  caller's role lacks that permission — see §7.

### Response envelope

Every response (success or error) is wrapped:

```json
{
  "code": "OK",
  "message": "success",
  "timestamp": "2026-09-01T12:00:00Z",
  "data": { }
}
```

Errors use the same envelope with `code: "ERROR"` and an `error` object instead of `data`:

```json
{
  "code": "ERROR",
  "message": "<human-readable message>",
  "timestamp": "2026-09-01T12:00:00Z",
  "error": {
    "code": "NOT_FOUND",
    "message": "user not found",
    "meta": { }
  }
}
```

`error.code` is one of `BAD_REQUEST` (400), `UNAUTHORIZED` (401), `FORBIDDEN` (403), `NOT_FOUND` (404),
`CONFLICT` (409), `UNPROCESSABLE_ENTITY` (422), `INTERNAL` (500), plus a few not used by this API today
(`TOO_MANY_REQUESTS`, `BAD_GATEWAY`, `SERVICE_UNAVAILABLE`, `PRECONDITION_*`). Validation failures (400) put
per-field detail in `error.meta`.

`DELETE` endpoints return **204 No Content** with an empty body (no envelope).

### Pagination (every `GET /` list endpoint)

Query params: `page` (1-based, default 1), `size` (default 20, max 100), `sort` (repeatable,
`field,ASC|DESC`, only allow-listed fields — unknown field → 400), plus each endpoint's own allow-listed
equality filters (unknown filter keys are silently ignored, not rejected).

Response `data` shape:

```json
{
  "items": [ /* array of the resource */ ],
  "total": 42,
  "page": 1,
  "size": 20,
  "total_pages": 3,
  "has_prev": false,
  "has_next": true
}
```

### Nested-resource ID scoping

Endpoints nested under a parent (`/events/{event_id}/workflow-steps`, `/event-categories/{category_id}/workflow-step-templates`,
`/events/{event_id}/staff`) always resolve the child **scoped to that parent**: a child that exists but belongs
to a different parent resolves as `404`, not a cross-parent leak. The parent ID is never part of the request
body — always the URL.

---

## 2. `auth` — `/api/v1/auth`

No resource model; issues/refreshes JWTs against the `users` table.

### `POST /api/v1/auth/login` — public

| | |
|---|---|
| Body | `{ "email": string (required, email), "password": string (required) }` |
| 200 | `TokenPair` (below) |
| 400 | invalid body |
| 401 | `invalid email or password` — same message for unknown email *and* wrong password (no enumeration) |

### `POST /api/v1/auth/refresh` — public

| | |
|---|---|
| Body | `{ "refresh_token": string (required) }` |
| 200 | `TokenPair` |
| 400 | invalid body |
| 401 | invalid/expired token, wrong token type (an access token can't be used here), or the user no longer exists |

**`TokenPair`:**

```json
{
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "token_type": "Bearer",
  "expires_in": 900
}
```

`expires_in` is the **access** token's TTL in seconds (`auth.token.issuer.default_ttl`, default 900 = 15m).
The refresh token's TTL is `auth.refresh_ttl` (default 168h = 7d) and is not returned separately.

---

## 3. `tenants` — `/api/v1/tenants`

CRUD + list. No parent scoping.

**`Tenant` (response):**

```json
{
  "id": "uuid",
  "name": "Acme Co",
  "type": "enterprise",
  "settings": {},
  "branding": {},
  "created_at": "2026-...",
  "updated_at": "2026-...",
  "deleted_at": null
}
```

| Method | Path | Body | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | — | 200 `PageResponse<Tenant>` | 400 invalid query |
| `GET` | `/{id}` | — | 200 `Tenant` | 400 bad UUID · 404 |
| `POST` | `/` | `TenantCreateInput` | 201 `Tenant` | 400 · 409 · 422 |
| `PUT` | `/{id}` | `TenantUpdateInput` | 200 `Tenant` | 400 · 404 |
| `DELETE` | `/{id}` | — | 204 | 400 · 404 |

**`TenantCreateInput`:** `{ "name": string (required), "type"?: string, "settings"?: object, "branding"?: object }` — omitted `settings`/`branding` default to `{}`.

**`TenantUpdateInput`:** same fields, all optional (partial update); omitted `settings`/`branding` leave the existing value untouched (not reset to `{}`).

**List:** sort allow-list `id, name, type, created_at, updated_at`. Filters: `name`, `type` (exact match).

---

## 4. `users` — `/api/v1/users`

CRUD + list. `role_id` **must reference a `system`-scope role** (see [roles](#8-roles--internal-only-no-endpoints)) — new in B6.

**`User` (response):** `password_hash` is **never** serialized.

```json
{
  "id": "uuid",
  "tenant_id": "uuid",
  "email": "a@acme.com",
  "role_id": "uuid",
  "is_tenant_master": false,
  "created_at": "2026-...",
  "updated_at": "2026-...",
  "deleted_at": null
}
```

| Method | Path | Body | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | — | 200 `PageResponse<User>` | 400 invalid query |
| `GET` | `/{id}` | — | 200 `User` | 400 bad UUID · 404 |
| `POST` | `/` | `UserCreateInput` | 201 `User` | 400 · 404 role not found · 400 role not system-scope · 409 · 422 |
| `PUT` | `/{id}` | `UserUpdateInput` | 200 `User` | 400 · 404 · 409 |
| `DELETE` | `/{id}` | — | 204 | 400 · 404 |

**`UserCreateInput`:** `{ "tenant_id": uuid (required), "email": string (required, email), "password": string (required, min 8), "role_id": uuid (required), "is_tenant_master"?: bool }`.

**`UserUpdateInput`:** `{ "email"?: string (email), "password"?: string (min 8), "role_id"?: uuid, "is_tenant_master"?: bool }` — `tenant_id` is immutable, not part of this input. A provided `password` is re-hashed; a provided `role_id` is re-validated as system-scope (404 if the role doesn't exist, 400 if it's `event`-scope).

**List:** sort allow-list `id, tenant_id, email, role_id, is_tenant_master, created_at, updated_at`. Filters: `tenant_id`, `email`, `role_id`, `is_tenant_master` (exact match).

> `tenant_id` is still taken from the request body here (not the JWT claim) — a known gap tracked separately, unlike `staffing` below which deliberately uses the JWT claim.

---

## 5. `events` — event categories, events, workflow steps, workflow step templates

Source: `internal/features/events`. Four related resources under one slice.

### 5.1 Event categories — `/api/v1/event-categories`

**`EventCategory`:** `{ "id", "source": "app"|"tenant", "tenant_id"?: uuid, "name", "created_at", "updated_at", "deleted_at"? }`.

| Method | Path | Body | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | — | 200 `PageResponse<EventCategory>` | 400 |
| `GET` | `/{id}` | — | 200 | 400 · 404 |
| `POST` | `/` | `CreateInput` | 201 | 400 · 409 · 422 |
| `PUT` | `/{id}` | `UpdateInput` | 200 | 400 · 404 |
| `DELETE` | `/{id}` | — | 204 | 400 · 404 |

**`CreateInput`:** `{ "source": "app"|"tenant" (required), "tenant_id"?: uuid, "name": string (required) }` — `tenant_id` must be null when `source="app"`, required when `source="tenant"` (400 otherwise).

**`UpdateInput`:** same fields, all optional.

**List:** sort allow-list `id, source, tenant_id, name, created_at, updated_at`. Filters: `name`, `source`, `tenant_id`.

### 5.2 Events — `/api/v1/events`

**`Event`:** `{ "id", "tenant_id", "category_id", "name", "description"?: string, "start_date", "end_date", "is_multi_day": bool, "created_at", "updated_at", "deleted_at"? }`. `is_multi_day` is **derived**, never caller-supplied.

| Method | Path | Body | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | — | 200 `PageResponse<Event>` | 400 |
| `GET` | `/{id}` | — | 200 | 400 · 404 |
| `POST` | `/` | `CreateEventInput` | 201 | 400 (incl. `end_date` before `start_date`) · 409 · 422 |
| `PUT` | `/{id}` | `UpdateEventInput` | 200 | 400 · 404 |
| `DELETE` | `/{id}` | — | 204 | 400 · 404 |

**`CreateEventInput`:** `{ "tenant_id": uuid (required), "category_id": uuid (required), "name": string (required), "description"?: string, "start_date": RFC3339 (required), "end_date": RFC3339 (required) }`. On create, the service also **best-effort copies** the category's `workflow_step_templates` into new `workflow_steps` for the event (silent no-op if none/failure — doesn't fail the create).

**`UpdateEventInput`:** all fields optional except `tenant_id`, which is immutable (not part of this input).

**List:** sort allow-list `id, tenant_id, category_id, name, start_date, end_date, is_multi_day, created_at, updated_at`. Filters: `tenant_id`, `category_id`, `name`.

### 5.3 Workflow steps — `/api/v1/events/{event_id}/workflow-steps`

**`WorkflowStep`:** `{ "id", "event_id", "name", "order_index": int, "allows_multiple": bool, "created_at", "updated_at", "deleted_at"? }`.

| Method | Path | Body | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | — | 200 `PageResponse<WorkflowStep>` | 400 |
| `PUT` | `/` | `[]SyncWorkflowStepInput` | 200 `WorkflowStep[]` | 400 (dup order_index / unknown id) · 422 |
| `GET` | `/{id}` | — | 200 | 400 · 404 |
| `POST` | `/` | `CreateWorkflowStepInput` | 201 | 400 · 409 (order_index conflict) · 422 |
| `PUT` | `/{id}` | `UpdateWorkflowStepInput` | 200 | 400 · 404 · 409 |
| `DELETE` | `/{id}` | — | 204 | 400 · 404 |

**`CreateWorkflowStepInput`:** `{ "name": string (required), "order_index": int (>=0), "allows_multiple"?: bool }`. `event_id` always from the URL.

**`UpdateWorkflowStepInput`:** all fields optional.

**`SyncWorkflowStepInput`** (array body for `PUT /`): `{ "id"?: uuid, "name": string (required), "order_index": int (>=0), "allows_multiple"?: bool }` — entries with `id` are updated, entries without are created, and any existing step **not** in the array is deleted. This is the single "one screen, one save button" endpoint a UI should use for add/update/delete/reorder; the single-item `POST`/`PUT {id}`/`DELETE {id}` above remain available for scripted/one-off use.

**List:** sort allow-list `id, name, order_index, allows_multiple, created_at, updated_at`. Filters: `name`. `event_id` is always forced from the URL, never a query filter.

### 5.4 Workflow step templates — `/api/v1/event-categories/{category_id}/workflow-step-templates`

**`WorkflowStepTemplate`:** `{ "id", "category_id", "name", "order_index": int, "allows_multiple": bool, "ticket_type_applicability"?: object, "created_at", "updated_at", "deleted_at"? }`.

| Method | Path | Body | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | — | 200 `PageResponse<WorkflowStepTemplate>` | 400 |
| `GET` | `/{id}` | — | 200 | 400 · 404 |
| `POST` | `/` | `CreateWorkflowStepTemplateInput` | 201 | 400 · 409 · 422 |
| `PUT` | `/{id}` | `UpdateWorkflowStepTemplateInput` | 200 | 400 · 404 · 409 |
| `DELETE` | `/{id}` | — | 204 | 400 · 404 |

**`CreateWorkflowStepTemplateInput`:** `{ "name": string (required), "order_index": int (>=0), "allows_multiple"?: bool, "ticket_type_applicability"?: object }` — the last field is opaque, passed through unvalidated. No **Sync** endpoint here (unlike event workflow steps) — templates are administered one at a time.

**`UpdateWorkflowStepTemplateInput`:** all fields optional.

**List:** sort allow-list `id, name, order_index, allows_multiple, created_at, updated_at`. Filters: `name`. `category_id` always forced from the URL.

---

## 6. `templates` — `/api/v1/message-templates`

CRUD + list. No parent scoping (source/tenant_id/event_id are body fields, not URL segments).

**`MessageTemplate`:** `{ "id", "source": "app"|"tenant"|"event", "tenant_id"?: uuid, "event_id"?: uuid, "name", "channel": "email"|"whatsapp", "subject"?: string, "body", "variables"?: object, "created_at", "updated_at", "deleted_at"? }`.

| Method | Path | Body | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | — | 200 `PageResponse<MessageTemplate>` | 400 |
| `GET` | `/{id}` | — | 200 | 400 · 404 |
| `POST` | `/` | `MessageTemplateCreateInput` | 201 | 400 (scope/channel invariants) · 409 · 422 |
| `PUT` | `/{id}` | `MessageTemplateUpdateInput` | 200 | 400 · 404 · 409 |
| `DELETE` | `/{id}` | — | 204 | 400 · 404 |

**`MessageTemplateCreateInput`:** `{ "source": "app"|"tenant"|"event" (required), "tenant_id"?: uuid, "event_id"?: uuid, "name": string (required), "channel": "email"|"whatsapp" (required), "subject"?: string, "body": string (required), "variables"?: object }`.

Cross-field rules (400 if violated):
- `source="app"` → `tenant_id`/`event_id` must both be null.
- `source="tenant"` → `tenant_id` required, `event_id` must be null.
- `source="event"` → both `tenant_id` and `event_id` required.
- `channel="email"` → `subject` required (non-empty).
- `channel="whatsapp"` → `subject` must be empty/omitted.

**`MessageTemplateUpdateInput`:** same fields, all optional; the resulting record is re-validated against the same rules.

**List:** sort allow-list `id, source, tenant_id, event_id, name, channel, created_at, updated_at`. Filters: `name`, `source`, `tenant_id`, `event_id`, `channel`.

Uniqueness (DB-enforced, surfaces as 409): `(name, channel)` per app scope, `(tenant_id, name, channel)` per tenant scope, `(event_id, name, channel)` per event scope.

---

## 7. `staffing` — `/api/v1/events/{event_id}/staff` — **new in B6**

Assigns a tenant user to an event with an **event-scoped role** (Usher, Photobooth Staff by the seeded
catalog). **Every route additionally requires the caller's system-level role to hold the `manage_staff`
permission** (Tenant Admin/Super Admin by the seeded catalog) — missing it → `403`, distinct from the usual
`401` for "not authenticated at all". See [FEATURES.md#staffing](FEATURES.md#staffing) and
[STAFFING_RBAC.md](STAFFING_RBAC.md) for the full rationale.

**Key difference from every other resource in this API:** `tenant_id` is **not** a request field anywhere in
this resource. It's resolved server-side from the `tenant_id` claim on the caller's access token. `event_id` in
the URL is verified to belong to that tenant (404, not 403, if it doesn't — same "don't reveal existence"
convention `workflow-steps` uses for a mismatched `event_id`).

**`EventStaffAssignment` (response):**

```json
{
  "id": "uuid",
  "event_id": "uuid",
  "user_id": "uuid",
  "role_id": "uuid",
  "created_at": "2026-...",
  "updated_at": "2026-...",
  "deleted_at": null
}
```

| Method | Path | Body | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | — | 200 `PageResponse<EventStaffAssignment>` | 400 · 401 · 403 · 404 event not found |
| `GET` | `/{id}` | — | 200 | 400 · 401 · 403 · 404 |
| `POST` | `/` | `CreateAssignmentInput` | 201 | 400 (incl. role not event-scope) · 401 · 403 · 404 (event/user/role) · 409 (already assigned) · 422 |
| `PUT` | `/{id}` | `UpdateAssignmentInput` | 200 | 400 (incl. role not event-scope) · 401 · 403 · 404 |
| `DELETE` | `/{id}` | — | 204 | 400 · 401 · 403 · 404 |

**`CreateAssignmentInput`:** `{ "user_id": uuid (required), "role_id": uuid (required) }`.

Validated in order (first failure wins): event exists & belongs to caller's tenant (404) → target user exists
& belongs to the **same tenant as the event** (404) → role exists (404) and is **`event`-scope**, not
`system`-scope (400 `role_id must reference an event-scope role`) → no other **active** assignment already
exists for this `(event_id, user_id)` pair (409 `user is already assigned to this event`).

**`UpdateAssignmentInput`:** `{ "role_id"?: uuid }` — **only** field. `event_id`/`user_id` are immutable; to
reassign a different user, `DELETE` the assignment and `POST` a new one. A provided `role_id` is re-validated
as event-scope (same 400 as create).

**Delete** is a soft delete; the same user **can** be re-assigned to the same event later (a fresh `POST`
creates a new row — it's not blocked by the deleted one, and it is not a "reactivation").

**List:** sort allow-list `id, user_id, role_id, created_at, updated_at`. Filters: `user_id`, `role_id`.
`event_id` is always forced from the URL, never a query filter — there is no cross-event staff listing.

---

## 8. `roles` — internal-only, no endpoints

Source: `internal/features/roles`. No HTTP surface this phase — ships model + repository only, consumed
in-process by `users` (validates `role_id` is `system`-scope), `staffing` (validates `role_id` is
`event`-scope), and `internal/core/authz` (resolves a role's permission codes to authorize `staffing`
requests). See [FEATURES.md#roles](FEATURES.md#roles) and [STAFFING_RBAC.md](STAFFING_RBAC.md) §3 for the
seeded starter catalog (role names, scopes, and permission grants) — useful for Frontend/Tester to know which
literal role/permission names exist, even with no CRUD endpoint to list them via API yet:

| Role | Scope | Grants |
|---|---|---|
| Super Admin | system | every permission |
| Tenant Admin | system | manage_users, manage_events, manage_staff, manage_guests, manage_workflows, check_in |
| Tenant Staff | system | manage_guests, check_in |
| Usher | event | check_in |
| Photobooth Staff | event | check_in |

A future phase may add admin CRUD endpoints for roles/permissions (out of scope now — see
[STAFFING_RBAC.md](STAFFING_RBAC.md) §7).
