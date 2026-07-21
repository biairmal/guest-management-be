# Testing

The enforceable rules are in [../AGENTS.md](../AGENTS.md#tests); this document is the how-to. The convention matches `go-sdk` exactly so the two codebases feel identical to work in.

> **Current state:** the repo has **zero tests** despite full test tooling — `make ci` is green only because `go test ./...` passes vacuously on packages with no tests. Establishing the pattern below is the most urgent item in [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md).

## Rules at a glance

- **Stdlib `testing` for the harness; `gomock` for collaborators.** No testify, no assert libraries. Assertions are hand-written `if got != want { t.Errorf(...) }`; collaborators are stubbed with **generated mocks**, never hand-written fakes.
- **Table-driven.** `[]struct{ name string; …; want… }` iterated with `t.Run(tt.name, …)`.
- **Same package.** `package events`, not `events_test`, so you can test unexported helpers freely.
- **File naming: `*__test.go`** (double underscore) — matches `go-sdk`'s convention.
- **Mocks are generated, never hand-written.** See [Mocks](#mocks-generated-not-hand-written) below. A hand-rolled fake rots the moment an interface gains a method; generated mocks regenerate and stay in sync.
- **New behaviour ships with tests in the same change**; rerun `make mocks` when a mocked interface changes.

## Mocks: generated, not hand-written

Mocks are `gomock` (`go.uber.org/mock`) and always **generated**, never hand-written. There are two sources.

### 1. go-sdk interfaces → the go-sdk `mocks` module

`go-sdk` ships generated mocks in a **nested module** `github.com/biairmal/go-sdk/mocks` (its own go.mod, so `go.uber.org/mock` never leaks into the SDK itself):

| Interface | Mock package | Type |
|---|---|---|
| `repository.Repository[T,ID]` (+ Read/Write/Transactional) | `github.com/biairmal/go-sdk/mocks/repository` (`mockrepository`) | `MockRepository[T,ID]` |
| `logger.Logger` | `.../mocks/logger` (`mocklogger`) | `MockLogger` |
| `redis.Client`, `redis.Pipeliner` | `.../mocks/redis` (`mockredis`) | `MockClient`, `MockPipeliner` |

Consume them by adding to **this repo's** `go.mod` (test-only dependency):

```gomod
require github.com/biairmal/go-sdk/mocks v0.0.0
replace github.com/biairmal/go-sdk/mocks => ../go-sdk/mocks
```

`go.uber.org/mock` becomes a test dependency of this service — that's fine; it's a test tool, not runtime.

> This is a key reason to complete the [A2 refactor](DEVELOPMENT_PLAN.md): once a feature repository is just
> `repository.Repository[EventCategory, uuid.UUID]` (no bespoke wrapper interface), the **go-sdk-generated
> `mockrepository.MockRepository` mocks it directly** — zero app mock code.

### 2. App-defined interfaces → generate the same way

For interfaces this service defines (feature **services**, and any app-specific interface), **do not hand-write a fake** — mirror `go-sdk`'s setup ([scripts/mocks.mk](../../go-sdk/scripts/mocks.mk)):

1. Add a `//go:generate` directive next to the interface:
   ```go
   //go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../mocks/events/mock_service.go -package=mockevents github.com/biairmal/guest-management-be/internal/features/events CategoryService
   ```
2. Generate into a nested `mocks/` module (own `go.mod` with `replace github.com/biairmal/guest-management-be => ../`) so `go.uber.org/mock` stays out of the main module.
3. Add a `make mocks` target (copy `go-sdk/scripts/mocks.mk`) and run it. Setting this up is item **A5** in [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md).

## Unit test template

Modelled on `go-sdk`'s `lib/errorz/error__test.go`, using a generated mock for the repository. Assumes the
[A2 refactor](DEVELOPMENT_PLAN.md) so the service depends on `repository.Repository[EventCategory, uuid.UUID]`:

```go
package events

import (
    "context"
    "testing"

    "github.com/biairmal/go-sdk/lib/logger"
    "github.com/biairmal/go-sdk/lib/repository"
    mockrepository "github.com/biairmal/go-sdk/mocks/repository"
    "github.com/google/uuid"
    "go.uber.org/mock/gomock"
)

func TestCategoryService_GetByID(t *testing.T) {
    tests := []struct {
        name     string
        repoRes  *EventCategory
        repoErr  error
        wantCode string // "" == success
    }{
        {name: "found", repoRes: &EventCategory{Name: "x"}},
        {name: "not found → 404", repoErr: repository.ErrNotFound, wantCode: errorz.CodeNotFound},
        {name: "db error → 500", repoErr: errors.New("boom"), wantCode: errorz.CodeInternal},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctrl := gomock.NewController(t) // auto-Finish in Go 1.14+; no defer needed
            repo := mockrepository.NewMockRepository[EventCategory, uuid.UUID](ctrl)
            repo.EXPECT().
                GetByID(gomock.Any(), gomock.Any()).
                Return(tt.repoRes, tt.repoErr)

            svc := NewCategoryService(logger.NewNoOp(), repo) // NoOp logger: nothing to stub
            _, err := svc.GetByID(context.Background(), uuid.New())
            assertErrorzCode(t, err, tt.wantCode)
        })
    }
}
```

Notes:
- **Logger:** use `logger.NewNoOp()` (a real SDK no-op), not `mocklogger.MockLogger` — there's nothing to assert, and a strict mock would fail on unexpected calls. Reach for `mocklogger` only when you assert log calls.
- **`gomock` is strict:** every method the code-under-test calls must have a matching `EXPECT()`. Use `.Times(n)`, `.AnyTimes()`, and `gomock.Any()` / `gomock.Eq(...)` matchers to scope expectations.

## Small assertion helpers

Keep them local; no library:

```go
// assertErrorzCode fails unless err carries the wanted errorz code (or is nil when want == "").
func assertErrorzCode(t *testing.T, err error, want string) {
    t.Helper()
    if want == "" {
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        return
    }
    var e *errorz.Error
    if !errors.As(err, &e) {
        t.Fatalf("expected *errorz.Error, got %T: %v", err, err)
    }
    if e.Code != want {
        t.Errorf("code = %q, want %q", e.Code, want)
    }
}
```

## Unit vs integration

- **Unit** (`make test-unit` → `go test -short ./...`): no live dependencies. Services against generated mock repositories, query parsers against `url.Values`, handlers against generated mock services.
- **Integration** (`make test-integration` → `go test ./...`): needs a live Postgres. Put these in `*_integration_test.go` and skip under `-short`:

  ```go
  func TestCategoryRepository_Integration(t *testing.T) {
      if testing.Short() {
          t.Skip("integration test: requires a live Postgres")
      }
      // ... build sqlkit.DB against the test database, exercise real SQL
  }
  ```

## What to cover first (priority)

1. **Service error-translation branches** — every sentinel → `errorz` code mapping, per feature. This is where bugs hide.
2. **Query parser** — allowed vs rejected sort/filter fields, size clamping, bad `page`/`size`, sort direction parsing.
3. **Validation** — required/format failures produce a `CodeBadRequest`.
4. **Business invariants** in services (e.g. source/tenant_id rules).

## Coverage

`make coverage` writes `out/coverage.{out,html}`; `make coverage-view` opens the report. Aim to make `make ci` **meaningful** — the coverage step should reflect real tests, not vacuous 0%. Prioritize the service and query layers (highest branch density) over trivial getters.
