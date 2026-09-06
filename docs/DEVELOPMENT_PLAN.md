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
| A8 | Foundations | Post-B6 cleanup: shared list helpers, decode/DTO/constants hygiene, `events` entity split | A6 | ✅ |
| A9 | Foundations | Event-scoped permission resolution in `authz.Checker` (gap found during B9 design) | B6 | ⬜ (not started — future work, see phase notes) |
| B1 | Domain | `tenants` | A6 | ✅ |
| B2 | Domain | `users` | B1 | ✅ |
| B3 | Domain | `auth` (login + route protection) | B2, go-sdk `auth` | ✅ |
| B4 | Domain | `events` (events + workflow steps; extend existing slice) | B1, B2 | ✅ |
| B5 | Domain | `templates` (event + message templates) | B4 | ✅ |
| B6 | Domain | `staffing` (event staff assignments, roles/permissions) | B2, B4 | ✅ |
| B7 | Domain | `tickets` (ticket types + tickets) | B4 | ✅ |
| B8 | Domain | `guests` | B4, B7 | ✅ |
| B9 | Domain | `scans` (check-in / scan logs) | B8 | ✅ |
| B10 | Domain | `post-event-comms` (thank-you message + documentation links, sent after event completion) | B8, B9 | ⬜ (not started — future work, see phase notes) |

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
3. ~~`serializer.ParseJSON` swap~~ — **reversed during implementation.** go-sdk's `serializer.ParseJSON(data []byte, v any)`
   takes `[]byte`, not `io.Reader`; `serializer.ParseJSON(r.Body, &body)` doesn't compile (`r.Body` is
   `io.ReadCloser`). Its real use is decoding an already-fetched value (see `repository/cache/decorator.go`'s cache
   reads), not an HTTP body. `json.NewDecoder(r.Body).Decode(&body)` — what every handler already does — is correct
   as-is: it streams the body without `ParseJSON`'s required full-body buffering. `AGENTS.md`/`PATTERNS.md`/
   `NEW_FEATURE_CHECKLIST.md` corrected to stop prescribing the swap. No handler code changed.
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

**Shipped** — all 7 steps landed (3 reversed as documented above), `make check` green (format, lint, test-unit,
coverage, deps-verify). Two things surfaced only during implementation, not anticipated in the plan above:

- **Stutter naming.** Once split, `category.CategoryService`/`category.CategoryHandler` (and the `event`/`workflowstep`/
  `workflowsteptemplate` equivalents) tripped `revive`'s stutter check. Renamed to `category.Service`/`category.Handler`
  per entity package (constructors `NewService`/`NewHandler`); model types (`EventCategory`, `Event`, `WorkflowStep`,
  `WorkflowStepTemplate`) don't stutter and were left as-is. `PATTERNS.md`'s multi-entity example should read `Service`/
  `Handler`, not `CategoryService`/`CategoryHandler`, next time it's touched.
- **Mock package collision.** All 4 entities' `Service` interfaces previously mocked into one shared `mocks/events`
  package (`mockevents`) — post-rename that's 4 distinct `Service` interfaces all generating a `MockService` in the same
  package, a redeclaration. Split mock destinations to mirror the source split: `mocks/events/category` (`mockcategory`),
  `mocks/events/event` (`mockevent`), `mocks/events/workflowstep` (`mockworkflowstep`),
  `mocks/events/workflowsteptemplate` (`mockworkflowsteptemplate`). Nothing currently consumes these mocks (no feature
  mocks another feature's service), so this was a build-fix with no behavioral impact.

Also: `staffing`'s `events.Event` reference became `event.Event` after the split; the local variable named `event` in
`assignment_service.go` (`event, err := s.eventRepo.GetByID(...)`) shadows the package identifier within its function
body, but compiles clean because neither function references the `event` package again after the local declaration —
flagged as a risk beforehand, turned out to be a non-issue, no rename needed.

### A9. Event-scoped permission resolution in `authz.Checker`

Gap identified during B9 (`scans`) design, not by this item itself: `internal/core/authz.Checker.Require` only
resolves a caller's **system-level** `role_id` claim — nothing checks a per-event `event_staff_assignments` row's
own `role_id`. B6 seeded event-scoped roles (Usher, Photobooth Staff) specifically so a user could hold
scanning-only access on one event without a matching system role, but no gated feature (`tickets`, `guests`,
`staffing`, and now `scans`) can actually recognize that grant — every one of them checks only the system role.
Tracked here rather than silently deferred so it doesn't get lost.

- Fix belongs in `internal/core/authz` (the shared `Checker`), not any one feature slice — every event-scoped
  feature benefits identically once it lands.
- Needs an `event_id` in scope for the check (already present on every gated route as `{event_id}` in the path)
  so the checker can look up that caller's active `event_staff_assignments` row for that event and consult its
  `role_id`'s permissions, in addition to (or as a fallback from) the existing system-role check.
- **Verify:** a user holding only an event-scoped Usher assignment (no elevated system role) can pass `check_in`
  on that event's `scans` endpoints; a user with no assignment on that event and no system permission still gets
  403.

No Story yet — write one (`product-manager`) when this is picked up. Not a blocker for B9 shipping: B9 reuses the
same system-role-only `Checker` every other gated feature already relies on, so it's neither better nor worse off
than `tickets`/`guests`/`staffing` are today.

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
| **B8** | `guests` | `000009` — `guests`, `tickets` | CRUD `/api/v1/events/{event_id}/guests`; assign ticket type; send invitation; guest RSVP; issue tickets |
| **B9** | `scans` | `000010` — `scan_logs` | `POST/GET /api/v1/events/{event_id}/scans` check-in; scan history |
| **B10** | `post-event-comms` | not yet designed | Thank-you message + documentation links sent after event completion (REQUIREMENT.md §4.7) |

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
- **B10 `post-event-comms`** — deferred future work, not being built now. Covers REQUIREMENT.md §4.7 (thank-you
  message after event completion) plus sending guests documentation links related to the event. No Story yet —
  write one when this phase is actually picked up, once `guests`/`scans` exist to know what "event completion"
  and "which guest" resolve against.

### B7. `tickets` (ticket types)

#### Story (product-manager)

As a tenant staff member configuring an event, I want to define ticket types for an event and specify which of
that event's workflow steps each ticket type includes, so that guests receive the right ticket (e.g. VIP vs
Regular) and check-in later enforces only the steps that ticket type is entitled to (REQUIREMENT.md §3.8, §4.4).

**Acceptance criteria**
1. Creating a ticket type under an event with a name and rules succeeds and the response is scoped to that event.
2. Creating a second ticket type with a name that already exists on the same event is rejected (409) — name is
   unique per event (DATABASE.md §3.11).
3. Listing ticket types for an event returns only that event's active (non-deleted) ticket types, paginated.
4. Fetching, updating, or deleting a ticket type that belongs to a different event resolves as 404, not a
   cross-event leak.
5. A ticket type's set of applicable workflow steps can be assigned/replaced, restricted to workflow steps
   belonging to the same event; a workflow step from a different event is rejected.
6. Retrieving a ticket type reports which of the event's workflow steps it currently applies to.
7. A newly created ticket type defaults to **all** of the event's current workflow steps (not none) — an
   organizer must explicitly narrow it down if a type should have limited access.
8. Deleting a ticket type is a soft delete: it disappears from subsequent list/get calls.
9. Every endpoint requires a valid access token; an unauthenticated call gets 401.

**Open question for solutions-architect:** B4's events/workflow-steps endpoints enforce auth only, with no
permission-code gate; B6 staffing gated its endpoints on `manage_staff`. Decide whether create/update/delete of
ticket types should be gated by an existing permission (e.g. `manage_events`) or left ungated for now. Also
decide the endpoint shape for setting a ticket type's workflow-step applicability — a dedicated junction
endpoint vs. a `workflow_step_ids` field on the ticket-type body (mirroring workflow-steps' Sync pattern).

No new domain concepts — `TicketType` is already modeled in REQUIREMENT.md §3.8 and the `ticket_type_workflow_steps`
junction in DATABASE.md §3.12, so no REQUIREMENT.md changes are needed.

#### Technical Design (solutions-architect)

**Slice boundary:** new slice `internal/features/tickets`, flat layout (not split into per-entity subpackages —
`TicketType` is the only entity this phase; `Ticket` itself arrives in B8). File names follow the entity:
`ticket_type_model.go`, `ticket_type_repository.go`, `ticket_type_workflow_step_repository.go`,
`ticket_type_dto.go`, `ticket_type_service.go`, `ticket_type_handler.go`, `ticket_type_routes.go`,
`tickets_constants.go`.

**Data model:** no new tables — migration `000008` already defines both:
- `ticket_types` (DATABASE.md §3.11) — standard soft-delete entity, `repository.Repository[TicketType, uuid.UUID]`
  via `internal/core/repository.NewRepository`, exactly like `events`/`event_category`.
- `ticket_type_workflow_steps` (DATABASE.md §3.12) — a composite-key junction with **no soft delete**. Same shape
  as `roles`' `role_permissions` join (FEATURES.md#roles): a small purpose-built
  `TicketTypeWorkflowStepRepository` interface (`SetWorkflowStepIDs(ctx, ticketTypeID, ids)`,
  `WorkflowStepIDsByTicketTypeID(ctx, ticketTypeID)`), not the generic `repository.Repository[T,TID]` — a junction
  row has no ID of its own to key a generic CRUD interface on.
- **Cross-feature read**: validating that a workflow-step ID belongs to the ticket type's event requires reading
  `workflow_steps`, owned by `events/workflowstep`. Follow the exact pattern `staffing` already uses for its
  `eventRepo`/`userRepo`/`roleRepo` (A8 step 6, AGENTS.md#repository-pattern--anti-duplication): the ticket-type
  service holds a `repository.ReadRepository[workflowstep.WorkflowStep, uuid.UUID]` field and calls `GetByID`
  per submitted ID, checking `.EventID` matches. No new published interface — this codebase already decided
  (A8, "explicitly out of scope") that a published-interface/adapter layer isn't warranted until a slice is an
  actual extraction candidate.

**API surface** (mirrors `events/workflowstep`'s nested-resource shape — see FEATURES.md#workflow-steps):

Base path `/api/v1/events/{event_id}/ticket-types`, `event_id` always from the URL, never the body. **Every route
requires the `manage_events` permission** (`authz.RequirePermission`, same mechanism `staffing` uses for
`manage_staff` — FEATURES.md#staffing) — decided per product owner: ticket types modify an event's flow, so they
gate on the same permission as event configuration, even though `events`/`workflow-steps` themselves don't yet
enforce it (that stays unchanged; out of scope here). `PermissionManageEvents = "manage_events"` is declared in
`tickets_constants.go` — permission codes are feature-owned (AGENTS.md), so this is a same-value constant local
to `tickets`, not a shared import from `events`.

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List for the event (paginated, filtered, sorted) | 200 | 400 invalid event id/query |
| `POST` | `/` | Create a ticket type under the event | 201 | 400 invalid body · 409 name conflict · 422 invalid entity |
| `GET` | `/{id}` | Get one, scoped to the event — response includes `workflow_step_ids` | 200 | 400 bad UUID · 404 not found |
| `PUT` | `/{id}` | Partial update (`name`, `rules`) | 200 | 400 · 404 not found · 409 name conflict |
| `DELETE` | `/{id}` | Soft delete | 204 | 400 · 404 not found |
| `PUT` | `/{id}/workflow-steps` | **Replace** the full set of workflow steps this ticket type applies to (bulk, same Sync-style full-replace semantics as `PUT /workflow-steps`, scoped to one ticket type instead of the whole event) | 200 | 400 a step id not on this event · 404 ticket type not found |

- `workflow_step_ids` is populated on `GetByID` only (one extra lookup), left empty on `List`/`Update` to
  avoid an N+1 join across every row of a list — the story's AC5/AC6 only requires it on a single retrieved
  ticket type, not the list view. `Create`'s response does populate it (see below — it's never empty by design).
- **Create seeds default workflow-step applicability to ALL of the event's current workflow steps** — not zero.
  Right after the row insert, the service looks up the event's `workflow_steps` and assigns every one via
  `TicketTypeWorkflowStepRepository.SetWorkflowStepIDs`, same best-effort/non-blocking treatment as
  `EventService.Create`'s template-copy (FEATURES.md#events): a lookup/assign failure is logged but does not fail
  ticket type creation. Rationale: an organizer forgetting to configure a new ticket type should fail **open**
  (full standard admission) rather than fail **closed** (guests silently blocked at every check-in step) — the
  latter is the costlier mistake during a live event. Narrowing a type (e.g. Regular loses VIP lounge) is then a
  deliberate `PUT .../workflow-steps` call that removes steps from the default set, not an easy-to-forget add.
- List query allow-list: sort `id, name, created_at, updated_at`; filter `name`; `event_id` forced from the URL,
  never a query param (same convention as `workflow-steps`).
- `name` uniqueness (`(event_id, name)`, DATABASE.md §3.11) is DB-enforced; the service maps
  `repository.ErrAlreadyExists` → `errorz.Conflict()`, same translation every other feature uses.
- `rules` is passed through as an opaque JSON document (`json.RawMessage`), unvalidated — same treatment as
  `workflow_step_templates.ticket_type_applicability` (FEATURES.md#workflow-step-templates); nothing in
  REQUIREMENT.md prescribes a `rules` schema yet.

**Non-goals:**
- **Not retrofitting `manage_events` onto `events`/`workflow-steps`** — those stay auth-only as they are today;
  only the new `ticket-types` routes gate on the permission. Revisit if a future story asks to lock down event
  editing itself.
- **No incremental add/remove of individual workflow-step associations** — only the full-replace `PUT
  .../workflow-steps`, mirroring the event-level workflow-steps Sync design. A UI needing to toggle one step
  resubmits the full set, same as today's workflow-steps Sync.
- **`Ticket` (the QR artifact issued to a guest) is out of scope** — that's B8, which depends on `guests`
  existing first.
- **No transaction around the workflow-step replace** — no feature in this codebase uses a DB transaction yet
  (see FEATURES.md#workflow-steps' Sync note); a failed replace is recovered by resubmitting, consistent with
  existing Sync behavior.

### B8. `guests`

#### Story (product-manager)

As a tenant staff member running an event, I want to manage the event's guest list, assign each guest a ticket
type, and invite them to RSVP — with a QR-coded ticket issued automatically once they confirm — so that guests
are admitted through check-in with only the access their ticket type grants them (REQUIREMENT.md §3.10, §4.1,
§4.2).

Guest fields stay exactly as modeled in REQUIREMENT.md §3.10 (`name`, `email`, `phone`) — no per-event custom
field builder. If that turns out to be needed, that's new scope to raise separately, not assumed here.

**Acceptance criteria**
1. Creating a guest under an event with `name` and `email` succeeds and the guest is scoped to that event; `rsvp_status` starts at `none`.
2. Listing guests for an event returns only that event's active (non-deleted) guests, paginated.
3. Fetching, updating, or deleting a guest that belongs to a different event resolves as 404, not a cross-event leak (same convention as `ticket-types`/`workflow-steps`).
4. A ticket type can be assigned to a guest, restricted to ticket types belonging to the same event as the guest; a ticket type from a different event is rejected (400).
5. Sending an invitation to a guest requires a ticket type to already be assigned (400 otherwise), moves `rsvp_status` from `none` to `invited`, and generates a unique, unguessable `invitationToken` for that guest.
6. An unauthenticated RSVP endpoint, addressed by `invitationToken` (no login — guests are not `User` accounts), lets the guest confirm or decline. An unknown/invalid token returns 404.
7. An event has an `rsvpRequired` setting (default `true`) that decides when a guest's ticket is issued:
   - **`rsvpRequired = true`** (default): confirming RSVP sets `rsvp_status` to `confirmed` and issues the guest's `Ticket` (QR code); declining sets `declined` and issues no ticket. Same behavior as before this setting existed.
   - **`rsvpRequired = false`**: sending the invitation (AC5) issues the guest's `Ticket` immediately, without waiting for any RSVP response. The RSVP endpoint remains reachable so the guest can still record confirm/decline for the organizer's records, but that response no longer gates ticket issuance.
8. Retrieving a guest reports their current `rsvp_status`, assigned ticket type, and — once issued — their ticket/QR reference.
9. Deleting a guest is a soft delete: it disappears from subsequent list/get calls.
10. Every staff-facing endpoint (list/get/create/update/delete, assign ticket type, send invitation) requires a valid access token; an unauthenticated call gets 401. The RSVP endpoint is the one deliberate exception — public by design, gated by token validity instead of a session.

**Open questions for solutions-architect:**
- **PII encryption approach.** Migration `000009` currently stores `guests.name`/`email`/`phone` as plain
  `TEXT`, and no encryption/crypto helper exists anywhere in this workspace yet (checked `go-sdk` — no
  `crypto`/`cipher` package). The user wants these fields encrypted at rest with no raw PII in the database.
  Needs a decision on: where encryption/decryption happens (repository-layer, transparent to the service —
  consistent with AGENTS.md's cross-cutting-concern-goes-in-go-sdk guidance), key management, and how guest
  lookup/dedup-by-email still works once `email` is ciphertext (equality filtering needs either a deterministic
  blind-index column or dropping email-based lookup). This likely means a follow-up migration altering
  `000009`'s column shapes, not just app-code — flag before `backend-developer` starts.
- **Invitation delivery mechanism.** No outbound email/SMS provider integration exists anywhere in this codebase
  today. Decide whether B8 includes wiring a minimal real sender (e.g. SMTP) or ships a stub/log-only "send"
  for now with real delivery as a fast-follow — either way, `message_templates` (B5) already has the
  invitation/ticket-delivery template bodies to resolve against.
- **Permission gating.** `manage_guests` is already seeded in the roles/permissions catalog (`roles`, B6) but
  unused by any slice yet — confirm all staff-facing guest endpoints gate on it (mirroring how `tickets` gates
  on `manage_events`), and confirm the RSVP endpoint needs no permission check at all (token possession is the
  only gate, since the caller isn't authenticated).
- **RSVP route shape.** Needs a route-policy entry marking it `public` (same mechanism `auth`'s `/login`/`/refresh`
  use) — decide the exact path, e.g. `POST /api/v1/guests/rsvp/{token}`.
- **`rsvpRequired` touches an already-shipped slice.** It's a new column on `events` (B4, already ✅), not a
  `guests`-owned field — needs a small migration adding `rsvp_required BOOLEAN NOT NULL DEFAULT true` plus
  exposing it on `events`' `CreateEventInput`/`UpdateEventInput`. Decide whether that lands as a preceding patch
  to the `events` slice or bundled into B8's migration set.
- **Decline after early ticket issuance.** When `rsvpRequired = false`, a ticket is already issued by the time
  the guest could hit the RSVP endpoint. If they then decline, decide whether that's purely informational
  (`rsvp_status = declined`, ticket stands) or should invalidate/void the already-issued ticket — not obvious
  from the request, and touches `Ticket.status` semantics (REQUIREMENT.md §3.9).

No new domain concepts beyond what's now reflected in REQUIREMENT.md (`invitationToken` added to §3.10; PII
encryption added to §5.4) — `Guest` and `Ticket` were already modeled, just not yet implemented.

#### Technical Design (solutions-architect)

**Slice boundary:** new slice `internal/features/guests`, **flat layout** — not split into per-entity
subpackages despite modeling two entities (`Guest`, `Ticket`). `Ticket` ships **model + repository only** this
phase (no DTO/service/handler/routes — same call already made for `roles` in B6, and the same shape
`workflow_step_templates` had in B4 before it grew its own HTTP surface in B5): nothing in the Story's
acceptance criteria needs a top-level `/tickets` endpoint, only a QR/ticket summary surfaced through
`GET .../guests/{id}`. Splitting a repo-only companion entity into its own subpackage would be ceremony with
no navigability payoff — flat stays until a second HTTP-facing entity actually shows up.
Files: `guest_model.go`, `guest_repository.go`, `guest_dto.go`, `guest_service.go`, `guest_handler.go`,
`guest_routes.go`, `guest_service__test.go`, `ticket_model.go`, `ticket_repository.go`,
`invitation_publisher.go`, `pii_encryptor.go`, `guests_constants.go`.

**Data model:**
- `guests` (migration `000009`, exists) needs several new columns via a follow-up migration:
  `ticket_type_id UUID NULL REFERENCES ticket_types(id)` (must be set before an invitation can be sent),
  `invitation_token TEXT NULL UNIQUE` (set on send; the public RSVP endpoint looks guests up by it), and — see
  PII encryption below — `email_hash TEXT NULL` / `phone_hash TEXT NULL` (each indexed) alongside the existing
  `email`/`phone` columns. `name` stays plain `TEXT`, unchanged.
- `tickets` (migration `000009`, exists) — used unmodified. `TicketRepository` is a plain typed
  `repository.Repository[Ticket, uuid.UUID]` via `internal/core/repository.NewRepository`, nothing special.
- `events` (migration `000005`, shipped in B4) needs one new column from a small preceding migration:
  `rsvp_required BOOLEAN NOT NULL DEFAULT true`, exposed on `events.CreateEventInput`/`UpdateEventInput`.
  `GuestService` reads it through a cross-feature `repository.ReadRepository[events.Event, uuid.UUID]` field —
  the same established pattern `tickets` already uses for its `workflowstep` read
  (AGENTS.md#repository-pattern--anti-duplication).
- **`ticket_type_id` cross-event validation** — same pattern as `tickets`' own workflow-step validation:
  `GuestService` holds `repository.ReadRepository[tickets.TicketType, uuid.UUID]`, loads the submitted ID on
  create/update, and checks `.EventID` matches the guest's own event (400 otherwise).
- **PII encryption, revised per product owner: only `email`/`phone` are encrypted; `name` stays plaintext**
  so it can be searched with a partial-match `LIKE` — a `name` column full of ciphertext couldn't support that
  at all.
- **Encryption sits behind an app-local `PIIEncryptor` interface — same abstraction shape as
  `InvitationPublisher`, for the same reason: the product owner may change the underlying scheme later.**
  `internal/features/guests/pii_encryptor.go`:
  ```go
  type PIIEncryptor interface {
      Encrypt(plaintext string) (string, error)
      Decrypt(ciphertext string) (string, error)
      BlindIndex(value string) (string, error)
  }
  ```
  `guest_repository.go` depends only on this interface — it wraps the result of
  `internal/core/repository.NewRepository`, calling `Encrypt` on `email`/`phone` immediately before every
  `Create`/`Update` and `Decrypt` on every row returned by `GetByID`/`List`. `internal/app` wires the only
  implementation this phase, `cryptoPIIEncryptor`, a thin adapter over the new go-sdk `crypto` package (below)
  plus the two configured keys. Swapping the scheme later (different cipher, a KMS-backed implementation, a
  remote encryption service) is a change to that one adapter + its wiring in `internal/app` — `guest_repository.go`
  and everything else in the slice stays untouched, exactly like `InvitationPublisher`. No column-type change
  either — ciphertext fits the existing `TEXT` columns.
- **Exact-match search on encrypted `email`/`phone` via a blind index — not by comparing ciphertext.**
  AES-GCM (the default implementation) is deliberately non-deterministic: encrypting the same email twice
  yields two different ciphertexts, so a `WHERE email = encrypt(?)` lookup can never match regardless of
  implementation. This is why `PIIEncryptor` exposes `BlindIndex` as its own method, separate from
  `Encrypt`/`Decrypt`: it's a **deterministic** HMAC-SHA256 of the normalized value
  (`strings.ToLower(strings.TrimSpace(...))` for email; digits-only for phone — normalization is `guests`'
  own business rule, done before calling `BlindIndex`), stored in `email_hash`/`phone_hash`. A search request
  goes through the same normalize-then-hash step and is matched by equality against the hash column — which is
  exactly why it can only match a **complete, correctly-formatted** value, never a partial one: a hash has no
  notion of "starts with". This is the accepted tradeoff (per product owner) in exchange for not
  storing/indexing plaintext PII at all. `PIIEncryptor` itself, `guest_repository.go`'s two-field wrapping, and
  the hash columns stay **local to `guests`**, not promoted to `internal/core` — `guests` is the only consumer
  today (same "don't extract until a second consumer exists" call this codebase already made in A8).
- **`name` search needs a shared-infra change — per-request operator, not a per-field config.** Per product
  owner's direction, a filter value carries its own operator inline: `?name=Ali;like` (operator suffixed after
  `;`); no suffix (`?name=Ali`) defaults to exact match, same as every filter today. This applies to
  `internal/core/query` generally (every feature's list endpoints get it, not just `guests`) rather than a
  guest-specific config, and it's actually **simpler** than a static `LikeFilterFields` allow-list: the client
  picks the operator per request, `AllowedFilterFields` still only gates *which fields* are filterable at all.
  Concretely:
  - `ListParams.Filters` changes from `map[string]string` to `map[string]FilterValue` where
    `FilterValue{Value string; Operator repository.FilterOperator}`. Nothing outside `internal/core/query`
    touches `.Filters` directly today (every shipped feature just passes `params` straight into
    `query.ToListOptions`), so this is safe to change now.
  - `parseFilters` splits each raw value on the first `;`; a present suffix is looked up against a small
    **whitelist** of supported operators — just `eq` (default) and `like` for this pass, an intentionally
    short list; an unrecognized suffix is a `400` (fail closed, same as an unknown sort/filter field today).
    Adding another go-sdk `FilterOperator` later (`gt`, `in`, ...) is one line in that whitelist, not a
    redesign.
  - `ToListOptions` wraps the value `%value%` only when the resolved operator is `like`; otherwise it's passed
    through as today.
  - **Implementation caution:** `;` is a reserved query-string character in some HTTP stacks/`net/url`
    versions — a client building the query string by hand must percent-encode it (`%3B`) or the value can be
    silently mis-parsed. Any real HTTP client library (axios, fetch, Postman, `net/http`'s own `url.Values.Encode`)
    does this automatically; only a hand-built raw query string is at risk. Worth a one-line callout in
    `tester`'s Postman collection and `FEATURES.md` once implemented.
- **`email`/`phone` only ever resolve to exact match, enforced in the service — not by trusting the client's
  chosen operator.** The client still filters by `?email=...`/`?phone=...` (allow-listed normally, and may say
  `;like` like any other field), but a blind-index column has no notion of partial match, so
  `GuestService.List` **rejects** (400) an `email`/`phone` filter whose resolved operator isn't `eq` before
  going any further. For an `eq` (or bare) `email`/`phone` filter, the service normalizes and hashes the value
  and rewrites the map entry to the real column (`email_hash`/`phone_hash`) before calling
  `query.ToListOptions` — `ValidateListParams` still runs first, against the client-facing `email`/`phone`
  names, so the allow-list check is unaffected. `ToListOptions` itself isn't reimplemented, only fed a
  pre-transformed map — the same shape of business-logic-before-shared-helper every other service already does.

**New go-sdk dependency — `crypto` package, consumed only by the `cryptoPIIEncryptor` adapter.** Per the
product owner's direction, field-level PII encryption belongs in `go-sdk`, not this app: `guest-management-be`
should only ever pass secret keys through config, never implement a cipher itself — and per the abstraction
above, only one file (`internal/app`'s adapter) ever imports `go-sdk/crypto` directly. Added as a new phase
stub to [go-sdk/docs/DEVELOPMENT_PLAN.md](../../go-sdk/docs/DEVELOPMENT_PLAN.md):
- `Encrypt(key, plaintext []byte) (string, error)` / `Decrypt(key []byte, ciphertext string) ([]byte, error)` —
  AES-256-GCM, random nonce prepended to the ciphertext (semantically secure — same input, different output
  every time, by design).
- `BlindIndex(key []byte, value string) string` — deterministic HMAC-SHA256 (hex-encoded), for exact-match
  lookup only. Uses a **separate key** from `Encrypt`/`Decrypt` (standard key-separation practice — an
  encryption key and a deterministic-MAC key serve different security purposes and shouldn't be reused).
- The SDK's usual `Config`/`DefaultConfig()` carries both keys (e.g. `EncryptionKey`, `BlindIndexKey`), loaded
  from env, never committed.

This service consumes it once it lands; designing the SDK package itself is out of scope for this document.
**Update:** shipped and wired — `internal/app/pii_encryptor.go`'s `cryptoPIIEncryptor` adapts
`go-sdk/lib/crypto.Encryptor` to `guests.PIIEncryptor`; keys are `Config.Crypto.EncryptionKey`/`BlindIndexKey`
(env `GUEST_PII_ENCRYPTION_KEY`/`GUEST_PII_BLIND_INDEX_KEY`, see `.env.example`).

**Invitation delivery — app-local abstraction, no go-sdk dependency yet.** `go-sdk` has no Kafka/queue package,
and building one is out of scope here (per product owner: an app-local interface now, wired to a real
`go-sdk` producer later). `internal/features/guests/invitation_publisher.go` declares:
```go
type InvitationPublisher interface {
    PublishInvitation(ctx context.Context, msg InvitationMessage) error
}
```
`GuestService.SendInvitation` calls it after generating the token — **best-effort**, the same non-blocking
treatment as `EventService.Create`'s template-copy (FEATURES.md#events): a publish failure is logged but does
not fail the invitation-send call, since the guest row/token are already correctly persisted. `internal/app`
wires a `LoggingInvitationPublisher` (logs the message, does nothing else) as the only implementation this
phase. Swapping in a real Kafka-backed publisher later is a change to `internal/app`'s wiring alone, once
go-sdk ships one — `guests` code doesn't change. Kept feature-local rather than in `internal/core`, since
`guests` is the only consumer right now.

**API surface** (mirrors `tickets`' nested-resource shape; gated on the already-seeded `manage_guests`
permission — seeded in B6's catalog but unused by any slice until now):

Base path `/api/v1/events/{event_id}/guests`:

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `GET` | `/` | List guests for the event (paginated, filtered, sorted) | 200 | 400 · 401 · 403 |
| `POST` | `/` | Create a guest (`name`, `email`, `phone?`, `ticket_type_id?`) | 201 | 400 · 401 · 403 · 409 · 422 |
| `GET` | `/{id}` | Get one, scoped to the event — includes `ticket_type_id`, `rsvp_status`, and the ticket/QR summary once issued | 200 | 400 · 401 · 403 · 404 |
| `PUT` | `/{id}` | Partial update (`name`, `email`, `phone`, `ticket_type_id`) | 200 | 400 ticket type cross-event · 401 · 403 · 404 |
| `DELETE` | `/{id}` | Soft delete | 204 | 400 · 401 · 403 · 404 |
| `POST` | `/{id}/invitation` | Send invitation — requires `ticket_type_id` already set; sets `rsvp_status=invited`, generates `invitation_token`, calls `InvitationPublisher`; if the event's `rsvp_required=false`, also issues the `Ticket` immediately | 200 | 400 no ticket type assigned · 401 · 403 · 404 |

Public, unauthenticated (route-policy `public: true`, same mechanism as `auth`'s `/login`/`/refresh`):

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `POST` | `/api/v1/guests/rsvp/{token}` | Guest confirms/declines (`{"status":"confirmed"\|"declined"}`); confirming issues the `Ticket` when `rsvp_required=true` and one isn't already issued | 200 | 400 invalid status · 404 unknown token |

`invitation_token` is returned in the `POST .../invitation` response body — there's no real email channel yet
to deliver the RSVP link through, so this is the only way to reach it (needed for `tester`'s e2e coverage of
the RSVP flow).

List query allow-list: sort `id, name, rsvp_status, created_at, updated_at`; filter `name` (exact by default,
`?name=Ali;like` for partial), `email`, `phone` (exact only — a `;like` suffix on either is rejected, see PII
encryption above), `rsvp_status` (exact); `event_id` forced from the URL, same convention as every other
nested-resource feature.

**Non-goals:**
- **No real Kafka producer/consumer, no real email/SMS/WhatsApp delivery.** B8 only produces an
  `InvitationMessage` through a logging stub; wiring an actual send is a fast-follow once go-sdk ships Kafka
  support and a provider is chosen.
- **No generic `internal/core` field-encryption decorator** — kept local to `guest_repository.go`'s two
  encrypted fields (`email`, `phone`) until a second consumer needs it.
- **No `ILIKE`/case-insensitive search, no trigram/GIN index for `name`** — plain `LIKE` and a normal B-tree
  are enough at guest-list scale; revisit if a tenant's guest list grows large enough for `LIKE '%x%'` scans
  to matter.
- **No operators beyond `eq`/`like` in the new `;operator` filter syntax this pass** — `internal/core/query`'s
  whitelist starts with just those two; `gt`/`lt`/`in`/`is_null`/etc. are one-line additions to the whitelist
  whenever a feature actually needs one, not built speculatively now.
- **No email/phone uniqueness constraint on `guests`** — the hash columns exist for lookup, not dedup; no
  Story acceptance criterion asks for guest uniqueness (unlike `users`' email uniqueness).
- **No voiding of an early-issued ticket on a later decline** (`rsvp_required=false` case) — per product
  owner, the ticket stands regardless of what the guest answers afterward.
- **No `Ticket` HTTP endpoints** — consumed internally by `GuestService` only.
- **No custom per-event guest fields** — `Guest` keeps the fixed `name`/`email`/`phone` shape from
  REQUIREMENT.md §3.10.
- **No dedicated `PUT .../ticket-type` endpoint** — ticket-type assignment is just a field on
  `CreateGuestInput`/`UpdateGuestInput`; a separate endpoint doesn't earn its keep for one field.

### B9. `scans`

#### Story (product-manager)

As event staff running check-in, I want to scan a guest's ticket QR code against a specific workflow step and
have the system enforce that step's one-time-vs-repeatable rule, so that single-use steps (e.g. souvenir
pickup) can't be claimed twice while repeatable steps (e.g. photo booth) accumulate an accurate, permanent count
of every visit for post-event reporting (REQUIREMENT.md §4.3, §4.5).

**Acceptance criteria**
1. Recording a scan for a ticket against a workflow step that the ticket's ticket type is entitled to, and that
   the ticket hasn't yet completed, succeeds and persists a `scan_logs` row (ticket, workflow step, event,
   timestamp).
2. Recording a second scan for the same ticket against the same workflow step is **rejected** when that step's
   `allowsMultiple = false` — a distinct "already completed" error (409), and no second `scan_logs` row is
   created.
3. Recording repeated scans for the same ticket against a workflow step with `allowsMultiple = true` succeeds
   every time with no cap, and each call persists its **own** `scan_logs` row — none are deduplicated,
   overwritten, or merged.
4. Scanning a ticket that does not belong to the event being scanned at, or whose status is `invalidated`, is
   rejected — not logged as a successful scan.
5. Scanning a ticket against a workflow step that its ticket type does not include (per
   `ticket_type_workflow_steps`, B7) is rejected — the ticket type isn't entitled to that step.
6. Scanning a ticket against a workflow step that belongs to a different event than the ticket is rejected
   (same cross-event guard convention as B7/B8), independent of criterion 5.
7. A scan-history endpoint returns the recorded scans for a given ticket and/or workflow step, so that the
   count of completions for a multi-use step (criterion 3) and the single completion of a single-use step
   (criterion 1) are both independently verifiable through the API after the fact.
8. Every scan endpoint (recording a scan and retrieving scan history) requires a valid access token; an
   unauthenticated call gets 401. Unlike B8's guest-facing RSVP, this is a staff action with no public
   counterpart.

**Open questions for solutions-architect:**
- **Endpoint shape.** Whether recording a scan is `POST /api/v1/scans` with `qr_code` + `workflow_step_id` in
  the body, a path nested under the ticket/event, and whether the client identifies the ticket by its `qr_code`
  (what the guest actually presents) or a resolved `ticket_id`.
- **Scan-history endpoint shape.** Whether it's a raw `scan_logs` listing (filterable by `ticket_id` and/or
  `workflow_step_id`, paginated, client counts rows) or the endpoint itself returns an aggregated count —
  criterion 7 only requires the data be retrievable and accurate, not a specific response shape.
- **Permission gating.** REQUIREMENT.md §4.6 lists "Only scanning" as a distinct staff permission tier from
  guest/workflow/full-admin management. Decide whether B9 introduces a new permission code (e.g.
  `record_scans`) seeded alongside `manage_events`/`manage_guests`/`manage_staff`, or reuses an existing one —
  and whether the scan-history read endpoint needs the same or a different permission than recording a scan.
- **Ticket status transition semantics.** `Ticket.status` is `active | used | invalidated` (B8, shipped), but
  with a ticket now scanned repeatedly across multiple workflow steps over an event's lifetime, it's not
  specified anywhere what (if anything) flips a ticket from `active` to `used` — decide whether B9 leaves
  `status` untouched (only `invalidated` blocks a scan, per criterion 4) or defines a transition trigger (e.g.
  last workflow step in the ticket type's set).

No new domain concepts — `WorkflowStep.allowsMultiple`, `ScanLog`, and the §4.5 validation list were already
modeled; REQUIREMENT.md's `ScanLog` entity (§3.11) is updated in this pass only to add the already-DB-modeled
`operatorUserId` (optional, DATABASE.md §3.15) that was implied but missing from the domain listing.

#### Technical Design (solutions-architect)

**Permission gating — corrected per product owner.** B9 gates on the **existing** seeded `check_in` permission
(migration `000014`, `docs/STAFFING_RBAC.md` §3 — *"scan tickets / record guest check-ins"*, and REQUIREMENT.md
§4.6's "Only scanning" tier), **not** a new `record_scans` code. Minting a new code here would have left it
ungranted to every role the seed data already gives `check_in` to (Usher, Photobooth Staff, Tenant Staff, Tenant
Admin, Super Admin) — a functional regression, not a style choice — so this was sent back and confirmed rather
than guessed through. Same permission gates both endpoints (recording a scan and reading scan history) — no
acceptance criterion asks for a narrower read-only tier.

**Slice boundary:** new slice `internal/features/scans`, flat layout (one entity, `ScanLog` — the write-heavy
audit row this slice owns; `Ticket`, `WorkflowStep`, and `TicketType` are read/written through already-exported
cross-feature repositories, never owned here). Files: `scan_log_model.go`, `scan_log_repository.go`,
`scan_log_dto.go`, `scan_log_service.go`, `scan_log_handler.go`, `scan_log_routes.go`,
`scan_log_service__test.go`, `scans_constants.go`. `ScanLogService` exposes only `RecordScan` and `ListScans` —
no `Update`/`Delete`/`GetByID` service methods, since `scan_logs` is a permanent, append-only audit log with no
endpoint needing them (the underlying repository still satisfies the full generic `repository.Repository[T,TID]`
interface only because that's the shared constructor's return type, not because those operations are exposed).

**Data model:** no new tables, no new migration — B9 only reads/writes rows in tables B7/B8/B9's own
prerequisite migrations already shipped:
- `scan_logs` (migration `000010`, DATABASE.md §3.15) — no soft delete, so `ScanLogRepository` is a plain
  `repository.Repository[ScanLog, uuid.UUID]` built via `internal/core/repository.NewRepositoryNoAudit` (the
  same call `roles` already makes for its own no-`deleted_at` table) — the audit decorator's unconditional
  `deleted_at IS NULL` filter and soft-delete-on-`Delete` would break against a table lacking that column. The
  generic `Repository.Count(ctx, filter)` method is reused as-is for the AC2 "already completed" check — no new
  repository method needed.
- `tickets` (migration `000009`, owned by `guests`) — resolving `qr_code` → ticket and flipping
  `status: active → used` both reuse the **same already-exported constructor**, `guests.NewTicketRepository(...)`,
  that `guests`' own service already uses. `internal/app` wires that one concrete instance into both
  `guests.Service` and `scans.Service`, this time typed as the full `repository.Repository[guests.Ticket,
  uuid.UUID]` (not the narrower `ReadRepository` `tickets` uses for `workflowstep.WorkflowStep`, since B9 needs
  `Update` too) — no new interface, no adapter, just requesting the read-write half of an already-generic
  interface. `guests_constants.go` gains two constants alongside the existing `TicketStatusActive` (whose doc
  comment already earmarks this as "a B9 concern"): `TicketStatusUsed = "used"`, `TicketStatusInvalidated =
  "invalidated"` — a small in-place addition to `guests`, not a new file; `scans` imports them rather than
  redeclaring the same three strings under a different name.
- `workflow_steps` (migration `000005`, owned by `events/workflowstep`) — read via
  `repository.ReadRepository[workflowstep.WorkflowStep, uuid.UUID]`, the exact pattern `tickets` already
  established for the same table (AGENTS.md#repository-pattern--anti-duplication).
- `ticket_type_workflow_steps` (migration `000008`, owned by `tickets`) — the entitlement check (AC5) reuses
  `tickets.TicketTypeWorkflowStepRepository.WorkflowStepIDsByTicketTypeID(ticketTypeID)` as-is (already exported
  for exactly this membership check); `scans` holds it as a field, no new method added to it.

**API surface** (mirrors `tickets`/`guests`' nested-resource shape; `event_id` always from the URL, both routes
gated on `check_in` via `authz.RequirePermission`, the same mechanism every other gated feature uses):

Base path `/api/v1/events/{event_id}/scans`:

| Method | Path | Purpose | Success | Notable errors |
|---|---|---|---|---|
| `POST` | `/` | Record a scan — body is `qr_code` + `workflow_step_id`; the server resolves the ticket internally, never a client-supplied `ticket_id` | 201 | 400 workflow step not on this event · 400 ticket type not entitled to this step · 404 ticket not found for this event · 409 ticket invalidated · 409 step already completed (`allowsMultiple=false`) · 401 · 403 |
| `GET` | `/` | Scan history for the event, paginated and filterable by `ticket_id` and/or `workflow_step_id` (reuses `internal/core/query`'s `ListParseConfig`/`ParseListParams`/`ToListOptions`, same as every other list endpoint) | 200 | 400 invalid query · 401 · 403 |

Validation order for `POST /`, all inside `ScanLogService.RecordScan` (transport-agnostic; the handler only
decodes and validates request shape):
1. Resolve `qr_code` to a `guests.Ticket` filtered on **both** `qr_code` and the URL `event_id` — a ticket that
   exists but belongs to another event is indistinguishable from an unknown QR code, same no-cross-event-leak
   convention as B7 AC4/B8 AC3 → 404 if none found.
2. `ticket.Status == TicketStatusInvalidated` → 409 (blocked by the ticket's own state — same category as the
   AC2 conflict in step 5).
3. Load the `workflow_steps` row by `workflow_step_id` → 404 if it doesn't exist at all; `.EventID != event_id`
   (URL) → 400 (same status B7 already uses for "a step id not on this event").
4. Confirm `workflow_step_id` is in
   `TicketTypeWorkflowStepRepository.WorkflowStepIDsByTicketTypeID(ticket.TicketTypeID)` (AC5) → 400 if absent
   (ticket type isn't entitled to this step; same 400 category as B8's ticket-type cross-event check).
5. If `!workflowStep.AllowsMultiple`, `ScanLogRepository.Count` filtered on `ticket_id`+`workflow_step_id` — a
   non-zero count → 409 "already completed" (AC2). Skipped entirely when `AllowsMultiple == true`, so every call
   reaches step 6 and inserts its own row with no cap (AC3).
6. Insert the `scan_logs` row (`operator_user_id` left `NULL` this phase — see non-goals).
7. If `ticket.Status == TicketStatusActive`, update it to `TicketStatusUsed` — a one-way,
   first-successful-scan-of-any-step transition that runs at most once per ticket. Once `used`, step 6 keeps
   inserting new `scan_logs` rows on every later call (including further repeatable-step scans) without ever
   touching `status` again, satisfying AC3 (repeated scans keep succeeding regardless of `status`) and AC4 (only
   `invalidated`, checked in step 2, ever blocks a scan). A failure updating `status` is logged but does not
   fail the scan response — the `scan_logs` row from step 6 is already persisted and is the record the product
   owner said they're relying on regardless.

List query allow-list (`GET /`): sort `id, scanned_at`; filter `ticket_id`, `workflow_step_id` (both exact-match
only — no `;like` use case for two UUID equality filters); `event_id` forced from the URL, same convention as
every other nested-resource feature.

**Non-goals:**
- **No `operator_user_id` population.** The column (already nullable, migration `000010`) is left `NULL` this
  pass — no acceptance criterion asks for "who scanned" reporting, and populating it would require a new
  exported `authz.UserIDFromContext` (mirroring the existing `RoleIDFromContext`/`TenantIDFromContext` pattern)
  plus wiring the auth middleware to actually set that claim into context — nothing in this codebase does that
  anywhere yet. Revisit once a story actually needs it.
- **No event-scoped permission resolution.** `authz.Checker.Require` only ever resolves the caller's
  **system-level** `role_id` claim (`internal/core/authz/checker.go`) — nothing in this codebase today checks an
  `event_staff_assignments` row's event-scoped role (Usher/Photobooth Staff) for a permission. A user whose only
  grant of `check_in` comes from an event-scoped assignment, not their system role, cannot pass this gate today.
  This is a pre-existing gap from B6's authz design, not introduced or worsened by B9 — B9 reuses the exact same
  system-role-only `Checker` every other gated feature (`tickets`, `guests`, `staffing`) already uses. Fixing it
  is an `internal/core/authz` change (a new event-scoped resolver path), not a `scans`-local one — flag as a
  fast-follow if event-scoped staff need to scan without also holding `check_in` on their system role.
- **No aggregated/counted scan-history response shape.** `GET /` returns raw, paginated `scan_logs` rows; AC7
  only requires the completion count be independently verifiable, and a paginated list (the client reads
  `total`) already satisfies that — a separate aggregation endpoint is unrequested scope.
- **No transaction around steps 5–7** (count check → insert → ticket status update) — no feature in this
  codebase uses a DB transaction yet (same accepted gap as B7's workflow-step replace); a narrow race between
  two concurrent scans of the same non-repeatable step could both pass the count check before either inserts.
  Acceptable at current scale; revisit if concurrent-scan collisions are ever reported.
- **No new permission code, no new migration** — reuses the already-seeded `check_in` rather than minting
  `record_scans` (see the corrected permission-gating decision above).
- **No new `Ticket`/`WorkflowStep`/`TicketType` HTTP surface** — `scans` only reads/writes them through
  already-exported cross-feature repositories; their own endpoints are unchanged.
- **No ticket-status rollback.** Once a ticket flips to `used`, nothing in this pass ever reverts it to `active`
  (e.g. to "undo" a scan) — no acceptance criterion asks for that, and `scan_logs` remains the permanent,
  un-editable record of what actually happened regardless of `status`.

### Cross-references to go-sdk

| This service needs | Provided by go-sdk phase |
|---|---|
| Boundary validation (A6) | `validator` |
| Login + route protection (B3) | `auth` |
| Metrics/tracing/rate-limit/breaker (A7) | `metrics`, `tracer`, `ratelimit`, `circuitbreaker` |
| Graceful shutdown (A7) | `lifecycle` |
| PII field encryption (B8) | `crypto` — shipped, see go-sdk `DEVELOPMENT_PLAN.md` Phase 9; wired via `internal/app/pii_encryptor.go` |

When one of these is missing, prefer **adding it to `go-sdk`** (it's a reusable cross-cutting concern) over
building an app-local version.
