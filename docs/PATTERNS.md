# Patterns

Copy-paste templates for the conventions required by [../AGENTS.md](../AGENTS.md). Each snippet is modelled on the real `events` feature in this repository — follow these shapes rather than inventing new ones.

> Where a snippet differs from current `events` code, the snippet is the **target** pattern and the difference is called out. The `events` slice predates these guidelines and has known issues (empty `Options{}`, a pass-through repository, service-level field validation, DTOs inline in the service file, a hand-rolled `listParamsToListOptions`, an unsplit multi-entity package) tracked in [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md).

## Feature slice layout

A feature is a package under `internal/features/<feature>` with one file per concern:

```
internal/features/<feature>/
  <entity>_model.go         # struct with db + json tags, TableName()
  <entity>_repository.go    # typed repository over go-sdk generic repo + audit decorator
  <entity>_dto.go           # input/output DTOs (CreateInput, UpdateInput, ...) with validate tags
  <entity>_service.go       # XService interface + implementation only — business rules, error translation
  <entity>_handler.go       # func(*http.Request)(any,error) handlers + Swagger annotations
  <entity>_routes.go        # InitXRoutes(r, handler) route registration
  <entity>_service__test.go
  <feature>_constants.go    # exported constants (permission codes, enum-like string values, ...)
```

List endpoints declare their `query.ListParseConfig` allow-list as a var in `<entity>_handler.go` (see [List query — allow-list parsing](#list-query--allow-list-parsing)); the shared parser lives in `internal/core/query`, so no per-feature `_query.go` file is needed.

### Multi-entity features: split by entity, not by layer

A feature slice that models more than one distinct entity (e.g. `events`: category, event, workflow step, workflow step template) splits into per-entity subpackages, each following the layout above:

```
internal/features/events/
  config.go                 # stays at the feature root — aggregates every entity's RepositoryConfig
  category/
    category_model.go
    category_repository.go
    category_dto.go
    category_service.go
    category_handler.go
    category_routes.go
    category_service__test.go
  event/
    event_model.go
    ...
  workflowstep/              # package name drops the underscore (Go convention); files keep workflow_step_*.go
    workflow_step_model.go
    ...
  workflowsteptemplate/
    workflow_step_template_model.go
    ...
```

Split **by entity, never by layer** — no `service`/`handler`/`repository` subpackages inside a slice. Entities in a feature don't call each other, so an entity subpackage introduces no new cross-package coupling; a layer subpackage would force unexported types like `categoryServiceImpl` to become exported just to cross the new boundary, and invites import cycles between the layer packages. A single-entity feature (`staffing`, `tenants`, `users`, `auth`) stays flat — don't split what isn't messy.

## Model

Models carry both `db` tags (for `repository/sql` reflection) and `json` tags (for the API), plus a `swagger:model` marker. Modelled on [`events/category_model.go`](../internal/features/events/category_model.go):

```go
// EventCategory represents a row in the event_categories table.
// Supports soft delete via deleted_at.
//
// swagger:model EventCategory
type EventCategory struct {
    ID        uuid.UUID  `json:"id" db:"id"`
    Source    string     `json:"source" db:"source"`
    TenantID  *uuid.UUID `json:"tenant_id,omitempty" db:"tenant_id"`
    Name      string     `json:"name" db:"name"`
    CreatedAt time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
    DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

func (EventCategory) TableName() string { return "event_categories" }
```

## Repository — thin, typed, no pass-through

Construct the `go-sdk` generic SQL repository via `internal/core/repository.NewRepository`, which wraps it in the audit decorator and — when `cacheOpts.Enabled` and a Redis client are available — the go-sdk cache decorator too. **Return the typed generic interface directly**. Do **not** hand-write a wrapper that forwards every method and re-widens the ID to `any`.

```go
const eventCategoriesTable = "event_categories"

// NewCategoryRepository returns a soft-delete-aware, optionally cached
// repository for event categories. TID is uuid.UUID — keep it typed; never
// widen to `any`. cacheOpts is resolved and threaded through by
// internal/app/repository.go, from Config.App.Events.Repository.CategoryCache
// + the Redis client — this constructor and main.go don't know about config.
func NewCategoryRepository(
    log logger.Logger, db *sqlkit.DB, cacheOpts corerepository.CacheOptions,
) repository.Repository[EventCategory, uuid.UUID] {
    return corerepository.NewRepository[EventCategory, uuid.UUID](
        log, db, eventCategoriesTable,
        []string{"id", "source", "tenant_id", "name", "created_at", "updated_at", "deleted_at"},
        cacheOpts,
    )
}
```

Caching is configured **per repository**, not with one app-wide switch: each feature embeds a `corerepository.CacheConfig` per repository on its `RepositoryConfig` (e.g. `events.Config.Repository.CategoryCache`), enabled by default. A feature's `Config` splits by layer the same way its code does (`RepositoryConfig` today; `ServiceConfig`/`HandlerConfig` get added only once one of those layers has a real field — an empty layer struct violates the no-empty-`Options{}` rule below). See [CONFIGURATION.md](CONFIGURATION.md#cache) for the full shape and the knobs (`enabled`, `ttl`, `prefix`, `strategy`).

> **Anti-pattern (current `events/category_repository.go`, to be removed):** a `categoryRepo` struct whose methods are `return r.repo.X(...)` pass-throughs, with `idStr, _ := id.(string)` silently swallowing a bad ID type. Banned by [AGENTS.md](../AGENTS.md#repository-pattern--anti-duplication). When you need a shared CRUD base, use the `internal/core` base helper (see [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md)), not a per-feature forwarder.

## DTOs — input/output shapes

Input/output DTOs live in `<entity>_dto.go`, not inline in the service file — the service file holds only the `XService` interface and its implementation. Modelled on a `category_dto.go` split out of [`events/category_service.go`](../internal/features/events/category_service.go):

```go
// CreateInput is the input for creating an event category.
//
// swagger:model CreateInput
type CreateInput struct {
    Source   string     `json:"source"    validate:"required,oneof=app tenant"`
    TenantID *uuid.UUID `json:"tenant_id" validate:"omitempty"`
    Name     string     `json:"name"      validate:"required"`
}
```

`validate:"..."` tags drive boundary validation (see [Request validation](#request-validation-boundary)); the service only enforces cross-field business invariants a struct tag can't express.

## Service — business rules & error translation

The service speaks domain types + `context.Context`, never HTTP. It translates repository sentinels into `errorz` codes and logs internal errors once. Modelled on [`events/category_service.go`](../internal/features/events/category_service.go):

```go
type Service interface {
    Create(ctx context.Context, in CreateInput) (*EventCategory, error)
    GetByID(ctx context.Context, id uuid.UUID) (*EventCategory, error)
    // ...
}

func (s *categoryService) Create(ctx context.Context, in CreateInput) (*EventCategory, error) {
    // Business-rule invariants live here (NOT field-presence checks — those are
    // validated at the HTTP boundary; see "Request validation" below).
    if in.Source == SourceApp && in.TenantID != nil {
        return nil, errorz.BadRequest().WithMessage("tenant_id must be null when source is 'app'")
    }

    entity := &EventCategory{ID: uuid.New(), Source: in.Source, TenantID: in.TenantID, Name: in.Name}

    if err := s.repo.Create(ctx, entity); err != nil {
        if errors.Is(err, repository.ErrAlreadyExists) {
            return nil, errorz.Conflict().WithMessage("event category already exists")
        }
        s.logger.ErrorWithContext(ctx, "event category create failed", logger.F("error", err))
        return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to create event category")
    }
    s.logger.InfoWithContext(ctx, "event category created", logger.F("id", entity.ID))
    return entity, nil
}

func (s *categoryService) List(
    ctx context.Context, params *query.ListParams,
) (*common.PageResponse[EventCategory], error) {
    // Enforce the allow-list here too, not just in the handler — see
    // "List query — allow-list parsing and enforcement" below for why.
    if err := query.ValidateListParams(params, CategoryListConfig); err != nil {
        return nil, errorz.BadRequest().WithMessage(err.Error())
    }
    // Reuse the shared conversion — never hand-roll this per feature.
    items, total, err := s.repo.List(ctx, query.ToListOptions(params))
    if err != nil {
        s.logger.ErrorWithContext(ctx, "event category list failed", logger.F("error", err))
        return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to list event categories")
    }
    return common.NewPageResponse(items, total, params.Page, params.Size), nil
}
```

Key rules: return `error` (not `*errorz.Error`); compare sentinels with `errors.Is`; wrap the cause on the internal path.

## Constants

Exported constants — permission codes, enum-like string values (`SourceApp`/`SourceTenant`), etc. — live in a single `<feature>_constants.go` (or `<entity>_constants.go` inside a per-entity subpackage), not scattered across model/service files. Modelled on `staffing`'s permission code:

```go
// staffing_constants.go

// PermissionManageStaff is the permission code required to assign/remove/update
// event staff. Gated on every route in assignment_routes.go via authz.RequirePermission.
const PermissionManageStaff = "manage_staff"
```

Permission codes stay feature-owned — `internal/core/authz` provides the generic `PermissionResolver`/`Checker`/`RequirePermission` mechanism only, never the codes themselves.

## Handler — go-sdk adapter + Swagger

Handlers are `func(*http.Request) (any, error)`; parse input, call the service, wrap the result. Every handler carries Swagger annotations. Modelled on [`events/category_handler.go`](../internal/features/events/category_handler.go):

```go
// Create godoc
//
//	@Summary		Create event category
//	@Tags			event-categories
//	@Accept			json
//	@Produce		json
//	@Param			body	body		events.CreateInput	true	"Event category payload"
//	@Success		201		{object}	events.EventCategory
//	@Failure		400		{object}	object	"Invalid request body or validation error"
//	@Failure		409		{object}	object	"Conflict"
//	@Router			/api/v1/event-categories [post]
func (h *Handler) Create(r *http.Request) (any, error) {
    var body CreateInput
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        return nil, errorz.BadRequest().WithMessage("invalid request body")
    }
    if err := h.validator.Struct(body); err != nil { // boundary validation → 400 w/ field detail
        return nil, err
    }
    entity, err := h.service.Create(r.Context(), body)
    if err != nil {
        return nil, err
    }
    return response.Created(entity), nil
}
```

Routes register the adapter (modelled on [`events/category_routes.go`](../internal/features/events/category_routes.go)):

```go
func InitCategoryRoutes(r chi.Router, h *Handler) {
    r.Route("/api/v1/event-categories", func(r chi.Router) {
        r.Get("/", handler.Handle(h.List))
        r.Post("/", handler.Handle(h.Create))
        r.Get("/{id}", handler.Handle(h.GetByID))
        r.Put("/{id}", handler.Handle(h.Update))
        r.Delete("/{id}", handler.Handle(h.Delete))
    })
}
```

> No empty `Options{}` struct threaded through these constructors — add a struct only when it holds a real field.

## Self-service vs. admin routes — the `/me` sub-resource pattern

Two different questions get two different route shapes — never one route with a permission branch inside
the handler/service:

- **"Act on my own resource"** (self-service — e.g. change my own password) → a `/me/...` sub-resource under
  the feature's base path (`POST /api/v1/users/me/password`). The service resolves the target identity
  **server-side** from the authenticated context (`authz.UserIDFromContext(ctx)`) — never from a
  caller-supplied `{id}` or body field, so the frontend cannot name a different user's id even by accident.
  Requires only a valid token; no extra permission check, since the caller can only ever act on themselves.
- **"Act on someone else's resource"** (admin/permissioned — e.g. an admin changing another user's password)
  → the existing `{id}` pattern (`POST /api/v1/users/{id}/password`), gated by the feature's permission
  (`RequirePermission(checker, PermissionX)`) at the route-group level, same as every other admin route in
  that group.

Give each shape its own route, handler entry point, and service method — not one route with an
`if id == callerID` branch inside it. A shared branch tangles two different authorization stories (resolve
from context vs. resolve from param; no permission vs. permission-gated) into one code path for no benefit;
splitting by route makes each one a plain route-level concern, consistent with every other gated route in
this codebase. Where the two share real logic (e.g. the actual hash-and-update), factor that into a small
unexported helper both call — don't duplicate it, don't force it through one public method's branch either.

**Chi routing:** a static `/me` segment and a sibling `{id}` param at the same path depth do not collide.
Chi v5's radix tree (`go-chi/chi/v5`, `tree.go`) tries static (`ntStatic`) children before param (`ntParam`)
children at the same position, regardless of registration order — `me` is always matched as a literal
segment first. No explicit route-registration-order workaround is needed.

**Scope discipline:** this pattern applies to *any* self-service action on your own user record (password,
profile, notification preferences, ...), but don't scaffold a route for one ahead of an actual story. Add
the specific `/me/...` route, following this shape, when that story lands — not before.

## Request validation (boundary)

Shape/format validation is driven by the `validate:"..."` tags on the input DTO (see [DTOs](#dtos--inputoutput-shapes)) and runs in the handler via the shared validator, **not** by hand-written `if x == ""` in the service. The service then only enforces **cross-field business invariants** (e.g. "tenant_id required when source is tenant") that a struct tag can't express cleanly.

## List query — allow-list parsing and enforcement

List endpoints declare their allowed sort/filter fields as a `query.ListParseConfig`, and it's enforced **twice, at two different layers, for two different reasons**:

1. **HTTP parsing (handler, fail fast)** — [`internal/core/query`](../internal/core/query/list.go)'s `ParseListParams(q url.Values, cfg)` is an HTTP-only adapter: it parses the query string into a `*query.ListParams` and rejects a disallowed field immediately with a `400`, so an HTTP caller doesn't round-trip to the service for a bad request. Pagination defaults (`DefaultPage`/`DefaultSize`/`MaxSize`) fall back to the package-level defaults when left zero.
2. **Service enforcement (transport-agnostic, authoritative)** — `query.ValidateListParams(params *query.ListParams, cfg) error` runs the same allow-list check against an already-built `*ListParams`, regardless of what produced it. The **service** calls this, not just the handler, because HTTP is not the only transport that will ever call it — a future gRPC handler or subscriber builds `*query.ListParams` from its own typed fields, never touches `ParseListParams`, and must still be unable to sort/filter on a field the entity doesn't allow.

This isn't duplication to clean up — it's the same category as client-side + server-side validation: one copy for fast feedback at the edge, one copy that's the actual guarantee no matter who calls in.

The `ListParseConfig` value itself is declared **once**, exported from the service file (the service is the authority enforcing it), and the HTTP handler imports that same value rather than declaring its own copy. Modelled on `events/category`:

```go
// category_service.go — the service owns and enforces the allow-list
var CategoryListConfig = query.ListParseConfig{
    AllowedSortFields:   []string{"id", "source", "tenant_id", "name", "created_at", "updated_at"},
    AllowedFilterFields: []string{"name", "source", "tenant_id"},
}

func (s *categoryServiceImpl) List(
    ctx context.Context, params *query.ListParams,
) (*common.PageResponse[EventCategory], error) {
    if err := query.ValidateListParams(params, CategoryListConfig); err != nil {
        return nil, errorz.BadRequest().WithMessage(err.Error())
    }
    items, total, err := s.repo.List(ctx, query.ToListOptions(params))
    // ...
}
```

```go
// category_handler.go — reuses the service's config, does not declare its own
params, err := query.ParseListParams(r.URL.Query(), events.CategoryListConfig)
```

`ParseListParams`, `ValidateListParams`, and `ToListOptions` together mean a feature never needs its own `XxxListParams` type, list-parsing code, or `listParamsToListOptions` conversion — only the one allow-list config above, declared once.

`ParseListParams` returns `*query.ListParams` (embeds `common.BasePageRequest` + `Filters map[string]query.FilterValue`) directly — a feature does not need its own `XxxListParams` type or `ParseXxxListParams` wrapper function. A filter value carries its operator inline as `value;operator` (e.g. `?name=Ali;like`); no suffix defaults to exact match. `ToListOptions` wraps a `like` value `%value%` automatically.

## Table-driven test

Stdlib `testing`, same-package, `*__test.go`, **generated `gomock` mocks** for collaborators (never hand-written fakes). For go-sdk interfaces use the `github.com/biairmal/go-sdk/mocks/*` module; for app interfaces generate app-side mocks. See [TESTING.md](TESTING.md) for the full convention and go.mod wiring.

```go
func TestCategoryService_Create(t *testing.T) {
    tests := []struct {
        name    string
        in      CreateInput
        repoErr error // repo.Create return; nil = not expected to be called
        expects bool  // whether Create is reached (invariant failures short-circuit)
        wantErr string // expected errorz code, "" for success
    }{
        {name: "app source rejects tenant_id", in: CreateInput{Source: SourceApp, TenantID: ptr(uuid.New()), Name: "x"}, wantErr: errorz.CodeBadRequest},
        {name: "conflict maps to 409", in: CreateInput{Source: SourceApp, Name: "x"}, expects: true, repoErr: repository.ErrAlreadyExists, wantErr: errorz.CodeConflict},
        {name: "happy path", in: CreateInput{Source: SourceApp, Name: "x"}, expects: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctrl := gomock.NewController(t)
            repo := mockrepository.NewMockRepository[EventCategory, uuid.UUID](ctrl)
            if tt.expects {
                repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tt.repoErr)
            }
            svc := NewService(logger.NewNoOp(), repo)
            _, err := svc.Create(context.Background(), tt.in)
            assertErrorzCode(t, err, tt.wantErr)
        })
    }
}
```
