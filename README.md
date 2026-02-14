# Guest Management — Backend

> 🚧 **Under development** — This project is under active development. APIs and behaviour may change.

Go backend for the Guest Management System. Supports multi-tenancy, events, guests, QR-based tickets, workflow steps, and configurable message templates. Uses PostgreSQL for persistence and Redis for caching/sessions. The HTTP API is built with [go-chi](https://github.com/go-chi/chi) and the shared [go-sdk](https://github.com/biairmal/go-sdk) (middleware, response envelope, error handling). Health and readiness endpoints are provided.

---

## Overview

The application serves a REST API for managing tenants, events, event categories, guests, ticket types, workflow steps, and related data. All tenant-scoped data is isolated by `tenant_id`. The API exposes CRUD and listing operations for core domains; response format and error envelope are described in the [go-sdk httpkit README](../go-sdk/httpkit/README.md).

---

## Features

- **Multi-tenancy** — Tenant-scoped data and configuration; isolation enforced in the application layer.
- **Events & categories** — Events belong to a tenant and an event category; categories can be app-level or tenant-level.
- **Event categories** — CRUD and list for event categories (source: app or tenant).
- **Workflow steps** — Category-level templates and event-level workflow steps (e.g. check-in, photo booth).
- **Tickets & guests** — QR-based tickets per event/ticket type; guests with RSVP status and optional ticket assignment.
- **Roles & permissions** — Role-based access; permissions configurable via system tables.
- **Message templates** — Email/WhatsApp templates at app, tenant, or event scope.
- **Health & readiness** — `GET /health` (liveness), `GET /ready` (readiness); ready checker can be wired to DB/Redis.
- **API documentation** — OpenAPI/Swagger generated from code annotations; optional Basic Auth when enabled.

Further API details are available via the in-app Swagger UI when enabled (see [Swagger](#swagger)).

---

## Tech Stack

| Layer        | Technology |
| ------------ | ---------- |
| Language     | Go 1.25+   |
| Router       | [go-chi](https://github.com/go-chi/chi) |
| Middleware   | [go-sdk httpkit](https://github.com/biairmal/go-sdk) (Recover, RequestID, Logging) |
| Database     | PostgreSQL 13+ |
| Cache/Session| Redis 7    |
| Migrations   | [golang-migrate](https://github.com/golang-migrate/migrate) |
| API docs     | [swaggo/swag](https://github.com/swaggo/swag) (OpenAPI 2.0) |
| Config       | Viper (YAML + env); [go-sdk config](https://github.com/biairmal/go-sdk) |

---

## Prerequisites

Before building or running the application, ensure the following are installed and available on your PATH:

| Requirement        | Purpose |
| ------------------ | ------- |
| **Go 1.25.1+**     | Build and run the application. Check with `go version`. |
| **Docker & Docker Compose** | Run PostgreSQL and Redis locally (see [Development with Docker](#development-with-docker)). |
| **Make**           | Run targets for build, migrations, Swagger, and tooling. On Windows, use Git Bash, WSL, or [GnuWin32 Make](http://gnuwin32.sourceforge.net/packages/make.htm). |

Optional (install via `make install-tools` when needed):

- **golang-migrate** — Apply or roll back database migrations (`make migration-up`, etc.).
- **swag** — Generate Swagger/OpenAPI docs from code annotations (`make swagger-generate`).

Ensure the **go-sdk** dependency is available. This project uses a local replace in `go.mod` pointing to `../go-sdk`; the `go-sdk` directory must exist at the repository root (sibling to `guest-management-be`).

---

## How to Develop

### Development with Docker

Start PostgreSQL, Redis, and Redis Insight for local development:

```bash
docker compose up -d
```

- **PostgreSQL:** `localhost:5432`, database `guest_management`, user `postgres`, password `postgres`.
- **Redis:** `localhost:6379`. Default password: `redis` (set via `REDIS_PASSWORD` in `.env` or docker-compose).
- **Redis Insight (GUI):** http://localhost:5540 — add Redis with host **`redis`** (service name), port `6379`, username `default`, password as above (use `redis` when connecting from inside Docker).

Stop services:

```bash
docker compose down
```

Data is persisted in Docker volumes (`postgres_data`, `redis_data`). Use `docker compose down -v` to remove volumes and reset data.

### Database migrations

Migrations live in `./migrations` and are applied with [golang-migrate](https://github.com/golang-migrate/migrate).

1. Install the migrate CLI (once):  
   `make install-migration`

2. Apply all pending migrations (default `DATABASE_URL` points to localhost Postgres as in docker-compose):

   ```bash
   make migration-up
   ```

   Or set the URL explicitly:

   ```bash
   make migration-up DATABASE_URL="postgres://postgres:postgres@localhost:5432/guest_management?sslmode=disable"
   ```

3. Other targets: `migration-down`, `migration-down-n N=n`, `migration-goto VERSION=v`, `migration-version`, `migration-force VERSION=v`, `migration-create NAME=your_migration_name`. See `make help-migration`.

Copy `.env.example` to `.env` and set `DATABASE_URL` (and `REDIS_URL` if needed) so the app and migrations use the same credentials.

### Swagger

API documentation is generated from Swag annotations in code and served at `/swagger` when enabled in config.

1. Install swag (once):  
   `make install-swagger`

2. Generate docs after changing annotations:  
   `make swagger-generate`  
   Output is written to `./api/swagger`.

3. Enable in config: set `Swagger.Enabled` to `true` in `configs/config.yaml` (or via env `SWAGGER_ENABLED`) and set `SWAGGER_USERNAME` and `SWAGGER_PASSWORD` in `.env`. The UI is protected by HTTP Basic Auth (realm `swagger`).

4. **Accessing Swagger when the app is running:** Start the application (e.g. `make run`), then open this URL in your browser:
   - **http://127.0.0.1:8080/swagger/**  
   You will be prompted for the Swagger username and password from your config (e.g. from `.env`). After signing in, the Swagger UI shows the API documentation and lets you try endpoints.

See `make help-swagger` for all Swagger-related targets.

### Running the application

1. Ensure PostgreSQL (and Redis if used) are running (e.g. `docker compose up -d`).
2. Apply migrations (see above).
3. Copy `.env.example` to `.env` and adjust `DATABASE_*` and optionally `REDIS_*`, `SWAGGER_*`.
4. Run the server:

   ```bash
   make run
   ```

   Or:

   ```bash
   go run ./cmd/api/main.go
   ```

The server listens on `127.0.0.1:8080`. Build a binary with `make build` (output in `bin/app` or `bin/app.exe`). For debugging with Delve: `make install-delve` then `make debug` (listens on port 2345 by default).

---

## Documentation references

| Document | Description |
| -------- | ----------- |
| [docs/DATABASE.md](docs/DATABASE.md) | PostgreSQL schema, tables, relationships, soft delete, and migration order. |
| [go-sdk httpkit](../go-sdk/httpkit/README.md) | Response envelope, error handling, and middleware options. |
| [go-sdk config](../go-sdk/config/README.md) | Configuration loading (files and env). |

For Make targets: run `make help` (and `make help-migration`, `make help-swagger`, `make help-build` as needed).
