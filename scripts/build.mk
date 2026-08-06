# build.mk - Build, clean, generate, run, and debug targets (guest-management-be)
#
# Provides: build, build-debug, clean, generate, kill-app, kill-port, run, debug, install-delve,
# help-build. Prerequisite: include vars.mk first (MAIN_PATH, output vars). Output convention:
# section "# Build" / "# Build (debug)" / "# Clean" / "# Generate" / "# Kill App" / "# Kill Port" /
# "# Run" / "# Debug". Fail-fast: no '-' prefix for build, generate, run, debug. clean, kill-app,
# kill-port are idempotent.
#
# build/build-debug always rebuild from a clean slate: clean -> mocks -> swagger-generate -> go
# build, so the binary never ships with stale mocks/Swagger docs. clean kills any running instance
# of the binary first (kill-app) — on Windows a running .exe is locked, so `rm`/overwriting it
# fails until the old process is gone; this is what actually resolves "app.exe is locked" when
# rebuilding after a previous run/debug session. build-debug additionally disables optimizations
# and inlining (-gcflags="all=-N -l") so breakpoints reliably bind and local variables aren't
# optimized away — used by both `make debug` and the "Debug (pre-built binary)" launch configs in
# .vscode/launch.json (see .vscode/tasks.json's "make build-debug" preLaunchTask). run and debug
# also kill-port before starting the new process, so a previous stuck process never blocks the new
# one from binding.

include $(SCRIPTS_DIR)/vars.mk

OUT_DIR ?= out
BUILD_DIR ?= bin
BINARY_NAME ?= app
DEBUG_PORT ?= 2345
SERVER_PORT ?= 8080

# PORT is what kill-port acts on; defaults to SERVER_PORT but can be overridden
# per-invocation (e.g. `make kill-port PORT=$(DEBUG_PORT)`, used by debug below).
PORT ?= $(SERVER_PORT)

DLV_BIN := $(GOPATH_BIN)$(PATH_SEP)dlv$(BIN_EXT)

build: ## Clean artefacts, regenerate mocks + Swagger docs, then build binary to $(BUILD_DIR)/$(BINARY_NAME)
	$(ECHO_EMPTY)
	@echo "# Build"
	$(ECHO_EMPTY)
	@$(MAKE) clean
	@$(MAKE) mocks
	@$(MAKE) swagger-generate
	@echo "$(INDENT)$(PREFIX_RUN)Building $(MAIN_PATH)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME)$(BIN_EXT) $(MAIN_PATH)
	@echo "$(INDENT)$(PREFIX_OK)Build succeeded: $(BUILD_DIR)/$(BINARY_NAME)$(BIN_EXT)"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

build-debug: ## Clean artefacts, regenerate mocks + Swagger docs, then build with debug symbols (no optimizations/inlining)
	$(ECHO_EMPTY)
	@echo "# Build (debug)"
	$(ECHO_EMPTY)
	@$(MAKE) clean
	@$(MAKE) mocks
	@$(MAKE) swagger-generate
	@echo "$(INDENT)$(PREFIX_RUN)Building $(MAIN_PATH) (debug symbols, no optimizations/inlining)..."
	@mkdir -p $(BUILD_DIR)
	@go build -gcflags="all=-N -l" -o $(BUILD_DIR)/$(BINARY_NAME)$(BIN_EXT) $(MAIN_PATH)
	@echo "$(INDENT)$(PREFIX_OK)Build succeeded: $(BUILD_DIR)/$(BINARY_NAME)$(BIN_EXT)"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

clean: ## Remove build and output artefacts ($(BUILD_DIR), $(OUT_DIR))
	$(ECHO_EMPTY)
	@echo "# Clean"
	$(ECHO_EMPTY)
	@$(MAKE) kill-app
	@echo "$(INDENT)$(PREFIX_RUN)Removing artefacts..."
	@rm -rf $(BUILD_DIR) $(OUT_DIR)
	@echo "$(INDENT)$(PREFIX_OK)Clean completed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

kill-app: ## Kill any running $(BINARY_NAME) (and, on Windows, stray dlv.exe) so the binary isn't locked
	$(ECHO_EMPTY)
	@echo "# Kill App"
	$(ECHO_EMPTY)
ifeq ($(GO_OS),windows)
	@taskkill //F //IM $(BINARY_NAME)$(BIN_EXT) >/dev/null 2>&1 \
		&& echo "$(INDENT)$(PREFIX_OK)Stopped running $(BINARY_NAME)$(BIN_EXT)" \
		|| echo "$(INDENT)$(PREFIX_SKIP)$(BINARY_NAME)$(BIN_EXT) was not running"
	@taskkill //F //IM dlv.exe >/dev/null 2>&1 \
		&& echo "$(INDENT)$(PREFIX_OK)Stopped stray dlv.exe" \
		|| echo "$(INDENT)$(PREFIX_SKIP)No dlv.exe running"
else
	@pkill -f "$(BUILD_DIR)/$(BINARY_NAME)$(BIN_EXT)" >/dev/null 2>&1 \
		&& echo "$(INDENT)$(PREFIX_OK)Stopped running $(BINARY_NAME)" \
		|| echo "$(INDENT)$(PREFIX_SKIP)$(BINARY_NAME) was not running"
endif
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

kill-port: ## Kill any process listening on PORT (default $(SERVER_PORT)); usage: make kill-port PORT=1234
	$(ECHO_EMPTY)
	@echo "# Kill Port"
	$(ECHO_EMPTY)
ifeq ($(GO_OS),windows)
	@PID="$$(netstat -ano | grep -E ':$(PORT)[[:space:]]+.*LISTENING' | awk '{print $$NF}' | sort -u)"; \
	if [ -n "$$PID" ]; then \
		echo "$(INDENT)$(PREFIX_RUN)Killing PID(s) on port $(PORT): $$PID"; \
		for p in $$PID; do taskkill //F //PID "$$p" >/dev/null 2>&1 || true; done; \
		echo "$(INDENT)$(PREFIX_OK)Port $(PORT) freed"; \
	else \
		echo "$(INDENT)$(PREFIX_SKIP)No process listening on port $(PORT)"; \
	fi
else
	@PID="$$(lsof -ti tcp:$(PORT) 2>/dev/null)"; \
	if [ -n "$$PID" ]; then \
		echo "$(INDENT)$(PREFIX_RUN)Killing PID(s) on port $(PORT): $$PID"; \
		kill -9 $$PID 2>/dev/null || true; \
		echo "$(INDENT)$(PREFIX_OK)Port $(PORT) freed"; \
	else \
		echo "$(INDENT)$(PREFIX_SKIP)No process listening on port $(PORT)"; \
	fi
endif
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

run: ## Kill $(SERVER_PORT), rebuild (clean+mocks+swagger+build), then run the built binary
	$(ECHO_EMPTY)
	@echo "# Run"
	$(ECHO_EMPTY)
	@$(MAKE) build
	@$(MAKE) kill-port PORT=$(SERVER_PORT)
	@echo "$(INDENT)$(PREFIX_RUN)Starting $(BUILD_DIR)/$(BINARY_NAME)$(BIN_EXT)..."
	$(ECHO_EMPTY)
	@$(BUILD_DIR)/$(BINARY_NAME)$(BIN_EXT)
	@echo "$(INDENT)$(PREFIX_OK)Application stopped"

debug: ## Kill $(SERVER_PORT)/$(DEBUG_PORT), rebuild w/ debug symbols, then debug the built binary under Delve
	$(ECHO_EMPTY)
	@echo "# Debug"
	$(ECHO_EMPTY)
	@$(MAKE) build-debug
	@$(MAKE) kill-port PORT=$(SERVER_PORT)
	@$(MAKE) kill-port PORT=$(DEBUG_PORT)
	@echo "$(INDENT)$(PREFIX_RUN)Starting debugger (Delve) for $(BUILD_DIR)/$(BINARY_NAME)$(BIN_EXT), port $(DEBUG_PORT)..."
	$(ECHO_EMPTY)
	@$(DLV_BIN) exec $(BUILD_DIR)/$(BINARY_NAME)$(BIN_EXT) --listen=:$(DEBUG_PORT) --headless=true --api-version=2 --accept-multiclient --continue
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
	@echo "  make build             ## Clean, regenerate mocks + Swagger, then build to $(BUILD_DIR)/$(BINARY_NAME)"
	@echo "  make build-debug       ## Same as build, but with debug symbols (no optimizations/inlining)"
	@echo "  make clean             ## kill-app, then remove $(BUILD_DIR), $(OUT_DIR)"
	@echo "  make generate          ## Run go generate ./..."
	@echo "  make kill-app          ## Kill any running $(BINARY_NAME) (unlocks the binary on Windows)"
	@echo "  make kill-port PORT=<p> ## Kill whatever is listening on PORT (default $(SERVER_PORT))"
	@echo "  make run               ## kill-port + build, then run the built binary"
	@echo "  make debug             ## kill-port + debug build, then run under Delve; 'make install-delve' first"
	@echo "  make install-delve     ## Install Delve debugger"
