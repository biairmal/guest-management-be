# security.mk - Dependency vulnerability checks via govulncheck (guest-management-be)
#
# Provides: install-govulncheck, vulncheck, help-security.
# Prerequisite: include vars.mk first (GOPATH_BIN, PATH_SEP, BIN_EXT, output vars).
# Output convention: section "# Security", body $(INDENT)$(PREFIX_*).

include $(SCRIPTS_DIR)/vars.mk

GOVULNCHECK_BIN := $(GOPATH_BIN)$(PATH_SEP)govulncheck$(BIN_EXT)

install-govulncheck: ## Install govulncheck into GOPATH/bin
	$(ECHO_EMPTY)
	@echo "# Security"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Installing govulncheck..."
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "$(INDENT)$(PREFIX_OK)govulncheck installed successfully"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

vulncheck: ## Run govulncheck ./... (dependency vulnerabilities)
	$(ECHO_EMPTY)
	@echo "# Security"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Running govulncheck..."
	$(ECHO_EMPTY)
	@$(GOVULNCHECK_BIN) ./...
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)Vulncheck passed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

help-security: ## Show security targets and descriptions
	@echo "# Security"
	@echo "  make install-govulncheck ## Install govulncheck into GOPATH/bin"
	@echo "  make vulncheck           ## Run govulncheck ./... (dependency vulnerabilities)"
