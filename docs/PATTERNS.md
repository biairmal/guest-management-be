# Patterns

Copy-paste templates for the conventions required by [../AGENTS.md](../AGENTS.md). Each snippet is modelled on the real `events` feature in this repository — follow these shapes rather than inventing new ones.

> Where a snippet differs from current `events` code, the snippet is the **target** pattern and the difference is called out. The `events` slice predates these guidelines and has known issues (empty `Options{}`, a pass-through repository, service-level field validation) tracked in [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md).

## Feature slice layout

A feature is a package under `internal/features/<feature>` with one file per concern:

```
internal/features/<feature>/
  <entity>_model.go        # struct with db + json tags, TableName(), domain consts
  <entity>_repository.go   # typed repository over go-sdk generic repo + audit decorator
  <entity>_service.go      # business rules, input DTOs, error translation
  <entity>_handler.go      # func(*http.Request)(any,error) handlers + Swagger annotations
  <entity>_routes.go       # InitXRoutes(r, handler) route registration
  <entity>_query.go        # list-query allow-list config (until lifted to internal/core)
  <entity>_service__test.go
  <entity>_query__test.go
```

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

Construct the `go-sdk` generic SQL repository, wrap it in the audit decorator, and **return the typed generic interface directly**. Do **not** hand-write a wrapper that forwards every method and re-widens the ID to `any`.

```go
const eventCategoriesTable = "event_categories"

// NewCategoryRepository returns a soft-delete-aware repository for event categories.
// TID is uuid.UUID — keep it typed; never widen to `any`.
func NewCategoryRepository(log logger.Logger, db *sqlkit.DB) repository.Repository[EventCategory, uuid.UUID] {
    sqlRepo := sql.NewSQLRepository[EventCategory, uuid.UUID](
        log, db, eventCategoriesTable,
        sql.WithSelectColumns[EventCategory, uuid.UUID]([]string{
            "id", "source", "tenant_id", "name", "created_at", "updated_at", "deleted_at",
        }),
    )
    return audit.NewAuditableRepository[EventCategory, uuid.UUID](sqlRepo)
}
```

> **Anti-pattern (current `events/category_repository.go`, to be removed):** a `categoryRepo` struct whose methods are `return r.repo.X(...)` pass-throughs, with `idStr, _ := id.(string)` silently swallowing a bad ID type. Banned by [AGENTS.md](../AGENTS.md#repository-pattern--anti-duplication). When you need a shared CRUD base, use the `internal/core` base helper (see [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md)), not a per-feature forwarder.

## Service — business rules & error translation

The service speaks domain types + `context.Context`, never HTTP. It translates repository sentinels into `errorz` codes and logs internal errors once. Modelled on [`events/category_service.go`](../internal/features/events/category_service.go):

```go
type CategoryService interface {
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
        s.log.ErrorWithContext(ctx, "event category create failed", logger.F("error", err))
        return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to create event category")
    }
    s.log.InfoWithContext(ctx, "event category created", logger.F("id", entity.ID))
    return entity, nil
}
```

Key rules: return `error` (not `*errorz.Error`); compare sentinels with `errors.Is`; wrap the cause on the internal path.

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
func (h *CategoryHandler) Create(r *http.Request) (any, error) {
    var body CreateInput
    if err := serializer.ParseJSON(r.Body, &body); err != nil {
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
func InitCategoryRoutes(r chi.Router, h *CategoryHandler) {
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

## Request validation (boundary)

Shape/format validation is driven by `validate:"..."` tags on the input DTO and runs in the handler via the shared validator, **not** by hand-written `if x == ""` in the service:

```go
// CreateInput is the request body for creating an event category.
//
// swagger:model CreateInput
type CreateInput struct {
    Source   string     `json:"source"    validate:"required,oneof=app tenant"`
    TenantID *uuid.UUID `json:"tenant_id" validate:"omitempty"`
    Name     string     `json:"name"      validate:"required"`
}
```

The service then only enforces **cross-field business invariants** (e.g. "tenant_id required when source is tenant") that a struct tag can't express cleanly.

## List query — allow-list parsing

List endpoints declare their allowed sort/filter fields as config and reject anything else with a 400. Modelled on [`events/category_query.go`](../internal/features/events/category_query.go):

```go
var eventCategoryListConfig = listParseConfig{
    DefaultPage: 1, DefaultSize: 20, MaxSize: 100,
    AllowedSortFields:   []string{"id", "source", "tenant_id", "name", "created_at", "updated_at"},
    AllowedFilterFields: []string{"name", "source", "tenant_id"},
}
```

The parser itself (`listParseConfig`, `ParseXxxListParams`) is being lifted to `internal/core` so every feature reuses one implementation. Until then, copy the shape but keep only the config per feature.

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
            svc := NewCategoryService(logger.NewNoOp(), repo)
            _, err := svc.Create(context.Background(), tt.in)
            assertErrorzCode(t, err, tt.wantErr)
        })
    }
}
```
