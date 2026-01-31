# build.mk - Build, clean, generate, run, and debug targets (guest-management-be)
#
# Provides: build, clean, generate, run, debug, install-delve, help-build.
# Prerequisite: include vars.mk first (MAIN_PATH, output vars). Output convention:
# section "# Build" / "# Clean" / "# Generate" / "# Run" / "# Debug".
# Fail-fast: no '-' prefix for build, generate, run, debug. clean is idempotent.

include $(SCRIPTS_DIR)/vars.mk

OUT_DIR ?= out
BUILD_DIR ?= bin
BINARY_NAME ?= app
DEBUG_PORT ?= 2345

DLV_BIN := $(GOPATH_BIN)$(PATH_SEP)dlv$(BIN_EXT)

build: ## Build application binary to $(BUILD_DIR)/$(BINARY_NAME)
	$(ECHO_EMPTY)
	@echo "# Build"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Building $(MAIN_PATH)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME)$(BIN_EXT) $(MAIN_PATH)
	@echo "$(INDENT)$(PREFIX_OK)Build succeeded: $(BUILD_DIR)/$(BINARY_NAME)$(BIN_EXT)"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

clean: ## Remove build and output artefacts ($(BUILD_DIR), $(OUT_DIR))
	$(ECHO_EMPTY)
	@echo "# Clean"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Removing artefacts..."
	@rm -rf $(BUILD_DIR) $(OUT_DIR)
	@echo "$(INDENT)$(PREFIX_OK)Clean completed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

generate: ## Run go generate ./...
	$(ECHO_EMPTY)
	@echo "# Generate"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Running go generate ./..."
	@go generate ./...
	@echo "$(INDENT)$(PREFIX_OK)Generate completed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

run: ## Run application with go run (development)
	$(ECHO_EMPTY)
	@echo "# Run"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Running $(MAIN_PATH)..."
	@go run $(MAIN_PATH)
	@echo "$(INDENT)$(PREFIX_OK)Application stopped"

debug: ## Run application under Delve (headless, listen on $(DEBUG_PORT)); requires install-delve
	$(ECHO_EMPTY)
	@echo "# Debug"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Starting debugger (Delve) for $(MAIN_PATH), port $(DEBUG_PORT)..."
	@$(DLV_BIN) debug $(MAIN_PATH) --listen=:$(DEBUG_PORT) --headless=true --api-version=2 --accept-multiclient
	@echo "$(INDENT)$(PREFIX_OK)Debug session ended"

install-delve: ## Install Delve debugger into GOPATH/bin
	$(ECHO_EMPTY)
	@echo "# Debug"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Installing Delve..."
	@go install github.com/go-delve/delve/cmd/dlv@latest
	@echo "$(INDENT)$(PREFIX_OK)Delve installed: $(DLV_BIN)"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

help-build: ## Show build/run/debug targets and descriptions
	@echo "# Build / Run / Debug"
	@echo "  make build         ## Build binary to $(BUILD_DIR)/$(BINARY_NAME)"
	@echo "  make clean         ## Remove $(BUILD_DIR), $(OUT_DIR)"
	@echo "  make generate      ## Run go generate ./..."
	@echo "  make run           ## Run application (go run $(MAIN_PATH))"
	@echo "  make debug         ## Run under Delve; run 'make install-delve' first"
	@echo "  make install-delve ## Install Delve debugger"
