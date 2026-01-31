# test.mk - Test targets (guest-management-be)
#
# Provides: test, test-unit, test-integration, test-race, bench, coverage, coverage-view, help-test.
# Artefacts: out/ (coverage.out, coverage.html). Convention: -short for unit; full for integration.
# Prerequisite: include vars.mk first (output vars). OUT_DIR shared with build.mk.

include $(SCRIPTS_DIR)/vars.mk

OUT_DIR ?= out
COVERAGE_OUT := $(OUT_DIR)/coverage.out
COVERAGE_HTML := $(OUT_DIR)/coverage.html

test: test-unit ## Alias for test-unit

test-unit: ## Run unit tests (go test -short ./...)
	$(ECHO_EMPTY)
	@echo "# Test"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Running unit tests..."
	$(ECHO_EMPTY)
	@go test -short -coverprofile=$(COVERAGE_OUT) -covermode=atomic ./...
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)Unit tests passed"
	@echo "$(INDENT)$(PREFIX_OK)Coverage written to $(COVERAGE_OUT) and $(COVERAGE_HTML)"
	$(ECHO_EMPTY)
	@go tool cover -func=$(COVERAGE_OUT)
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

test-integration: ## Run all tests including integration (go test ./...)
	$(ECHO_EMPTY)
	@echo "# Test"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Running tests (including integration)..."
	$(ECHO_EMPTY)
	@go test ./...
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)Tests passed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

test-race: ## Run unit tests with race detector
	$(ECHO_EMPTY)
	@echo "# Test"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Running unit tests with -race..."
	$(ECHO_EMPTY)
	@go test -race -short ./...
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)Race tests passed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

bench: ## Run benchmarks (go test -bench=. -benchmem ./...)
	$(ECHO_EMPTY)
	@echo "# Test"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Running benchmarks..."
	@go test -bench=. -benchmem ./...
	@echo "$(INDENT)$(PREFIX_OK)Benchmarks completed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

coverage: ## Generate coverage.out and coverage.html in out/
	$(ECHO_EMPTY)
	@echo "# Test"
	$(ECHO_EMPTY)
	@mkdir -p $(OUT_DIR)
	@echo "$(INDENT)$(PREFIX_RUN)Running tests with coverage..."
	$(ECHO_EMPTY)
	@go test -short -coverprofile=$(COVERAGE_OUT) -covermode=atomic ./...
	$(ECHO_EMPTY)
	@go tool cover -html=$(COVERAGE_OUT) -o $(COVERAGE_HTML)
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)Coverage written to $(COVERAGE_OUT) and $(COVERAGE_HTML)"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

coverage-view: ## Open coverage report in browser (platform-specific; requires make coverage first)
	$(ECHO_EMPTY)
	@echo "# Test"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Opening $(COVERAGE_HTML)..."
	@if [ -f $(COVERAGE_HTML) ]; then \
		(command -v xdg-open >/dev/null 2>&1 && xdg-open $(COVERAGE_HTML)) || \
		(command -v open >/dev/null 2>&1 && open $(COVERAGE_HTML)) || \
		(start $(COVERAGE_HTML) 2>/dev/null) || true; \
	else \
		echo "$(INDENT)$(PREFIX_FAIL)Run 'make coverage' first"; exit 1; \
	fi

help-test: ## Show test targets and descriptions
	@echo "# Test"
	@echo "  make test             ## Alias for test-unit"
	@echo "  make test-unit        ## Run unit tests (go test -short ./...)"
	@echo "  make test-integration ## Run all tests including integration"
	@echo "  make test-race        ## Run unit tests with race detector"
	@echo "  make bench            ## Run benchmarks"
	@echo "  make coverage         ## Generate coverage.out and coverage.html in out/"
	@echo "  make coverage-view    ## Open coverage report in browser"
