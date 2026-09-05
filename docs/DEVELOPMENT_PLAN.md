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
| A4 | Foundations | Real readiness (DB ping) | A3 | ✅ |
| A5 | Foundations | **Testing foundation** (generated-mock setup + first table-driven tests) | A2 | ✅ |
| A6 | Foundations | Shared building blocks in `internal/core` (base repo, list-query parser, validator) | A5, go-sdk `validator` | ✅ |
| A7 | Foundations | Cross-cutting middleware/observability + go-sdk `lifecycle` shutdown | go-sdk phases | ✅ |
| A8 | Foundations | Post-B6 cleanup: shared list helpers, decode/DTO/constants hygiene, `events` entity split | A6 | ⬜ |
| B1 | Domain | `tenants` | A6 | ✅ |
| B2 | Domain | `users` | B1 | ✅ |
| B3 | Domain | `auth` (login + route protection) | B2, go-sdk `auth` | ✅ |
| B4 | Domain | `events` (events + workflow steps; extend existing slice) | B1, B2 | ✅ |
| B5 | Domain | `templates` (event + message templates) | B4 | ✅ |
| B6 | Domain | `staffing` (event staff assignments, roles/permissions) | B2, B4 | ✅ |
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
| `internal/core/repository/base.go` | ✅ Done — a thin typed constructor that composes `sql.NewSQLRepository` + `audit.NewAuditableRepository` + an optional `repository/cache` decorator so a feature repo is one call (`core.NewRepository[T, uuid.UUID](log, db, table, cols, cacheOpts)`). Also provides `CacheConfig` (YAML shape + `Validate`/`ResolveStrategy`/`ToOptions`), which each feature embeds per repository it wants configurable caching for (on by default) — see `events.Config`. No per-feature pass-through. |
| `internal/core/query/list.go` | ✅ Done — the generic `ListParseConfig` + `ParseListParams(url.Values, cfg)`, lifted out of the old per-feature `events/category_query.go`; features supply only their allow-lists (pagination defaults fall back to package-level defaults). |
| `internal/core/validation/validator.go` | Adapter over `go-sdk` `validator` (or `go-playground/validator` until the SDK phase lands) exposing `Struct(any) error` returning `errorz` field errors, injected into handlers. |

- Migrate the `events` slice onto these (removes the last of its bespoke plumbing) as the reference migration.
- **Depends on:** go-sdk `validator` phase (optional — can start on go-playground and swap later behind the adapter).
- **Verify:** events uses only `internal/core` helpers + its own config; `make check` green.

### A7. Cross-cutting middleware & lifecycle ✅

Adopted the `go-sdk` middleware chain from
[go-sdk DEVELOPMENT_PLAN "Recommended middleware chain"](../../go-sdk/docs/DEVELOPMENT_PLAN.md#recommended-middleware-chain)
in `main.go` (split into `buildDependencies`/`buildRouter`/`runLifecycle` helpers to keep `main` under the
cognitive-complexity lint cap):

- Chain order: `middleware.Metrics(rec, nil)` (outermost, counts panics) → `Recover` → `RequestID` →
  `Correlation` → `Tracing(tr)` → `Logging` → `RateLimit(limiter, KeyByIP)`. No `Auth` yet — waits on B3.
- `MetricsConfig`/`RateLimitConfig` (`internal/config/metrics.go` / `ratelimit.go`) wrap the go-sdk `Config`
  with an app-level `Enabled` switch, mirroring `TracingConfig`: disabled → `metrics.NewNoOp()` /
  nil `ratelimit.Limiter` (the `RateLimit` middleware treats nil as pass-through) and no `/metrics` route.
  `Lifecycle lifecycle.Config` embeds directly (shutdown is unconditional, no switch).
- `/metrics` (via `promhttp.Handler()`, only mounted when `cfg.Metrics.Enabled`), `/ready` now also honors an
  `atomic.Bool` readiness flag that `lifecycle.Run` flips to `false` on the first shutdown signal, ahead of the
  DB/Redis ping checks.
- Replaced the hand-rolled `startServer`/`gracefulShutdown` with `lifecycle.Run(ctx, server, cfg.Lifecycle,
  WithReadiness, WithLogger, WithCloser("tracer"/"redis"/"db"))` — registration order matches the go-sdk
  README's least-recoverable-first guidance (tracer flush, then redis, then db last).
- **Circuit breaker not wired**: this service makes no outbound calls today (grepped for `httpkit/client`/
  `http.Client` — none), so there is nothing to wrap. Add `circuitbreaker.Do[T]` around the first outbound
  dependency call when one is introduced (e.g. a future remote `auth` mode in B3).
- New config: `configs/config.yaml` `metrics:`/`ratelimit:`/`lifecycle:` blocks, `.env.example`
  `METRICS_*`/`RATELIMIT_*` vars. Tests: `internal/config/metrics__test.go`, `ratelimit__test.go`.

### A8. Post-B6 cleanup: shared list helpers, decode/DTO/constants hygiene, `events` entity split

Debt identified by a full-repo pattern review after B1–B6 shipped (`events`/`staffing` used as the worked examples).
The new rules are already live in [AGENTS.md](../AGENTS.md) / [PATTERNS.md](PATTERNS.md) / [NEW_FEATURE_CHECKLIST.md](NEW_FEATURE_CHECKLIST.md#7-service) —
this item is bringing existing code into line with them. Suggested order (each step's diff stays reviewable on its own):

1. **`query.ToListOptions`** — add to `internal/core/query`, converting `*query.ListParams` to `*repository.ListOptions`
   (page/size clamp, filter conditions, sort-direction mapping). Delete the verbatim `listParamsToListOptions` copies —
   confirmed in `events/category_service.go`, `staffing/assignment_service.go`, `users/user_service.go`,
   `templates/message_template_service.go`, `tenants/tenant_service.go` (grep `func listParamsToListOptions` for the
   final count — `events/event_service.go`/`workflow_step_service.go`/`workflow_step_template_service.go` weren't
   confirmed and may have their own copies too). Every call site switches to `query.ToListOptions(params)`.
2. **`query.ValidateListParams`** — add alongside it: the same allow-list check as `ParseListParams`, but against an
   already-built `*ListParams` so it works regardless of transport. Move each feature's `ListParseConfig` var from the
   handler to the service file (exported, e.g. `CategoryListConfig`), call `query.ValidateListParams` first thing in
   each service `List` method, and have the handler's `ParseListParams` call reuse that same exported value instead of
   declaring its own copy. See [PATTERNS.md#list-query--allow-list-parsing-and-enforcement](PATTERNS.md#list-query--allow-list-parsing-and-enforcement).
3. **`serializer.ParseJSON` swap** — replace every `json.NewDecoder(r.Body).Decode(&body)` with
   `serializer.ParseJSON(r.Body, &body)` (go-sdk). Confirmed in 15 spots across `events`, `staffing`, `users`,
   `tenants`, `templates`, `auth` handlers — grep `json.NewDecoder` for the full list.
4. **DTO split** — move `CreateInput`/`UpdateInput`/etc. out of every `<entity>_service.go` into a new
   `<entity>_dto.go`; the service file keeps only the `XService` interface + implementation.
5. **Constants file** — `staffing_constants.go` for `PermissionManageStaff` (currently inline at the top of
   `assignment_service.go`); an equivalent `<feature>_constants.go` for any other feature with inline domain constants
   (e.g. `events`' `SourceApp`/`SourceTenant`).
6. **`staffing` repository narrowing** — `staffAssignmentServiceImpl`'s `eventRepo`/`userRepo`/`roleRepo` fields go
   from `repository.Repository[T, uuid.UUID]` to `repository.ReadRepository[T, uuid.UUID]` (go-sdk, already exists) —
   `assignment_service.go` only ever calls `GetByID` on them. Re-run `make mocks`.
7. **`events` entity split (do last)** — once 1–6 land, split the now-cleaner `events` package into per-entity
   subpackages: `events/category`, `events/event`, `events/workflowstep`, `events/workflowsteptemplate` (package
   names drop underscores; files keep `workflow_step_*.go` naming). `events/config.go` stays at the feature root.
   Every moved `*__test.go` moves with its file in the same commit. See
   [PATTERNS.md#multi-entity-features-split-by-entity-not-by-layer](PATTERNS.md#multi-entity-features-split-by-entity-not-by-layer).

**Explicitly out of scope** (discussed and declined this pass — don't reopen without a new reason):
permission-code centralization in `internal/core/authz` (staying feature-owned); a published-interface/adapter layer
decoupling `staffing` from `events`/`users`/`roles` (no slice besides `scans` is an actual extraction candidate per
this roadmap — `ReadRepository` narrowing in step 6 is the right-sized fix).

- **Verify:** `make check` green; `make swagger-generate` re-run if any DTO moved package; `make mocks` re-run for
  every mocked interface touched (staffing's narrowed repos, any new/moved service interface after the entity split).

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
| **B5** | `templates` | `000004` (event templates), `000006` (`message_templates`) | CRUD `/api/v1/event-categories/{category_id}/workflow-step-templates`, `/api/v1/message-templates` |
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
- **B5 `templates`** — two independent pieces. `workflow_step_templates` (migration `000004`) completes the
  `events` slice — it already had model + repository from B4 (needed by `EventService.Create`'s default-step
  copy); B5 adds service/handler/routes, nested under `event-categories/{category_id}` the same way workflow
  steps nest under `events/{event_id}`. `message_templates` (migration `000006`) is a new `internal/features/templates`
  slice — app/tenant/event scoped email/WhatsApp bodies, with no dependency on the `events` package (kept
  separate rather than folded into `events` since it scopes to tenants/events by ID reference, not ownership,
  mirroring `event_categories`' own source/tenant_id discriminator).
- **B6 `staffing`** — introduces role/permission enforcement. `internal/features/roles` ships model + repository
  only (no HTTP surface — mirrors `workflow_step_templates` in B4/B5); `internal/core/authz` is the reusable
  `Checker`/`RequirePermission` building block later phases (B7–B9) will reuse with their own permission codes.
  `roles.scope` (`system`/`event`) and the starter role/permission catalog are seeded by migrations
  000013–000014 (see [DATABASE.md](DATABASE.md) §6); `event_staff_assignments`' unique constraint is fixed to be
  active-only by migration 000015. `auth`'s issued tokens gain a `role_id` claim (read fresh on every
  `Refresh`, so a role change takes effect promptly); `users` gains system-scope role validation. See
  [docs/STAFFING_RBAC.md](STAFFING_RBAC.md) for the full requirement and [FEATURES.md](FEATURES.md#staffing) for
  the shipped behaviour.
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
