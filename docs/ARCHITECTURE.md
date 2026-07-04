# Architecture

Reference for *why* this service is shaped the way it is. The enforceable rules live in [../AGENTS.md](../AGENTS.md); this document explains the reasoning behind them.

## Layered, feature-sliced, built on go-sdk

The service is a thin **application** layered on top of the `go-sdk` **building-blocks library**. Almost all infrastructure — HTTP adapter, DB access, error taxonomy, structured logging, pagination — comes from `go-sdk`. This repo contributes the *domain*: models, business rules, and the HTTP surface.

Two organizing ideas:

1. **Layering** — a request flows through fixed layers, and dependencies point one way.
2. **Feature slices** — domain code is grouped by feature, not by layer, so a feature can be lifted out into its own service later.

## Layering & dependency direction

```
HTTP boundary        httpkit (handler adapter, middleware, response envelope)   ← go-sdk
      │  maps errorz codes → HTTP status (the only HTTP-specific step)
Handler              internal/features/<feature>/*_handler.go
      │  parse + validate request, call service, wrap result
Service / usecase    internal/features/<feature>/*_service.go
      │  business rules, orchestration; speaks domain types + context only
Repository           internal/features/<feature>/*_repository.go
      │  → internal/core/audit (soft-delete/audit decorator)
Data access          repository + repository/sql                                ← go-sdk
      │
Infrastructure       sqlkit (database/sql, leader/follower)                     ← go-sdk
```

Dependencies point **downward only**. Concretely:

- A **handler** may import the service interface and `httpkit`/`errorz`; it MUST NOT import `sqlkit` or touch a `*sql.DB`.
- A **service** may import the repository interface, `errorz`, `logger`, domain types; it MUST NOT import `net/http`, `chi`, or know about status codes.
- A **repository** may import `go-sdk`'s `repository`/`sqlkit` and the audit decorator; it knows nothing above it.

This is what makes the layers independently testable — a service is tested against a generated mock repository, a handler against a generated mock service (see [TESTING.md](TESTING.md)).

## Feature slice = future service boundary

Domain code lives in `internal/features/<feature>`, each slice holding its own model, repository, service, handler, routes, and query parsing. The rule **"no feature imports another feature's internals"** is not stylistic — it is what lets you later run `users` and `events` as separate deployments with only the wiring in `internal/app` and a network hop changing. Cross-feature needs are met by:

- **`internal/core`** — shared, feature-agnostic building blocks (the audit decorator today; a base repository helper, list-query parser, and validator adapter per the [roadmap](DEVELOPMENT_PLAN.md)).
- **A published interface** — if feature A genuinely needs feature B, B exposes a small interface that A depends on, so B can later become a remote client behind the same interface.

`internal/app` is the **composition root** — the single place that imports every feature and wires repositories → services → handlers → routes. Keeping wiring here (and out of the slices) means adding/removing a feature is a localized change.

## Errors: errorz as the ecosystem-wide error type

`errorz` (from `go-sdk`) is the shared structured-error type for the **whole ecosystem**, not an HTTP concern. Every layer returns `errorz`-coded errors, and `httpkit` maps the code to an HTTP status **at the edge only** (`CodeNotFound` → 404, `CodeConflict` → 409, …). The same taxonomy would map to gRPC codes if a slice becomes a gRPC service.

The established flow in this service:

- **Repository** returns `go-sdk` sentinels (`repository.ErrNotFound`, `ErrAlreadyExists`, `ErrInvalidEntity`).
- **Service** translates them: `errors.Is(err, repository.ErrNotFound)` → `errorz.NotFound().WithMessage("...")`; unknown errors → `errorz.Wrap(err).WithCode(errorz.CodeInternal)` (preserving the chain) and logs at error level.
- **Handler** returns the `errorz` error as-is; `httpkit`'s `StatusCodeFromError` maps it.

Two hard rules keep this safe: compare sentinels with `errors.Is` (never type-assert), and declare `error` in signatures (never `*errorz.Error`, to avoid Go's typed-nil trap).

## Logging responsibility

Logging is **structured and context-aware** via `go-sdk` `logger` + `ctxkit`. The convention: infrastructure/`go-sdk` code returns errors and does not log; the **service** decides what is noteworthy and logs it once, on the internal-error path, with `ErrorWithContext(ctx, msg, logger.F("key", val))`. Because middleware seeds `ctxkit` values, every log line automatically carries `request_id`/`correlation_id`. Stdlib `log` is banned.

## Where go-sdk ends and the app begins

| Concern | Owned by go-sdk | Owned by this app |
|---|---|---|
| HTTP adapter, middleware, response envelope | ✅ `httpkit` | route registration, handler bodies |
| Error taxonomy | ✅ `errorz` | which code each business case maps to |
| DB pool, leader/follower, tx injection | ✅ `sqlkit` | connection config values |
| Generic CRUD, filtering, pagination | ✅ `repository`, `repository/sql` | entity models, allowed filter/sort fields |
| Structured logging, context keys | ✅ `logger`, `ctxkit` | what to log and at which layer |
| Config loading (Viper/YAML/`.env`) | ✅ `config` | the app `Config` tree and its values |
| Soft-delete/audit | ❌ | ✅ `internal/core/audit` |
| Domain models, business rules, endpoints | ❌ | ✅ `internal/features/*` |

**Rule of thumb:** if a capability is a reusable cross-cutting concern, it belongs in `go-sdk` (see its [DEVELOPMENT_PLAN.md](../../go-sdk/docs/DEVELOPMENT_PLAN.md)); if it encodes *this domain*, it belongs here.

## Key behaviours to know

- **Soft delete** — the `internal/core/audit` decorator wraps the SQL repository, injecting `deleted_at IS NULL` into list/count filters and stamping `created_at`/`updated_at`/`deleted_at` on writes. Deletes are soft. See [DATABASE.md](DATABASE.md).
- **Handler adapter** — handlers are `func(*http.Request) (any, error)` wrapped by `handler.Handle`; return `response.OK/Created/NoContent(...)` on success, an `errorz` error on failure. No `http.ResponseWriter` boilerplate.
- **List queries** — parsed against a per-endpoint **allow-list** of sort/filter fields (rejecting unknown fields with a 400), then translated to `repository.ListOptions`. This parsing is being centralized in `internal/core` to remove per-feature duplication.
