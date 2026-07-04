# Contributing

Thanks for working on `github.com/biairmal/guest-management-be`. This page covers human onboarding and workflow. The **authoritative coding rules** — for both humans and AI assistants — live in **[AGENTS.md](./AGENTS.md)**; read it before you start.

## Prerequisites

| Requirement | Purpose |
|---|---|
| **Go 1.25.1+** | Build and test. Check with `go version`. |
| **The sibling `go-sdk` checkout** | This service depends on `github.com/biairmal/go-sdk` via `replace => ../go-sdk` in `go.mod`. Clone it next to this repo (`../go-sdk`) — the project will not build without it. |
| **Make** | Format, lint, test, coverage, tooling. On Windows use Git Bash, WSL, or GnuWin32 Make. |
| **Docker + Docker Compose** | Local Postgres (and Redis) via `docker-compose.yaml`. |
| **PostgreSQL** | Primary datastore. Migrations live in `migrations/` (golang-migrate). |

Optional tools (installed on demand):

```bash
make install-tools   # gofumpt, golangci-lint, govulncheck, delve
```

## First run

```bash
cp .env.example .env          # adjust as needed
docker compose up -d          # start Postgres (+ Redis)
make migration-up             # apply schema
make run                      # start the API on the configured address
```

Swagger UI is served at `/swagger` (Basic Auth) when `SWAGGER_ENABLED=true`.

## Development loop

1. Make your change in the relevant layer/feature slice (`internal/features/<feature>` for domain work; `internal/core` for shared infrastructure).
2. Follow the [Authoring rules](AGENTS.md#authoring-rules).
3. Add **table-driven `*__test.go` tests** alongside the change.
4. Regenerate API docs if you touched endpoints: `make swagger-generate`.
5. Run the CI gate and make sure it is green:

   ```bash
   make check          # format-check → lint → test-unit → coverage → vulncheck → deps-verify
   ```

6. Walk the [Definition of Done](AGENTS.md#definition-of-done) before opening a PR.

Adding a new feature? Follow [docs/NEW_FEATURE_CHECKLIST.md](docs/NEW_FEATURE_CHECKLIST.md).

## Tests

- **Unit:** `make test-unit` → `go test -short ./...`. Stdlib `testing` + generated `gomock` mocks (no testify, no hand-written fakes); table-driven; guard slow code with `testing.Short()`. Consume go-sdk's generated mocks from the `github.com/biairmal/go-sdk/mocks` module; regenerate app-side mocks with `make mocks` after changing a mocked interface.
- **Integration (live DB):** `make test-integration` → `go test ./...`. Put these in `*_integration_test.go`.
- **Coverage:** `make coverage` writes `out/coverage.{out,html}`; `make coverage-view` opens the report.

## Pull requests

- Keep PRs scoped to one concern; update [docs/FEATURES.md](docs/FEATURES.md) and the [feature map](AGENTS.md#feature-map) when you add or change a feature.
- Don't merge with a failing `make check`.
- Document any new convention in [AGENTS.md](./AGENTS.md) (rules) or [docs/](docs/) (reference) — not in tool-specific files.

## How the guideline files fit together

One canonical rules file, picked up automatically by every major AI tool — no duplicated content to drift:

| File | Read by |
|---|---|
| **[AGENTS.md](./AGENTS.md)** | Canonical rules. Cursor, Zed, Aider, GitHub Copilot coding agent read it natively. |
| [CLAUDE.md](./CLAUDE.md) | Claude Code — a pointer to `AGENTS.md`. |
| [.github/copilot-instructions.md](.github/copilot-instructions.md) | GitHub Copilot — a pointer to `AGENTS.md`. |

Change the rules in **`AGENTS.md` only**. The pointer files carry no rules of their own.
