# Makefile for guest-management-be
#
# Aggregates script modules from scripts/: vars, formatter, linter, test, security,
# deps, build (run, debug), swagger. Default goal: help. Use make check for CI;
# make install-tools to install formatter, linter, govulncheck, and Delve.
# Override MAIN_PATH, BUILD_DIR, BINARY_NAME, DEBUG_PORT, SWAGGER_OUTPUT_DIR as needed.

.PHONY: help help-formatter help-linter help-test help-security help-deps help-build help-swagger help-migration
.PHONY: format format-check install-formatter lint lint-fix install-linter
.PHONY: test test-unit test-integration test-race bench coverage coverage-view
.PHONY: vulncheck install-govulncheck deps-tidy deps-verify deps deps-outdated deps-upgrade
.PHONY: build clean generate run debug install-delve
.PHONY: install-swagger swagger-generate swagger-serve
.PHONY: install-migration migration-create migration-up migration-up-n migration-down migration-down-n migration-goto migration-version migration-force
.PHONY: check ci install-tools

.DEFAULT_GOAL := help

SCRIPTS_DIR := ./scripts

MAKE := $(MAKE) --no-print-directory

include $(SCRIPTS_DIR)/vars.mk
include $(SCRIPTS_DIR)/formatter.mk
include $(SCRIPTS_DIR)/linter.mk
include $(SCRIPTS_DIR)/test.mk
include $(SCRIPTS_DIR)/security.mk
include $(SCRIPTS_DIR)/deps.mk
include $(SCRIPTS_DIR)/build.mk
include $(SCRIPTS_DIR)/swagger.mk
include $(SCRIPTS_DIR)/migration.mk

help: ## Show all targets (aggregates help from all scripts)
	@echo ">>>> guest-management-be Makefile targets <<<<"
	$(ECHO_EMPTY)
	@$(MAKE) -s help-formatter
	$(ECHO_EMPTY)
	@$(MAKE) -s help-linter
	$(ECHO_EMPTY)
	@$(MAKE) -s help-test
	$(ECHO_EMPTY)
	@$(MAKE) -s help-security
	$(ECHO_EMPTY)
	@$(MAKE) -s help-deps
	$(ECHO_EMPTY)
	@$(MAKE) -s help-build
	$(ECHO_EMPTY)
	@$(MAKE) -s help-swagger
	$(ECHO_EMPTY)
	@$(MAKE) -s help-migration
	$(ECHO_EMPTY)
	@echo "# Other"
	@echo "  make ci            ## Run format-check, lint, test-unit, coverage, vulncheck, deps-verify (CI)"
	@echo "  make check         ## Alias for ci"
	@echo "  make install-tools ## Install formatter, linter, govulncheck, Delve"
	$(ECHO_EMPTY)

check: ci ## Alias for ci

ci: ## Run all checks (format-check, lint, test-unit, coverage, vulncheck, deps-verify); fail-fast
	@echo ">>>> CI <<<<"
	$(ECHO_EMPTY)
	@$(MAKE) format-check
	@$(MAKE) lint
	@$(MAKE) test-unit
	@$(MAKE) deps-verify
	$(ECHO_EMPTY)
	@echo "[OK] CI COMPLETED SUCCESSFULLY"

install-tools: ## Install formatter, linter, govulncheck, and Delve; fail-fast
	@echo ">>>> Install tools <<<<"
	$(ECHO_EMPTY)
	@$(MAKE) install-formatter
	@$(MAKE) install-linter
	@$(MAKE) install-govulncheck
	@$(MAKE) install-delve
	@$(MAKE) install-swagger
	@$(MAKE) install-migration
	$(ECHO_EMPTY)
	@echo "[OK] Install tools completed successfully"
