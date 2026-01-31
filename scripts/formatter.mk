# formatter.mk - Gofumpt-based formatting targets (guest-management-be)
#
# Provides: format, format-check, install-formatter, help-formatter.
# Prerequisite: include vars.mk first (GOPATH_BIN, PATH_SEP, BIN_EXT, output vars).
# Output convention: section "# Formatter" / "# Format", body $(INDENT)$(PREFIX_*).
# Uses gofumpt exit code for format-check (cross-platform).

include $(SCRIPTS_DIR)/vars.mk

GOFUMPT_VERSION ?= latest
GOFUMPT_BIN := $(GOPATH_BIN)$(PATH_SEP)gofumpt$(BIN_EXT)

install-formatter: ## Install gofumpt into GOPATH/bin
	$(ECHO_EMPTY)
	@echo "# Formatter"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Installing gofumpt@$(GOFUMPT_VERSION)..."
	@go install mvdan.cc/gofumpt@$(GOFUMPT_VERSION)
	@echo "$(INDENT)$(PREFIX_OK)gofumpt installed successfully"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

help-formatter: ## Show formatter targets and descriptions
	@echo "# Formatter"
	@echo "  make format            ## Format code with gofumpt"
	@echo "  make format-check      ## Check format only; exit non-zero if unformatted (CI)"
	@echo "  make install-formatter ## Install gofumpt into GOPATH/bin"

format: ## Format code with gofumpt
	$(ECHO_EMPTY)
	@echo "# Format"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Formatting code with gofumpt@$(GOFUMPT_VERSION)..."
	@$(GOFUMPT_BIN) -l -w .
	@echo "$(INDENT)$(PREFIX_OK)Code formatted successfully"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

format-check: ## Check formatting only; exit non-zero if any file would be changed (CI)
	$(ECHO_EMPTY)
	@echo "# Format"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Checking format with gofumpt -l..."
	@$(GOFUMPT_BIN) -l .
	@echo "$(INDENT)$(PREFIX_OK)Format check passed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"
