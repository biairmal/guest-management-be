# deps.mk - Go module hygiene and dependency upgrade targets (guest-management-be)
#
# Provides: deps-tidy, deps-verify, deps, deps-outdated, deps-upgrade, help-deps.
# Prerequisite: include vars.mk first (output vars). Output convention: section "# Dependencies".

include $(SCRIPTS_DIR)/vars.mk

deps-tidy: ## Run go mod tidy
	$(ECHO_EMPTY)
	@echo "# Dependencies"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Running go mod tidy..."
	@go mod tidy
	@echo "$(INDENT)$(PREFIX_OK)go mod tidy completed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

deps-verify: ## Run go mod verify
	$(ECHO_EMPTY)
	@echo "# Dependencies"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Running go mod verify..."
	$(ECHO_EMPTY)
	@go mod verify
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)go mod verify completed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

deps: deps-tidy deps-verify ## Run deps-tidy and deps-verify

deps-outdated: ## List modules with available upgrades (go list -u -m all)
	$(ECHO_EMPTY)
	@echo "# Dependencies"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Listing modules with available upgrades..."
	@go list -u -m all 2>/dev/null || true
	@echo "$(INDENT)$(PREFIX_OK)List completed"

deps-upgrade: ## Update dependencies (go get -u ./...). Use with care.
	$(ECHO_EMPTY)
	@echo "# Dependencies"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Running go get -u ./..."
	@go get -u ./...
	@go mod tidy
	@echo "$(INDENT)$(PREFIX_OK)Dependencies upgraded; run tests and commit go.mod/go.sum"

help-deps: ## Show dependency targets and descriptions
	@echo "# Dependencies"
	@echo "  make deps-tidy     ## Run go mod tidy"
	@echo "  make deps-verify   ## Run go mod verify"
	@echo "  make deps          ## Run deps-tidy and deps-verify"
	@echo "  make deps-outdated ## List modules with available upgrades"
	@echo "  make deps-upgrade  ## Update dependencies (go get -u ./...)"
