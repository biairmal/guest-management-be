# Configuration

The enforceable rules are in [../AGENTS.md](../AGENTS.md#configuration); this document explains the pattern and how to extend it.

## The idea: one orchestrated `Config`, embedding go-sdk configs

Configuration is **struct-first, YAML-first**. There is a single root [`config.Config`](../internal/config/config.go) that **embeds `go-sdk` config structs** for each infrastructure concern plus app-specific sections. The whole tree is populated from one YAML file + `.env` via `go-sdk`'s `config.Load` (Viper + mapstructure) in [`cmd/api/main.go`](../cmd/api/main.go):

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
    Logger   logger.Options   // go-sdk
    Server   ServerConfig      // app-specific (host/port/timeouts)
    Database sqlkit.Config     // go-sdk
    Redis    redis.Config      // go-sdk
    Swagger  SwaggerConfig     // app-specific
}
```

Because each embedded type is a `go-sdk` `Config`/`Options` with its own `mapstructure` tags, defaults, and `Validate()`, the app config tree is assembled from building blocks — the app doesn't re-describe how to configure a DB pool or a logger.

## Sources & precedence

1. **`.env`** — loaded first (via `config.EnvFile`); provides secrets/host values as environment variables.
2. **`configs/config.yaml`** — the structured config; values reference env vars with **`${VAR}` / `${VAR:default}`** substitution.
3. Later files passed to `config.Files(...)` override earlier ones.

Example (`configs/config.yaml`):

```yaml
Database:
  Leader:
    Host: ${DATABASE_HOST}
    Port: ${DATABASE_PORT}
    SSLMode: ${DATABASE_SSL_MODE:disable}      # default when env unset
    ConnectTimeout: ${DATABASE_CONNECT_TIMEOUT:5s}   # time.Duration decodes out of the box
```

`time.Duration` (`"5s"`) and comma-slices decode without extra wiring — `config.Load` uses Viper's default unmarshal hooks.

## Adding a config section

To make a new concern configurable:

1. **Add a field** to `config.Config` — an embedded `go-sdk` `Config` (preferred, if the SDK owns the concern) or a new app struct.
2. For an **app-specific** struct, give every serializable field a `mapstructure:"snake_case"` tag; tag non-serializable fields (funcs, interfaces, live clients) `mapstructure:"-"`. Provide a `DefaultConfig()` and a `Validate() error` (return an `errorz` error), and call `Validate()` where the value is consumed.
3. **Add a YAML block** and any `${VAR}` references; document new vars in `.env.example`.
4. **Consume it in `main.go`/`app`** by reading from `cfg`, never by hardcoding.

## Rules & current gaps (see DEVELOPMENT_PLAN)

- **No hardcoded runtime values.** Server host/port/timeouts live in `internal/config.ServerConfig` (`configs/config.yaml`'s `Server:` block), consumed via `cfg.Server.Addr()` in `main.go` — never hardcode `"127.0.0.1:8080"` or timeouts again.
- **No dangling keys.** Every top-level `config.yaml` key maps to a real, populated struct.
- **Redis is wired.** `internal/config.Config` embeds `redis.Config` (`Redis:` block in `config.yaml`); `main.go` constructs a client via `redis.NewClient(&cfg.Redis)`. No feature consumes it yet — that lands with the first caching/session need.
- **Secrets stay in `.env`**, never committed; `configs/config.yaml` references them via `${VAR}`.
