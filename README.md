# Guest Management — Backend

Go backend for the Guest Management System (multi-tenant, events, guests, tickets, workflows). Uses PostgreSQL and Redis.

The HTTP server uses [go-chi](https://github.com/go-chi/chi) for routing and [go-sdk httpkit](../go-sdk/httpkit/README.md) for middleware (recover, request ID, request/response logging), response envelope, and error handling. Health and readiness endpoints are provided by httpkit. The server shuts down **gracefully by default** on SIGINT/SIGTERM (see [Graceful shutdown](#graceful-shutdown)).

## API (overview)

- **Router**: go-chi (`chi.NewRouter()`).
- **Middleware**: httpkit Recover, RequestID, Logging (path, IP, method, body; see go-sdk httpkit README for options).
- **Endpoints**:
  - `GET /health` — liveness (httpkit.Health).
  - `GET /ready` — readiness (httpkit.Readiness; checker can be wired to DB/Redis later).
  - `GET /api/v1/ping` — sample success response.
  - `GET /api/v1/items` — list items (dummy CRUD).
  - `GET /api/v1/items/{id}` — get one item.
  - `POST /api/v1/items` — create item (body: `{"name":"..."}`).
  - `PUT /api/v1/items/{id}` — update item (body: `{"name":"..."}`).
  - `DELETE /api/v1/items/{id}` — delete item.

Response format and error envelope are described in [go-sdk httpkit README](../go-sdk/httpkit/README.md).

## Graceful shutdown

On SIGINT or SIGTERM, the server calls `server.Shutdown(ctx)` with a 30s timeout, then waits for in-flight requests to finish. If the timeout is exceeded, the server is closed. This is the default behaviour; no configuration is required.

## Development with Docker

Start PostgreSQL, Redis, and a Redis GUI for local development:

```bash
docker compose up -d
```

- **PostgreSQL:** `localhost:5432`, database `guest_management`, user `postgres`, password `postgres`.
- **Redis:** `localhost:6379`.
- **Redis Insight (GUI):** http://localhost:5540 — add Redis with URL `redis://default:redis@redis:6379/0`. Use host **`redis`** (service name), not `localhost`, because Redis Insight runs inside Docker and connects over the container network.

Apply migrations (use the same credentials as Docker):

```bash
make migration-up DATABASE_URL="postgres://postgres:postgres@localhost:5432/guest_management?sslmode=disable"
```

Or copy `.env.example` to `.env`, set `DATABASE_URL` and `REDIS_URL`, and load them when running the app or migrations.

Stop services:

```bash
docker compose down
```

Data is persisted in Docker volumes (`postgres_data`, `redis_data`). Use `docker compose down -v` to remove volumes and reset data.

## Database

Schema and migrations are documented in [docs/DATABASE.md](docs/DATABASE.md). Migrations live in `./migrations` and are applied with [golang-migrate](https://github.com/golang-migrate/migrate).
