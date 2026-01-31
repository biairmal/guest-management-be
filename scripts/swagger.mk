# swagger.mk - Swagger (OpenAPI) documentation generation (guest-management-be)
#
# Uses swag (github.com/swaggo/swag). Provides: install-swagger, swagger-generate,
# swagger-serve, help-swagger. Prerequisite: include vars.mk first (MAIN_PATH,
# GOPATH_BIN, PATH_SEP, BIN_EXT, output vars). Output convention: section "# Swagger".
# Fail-fast: no '-' prefix for install/generate. swagger-serve runs a local HTTP server.
#
# Annotate main or handlers with swag comments; then run make swagger-generate.
# See: https://github.com/swaggo/swag#declarative-comments-format

include $(SCRIPTS_DIR)/vars.mk

SWAGGER_OUTPUT_DIR ?= ./api/swagger
SWAG_VERSION ?= latest

SWAG_BIN := $(GOPATH_BIN)$(PATH_SEP)swag$(BIN_EXT)

install-swagger: ## Install swag CLI into GOPATH/bin
	$(ECHO_EMPTY)
	@echo "# Swagger"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Installing swag@$(SWAG_VERSION)..."
	@go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
	@echo "$(INDENT)$(PREFIX_OK)swag installed: $(SWAG_BIN)"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

swagger-generate: ## Generate Swagger docs from annotations; main entry: $(MAIN_PATH)
	$(ECHO_EMPTY)
	@echo "# Swagger"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Generating Swagger docs (entry: $(MAIN_PATH), out: $(SWAGGER_OUTPUT_DIR))..."
	@mkdir -p $(SWAGGER_OUTPUT_DIR)
	@$(SWAG_BIN) init -g $(MAIN_PATH) -o $(SWAGGER_OUTPUT_DIR) --parseDependency --parseInternal
	@echo "$(INDENT)$(PREFIX_OK)Swagger docs generated: $(SWAGGER_OUTPUT_DIR)"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

swagger-serve: ## Serve Swagger UI for $(SWAGGER_OUTPUT_DIR) (requires Python; default port 8080)
	$(ECHO_EMPTY)
	@echo "# Swagger"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Serving $(SWAGGER_OUTPUT_DIR) (e.g. http://localhost:8080)..."
	@cd $(SWAGGER_OUTPUT_DIR) && (python -m http.server 8080 || python3 -m http.server 8080)
	@echo "$(INDENT)$(PREFIX_OK)Server stopped"

help-swagger: ## Show Swagger targets and descriptions
	@echo "# Swagger"
	@echo "  make install-swagger   ## Install swag CLI (run before swagger-generate)"
	@echo "  make swagger-generate  ## Generate docs from code annotations"
	@echo "  make swagger-serve     ## Serve generated docs (Python HTTP server)"
