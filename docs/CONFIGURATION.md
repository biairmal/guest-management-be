# Configuration

The enforceable rules are in [../AGENTS.md](../AGENTS.md#configuration); this document explains the pattern and how to extend it.

## The idea: one orchestrated `Config`, embedding go-sdk configs

Configuration is **struct-first, YAML-first**. There is a single root [`config.Config`](../internal/config/config.go) that **embeds `go-sdk` config structs** for each infrastructure concern, app-specific sections, and one `App` field ([`FeatureConfig`](../internal/config/app.go)) that aggregates every registered feature's own config under the `app:` YAML key. The whole tree is populated from one YAML file + `.env` via `go-sdk`'s `config.Load` (Viper + mapstructure) in [`cmd/api/main.go`](../cmd/api/main.go):

```go
var cfg appconfig.Config
err := config.Load(&cfg,
    config.EnvFile(".env"),
    config.Files("configs/config.yaml"),
)
```

The current root struct:

```go
type Config struct {
    Logger    logger.Options   // go-sdk
    Server    ServerConfig      // app-specific (host/port/timeouts)
    Database  sqlkit.Config     // go-sdk
    Redis     redis.Config      // go-sdk
    Validator validator.Config  // go-sdk
    Swagger   SwaggerConfig     // app-specific
    App       FeatureConfig     // app.<feature>.* — every registered feature's own config
}

// FeatureConfig aggregates per-feature config, one field per feature.
type FeatureConfig struct {
    Events events.Config `mapstructure:"events"` // internal/features/events
}
```

Within a feature's own `Config`, settings are further split **by layer** (`app.<feature>.<layer>.*`), so a feature can later be lifted into its own service with its config already separated the same way its code is (handler/service/repository):

```go
// internal/features/events/config.go
type Config struct {
    Repository RepositoryConfig `mapstructure:"repository"`
    // Service, Handler: added when they have a real field to hold.
}
```

Because each embedded type is a `go-sdk` `Config`/`Options` with its own `mapstructure` tags, defaults, and `Validate()`, the app config tree is assembled from building blocks — the app doesn't re-describe how to configure a DB pool or a logger.

## Sources & precedence

1. **`.env`** — loaded first (via `config.EnvFile`); provides secrets/host values as environment variables.
2. **`configs/config.yaml`** — the structured config; values reference env vars with **`${VAR}` / `${VAR:default}`** substitution.
3. Later files passed to `config.Files(...)` override earlier ones.

Example (`configs/config.yaml`):

```yaml
database:
  leader:
    host: ${DATABASE_HOST}
    port: ${DATABASE_PORT}
    ssl_mode: ${DATABASE_SSL_MODE:disable}      # default when env unset
    connect_timeout: ${DATABASE_CONNECT_TIMEOUT:5s}   # time.Duration decodes out of the box
```

`time.Duration` (`"5s"`) and comma-slices decode without extra wiring — `config.Load` uses Viper's default unmarshal hooks.

## Adding a config section

To make a new concern configurable:

1. **Cross-cutting infra concern** (owned by go-sdk or genuinely app-wide, like `Server`/`Swagger`): add a field to `config.Config` directly.
2. **Feature-owned concern** (e.g. a repository's cache policy — see [Cache](#cache)): add a field to the relevant **layer** struct on that feature's own `Config` type in `internal/features/<feature>/config.go` (`RepositoryConfig` today; add `ServiceConfig`/`HandlerConfig` the first time one of those layers needs a real setting — **not** before, per the no-empty-`Options{}` rule below). Register the feature in [`FeatureConfig`](../internal/config/app.go) if it isn't there yet. Never add feature-specific fields to the root `Config` directly — they nest under `App` instead.
3. For any new struct, give every serializable field a `mapstructure:"snake_case"` tag; tag non-serializable fields (funcs, interfaces, live clients) `mapstructure:"-"`. Provide a `DefaultConfig()` and a `Validate() error` (return an `errorz` error), and wire `Validate()` into the parent's `Validate()` so one call at startup (`cfg.App.Validate()`) checks every feature and every layer within it.
4. **Add a YAML block** and any `${VAR}` references; document new vars in `.env.example`.
5. **Consume it in `internal/app`**, not `main.go` — the composition root is the only layer that knows every feature, so it reads `featureConfig.<Feature>.<Layer>.<Field>` itself when wiring that feature (see `internal/app/repository.go`). `main.go` just passes `cfg.App` through untouched.

## Rules & current gaps (see DEVELOPMENT_PLAN)

- **No hardcoded runtime values.** Server host/port/timeouts live in `internal/config.ServerConfig` (`configs/config.yaml`'s `Server:` block), consumed via `cfg.Server.Addr()` in `main.go` — never hardcode `"127.0.0.1:8080"` or timeouts again.
- **No dangling keys.** Every top-level `config.yaml` key maps to a real, populated struct.
- **Redis is wired.** `internal/config.Config` embeds `redis.Config` (`Redis:` block in `config.yaml`); `main.go` constructs a client via `redis.NewClient(&cfg.Redis)`, which feeds the cache decorator described below.
- **Secrets stay in `.env`**, never committed; `configs/config.yaml` references them via `${VAR}`.

## Cache

Repository caching is **per repository**, not one app-wide switch: `internal/core/repository.CacheConfig` is a reusable, YAML-decodable shape (`Enabled`, `TTL`, `Prefix`, `Strategy`) that a **feature embeds once per repository** it wants configurable caching for, on its `RepositoryConfig` (the repository-layer slice of that feature's `Config`). This keeps the setting with the feature *and layer* that owns the repository, instead of baking every repository's name into the shared root `Config` schema — the full YAML path is `app.<feature>.repository.<cache field>`.

For `events`, that's [`internal/features/events.Config`](../internal/features/events/config.go):

```go
type Config struct {
    Repository RepositoryConfig `mapstructure:"repository"`
}

type RepositoryConfig struct {
    CategoryCache corerepository.CacheConfig `mapstructure:"category_cache"`
}
```

registered as `FeatureConfig.Events` (see [`internal/config/app.go`](../internal/config/app.go)), giving the full YAML path `app.events.repository.category_cache`. Unlike the infra sections above, these values are set directly in `config.yaml` rather than via `${VAR}` — there's no operational need to override them per-environment yet:

```yaml
app:
  events:
    repository:
      category_cache:
        enabled: true
        ttl: 5m
        prefix: guest-management
        strategy: write_around # write_around, write_through, write_behind
```

- **`enabled`** — caching is **on by default** for this repository; set to `false` to fall back to a plain audit-wrapped repository (e.g. local dev without Redis) without touching code.
- **`ttl`** — how long a cached entity survives before the next read repopulates it from the database.
- **`prefix`** — namespaces this repository's cache keys (`<prefix>:<table>:id:<id>`) so multiple environments/services sharing one Redis instance don't collide.
- **`strategy`** — how writes affect the cache: `write_around` (invalidate, default), `write_through` (re-fetch and re-cache), or `write_behind` (falls back to `write_around`; not yet implemented in go-sdk).

`main.go` validates the whole tree once via `cfg.App.Validate()` (which delegates to `events.Config.Validate()` → `RepositoryConfig.Validate()` → `CategoryCache.Validate()`), then passes `cfg.App` and the Redis client straight into `app.NewApp` — it does **not** know that `events` even has a category cache. `internal/app` — the composition root, the only layer that knows every feature — resolves `featureConfig.Events.Repository.CategoryCache.ToOptions(redisClient)` itself in `initializeRepository`, turning config into runtime `corerepository.CacheOptions{Enabled, Client, TTL, Prefix, Strategy}` right before calling `events.NewCategoryRepository`.

**Registering a new feature or repository:**
1. Add a `CacheConfig` field to the feature's `RepositoryConfig` (new feature: create `internal/features/<feature>/config.go` with a `Config{Repository RepositoryConfig}`).
2. Add that feature's `Config` as a field on `FeatureConfig` in `internal/config/app.go`, and wire its `Validate()` into `FeatureConfig.Validate()`.
3. In `internal/app/repository.go`, resolve the new `CacheConfig` via `.ToOptions(redisClient)` and pass it into the feature's `NewXRepository`.

`main.go` and the root `Config` struct need no changes for either step. The same pattern extends to the service and handler layers — add `ServiceConfig`/`HandlerConfig` to a feature's `Config` (`app.<feature>.service.*` / `app.<feature>.handler.*`) the first time one of them has a real setting to hold; an empty layer struct with no fields is a lint/Definition-of-Done violation (`docs/PATTERNS.md`), so don't pre-create them.
