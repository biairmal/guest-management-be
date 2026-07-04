# Development Plan — Foundations & Domain Buildout

> **Purpose.** Canonical, agent-executable roadmap for taking `guest-management-be` from a one-feature
> skeleton to a production-ready, fully-featured service. Written so an AI agent (or human) can pick up any
> phase and implement it without re-deriving context. Read [../AGENTS.md](../AGENTS.md) first for the hard rules,
> and [PATTERNS.md](PATTERNS.md) / [NEW_FEATURE_CHECKLIST.md](NEW_FEATURE_CHECKLIST.md) for the shapes.

The plan has two tracks. **Track A (foundations) comes first** — it establishes the test discipline, removes
debt, and builds the shared building blocks that Track B features depend on. Several items also depend on
**upstream `go-sdk` phases** (`validator`, `auth`, `metrics`, `tracer`, `ratelimit`, `circuitbreaker`,
`lifecycle`) from [go-sdk/docs/DEVELOPMENT_PLAN.md](../../go-sdk/docs/DEVELOPMENT_PLAN.md) — cross-referenced below.

## Status at a glance

| # | Track | Item | Depends on | Status |
|---|---|---|---|---|
| A1 | Foundations | Remove dead code + build artefacts | — | ✅ |
| A2 | Foundations | Kill empty-`Options` ceremony; fix pass-through repository | — | ✅ |
| A3 | Foundations | Config hardening (server section, Redis decision, empty `App:`) | — | ✅ |
| A4 | Foundations | Real readiness (DB ping) | A3 | ⬜ |
| A5 | Foundations | **Testing foundation** (generated-mock setup + first table-driven tests) | A2 | ⬜ |
| A6 | Foundations | Shared building blocks in `internal/core` (base repo, list-query parser, validator) | A5, go-sdk `validator` | ⬜ |
| A7 | Foundations | Cross-cutting middleware/observability + go-sdk `lifecycle` shutdown | go-sdk phases | ⬜ |
| B1 | Domain | `tenants` | A6 | ⬜ |
| B2 | Domain | `users` | B1 | ⬜ |
| B3 | Domain | `auth` (login + route protection) | B2, go-sdk `auth` | ⬜ |
| B4 | Domain | `events` (events + workflow steps; extend existing slice) | B1, B2 | ⬜ |
| B5 | Domain | `templates` (event + message templates) | B4 | ⬜ |
| B6 | Domain | `staffing` (event staff assignments, roles/permissions) | B2, B4 | ⬜ |
| B7 | Domain | `tickets` (ticket types + tickets) | B4 | ⬜ |
| B8 | Domain | `guests` | B4, B7 | ⬜ |
| B9 | Domain | `scans` (check-in / scan logs) | B8 | ⬜ |

---

## Track A — Foundations & production-readiness

### A1. Remove dead code & build artefacts

- Delete `internal/core/domain/entity.go` (`Entity`/`AuditableEntity` — never imported; `EventCategory`
  redefines the fields inline). If a shared base entity is wanted, reintroduce it *used* under A6.
- Delete `internal/core/handler/dto.go` + `helper.go` (self-admittedly superseded; `parseLimitOffset`/`parseSort`
  unused).
- Remove the committed `api.exe` (~9.6 MB) and `out/` artefacts; add them to `.gitignore`.
- **Verify:** `make lint` clean; `go build ./...` still green; grep confirms no references to the deleted symbols.

### A2. Kill empty-`Options` ceremony; fix the pass-through repository

- Remove the empty `struct{}` options: `CategoryHandlerOptions`, `CategoryServiceOptions`,
  `CategoryRepositoryOptions`, `CategoryRouterOptions`, and the `Options` fields threaded through
  `internal/app/{repository,service,handler,router}.go` and `app.go`. Constructors take only real dependencies.
- Replace `categoryRepo` (the pass-through in `category_repository.go`) with the typed pattern from
  [PATTERNS.md](PATTERNS.md#repository--thin-typed-no-pass-through): return
  `repository.Repository[EventCategory, uuid.UUID]` directly from `NewCategoryRepository`, drop the
  `id.(string)` assertions, and have the service call `repo.GetByID(ctx, id)` with the typed UUID.
- **Verify:** events endpoints behave identically; a bad ID now surfaces as an error, not an empty lookup.

### A3. Config hardening

- Add a `Server` section to `internal/config.Config` (`Host`, `Port`, `ReadHeaderTimeout`, `ReadTimeout`,
  `WriteTimeout`, `ShutdownTimeout`) with `mapstructure` tags, `DefaultConfig()`, `Validate()`. Consume it in
  `main.go` — remove the hardcoded `"127.0.0.1:8080"` and inline timeouts.
- Remove the trailing empty `App:` key in `configs/config.yaml` (or give `app.Options` real fields).
- **Redis decision:** either add a `Redis redis.Config` (go-sdk) section, load it, and wire a client, **or** remove
  Redis from `.env.example`, README, and `docker-compose.yaml`. No config that lies.
- Follow [CONFIGURATION.md](CONFIGURATION.md). **Verify:** `make run` starts on the configured address; changing YAML changes behaviour.

### A4. Real readiness

- Replace the `httpkit.Readiness(func(_ context.Context) error { return nil })` stub in `main.go` with a real
  check that pings the DB (`db.Leader().PingContext(ctx)`), and Redis if adopted in A3. Liveness (`/health`)
  stays always-200.
- **Verify:** stop Postgres → `/ready` returns 503; start it → 200.

### A5. Testing foundation (highest priority)

Establish the generated-mock testing pattern from [TESTING.md](TESTING.md) — **no hand-written fakes**.

- **Wire go-sdk mocks into `go.mod`** (test-only): `require github.com/biairmal/go-sdk/mocks v0.0.0` +
  `replace github.com/biairmal/go-sdk/mocks => ../go-sdk/mocks`. After A2, the events repository is
  `repository.Repository[EventCategory, uuid.UUID]`, so `mockrepository.MockRepository` mocks it directly.
- **Set up app-side mock generation** for app-defined interfaces (feature services): add a nested `mocks/` module
  (own `go.mod` with `replace github.com/biairmal/guest-management-be => ../`), copy `go-sdk/scripts/mocks.mk` to
  `scripts/mocks.mk`, include it in the `Makefile`, and add `//go:generate mockgen ...` directives next to each
  service interface. `go.uber.org/mock` stays out of the main module.
- Add the first tests using generated mocks + `gomock.NewController(t)`:
  - `internal/features/events/category_service__test.go` — table-driven over a `mockrepository.MockRepository`,
    covering every sentinel→`errorz` branch and the source/tenant invariants.
  - `internal/core/query/list__test.go` — allowed/rejected sort & filter fields, size clamping,
    bad `page`/`size`, sort-direction parsing (covers the shared parser once for every feature).
- Make `make ci` **meaningful**: coverage reflects real tests, not vacuous 0%.
- **Verify:** `make mocks` regenerates cleanly; `make test-unit` runs non-trivial tests; `make coverage` shows
  real numbers for the events package.

### A6. Shared building blocks in `internal/core`

Build the reuse primitives the AGENTS.md rules assume. Document each with a package doc comment + tests.

| File | Provides |
|---|---|
| `internal/core/repository/base.go` | A thin typed constructor that composes `sql.NewSQLRepository` + `audit.NewAuditableRepository` so a feature repo is one call (`core.NewRepository[T, uuid.UUID](log, db, table, cols)`). No per-feature pass-through. |
| `internal/core/query/list.go` | ✅ Done — the generic `ListParseConfig` + `ParseListParams(url.Values, cfg)`, lifted out of the old per-feature `events/category_query.go`; features supply only their allow-lists (pagination defaults fall back to package-level defaults). |
| `internal/core/validation/validator.go` | Adapter over `go-sdk` `validator` (or `go-playground/validator` until the SDK phase lands) exposing `Struct(any) error` returning `errorz` field errors, injected into handlers. |

- Migrate the `events` slice onto these (removes the last of its bespoke plumbing) as the reference migration.
- **Depends on:** go-sdk `validator` phase (optional — can start on go-playground and swap later behind the adapter).
- **Verify:** events uses only `internal/core` helpers + its own config; `make check` green.

### A7. Cross-cutting middleware & lifecycle

As `go-sdk` phases land, adopt them here (config-first, via the middleware chain in
[go-sdk DEVELOPMENT_PLAN "Recommended middleware chain"](../../go-sdk/docs/DEVELOPMENT_PLAN.md#recommended-middleware-chain)):

- `middleware.Correlation()` (already available) — add to the chain in `main.go`.
- `metrics` → `middleware.Metrics(...)`; `tracer` → `middleware.Tracing(...)`; `ratelimit` →
  `middleware.RateLimit(...)`; wrap outbound calls with `circuitbreaker`.
- Replace the hand-rolled `startServer`/`gracefulShutdown` in `main.go` with `go-sdk` `lifecycle.Run(...)`
  (signal trap → readiness drain → ordered closers under deadline).
- **Verify:** metrics endpoint scrapes; traces appear; shutdown drains cleanly.

---

## Track B — Domain buildout

Each phase is a full vertical slice per [NEW_FEATURE_CHECKLIST.md](NEW_FEATURE_CHECKLIST.md): model → repository →
service → handler → routes → Swagger → wire in `internal/app` → **table-driven tests** → [FEATURES.md](FEATURES.md)
section → [feature-map](../AGENTS.md#feature-map) row. Migrations already exist (see [DATABASE.md](DATABASE.md)) —
each phase names its migration and tables. Ordered by data dependency.

| Phase | Slice | Migration / tables | Core endpoints |
|---|---|---|---|
| **B1** | `tenants` | `000001` — `tenants` | CRUD `/api/v1/tenants` |
| **B2** | `users` | `000003` — `users` | CRUD `/api/v1/users`; scoped by tenant |
| **B3** | `auth` | uses `000002` (`roles`,`permissions`,`role_permissions`) + `000003` | `POST /auth/login`, `POST /auth/refresh`; route protection middleware |
| **B4** | `events` (extend) | `000005` — `events`, `workflow_steps` | CRUD `/api/v1/events`; workflow-step management |
| **B5** | `templates` | `000004` (event templates), `000006` (`message_templates`) | CRUD `/api/v1/event-templates`, `/message-templates` |
| **B6** | `staffing` | `000002` (roles/permissions), `000007` (`event_staff_assignments`) | assign/list staff on an event; permission checks |
| **B7** | `tickets` | `000008` — `ticket_types` + junction | CRUD ticket types; associate to events |
| **B8** | `guests` | `000009` — `guests`, `tickets` | CRUD `/api/v1/guests`; issue tickets |
| **B9** | `scans` | `000010` — `scan_logs` | `POST /api/v1/scans` check-in; scan history |

### Phase notes

- **B1 `tenants`** — foundational; almost every other table has a `tenant_id`. Establishes the multi-tenant
  scoping convention (tenant id from auth context once B3 lands; explicit until then).
- **B2 `users`** — password hashing stays app-side unless `go-sdk` `auth` provides it; store hashes only.
- **B3 `auth`** — build on `go-sdk` `auth` (monolith-first: in-process HS256/RS256 issue + validate; config flip
  to `remote`/JWKS when split). Route protection via `httpkit/middleware/auth.go`; `user_id` flows through
  `ctxkit`. This is the seam that lets `users` later become a separate identity service.
- **B4 `events`** — the current `event_categories` slice grows into the full events feature; keep categories as a
  sub-concern. Workflow steps model the event's lifecycle stages.
- **B6 `staffing`** — introduces role/permission enforcement; wire a permission check into the middleware/service
  layer, reusing `ctxkit` user identity.
- **B7–B9 `tickets`/`guests`/`scans`** — the check-in critical path. `scans` is write-heavy and latency-sensitive;
  when it becomes a hotspot, it's the first candidate to extract into its own service (the slice boundary already
  isolates it).

### Cross-references to go-sdk

| This service needs | Provided by go-sdk phase |
|---|---|
| Boundary validation (A6) | `validator` |
| Login + route protection (B3) | `auth` |
| Metrics/tracing/rate-limit/breaker (A7) | `metrics`, `tracer`, `ratelimit`, `circuitbreaker` |
| Graceful shutdown (A7) | `lifecycle` |

When one of these is missing, prefer **adding it to `go-sdk`** (it's a reusable cross-cutting concern) over
building an app-local version.
