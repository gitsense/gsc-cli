# Component: GSC CLI Makefile
# Block-UUID: 07fed727-0f26-4c8f-9cd8-d070edb2c5e5
# Parent-UUID: 6aea6f9e-a9bb-41da-8825-c8e2404ad596
# Version: 1.1.0
# Description: Makefile for building, installing, and testing the gsc-cli tool with metadata injection support.
# Language: Makefile
# Created-at: 2026-02-02T06:50:00.000Z
# Authors: GLM-4.7 (v1.0.0), Gemini 3 Flash (v1.1.0)


# GSC CLI Makefile
# This makefile provides commands for building, installing, and testing the gsc-cli tool.

.PHONY: build install clean test run help

# Binary name
BINARY_NAME=gsc
# Build directory
DIST_DIR=dist
# Main package path (changed to ./cmd/gsc to avoid stdlib confusion)
MAIN_PATH=./cmd/gsc

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Metadata for injection
VERSION=$(shell grep 'Version =' internal/version/version.go | cut -d '"' -f 2)
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-ldflags "-X github.com/gitsense/gsc-cli/internal/version.Version=$(VERSION) -X github.com/gitsense/gsc-cli/internal/version.GitCommit=$(COMMIT) -X github.com/gitsense/gsc-cli/internal/version.BuildTime=$(BUILD_TIME)"

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary for the current platform
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(DIST_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(DIST_DIR)/$(BINARY_NAME)"

install: build ## Install the binary to $GOPATH/bin or /usr/local/bin
	@echo "Installing $(BINARY_NAME)..."
	@cp $(DIST_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Installed to /usr/local/bin/$(BINARY_NAME)"

clean: ## Remove build artifacts
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -rf $(DIST_DIR)
	@echo "Clean complete"

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

run: ## Run the CLI (useful for quick testing)
	@echo "Running $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	./$(DIST_DIR)/$(BINARY_NAME) $(ARGS)

# Cross-compilation targets
build-linux: ## Build for Linux
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)

build-darwin: ## Build for macOS
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)

build-windows: ## Build for Windows
	@mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
