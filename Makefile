# DynamoDB TUI Explorer Makefile

# Variables
BINARY_NAME=ddb-explorer
GO_FILES=$(shell find . -name "*.go" -not -path "./vendor/*")
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Colors
GREEN=\033[32m
BLUE=\033[34m
YELLOW=\033[33m
RED=\033[31m
BOLD=\033[1m
RESET=\033[0m

# Default target
.PHONY: all
all: run

# Build the application
.PHONY: build
build:
	@echo "$(BLUE)🔨 Building $(BINARY_NAME)...$(RESET)"
	@go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY_NAME)
	@echo "$(GREEN)✓ Built $(BINARY_NAME)$(RESET)"
	@echo

# Build and install to PATH
.PHONY: install
install: build
	@echo "$(YELLOW)📦 Installing $(BINARY_NAME) to PATH...$(RESET)"
	@if ! grep -q "$(PWD)" ~/.zshrc 2>/dev/null; then \
		echo "export PATH=\"$(PWD):\$$PATH\"" >> ~/.zshrc; \
		echo "$(GREEN)✓ Added $(PWD) to PATH in ~/.zshrc$(RESET)"; \
	else \
		echo "$(BLUE)ℹ PATH already contains $(PWD)$(RESET)"; \
	fi
	@echo "$(GREEN)✓ Installed $(BINARY_NAME) - restart your shell or run 'source ~/.zshrc'$(RESET)"
	@echo

# Run the application with dev profile
.PHONY: run
run: build
	@echo "$(BLUE)▶️  Running $(BINARY_NAME) with dev profile...$(RESET)"
	@./$(BINARY_NAME)

# Run the application with prod profile
.PHONY: run-prod
run-prod: build
	@echo "$(BLUE)▶️  Running $(BINARY_NAME) with prod profile...$(RESET)"
	@./$(BINARY_NAME) --profile prod

# Show help for the CLI
.PHONY: help-cli
help-cli: build
	@./$(BINARY_NAME) --help

# Run tests
.PHONY: test
test:
	@echo "$(BLUE)🧪 Running tests...$(RESET)"
	@go test ./...
	@echo "$(GREEN)✓ Tests completed$(RESET)"
	@echo

# Run tests with verbose output
.PHONY: test-verbose
test-verbose:
	@echo "$(BLUE)🧪 Running tests with verbose output...$(RESET)"
	@go test -v ./...
	@echo

# Clean up dependencies and cache
.PHONY: tidy
tidy:
	@echo "$(BLUE)🧹 Tidying dependencies...$(RESET)"
	@go mod tidy
	@echo "$(GREEN)✓ Dependencies tidied$(RESET)"
	@echo

# Format code
.PHONY: fmt
fmt:
	@echo "$(BLUE)✨ Formatting code...$(RESET)"
	@go fmt ./...
	@echo "$(GREEN)✓ Code formatted$(RESET)"
	@echo

# Run linter (requires golangci-lint)
.PHONY: lint
lint:
	@echo "$(BLUE)🔍 Running linter...$(RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
		echo "$(GREEN)✓ Linter completed$(RESET)"; \
	else \
		echo "$(YELLOW)⚠ golangci-lint not installed, skipping...$(RESET)"; \
	fi
	@echo

# Clean build artifacts
.PHONY: clean
clean:
	@echo "$(BLUE)🗑️  Cleaning build artifacts...$(RESET)"
	@rm -f $(BINARY_NAME)
	@go clean
	@echo "$(GREEN)✓ Cleaned$(RESET)"
	@echo

# Development workflow: format, tidy, test, build
.PHONY: dev
dev: fmt tidy build
	@echo "$(GREEN)$(BOLD)🎉 Development workflow completed$(RESET)"
	@echo

# Pre-commit checks: format, tidy, lint, test
.PHONY: check
check: fmt tidy lint test
	@echo "$(GREEN)$(BOLD)🚀 Pre-commit checks completed$(RESET)"
	@echo

# Show available make targets
.PHONY: help
help:
	@echo "$(BOLD)$(BLUE)DynamoDB TUI Explorer - Available Make Targets:$(RESET)"
	@echo
	@echo "$(YELLOW)Building:$(RESET)"
	@echo "  $(GREEN)build$(RESET)          Build the $(BINARY_NAME) binary"
	@echo "  $(GREEN)install$(RESET)        Build and add project directory to PATH"
	@echo "  $(GREEN)clean$(RESET)          Remove build artifacts"
	@echo
	@echo "$(YELLOW)Running:$(RESET)"
	@echo "  $(GREEN)run$(RESET)            Build and run with dev profile"
	@echo "  $(GREEN)run-prod$(RESET)       Build and run with prod profile"
	@echo "  $(GREEN)help-cli$(RESET)       Show CLI help"
	@echo
	@echo "$(YELLOW)Testing & Quality:$(RESET)"
	@echo "  $(GREEN)test$(RESET)           Run all tests"
	@echo "  $(GREEN)test-verbose$(RESET)   Run tests with verbose output"
	@echo "  $(GREEN)fmt$(RESET)            Format Go code"
	@echo "  $(GREEN)lint$(RESET)           Run golangci-lint (if installed)"
	@echo "  $(GREEN)tidy$(RESET)           Clean up Go dependencies"
	@echo
	@echo "$(YELLOW)Development:$(RESET)"
	@echo "  $(GREEN)dev$(RESET)            Development workflow (fmt + tidy + build)"
	@echo "  $(GREEN)check$(RESET)          Pre-commit checks (fmt + tidy + lint + test)"
	@echo "  $(GREEN)watch$(RESET)          Watch files and rebuild on changes"
	@echo
	@echo "$(YELLOW)Help:$(RESET)"
	@echo "  $(GREEN)help$(RESET)           Show this help message"
	@echo

# File watching for development (requires entr)
.PHONY: watch
watch:
	@if command -v find >/dev/null 2>&1 && command -v entr >/dev/null 2>&1; then \
		echo "$(BLUE)👀 Watching for changes... (press Ctrl+C to stop)$(RESET)"; \
		find . -name "*.go" | entr -r make dev; \
	else \
		echo "$(YELLOW)⚠ 'find' or 'entr' not available, cannot watch files$(RESET)"; \
		echo "$(YELLOW)Install entr: brew install entr (macOS) or apt-get install entr (Linux)$(RESET)"; \
	fi
	@echo
