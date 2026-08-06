# vars.mk - Common variables for cross-platform Makefile scripts (guest-management-be)
#
# Defines: GO_OS, GOPATH, GOPATH_BIN, PATH_SEP, BIN_EXT for paths and binaries.
# Defines: MAIN_PATH (application entry point). Defines output convention variables:
# ECHO_EMPTY, INDENT, PREFIX_RUN, PREFIX_OK, PREFIX_FAIL, PREFIX_SKIP.
# No user-facing targets; include this file first in script modules that need it.
#
# Conventions: All scripts are fail-fast (no Make '-' prefix). Output uses section
# header "# SectionName" and body lines with $(INDENT)$(PREFIX_*).
#
# Shell compatibility: On Windows, Git Bash and MSYS2 use a Unix-like shell where
# backslashes in paths are escape characters. We detect MSYSTEM (set by Git for
# Windows / MSYS2) and use forward slashes so MIGRATE_BIN and similar paths work
# in both PowerShell and Git Bash.
#
# Recipes across these scripts use POSIX commands (rm -rf, mkdir -p, grep,
# awk, ...) and, in kill-app/kill-port, compound shell syntax (if/for/fi).
# On Windows this needs two separate fixes, since make is often launched from
# PowerShell/cmd (e.g. a VS Code task) rather than a Git Bash terminal, and the
# Git for Windows installer puts Git\cmd (git.exe) on PATH but not
# Git\bin/Git\usr\bin (sh.exe, bash.exe, rm.exe, mkdir.exe, grep.exe, ...):
#  1. Compound recipe lines need a real shell to interpret if/for/fi etc., so
#     pin SHELL to Git's sh.exe explicitly rather than relying on PATH lookup.
#  2. Simple recipe lines (no ;/&&/| etc., e.g. `rm -rf bin out`) hit a
#     documented GNU Make Windows optimization: it bypasses the shell and
#     spawns them directly via CreateProcess *even when SHELL is set* — which
#     fails to find rm.exe/mkdir.exe unless their directory is on PATH
#     ("CreateProcess(NULL, rm -rf ..., ...) failed"). So also prepend Git's
#     usr/bin to PATH.
# Override GIT_DIR if Git for Windows is installed somewhere else (e.g.
# `make GIT_DIR="D:/Git" build`).
ifeq ($(OS),Windows_NT)
	GIT_DIR ?= C:/Program Files/Git
	SHELL := $(GIT_DIR)/bin/sh.exe
	.SHELLFLAGS := -c
	export PATH := $(GIT_DIR)/usr/bin;$(PATH)
endif

GO_OS := $(shell go env GOOS)
GOPATH := $(shell go env GOPATH)
# MSYSTEM is set in Git Bash and MSYS2 (e.g. MINGW64); use it to detect Unix-like shell on Windows
MSYSTEM ?=

ifeq ($(GO_OS),windows)
	BIN_EXT := .exe
	# Use forward slashes when running under Git Bash/MSYS2 so paths are not mangled by the shell
	ifeq ($(MSYSTEM),)
		PATH_SEP := \\
		GOPATH_BIN := $(GOPATH)$(PATH_SEP)bin
	else
		PATH_SEP := /
		GOPATH_BIN := $(subst \,/,$(GOPATH))/bin
	endif
else
	PATH_SEP := /
	BIN_EXT :=
	GOPATH_BIN := $(GOPATH)/bin
endif

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
