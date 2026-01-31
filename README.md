# Guest Management — Backend

Go backend for the Guest Management System (multi-tenant, events, guests, tickets, workflows). Uses PostgreSQL and Redis.

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
