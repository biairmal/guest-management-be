# migration.mk - Database migrations via golang-migrate (guest-management-be)
#
# Provides: install-migration, migration-create, migration-up, migration-up-n,
# migration-down, migration-down-n, migration-goto, migration-version,
# migration-force, help-migration.
# Prerequisite: include vars.mk first (GOPATH_BIN, PATH_SEP, BIN_EXT, output vars).
# DB connection: pass DATABASE_URL (default localhost Postgres). Override MIGRATIONS_DIR,
# NAME (create), N (steps), VERSION (goto/down-to) as needed.
# Output convention: section "# Migration". Fail-fast; idempotent where noted.
#
# See: https://github.com/golang-migrate/migrate

include $(SCRIPTS_DIR)/vars.mk

MIGRATIONS_DIR ?= ./migrations
MIGRATE_VERSION ?= latest
# Build tags for migrate binary (postgres, mysql, sqlite3). Default: postgres.
MIGRATE_TAGS ?= postgres

# Default: localhost Postgres (matches docker-compose postgres service). Override with make migration-up DATABASE_URL=...
# Example: postgres://user:pass@localhost:5432/dbname?sslmode=disable
DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/guest_management?sslmode=disable

MIGRATE_BIN := $(GOPATH_BIN)$(PATH_SEP)migrate$(BIN_EXT)

install-migration: ## Install golang-migrate CLI (Postgres driver by default; set MIGRATE_TAGS for others)
	$(ECHO_EMPTY)
	@echo "# Migration"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Installing golang-migrate@$(MIGRATE_VERSION) (tags: $(MIGRATE_TAGS))..."
	$(ECHO_EMPTY)
	@go install -tags '$(MIGRATE_TAGS)' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)migrate installed: $(MIGRATE_BIN)"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

migration-create: ## Create new migration files; requires NAME= (e.g. make migration-create NAME=create_users_table)
	@test -n "$(NAME)" || (echo "$(INDENT)$(PREFIX_FAIL)NAME is required. Usage: make migration-create NAME=create_users_table"; exit 1)
	$(ECHO_EMPTY)
	@echo "# Migration"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Creating migration '$(NAME)' in $(MIGRATIONS_DIR)..."
	@mkdir -p $(MIGRATIONS_DIR)
	$(ECHO_EMPTY)
	@$(MIGRATE_BIN) create -ext sql -dir $(MIGRATIONS_DIR) -seq $(NAME)
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)Migration files created in $(MIGRATIONS_DIR)"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

migration-up: ## Apply all pending migrations (DB: DATABASE_URL, default localhost)
	$(ECHO_EMPTY)
	@echo "# Migration"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Applying all pending migrations (path: $(MIGRATIONS_DIR))..."
	$(ECHO_EMPTY)
	@$(MIGRATE_BIN) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)Migrations applied"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

migration-up-n: ## Apply N migrations; requires N= (e.g. make migration-up-n N=2)
	@test -n "$(N)" || (echo "$(INDENT)$(PREFIX_FAIL)N is required. Usage: make migration-up-n N=2"; exit 1)
	$(ECHO_EMPTY)
	@echo "# Migration"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Applying $(N) migration(s)..."
	$(ECHO_EMPTY)
	@$(MIGRATE_BIN) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up $(N)
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)Migrations applied"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

migration-down: ## Rollback one migration
	$(ECHO_EMPTY)
	@echo "# Migration"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Rolling back one migration..."
	$(ECHO_EMPTY)
	@$(MIGRATE_BIN) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)Migration rolled back"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

migration-down-n: ## Rollback N migrations; requires N= (e.g. make migration-down-n N=2)
	@test -n "$(N)" || (echo "$(INDENT)$(PREFIX_FAIL)N is required. Usage: make migration-down-n N=2"; exit 1)
	$(ECHO_EMPTY)
	@echo "# Migration"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Rolling back $(N) migration(s)..."
	$(ECHO_EMPTY)
	@$(MIGRATE_BIN) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down $(N)
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)Migrations rolled back"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

migration-goto: ## Migrate to specific version (up or down); requires VERSION= (e.g. make migration-goto VERSION=3)
	@test -n "$(VERSION)" || (echo "$(INDENT)$(PREFIX_FAIL)VERSION is required. Usage: make migration-goto VERSION=3"; exit 1)
	$(ECHO_EMPTY)
	@echo "# Migration"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Migrating to version $(VERSION)..."
	$(ECHO_EMPTY)
	@$(MIGRATE_BIN) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" goto $(VERSION)
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)Now at version $(VERSION)"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

migration-version: ## Show current migration version
	$(ECHO_EMPTY)
	@echo "# Migration"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Current version:"
	$(ECHO_EMPTY)
	@$(MIGRATE_BIN) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" version
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)Done"

migration-force: ## Force set schema version (recovery after failed migration); requires VERSION=
	@test -n "$(VERSION)" || (echo "$(INDENT)$(PREFIX_FAIL)VERSION is required. Usage: make migration-force VERSION=1"; exit 1)
	$(ECHO_EMPTY)
	@echo "# Migration"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Forcing version to $(VERSION)..."
	$(ECHO_EMPTY)
	@$(MIGRATE_BIN) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" force $(VERSION)
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)Version forced to $(VERSION)"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

help-migration: ## Show migration targets and descriptions
	@echo "# Migration"
	@echo "  make install-migration     ## Install migrate CLI (default: postgres; set MIGRATE_TAGS for mysql etc.)"
	@echo "  make migration-create NAME=<name> ## Create new migration files (e.g. NAME=create_users_table)"
	@echo "  make migration-up         ## Apply all pending migrations (default DB: localhost)"
	@echo "  make migration-up-n N=<n> ## Apply N migrations (e.g. N=2)"
	@echo "  make migration-down       ## Rollback one migration"
	@echo "  make migration-down-n N=<n> ## Rollback N migrations (e.g. N=2)"
	@echo "  make migration-goto VERSION=<v> ## Migrate to version V (up or down)"
	@echo "  make migration-version    ## Show current migration version"
	@echo "  make migration-force VERSION=<v> ## Force set version (recovery)"
	@echo "  DB: DATABASE_URL (default: postgres://postgres:postgres@localhost:5432/guest_management?sslmode=disable)"
