# QuantumLife Makefile
#
# Reference:
#   - docs/QUANTUMLIFE_CANON_V1.md (meaning)
#   - docs/TECHNICAL_SPLIT_V1.md (boundaries)
#   - docs/TECHNOLOGY_SELECTION_V1.md (technology)
#
# Guardrails enforce Canon invariants at build time.

.PHONY: all build test fmt lint vet guardrails ci clean help ingest-once

# Default target
all: ci

# Help
help:
	@echo "QuantumLife Build Targets"
	@echo ""
	@echo "  make build      - Build all packages"
	@echo "  make test       - Run all tests"
	@echo "  make fmt        - Format code with gofmt"
	@echo "  make fmt-check  - Check if code is formatted"
	@echo "  make lint       - Run go vet"
	@echo "  make vet        - Run go vet (alias)"
	@echo "  make guardrails - Run all guardrail checks"
	@echo "  make ci         - Run full CI pipeline"
	@echo "  make clean      - Clean build artifacts"
	@echo "  make ingest-once- Run read-only ingestion (one-shot)"
	@echo ""
	@echo "Guardrail Checks:"
	@echo "  make check-terms              - Check for forbidden terms"
	@echo "  make check-imports            - Check for forbidden imports"
	@echo "  make check-deps               - Check dependency policy"
	@echo "  make check-time-now           - Check for forbidden time.Now() (v9.6.2)"
	@echo "  make check-background-async   - Check for background execution (v9.7)"
	@echo "  make check-no-auto-retry      - Check for auto-retry patterns (v9.8)"
	@echo "  make check-single-trace-final - Check for single trace finalization (v9.8)"
	@echo "  make check-write-provider-reg - Check write provider registry (v9.9)"
	@echo "  make check-free-text-recipient- Check free-text recipient elimination (v9.10)"
	@echo "  make check-policy-snapshot    - Check policy snapshot enforcement (v9.12.1)"
	@echo ""

# Build
build:
	@echo "Building all packages..."
	go build ./...

# Test
test:
	@echo "Running tests..."
	go test ./...

# Format
fmt:
	@echo "Formatting code..."
	gofmt -w -s .

# Format check (for CI)
fmt-check:
	@echo "Checking code formatting..."
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "ERROR: The following files are not formatted:"; \
		gofmt -l .; \
		echo ""; \
		echo "Run 'make fmt' to fix."; \
		exit 1; \
	fi
	@echo "All files are properly formatted."

# Lint (go vet)
lint: vet

vet:
	@echo "Running go vet..."
	go vet ./...

# Individual guardrail checks
check-terms:
	@echo "Checking for forbidden terms..."
	@./scripts/guardrails/forbidden_terms.sh

check-imports:
	@echo "Checking for forbidden imports..."
	@./scripts/guardrails/forbidden_imports.sh

check-deps:
	@echo "Checking dependency policy..."
	@./scripts/guardrails/dependency_policy.sh

check-time-now:
	@echo "Checking for forbidden time.Now() usage (v9.6.2)..."
	@./scripts/guardrails/forbidden_time_now.sh --check

check-background-async:
	@echo "Checking for forbidden background execution (v9.7)..."
	@./scripts/guardrails/forbidden_background_async.sh --check

check-no-auto-retry:
	@echo "Checking for forbidden auto-retry patterns (v9.8)..."
	@./scripts/guardrails/forbidden_auto_retry.sh --check

check-single-trace-final:
	@echo "Checking for single trace finalization (v9.8)..."
	@./scripts/guardrails/single_trace_finalization.sh --check

check-write-provider-reg:
	@echo "Checking write provider registry enforcement (v9.9)..."
	@./scripts/guardrails/forbidden_unregistered_write_provider.sh --check

check-free-text-recipient:
	@echo "Checking free-text recipient elimination (v9.10)..."
	@./scripts/guardrails/forbidden_free_text_recipient.sh --check

check-policy-snapshot:
	@echo "Checking policy snapshot enforcement (v9.12.1)..."
	@./scripts/guardrails/policy_snapshot_enforced.sh --check

# All guardrails
guardrails: check-terms check-imports check-deps check-time-now check-background-async check-no-auto-retry check-single-trace-final check-write-provider-reg check-free-text-recipient check-policy-snapshot
	@echo ""
	@echo "All guardrails passed."

# Full CI pipeline
ci: fmt-check vet build test guardrails
	@echo ""
	@echo "========================================"
	@echo "CI Pipeline Complete - All Checks Passed"
	@echo "========================================"
	@echo ""
	@echo "Canon invariants enforced:"
	@echo "  ✓ Code formatting"
	@echo "  ✓ Static analysis (go vet)"
	@echo "  ✓ Tests pass"
	@echo "  ✓ No forbidden terms"
	@echo "  ✓ No forbidden imports"
	@echo "  ✓ Dependency policy"
	@echo "  ✓ No forbidden time.Now() (v9.6.2)"
	@echo "  ✓ No background execution (v9.7)"
	@echo "  ✓ No auto-retry patterns (v9.8)"
	@echo "  ✓ Single trace finalization (v9.8)"
	@echo "  ✓ Write provider registry (v9.9)"
	@echo "  ✓ Free-text recipient elimination (v9.10)"
	@echo "  ✓ Policy snapshot enforcement (v9.12.1)"

# Clean
clean:
	@echo "Cleaning..."
	go clean ./...
	rm -f coverage.out

# Development helpers
.PHONY: dev-setup
dev-setup:
	@echo "Setting up development environment..."
	@echo "Ensuring scripts are executable..."
	chmod +x scripts/guardrails/*.sh
	@echo "Done."

# Run a quick check (faster than full CI)
.PHONY: quick
quick: fmt-check vet build
	@echo "Quick check passed."

# Run read-only ingestion once (synchronous, no background polling)
# CRITICAL: This command runs once and exits. For continuous ingestion,
# use an external scheduler (e.g., cron) to invoke this periodically.
ingest-once:
	@echo "Running read-only ingestion..."
	go run ./cmd/quantumlife-ingest
