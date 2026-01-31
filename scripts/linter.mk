# linter.mk - Golangci-lint targets (guest-management-be)
#
# Provides: install-linter, lint, lint-fix, help-linter.
# Prerequisite: include vars.mk first (GOPATH_BIN, PATH_SEP, BIN_EXT, output vars).
# Config: root .golangci.yml. Override GOLANGCI_LINT_VERSION if needed (default: latest).

include $(SCRIPTS_DIR)/vars.mk

GOLANGCI_LINT_VERSION ?= latest
GOLANGCI_LINT_BIN := $(GOPATH_BIN)$(PATH_SEP)golangci-lint$(BIN_EXT)

install-linter: ## Install golangci-lint into GOPATH/bin
	$(ECHO_EMPTY)
	@echo "# Linter"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Installing golangci-lint@$(GOLANGCI_LINT_VERSION)..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@echo "$(INDENT)$(PREFIX_OK)golangci-lint installed successfully"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

lint: ## Run golangci-lint (config: .golangci.yml)
	$(ECHO_EMPTY)
	@echo "# Linter"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Running golangci-lint..."
	@$(GOLANGCI_LINT_BIN) run ./...
	@echo "$(INDENT)$(PREFIX_OK)Lint passed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

lint-fix: ## Run golangci-lint with --fix
	$(ECHO_EMPTY)
	@echo "# Linter"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Running golangci-lint --fix..."
	@$(GOLANGCI_LINT_BIN) run --fix ./...
	@echo "$(INDENT)$(PREFIX_OK)Lint fix completed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

help-linter: ## Show linter targets and descriptions
	@echo "# Linter"
	@echo "  make install-linter ## Install golangci-lint into GOPATH/bin"
	@echo "  make lint          ## Run golangci-lint (config: .golangci.yml)"
	@echo "  make lint-fix      ## Run golangci-lint with --fix"
