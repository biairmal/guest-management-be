# New feature checklist

Follow these steps in order when adding a feature vertical slice. The rules referenced here are defined in [../AGENTS.md](../AGENTS.md); templates are in [PATTERNS.md](PATTERNS.md).

## 1. Decide the boundary

- Is this a distinct part of the domain (tenants, users, events, guests, tickets…)? Give it its own slice under `internal/features/<feature>`.
- Will it need data from another feature? Depend on a **published interface**, not the other slice's internals — the slice must stay independently extractable into its own service. Shared, feature-agnostic infrastructure goes in `internal/core`, not in a slice.

## 2. Schema & migration

- The schema is already defined in `migrations/` (see [DATABASE.md](DATABASE.md)). If you need a change, `make migration-create NAME=...` and write both `.up.sql` and `.down.sql`.
- Confirm column names match the `db` tags you'll put on the model, and note the soft-delete columns (`created_at`, `updated_at`, `deleted_at`).

## 3. Model

- `internal/features/<feature>/<entity>_model.go`: struct with `db` + `json` tags, `swagger:model` marker, a `TableName()` method, and any domain constants. Template: [PATTERNS.md#model](PATTERNS.md#model).

## 4. Repository

- `<entity>_repository.go`: build `sql.NewSQLRepository[Entity, uuid.UUID]`, wrap with `audit.NewAuditableRepository`, **return the typed generic interface** (`repository.Repository[Entity, uuid.UUID]`). Keep `TID` typed.
- **Do not** write a pass-through wrapper or widen the ID to `any`. Template: [PATTERNS.md#repository--thin-typed-no-pass-through](PATTERNS.md#repository--thin-typed-no-pass-through).

## 5. Service

- `<entity>_service.go`: define the `XService` interface and its implementation. Input DTOs carry `validate:"..."` tags. Enforce **business invariants** only; translate repository sentinels to `errorz` codes; wrap the cause on internal errors; log with `*WithContext`. Template: [PATTERNS.md#service--business-rules--error-translation](PATTERNS.md#service--business-rules--error-translation).

## 6. Handler + validation

- `<entity>_handler.go`: `func(*http.Request)(any,error)` handlers. Parse with `serializer.ParseJSON`, validate the DTO at the boundary via the shared validator, call the service, return `response.OK/Created/NoContent`.
- **Add Swagger annotations** to every handler (`@Summary`, `@Param`, `@Success`, `@Failure`, `@Router`). Template: [PATTERNS.md#handler--go-sdk-adapter--swagger](PATTERNS.md#handler--go-sdk-adapter--swagger).

## 7. Routes + list query

- `<entity>_routes.go`: `InitXRoutes(r chi.Router, h *XHandler)` registering `handler.Handle(...)`.
- For list endpoints, declare a `query.ListParseConfig` with allowed sort/filter fields and call the shared `internal/core/query.ParseListParams` — do not reimplement parsing per feature. Template: [PATTERNS.md#list-query--allow-list-parsing](PATTERNS.md#list-query--allow-list-parsing).

## 8. Wire into the composition root

- In `internal/app`, add the feature to `repository.go` → `service.go` → `handler.go` → `router.go`. This is the **only** place that imports the slice for wiring. Don't add empty `Options{}` structs.

## 9. Tests

- **Table-driven `*__test.go`** for the service (against a **generated `gomock` mock** repository — `mockrepository.MockRepository` for a go-sdk repo interface, or an app-generated mock). The shared `internal/core/query` parser has its own tests; a feature only needs to cover its `ListParseConfig` if it adds custom behaviour. Cover error-translation branches (not-found → 404, conflict → 409, invalid → 422) and validation. If you added a new app interface, add a `//go:generate` directive and run `make mocks`. Live-DB tests go in `*_integration_test.go` guarded by `testing.Short()`. See [TESTING.md](TESTING.md).

## 10. Docs

- Regenerate API docs: `make swagger-generate` (commit the result).
- Add a **behaviour section** to [FEATURES.md](FEATURES.md) (intent, invariants, endpoints, states).
- **Add a row to the [feature map](../AGENTS.md#feature-map)** in `AGENTS.md`.

## 11. Verify

```bash
make check          # MUST pass: format-check → lint → test-unit → coverage → vulncheck → deps-verify
```

Then walk the [Definition of Done](../AGENTS.md#definition-of-done).
