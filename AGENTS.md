# AGENTS.md

**Canonical development guidelines for `github.com/biairmal/guest-management-be`.**

This is the single source of truth for how code in this microservice must be written — by humans and by AI assistants alike. It is read automatically by AGENTS.md-aware tools (Cursor, Zed, Aider, GitHub Copilot coding agent, Claude Code via [CLAUDE.md](./CLAUDE.md)). If you are an AI agent: **read this file before making any change**, and treat the [Authoring rules](#authoring-rules) and [Definition of Done](#definition-of-done) as hard requirements.

For deeper reference: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) · [docs/PATTERNS.md](docs/PATTERNS.md) · [docs/NEW_FEATURE_CHECKLIST.md](docs/NEW_FEATURE_CHECKLIST.md) · [docs/TESTING.md](docs/TESTING.md) · [docs/CONFIGURATION.md](docs/CONFIGURATION.md) · [docs/FEATURES.md](docs/FEATURES.md) · [docs/DEVELOPMENT_PLAN.md](docs/DEVELOPMENT_PLAN.md) · human onboarding in [CONTRIBUTING.md](CONTRIBUTING.md).

> This service consumes the sibling **`go-sdk`** (`github.com/biairmal/go-sdk`) via a local `replace => ../go-sdk` directive. `go-sdk` is a building-blocks library with its own [AGENTS.md](../go-sdk/AGENTS.md); when a capability belongs in the SDK, add it there, not here. Reuse before you build.

---

## Orientation

A production-oriented guest-management HTTP API (module `github.com/biairmal/guest-management-be`, Go 1.25.1). It is a **layered, feature-sliced** service built on top of `go-sdk`. The design goal is **low coupling**: each feature is a self-contained vertical slice so that features (e.g. users vs. events) can later be extracted into independent microservices with minimal churn.

### Layer map

Dependencies point **inward/downward only** (see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)). Higher layers may import lower ones; never the reverse.

| Layer | Path | Role |
|---|---|---|
| Entry point | `cmd/api` | `main.go` — config load, dependency construction, server lifecycle. No business logic. |
| Composition root | `internal/app` | Hand-rolled DI: wires repositories → services → handlers → routes. The only place that knows every feature. |
| App config | `internal/config` | Root `Config` struct embedding `go-sdk` configs + app-specific config. |
| Shared building blocks | `internal/core` | Cross-feature infrastructure reused by every slice (audit repository decorator; future: base repository helper, list-query parser, validator). **No feature-specific code here.** |
| Feature slices | `internal/features/<feature>` | Vertical slices: model, repository, service, handler, routes, query, tests. Self-contained; a slice is a future service boundary. |
| Generated API | `api/swagger` | Swagger/OpenAPI generated from handler annotations (`make swagger-generate`). Do not hand-edit. |

### Feature map

One row per feature slice. **When you add a feature, add a row here** (see [Authoring rules](#authoring-rules)).

| Feature | Path | Endpoints | Behaviour doc |
|---|---|---|---|
| `events` | `internal/features/events` | `/api/v1/event-categories`, `/api/v1/events`, `/api/v1/events/{event_id}/workflow-steps`, `/api/v1/event-categories/{category_id}/workflow-step-templates` (CRUD + list) | [docs/FEATURES.md#events](docs/FEATURES.md#events) |
| `tenants` | `internal/features/tenants` | `/api/v1/tenants` (CRUD + list) | [docs/FEATURES.md#tenants](docs/FEATURES.md#tenants) |
| `users` | `internal/features/users` | `/api/v1/users` (CRUD + list, `POST /{id}/password`, `POST /{id}/transfer-master`, all gated on `manage_users`); `POST /api/v1/users/me/password` (self-service, outside the `manage_users` group — target id from the JWT, valid token only) | [docs/FEATURES.md#users](docs/FEATURES.md#users) |
| `auth` | `internal/features/auth` | `/api/v1/auth/login`, `/api/v1/auth/refresh` | [docs/FEATURES.md#auth](docs/FEATURES.md#auth) |
| `templates` | `internal/features/templates` | `/api/v1/message-templates` (CRUD + list) | [docs/FEATURES.md#templates](docs/FEATURES.md#templates) |
| `roles` | `internal/features/roles` | *internal-only, no HTTP endpoints this phase* — model + repository consumed by `users`/`staffing`/`internal/core/authz` | [docs/FEATURES.md#roles](docs/FEATURES.md#roles) |
| `staffing` | `internal/features/staffing` | `/api/v1/events/{event_id}/staff` (CRUD + list, all gated on the `manage_staff` permission) | [docs/FEATURES.md#staffing](docs/FEATURES.md#staffing) |
| `tickets` | `internal/features/tickets` | `/api/v1/events/{event_id}/ticket-types` (CRUD + list + `PUT /{id}/workflow-steps`, all gated on the `manage_events` permission) | [docs/FEATURES.md#tickets](docs/FEATURES.md#tickets) |
| `guests` | `internal/features/guests` | `/api/v1/events/{event_id}/guests` (CRUD + list + `POST /{id}/invitation`, gated on `manage_guests`); public `POST /api/v1/guests/rsvp/{token}` | [docs/FEATURES.md#guests](docs/FEATURES.md#guests) |
| `scans` | `internal/features/scans` | `/api/v1/events/{event_id}/scans` (record a scan + list scan history, both gated on the `check_in` permission) | [docs/FEATURES.md#scans](docs/FEATURES.md#scans) |

> 18 migrations define ~16 tables (tenants, users, roles/permissions, events, guests, tickets, scans, templates) plus role scope/seed data and follow-up columns (`events.rsvp_required`; `guests.ticket_type_id`/`invitation_token`/`email_hash`/`phone_hash`; `users.must_change_password`); `event_categories`/`events`/`workflow_steps`/`workflow_step_templates`, `tenants`, `users`, `auth`, `message_templates`, `roles` (model+repo only), `staffing`, `tickets` (ticket types only), `guests` (guests + the `Ticket` QR artifact, model+repo only), and `scans` (`ScanLog`) have code. The build order is specified in [docs/DEVELOPMENT_PLAN.md](docs/DEVELOPMENT_PLAN.md).

---

## Commands

```bash
# First-time setup — installs gofumpt, golangci-lint, govulncheck, delve
make install-tools

# CI gate — format-check → lint → test-unit → coverage → vulncheck → deps-verify (fail-fast).
# MUST pass before any change is considered complete.
make check               # alias: make ci

# Run / debug
make run                 # go run the API (development)
make debug               # run under Delve (headless)

# Tests
make test-unit           # go test -short ./...
make test-integration    # go test ./...   (includes live-service integration tests)
make test-race           # go test -race -short ./...
make coverage            # → out/coverage.{out,html}
make coverage-view

# Single package / single test
go test -short ./internal/features/events/...
go test -run TestName ./internal/features/events/...

# Format, lint, vulnerabilities
make format              # gofumpt
make lint-fix            # golangci-lint --fix
make vulncheck           # govulncheck ./...

# API docs
make swagger-generate    # regenerate api/swagger from handler annotations

# Mocks (gomock) — regenerate after changing any mocked interface
make mocks               # go generate → ./mocks/ (same module; go.uber.org/mock)

# Migrations (DATABASE_URL from .env)
make migration-create NAME=create_x_table
make migration-up
make migration-down

# Discover everything
make help
```

---

## Authoring rules

These are **MUST**-level unless stated otherwise. They are derived from the existing conventions in this codebase and its `go-sdk` dependency — see [docs/PATTERNS.md](docs/PATTERNS.md) for the canonical templates.

### Architecture & layering

- **Dependency direction points inward only:** `handler → service → repository → go-sdk infra`. A handler MUST NOT touch the database; a service MUST NOT touch HTTP (`http.Request`, status codes, `chi`). The service layer speaks in domain types and `context.Context`, never transport types.
- **Each feature is a self-contained vertical slice** under `internal/features/<feature>`. Treat the slice boundary as a **future service boundary**: no feature may import another feature's internals. Cross-feature reuse goes through `internal/core` (shared building blocks) or a small published interface — never a direct reach into a sibling slice.
- **A feature slice with more than one distinct entity splits into per-entity subpackages**: `internal/features/<feature>/<entity>/` (e.g. `events/category`, `events/event`, `events/workflowstep`, `events/workflowsteptemplate`), each a flat, one-file-per-concern package per [Feature slice layout](docs/PATTERNS.md#feature-slice-layout) — split by entity, never by layer (no `service`/`handler`/`repository` subpackages within a slice; that forces internals to be exported across a boundary that doesn't need to exist and invites import cycles). Package names drop underscores per Go convention (`workflowstep`, not `workflow_step`) even though files inside keep `workflow_step_*.go` naming. A single-entity feature (`staffing`, `tenants`, `users`, `auth`) stays flat — don't split what isn't messy. `<feature>/config.go` stays at the feature root, aggregating every entity's `RepositoryConfig`.
- **`internal/core` is for cross-feature infrastructure only.** Never put feature-specific types there.
- **`internal/app` is the only layer that knows all features.** Wiring lives there; features do not wire themselves into the router from inside the slice beyond exposing an `InitXRoutes(...)` function.

### go-sdk first

- **No third-party package inside the app unless `go-sdk` genuinely lacks the capability.** Prefer `go-sdk`'s `errorz`, `logger`, `httpkit`, `sqlkit`, `repository` (+ `repository/sql`), `common/dto`, `config`, `ctxkit`, `serializer`. `chi` (routing), `google/uuid`, and `lib/pq` (driver) are the accepted exceptions already in use.
- If a capability is missing, first consider **adding it to `go-sdk`** (it has a [DEVELOPMENT_PLAN.md](../go-sdk/docs/DEVELOPMENT_PLAN.md) for exactly these cross-cutting concerns: `validator`, `tracer`, `auth`, `metrics`, `ratelimit`, `circuitbreaker`, `lifecycle`). Only add a new direct third-party dependency to this repo when the concern is genuinely app-specific, and record the justification in the PR.

### Errors — `errorz` everywhere

- **`errorz` is the ecosystem-wide structured error type.** Use it across every layer, not just the HTTP edge. `httpkit` maps `errorz` codes to HTTP status at the boundary; the codes are a transport-agnostic taxonomy.
- **Services translate repository sentinels into `errorz` codes.** Compare with `errors.Is(err, repository.ErrNotFound)` / `ErrAlreadyExists` / `ErrInvalidEntity` — **never** type-assert. Map to `errorz.NotFound()`, `errorz.Conflict()`, `errorz.UnprocessableEntity()`, etc.
- **Wrap the underlying cause** when returning an internal error: `errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("...")` so the chain is preserved.
- **Declare `error` in signatures — never the concrete `*errorz.Error`** (typed-nil trap). Construct `errorz` values inside implementations and return them as `error`.

### Repository pattern & anti-duplication

- Persistence uses `go-sdk`'s generic `repository.Repository[TEntity, TID]` + `repository/sql.NewSQLRepository`, wrapped by the `internal/core/audit` decorator for soft-delete/audit fields.
- **A per-feature repository MUST NOT be a hand-written pass-through** that re-widens a typed `TID` back to `any` or swallows type assertions (`idStr, _ := id.(string)` is banned — it turns a bad ID into a silent empty-string lookup). Use the typed generic repository directly, or the shared base helper once it exists (see [docs/DEVELOPMENT_PLAN.md](docs/DEVELOPMENT_PLAN.md)). If you write a repository interface, keep the ID **typed** (`uuid.UUID` / `string`), not `any`.
- **Filter/sort/pagination parsing MUST reuse the shared allow-list parser** in `internal/core/query` (`ListParseConfig` + `ParseListParams`), not be copy-pasted per feature. `ParseListParams` is an HTTP-only adapter (it takes `url.Values`) used by HTTP handlers to fail fast on a bad query string; pagination defaults fall back to the package-level defaults when left zero. See `events/category_handler.go` for the shape.
- **The allow-list is enforced again in the service, transport-agnostically, via `query.ValidateListParams(params, cfg)`** — the service, not the handler, is the actual authority: every transport (HTTP, and eventually gRPC/a subscriber) ends up with a `*query.ListParams` before calling the service, but only HTTP goes through `ParseListParams`. A handler that builds `ListParams` some other way must not be able to skip the allow-list. `ListParseConfig` (the allow-list itself) is declared once, exported from the `<entity>_service.go` file (e.g. `CategoryListConfig`); the HTTP handler imports that same value for its `ParseListParams` call rather than declaring its own copy. Running the same check at both the HTTP edge (fast client feedback) and the service (guaranteed enforcement regardless of caller) is deliberate defense-in-depth, not duplication to clean up.
- **Converting `*query.ListParams` to `*repository.ListOptions` MUST reuse `query.ToListOptions`** in `internal/core/query` — never hand-roll a per-feature `listParamsToListOptions`. The conversion (page/size clamping, filter conditions, sort-direction mapping) is identical across every feature; it drifted into 5+ verbatim copies before this rule existed.

### Request validation

- **Validate request payloads at the HTTP boundary** via the shared validator (`go-sdk` `validator` when it lands, or `go-playground/validator` meanwhile), driven by `validate:"..."` struct tags on the input DTO. Validation failures become clean `400`s with per-field detail.
- **Do not scatter hand-written `if in.X == ""` checks in the service** for shape/format validation. The service layer owns **business-rule invariants** (e.g. "tenant_id required when source is tenant"), not field-presence checks.
- **Decode request bodies via stdlib `json.NewDecoder(r.Body).Decode(&body)`.** go-sdk's `serializer.ParseJSON(data []byte, v any)` takes an in-memory `[]byte` (its real use is decoding an already-fetched value, e.g. a cache read in `repository/cache`) — it is not an HTTP-body decoder, and passing `r.Body` (`io.ReadCloser`) to it doesn't compile. Don't reach for it here; `json.NewDecoder` already streams the body without an extra buffering copy.
- **Input/output DTOs live in `<entity>_dto.go`**, not inline in `<entity>_service.go`. The service file holds only the `XService` interface and its implementation — struct definitions (`CreateInput`, `UpdateInput`, `SyncXInput`, etc.) belong in a dedicated DTO file so the service file is pure logic.

### API shape & options

- Constructors are named **`NewX`**. Data/I/O functions take **`context.Context` as the first parameter**.
- **Only introduce a per-layer `Options`/`Config` struct when it carries a real field.** Empty `struct{}` options threaded through layers are **banned** — add the struct when there is something to configure, not before.
- Prefer **interface before implementation** for services and repositories (return the interface from the constructor) so layers are testable with generated mocks.

### Constants

- **Each feature slice owns a single `<feature>_constants.go`** (or `<entity>_constants.go` inside a per-entity subpackage) for its exported constants — permission codes, enum-like string values (`SourceApp`/`SourceTenant`), etc. Don't scatter `const` blocks across model/service files; once a feature has more than one or two constants, give them one place to scan.
- **Permission codes stay feature-owned, not centralized in `internal/core/authz`.** A feature slice is the authority on what it gates; `internal/core/authz` only provides the generic `PermissionResolver`/`Checker`/`RequirePermission` mechanism, never the codes themselves.

### Configuration

- Configuration is **struct-first, YAML-first**. The root [`internal/config.Config`](internal/config/config.go) **embeds `go-sdk` config structs** (`logger.Options`, `sqlkit.Config`, …) and app-specific sections, loaded from `configs/config.yaml` + `.env` via `go-sdk` `config.Load` with `${VAR:default}` substitution.
- **All runtime knobs live in config** — server host, port, and timeouts MUST come from `Config`, never be hardcoded in `main.go`. New config sections follow the embed pattern in [docs/CONFIGURATION.md](docs/CONFIGURATION.md); serializable fields carry `mapstructure` tags, non-serializable ones `mapstructure:"-"`.

### Observability

- Logging is **structured and context-aware** via `go-sdk` `logger` + `ctxkit`: use `InfoWithContext`/`ErrorWithContext` with `logger.F(...)` fields so `request_id`/`correlation_id` surface automatically. Never use stdlib `log`. Libraries return errors; the app logs at the layer that decides an error is noteworthy (typically the service on the internal-error path).
- **Readiness MUST perform real dependency checks** (DB ping, and Redis if adopted) via `httpkit.Readiness` — a `return nil` stub is not acceptable for production.

### Tests

- **Unit tests use stdlib `testing` + generated `gomock` mocks** — no testify, no hand-written fakes. Assertions are hand-written `if got != want { t.Errorf(...) }`; collaborators are stubbed with generated mocks.
- **Mock collaborators with generated mocks, never manual fakes.** A hand-rolled fake silently rots — adding a method to an interface breaks every fake at once, and stale fakes hide gaps. For **`go-sdk` interfaces** (`repository.Repository[T,ID]`, `logger.Logger`, `redis.Client`) consume the generated mocks from the **`github.com/biairmal/go-sdk/mocks`** module (`mockrepository`, `mocklogger`, `mockredis`). For **app-defined interfaces** (feature services, etc.) generate mocks the same way — `//go:generate` + `mockgen` into a nested `mocks` module + `make mocks` — mirroring `go-sdk`'s setup ([scripts/mocks.mk](../go-sdk/scripts/mocks.mk)). Do **not** hand-write a fake; add a mockgen directive and regenerate. (A real no-op like `logger.NewNoOp()` is fine where there's nothing to stub.)
- **Table-driven**, same-package (`package events`, not `events_test`), file named `*__test.go` (double underscore).
- **When a file moves — e.g. into a new per-entity subpackage — its `*__test.go` moves with it in the same change.** A test file left behind in the old location is an orphaned test, not a passing one.
- Integration tests that need a live DB go in **`*_integration_test.go`** and skip under `-short` (`if testing.Short() { t.Skip(...) }`); run via `make test-integration`.
- **New behaviour ships with tests in the same change**; regenerate mocks (`make mocks`) when you change a mocked interface. See [docs/TESTING.md](docs/TESTING.md).

### Documentation

- Every exported type and function gets a **doc comment**.
- **Every HTTP endpoint MUST carry Swagger annotations** (`@Summary`, `@Param`, `@Success`, `@Failure`, `@Router`); run `make swagger-generate` and commit the result.
- **Every feature gets a behaviour section** in [docs/FEATURES.md](docs/FEATURES.md) (intent, invariants, endpoints, states).
- **Add a row to the [Feature map](#feature-map)** (and adjust the [Layer map](#layer-map) if you add a layer) when you add a slice.

### Linter compliance (enforced by [`.golangci.yml`](.golangci.yml))

- Line length **≤ 120** (`lll`); function **≤ 100 lines / 50 statements** (`funlen`); cyclomatic complexity **< 15** (`gocyclo`); cognitive complexity **< 25** (`gocognit`).
- **Always handle type-assertion failures** (`errcheck.check-type-assertions`) and blank assignments. No swallowed `_` on assertions that can fail meaningfully.
- File names are `lower_with_underscores.go`; `gofumpt` + `goimports` formatting is mandatory (`make format`).
- Test files are exempt from several linters; `.gen.go` / generated Swagger is excluded.

---

## Definition of Done

A change is **not complete** until every box is checked:

- [ ] `make check` passes (format-check, lint, unit tests, coverage, vulncheck, deps-verify).
- [ ] New/changed behaviour has **table-driven `*__test.go` tests** using **generated `gomock` mocks** (go-sdk `mocks` module for SDK interfaces; regenerated app mocks for app interfaces) — **no hand-written fakes**; live-DB paths covered by `*_integration_test.go`.
- [ ] If a mocked interface changed, `make mocks` was rerun and the regenerated mocks are committed.
- [ ] Errors use `errorz` with an appropriate code and **wrap the underlying cause**; sentinels compared via `errors.Is`; signatures return `error`, not `*errorz.Error`.
- [ ] Handlers don't touch the DB; services don't touch HTTP; no cross-feature imports.
- [ ] Request shape validated at the boundary via the shared validator; only business invariants live in the service.
- [ ] Request bodies decoded via `serializer.ParseJSON`, not `encoding/json` directly.
- [ ] List endpoints convert `*query.ListParams` via the shared `query.ToListOptions` — no per-feature reimplementation.
- [ ] The list allow-list (`ListParseConfig`) is declared once in the service and enforced there via `query.ValidateListParams`, not only in the handler.
- [ ] DTOs live in `<entity>_dto.go`; constants in `<feature>_constants.go`; a multi-entity feature is split into per-entity subpackages.
- [ ] No new third-party dependency unless `go-sdk` truly lacks it (justification recorded); **no empty `Options{}` structs**; **no pass-through repository / swallowed ID assertion**.
- [ ] Any configuration is in the `Config` tree (`mapstructure`-tagged, `go-sdk` configs embedded); **no hardcoded runtime values** in `main.go`.
- [ ] New endpoints have **Swagger annotations** and `make swagger-generate` was run; exported symbols have doc comments.
- [ ] Behaviour documented in [docs/FEATURES.md](docs/FEATURES.md); **[Feature map](#feature-map) row exists**.
- [ ] Readiness/observability wired for any new external dependency (real health check, not a stub).

---

## Deeper reference

| Document | What it covers |
|---|---|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Why the service is layered/feature-sliced; dependency direction; errorz at every layer; the go-sdk boundary |
| [docs/PATTERNS.md](docs/PATTERNS.md) | Copy-paste templates: feature slice layout, thin repository, service error-translation, handler adapter, table-driven tests, list-query parsing |
| [docs/NEW_FEATURE_CHECKLIST.md](docs/NEW_FEATURE_CHECKLIST.md) | Step-by-step for adding a feature vertical slice |
| [docs/TESTING.md](docs/TESTING.md) | Stdlib table-driven convention, generated `gomock` mocks (go-sdk + app), unit vs integration, coverage |
| [docs/CONFIGURATION.md](docs/CONFIGURATION.md) | Embed-go-sdk-Config pattern, YAML + `.env` + `${VAR:default}`, adding a config section |
| [docs/FEATURES.md](docs/FEATURES.md) | Per-feature behaviour docs (intent, invariants, endpoints, states) |
| [docs/DEVELOPMENT_PLAN.md](docs/DEVELOPMENT_PLAN.md) | Roadmap: foundations/production-readiness + phased domain buildout |
| [docs/DATABASE.md](docs/DATABASE.md) | Full schema, ER diagram, soft-delete policy, migration order |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Human setup, dev loop, PR expectations |
