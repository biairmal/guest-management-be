# New feature checklist

Follow these steps in order when adding a feature vertical slice. The rules referenced here are defined in [../AGENTS.md](../AGENTS.md); templates are in [PATTERNS.md](PATTERNS.md).

## 1. Decide the boundary

- Is this a distinct part of the domain (tenants, users, events, guests, tickets…)? Give it its own slice under `internal/features/<feature>`.
- Will it need data from another feature? Depend on a **published interface**, not the other slice's internals — the slice must stay independently extractable into its own service. Shared, feature-agnostic infrastructure goes in `internal/core`, not in a slice.
- Does this feature already model (or will it soon model) more than one distinct entity? Split into per-entity subpackages, `internal/features/<feature>/<entity>/` — see [PATTERNS.md#multi-entity-features-split-by-entity-not-by-layer](PATTERNS.md#multi-entity-features-split-by-entity-not-by-layer). A single-entity feature stays flat.

## 2. Schema & migration

- The schema is already defined in `migrations/` (see [DATABASE.md](DATABASE.md)). If you need a change, `make migration-create NAME=...` and write both `.up.sql` and `.down.sql`.
- Confirm column names match the `db` tags you'll put on the model, and note the soft-delete columns (`created_at`, `updated_at`, `deleted_at`).

## 3. Model

- `internal/features/<feature>/<entity>_model.go`: struct with `db` + `json` tags, `swagger:model` marker, and a `TableName()` method. Template: [PATTERNS.md#model](PATTERNS.md#model).

## 4. Repository

- `<entity>_repository.go`: build `sql.NewSQLRepository[Entity, uuid.UUID]`, wrap with `audit.NewAuditableRepository`, **return the typed generic interface** (`repository.Repository[Entity, uuid.UUID]`). Keep `TID` typed.
- **Do not** write a pass-through wrapper or widen the ID to `any`. Template: [PATTERNS.md#repository--thin-typed-no-pass-through](PATTERNS.md#repository--thin-typed-no-pass-through).

## 5. DTOs

- `<entity>_dto.go`: input/output DTOs (`CreateInput`, `UpdateInput`, ...) carrying `validate:"..."` tags. Not inline in the service file. Template: [PATTERNS.md#dtos--inputoutput-shapes](PATTERNS.md#dtos--inputoutput-shapes).

## 6. Constants

- `<feature>_constants.go` (or `<entity>_constants.go` inside a per-entity subpackage): exported constants — permission codes, enum-like string values, etc. Not scattered across model/service files; permission codes stay feature-owned. Template: [PATTERNS.md#constants](PATTERNS.md#constants).

## 7. Service

- `<entity>_service.go`: define the `XService` interface and its implementation only — no DTOs, no constants. Enforce **business invariants** only; translate repository sentinels to `errorz` codes; wrap the cause on internal errors; log with `*WithContext`. List methods convert `*query.ListParams` via the shared `query.ToListOptions` — never a per-feature `listParamsToListOptions`.
- For a list method, declare the exported `query.ListParseConfig` (allow-listed sort/filter fields) here and enforce it via `query.ValidateListParams` before converting — the service is the transport-agnostic authority, not the handler. Template: [PATTERNS.md#service--business-rules--error-translation](PATTERNS.md#service--business-rules--error-translation).

## 8. Handler + validation

- `<entity>_handler.go`: `func(*http.Request)(any,error)` handlers. Parse with `json.NewDecoder(r.Body).Decode(&body)` (go-sdk's `serializer.ParseJSON` takes `[]byte`, not `io.Reader` — it's for decoding an already-fetched value like a cache read, not an HTTP body), validate the DTO at the boundary via the shared validator, call the service, return `response.OK/Created/NoContent`.
- **Add Swagger annotations** to every handler (`@Summary`, `@Param`, `@Success`, `@Failure`, `@Router`). Template: [PATTERNS.md#handler--go-sdk-adapter--swagger](PATTERNS.md#handler--go-sdk-adapter--swagger).

## 9. Routes + list query

- `<entity>_routes.go`: `InitXRoutes(r chi.Router, h *XHandler)` registering `handler.Handle(...)`.
- For list endpoints, the handler calls the shared `internal/core/query.ParseListParams` with the **service's** exported `ListParseConfig` (don't declare a second copy in the handler) — do not reimplement parsing per feature. Template: [PATTERNS.md#list-query--allow-list-parsing-and-enforcement](PATTERNS.md#list-query--allow-list-parsing-and-enforcement).

## 10. Wire into the composition root

- In `internal/app`, add the feature to `repository.go` → `service.go` → `handler.go` → `router.go`. This is the **only** place that imports the slice for wiring. Don't add empty `Options{}` structs.

## 11. Tests

- **Table-driven `*__test.go`** for the service (against a **generated `gomock` mock** repository — `mockrepository.MockRepository` for a go-sdk repo interface, or an app-generated mock). The shared `internal/core/query` parser has its own tests; a feature only needs to cover its `ListParseConfig` if it adds custom behaviour. Cover error-translation branches (not-found → 404, conflict → 409, invalid → 422) and validation. If you added a new app interface, add a `//go:generate` directive and run `make mocks`. Live-DB tests go in `*_integration_test.go` guarded by `testing.Short()`. See [TESTING.md](TESTING.md).
- **If a file moved** (e.g. into a new per-entity subpackage), its `*__test.go` moved with it in the same change.

## 12. Docs

- Regenerate API docs: `make swagger-generate` (commit the result).
- Add a **behaviour section** to [FEATURES.md](FEATURES.md) (intent, invariants, endpoints, states).
- **Add a row to the [feature map](../AGENTS.md#feature-map)** in `AGENTS.md`.

## 13. Verify

```bash
make check          # MUST pass: format-check → lint → test-unit → coverage → vulncheck → deps-verify
```

Then walk the [Definition of Done](../AGENTS.md#definition-of-done).
