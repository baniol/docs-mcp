.PHONY: help build run test test-coverage test-race lint fmt fmt-check vet \
        check-all clean setup-hooks \
        bump-patch bump-minor bump-major \
        t ca

BINARY := bin/docs-mcp
MODULE  := github.com/baniol/docs-mcp

# Default target
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ─── Build ────────────────────────────────────────────────────────────────────

build: ## Build binary to ./bin/docs-mcp
	@mkdir -p bin
	go build -o $(BINARY) ./cmd/server

run: build ## Build and run locally (requires .env or exported vars)
	./$(BINARY)

# ─── Quality ──────────────────────────────────────────────────────────────────

fmt: ## Format code with gofmt + goimports
	gofmt -w .
	@which goimports > /dev/null 2>&1 && goimports -w . || true

fmt-check: ## Check formatting (non-zero exit if unformatted)
	@diff=$$(gofmt -l .); \
	if [ -n "$$diff" ]; then \
		echo "unformatted files:"; echo "$$diff"; exit 1; \
	fi

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (install: https://golangci-lint.run/usage/install/)
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

# ─── Tests ────────────────────────────────────────────────────────────────────

test: ## Run all tests
	go test ./...

test-race: ## Run tests with race detector
	go test -race ./...

test-coverage: ## Run tests with coverage, open HTML report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# ─── Combined ─────────────────────────────────────────────────────────────────

check-all: ## Run all checks (fmt, vet, lint, tests)
	@echo ""
	@echo ">>> gofmt"
	@$(MAKE) --no-print-directory fmt-check
	@echo ""
	@echo ">>> go vet"
	@$(MAKE) --no-print-directory vet
	@echo ""
	@echo ">>> golangci-lint"
	@$(MAKE) --no-print-directory lint
	@echo ""
	@echo ">>> tests"
	@$(MAKE) --no-print-directory test
	@echo ""
	@echo ">>> All checks passed!"

# ─── Maintenance ──────────────────────────────────────────────────────────────

clean: ## Remove build artifacts
	rm -rf bin/ coverage.out

setup-hooks: ## Configure git hooks
	git config core.hooksPath .githooks
	@echo "Git hooks configured (.githooks/)"

# ─── Release ──────────────────────────────────────────────────────────────────

bump-patch: ## Bump patch (vX.Y.Z → vX.Y.Z+1): update CHANGELOG, commit, tag
	@scripts/bump.sh patch

bump-minor: ## Bump minor (vX.Y.Z → vX.Y+1.0): update CHANGELOG, commit, tag
	@scripts/bump.sh minor

bump-major: ## Bump major (vX.Y.Z → vX+1.0.0): update CHANGELOG, commit, tag
	@scripts/bump.sh major

# ─── Aliases ──────────────────────────────────────────────────────────────────

t:  test       ## Alias for test
ca: check-all  ## Alias for check-all
