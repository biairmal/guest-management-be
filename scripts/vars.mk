# vars.mk - Common variables for cross-platform Makefile scripts (guest-management-be)
#
# Defines: GO_OS, GOPATH, GOPATH_BIN, PATH_SEP, BIN_EXT for paths and binaries.
# Defines: MAIN_PATH (application entry point). Defines output convention variables:
# ECHO_EMPTY, INDENT, PREFIX_RUN, PREFIX_OK, PREFIX_FAIL, PREFIX_SKIP.
# No user-facing targets; include this file first in script modules that need it.
#
# Conventions: All scripts are fail-fast (no Make '-' prefix). Output uses section
# header "# SectionName" and body lines with $(INDENT)$(PREFIX_*).

GO_OS := $(shell go env GOOS)
GOPATH := $(shell go env GOPATH)

ifeq ($(GO_OS),windows)
	PATH_SEP := \\
	BIN_EXT := .exe
else
	PATH_SEP := /
	BIN_EXT :=
endif

GOPATH_BIN := $(GOPATH)$(PATH_SEP)bin

# Application entry point (override via make MAIN_PATH=...)
MAIN_PATH ?= ./cmd/api/main.go

# Output convention: top-level >>>> TITLE <<<<, section # Name, 2-space indent
ECHO_EMPTY := @echo ""
EMPTY :=
SPACE := $(EMPTY)  $(EMPTY)
INDENT := $(SPACE)
PREFIX_RUN := [RUN] 
PREFIX_OK := [OK] 
PREFIX_FAIL := [FAIL] 
PREFIX_SKIP := [SKIP] 
