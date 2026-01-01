# QuantumLife Makefile
#
# Reference:
#   - docs/QUANTUMLIFE_CANON_V1.md (meaning)
#   - docs/TECHNICAL_SPLIT_V1.md (boundaries)
#   - docs/TECHNOLOGY_SELECTION_V1.md (technology)
#
# Guardrails enforce Canon invariants at build time.

.PHONY: all build test fmt lint vet guardrails ci clean help ingest-once demo-phase2 demo-phase3 demo-phase4 demo-phase5 demo-phase6 demo-phase7 demo-phase8 demo-phase9 demo-phase10 demo-phase11 demo-phase12 demo-phase13 demo-phase13-1 demo-phase14 demo-phase15 demo-phase16 demo-phase18 web web-mock web-demo web-app web-stop web-status

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
	@echo "Demos:"
	@echo "  make demo-phase2  - Run Phase 2 obligation extraction demo"
	@echo "  make demo-phase3  - Run Phase 3 interruption engine demo"
	@echo "  make demo-phase4  - Run Phase 4 drafts-only assistance demo"
	@echo "  make demo-phase5  - Run Phase 5 calendar execution demo"
	@echo "  make demo-phase6  - Run Phase 6 quiet loop demo"
	@echo "  make demo-phase7  - Run Phase 7 email execution demo"
	@echo "  make demo-phase8  - Run Phase 8 commerce mirror demo"
	@echo "  make demo-phase9  - Run Phase 9 commerce action drafts demo"
	@echo "  make demo-phase10 - Run Phase 10 execution routing demo"
	@echo "  make demo-phase11 - Run Phase 11 multi-circle demo"
	@echo "  make demo-phase12 - Run Phase 12 persistence and replay demo"
	@echo "  make demo-phase13 - Run Phase 13 identity graph demo"
	@echo "  make demo-phase13-1 - Run Phase 13.1 identity routing + people UI demo"
	@echo "  make demo-phase14 - Run Phase 14 policy learning demo"
	@echo "  make demo-phase15 - Run Phase 15 household approvals demo"
	@echo "  make demo-phase16 - Run Phase 16 notification projection demo"
	@echo "  make demo-phase18 - Run Phase 18 product language demo"
	@echo ""
	@echo "Web Server:"
	@echo "  make web          - Run web server on :8080 (real mode)"
	@echo "  make web-mock     - Run web server on :8080 with mock data"
	@echo "  make web-demo     - Run web server on :8080 in demo mode"
	@echo "  make web-app      - Run web server on :8080 in app mode"
	@echo "  make web-stop     - Stop whatever is listening on :8080"
	@echo "  make web-status   - Check if :8080 is bound"
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
	@echo "  make check-email-execution    - Check email execution boundary (Phase 7)"
	@echo "  make check-commerce-drafts    - Check commerce drafts boundary (Phase 9)"
	@echo "  make check-execute-routing    - Check execute routing boundary (Phase 10)"
	@echo "  make check-multicircle        - Check multi-circle constraints (Phase 11)"
	@echo "  make check-persistence-replay - Check persistence and replay constraints (Phase 12)"
	@echo "  make check-identity-graph     - Check identity graph constraints (Phase 13)"
	@echo "  make check-identity-routing-web - Check identity routing + people UI (Phase 13.1)"
	@echo "  make check-policy-learning - Check policy learning constraints (Phase 14)"
	@echo "  make check-household-approvals - Check household approvals constraints (Phase 15)"
	@echo "  make check-notification-projection - Check notification projection constraints (Phase 16)"
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

check-email-execution:
	@echo "Checking email execution boundary (Phase 7)..."
	@./scripts/guardrails/email_execution_enforced.sh

check-commerce-drafts:
	@echo "Checking commerce drafts boundary (Phase 9)..."
	@./scripts/guardrails/commerce_drafts_enforced.sh

check-execute-routing:
	@echo "Checking execute routing boundary (Phase 10)..."
	@./scripts/guardrails/execute_routing_enforced.sh

check-multicircle:
	@echo "Checking multi-circle constraints (Phase 11)..."
	@./scripts/guardrails/multicircle_enforced.sh

check-persistence-replay:
	@echo "Checking persistence and replay constraints (Phase 12)..."
	@./scripts/guardrails/persistence_replay_enforced.sh

check-identity-graph:
	@echo "Checking identity graph constraints (Phase 13)..."
	@./scripts/guardrails/identity_graph_enforced.sh

check-identity-routing-web:
	@echo "Checking identity routing + people UI constraints (Phase 13.1)..."
	@./scripts/guardrails/identity_routing_web_enforced.sh

check-policy-learning:
	@echo "Checking policy learning constraints (Phase 14)..."
	@./scripts/guardrails/policy_learning_enforced.sh

check-household-approvals:
	@echo "Checking household approvals constraints (Phase 15)..."
	@./scripts/guardrails/household_approvals_enforced.sh

check-notification-projection:
	@echo "Checking notification projection constraints (Phase 16)..."
	@./scripts/guardrails/notification_projection_enforced.sh

check-finance-execution:
	@echo "Checking finance execution constraints (Phase 17)..."
	@./scripts/guardrails/finance_execution_enforced.sh

# All guardrails
guardrails: check-terms check-imports check-deps check-time-now check-background-async check-no-auto-retry check-single-trace-final check-write-provider-reg check-free-text-recipient check-policy-snapshot check-finance-execution
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
	@echo "  ✓ Finance execution boundary (Phase 17)"

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

# Phase 2 Demo: Obligation Extraction + Daily View Generation
# Reference: docs/ADR/ADR-0019-phase2-obligation-extraction.md
demo-phase2:
	@echo "Running Phase 2 Demo: Obligation Extraction..."
	go run ./internal/demo_phase2_obligations

# Phase 3 Demo: Interruption Engine + Weekly Digest
# Reference: docs/ADR/ADR-0020-phase3-interruptions-and-digest.md
demo-phase3:
	@echo "Running Phase 3 Demo: Interruption Engine..."
	go run ./internal/demo_phase3_interruptions

# Phase 4 Demo: Drafts-Only Assistance
# Reference: docs/ADR/ADR-0021-phase4-drafts-only-assistance.md
demo-phase4:
	@echo "Running Phase 4 Demo: Drafts-Only Assistance..."
	go run ./internal/demo_phase4_drafts/main.go

# Phase 5 Demo: Calendar Execution Boundary
# Reference: docs/ADR/ADR-0022-phase5-calendar-execution-boundary.md
demo-phase5:
	@echo "Running Phase 5 Demo: Calendar Execution Boundary..."
	go test -v ./internal/demo_phase5_calendar_execution/...

# Phase 6 Demo: The Quiet Loop (Web)
# Reference: docs/ADR/ADR-0023-phase6-quiet-loop-web.md
demo-phase6:
	@echo "Running Phase 6 Demo: The Quiet Loop..."
	go test -v ./internal/demo_phase6_quiet_loop/...

# Phase 7 Demo: Email Execution Boundary
# Reference: Phase 7 Email Execution Boundary
demo-phase7:
	@echo "Running Phase 7 Demo: Email Execution Boundary..."
	go test -v ./internal/demo_phase7_email_execution/...

# Phase 8 Demo: Commerce Mirror
# Reference: docs/ADR/ADR-0024-phase8-commerce-mirror-email-derived.md
demo-phase8:
	@echo "Running Phase 8 Demo: Commerce Mirror..."
	go test -v ./internal/demo_phase8_commerce_mirror/...

# Phase 9 Demo: Commerce Action Drafts
# Reference: docs/ADR/ADR-0025-phase9-commerce-action-drafts.md
demo-phase9:
	@echo "Running Phase 9 Demo: Commerce Action Drafts..."
	go test -v ./internal/demo_phase9_commerce_drafts/...

# Phase 10 Demo: Approved Draft → Execution Routing
# Reference: Phase 10 Execution Routing
demo-phase10:
	@echo "Running Phase 10 Demo: Execution Routing..."
	go run ./demo/demo_phase10_execute_routing

# Phase 11 Demo: Real Data Quiet Loop (Multi-account)
# Reference: docs/ADR/ADR-0026-phase11-multicircle-real-loop.md
demo-phase11:
	@echo "Running Phase 11 Demo: Multi-Circle..."
	go test -v ./internal/demo_phase11_multicircle/...

# Phase 12 Demo: Persistence + Deterministic Replay
# Reference: docs/ADR/ADR-0027-phase12-persistence-replay.md
demo-phase12:
	@echo "Running Phase 12 Demo: Persistence & Replay..."
	go test -v ./internal/demo_phase12_persistence_replay/...

# Phase 13 Demo: Identity + Contact Graph Unification
# Reference: docs/ADR/ADR-0028-phase13-identity-graph.md
demo-phase13:
	@echo "Running Phase 13 Demo: Identity Graph..."
	go test -v ./internal/demo_phase13_identity_graph/...

# Phase 13.1 Demo: Identity-Driven Routing + People UI
# Reference: docs/ADR/ADR-0029-phase13-1-identity-driven-routing-and-people-ui.md
demo-phase13-1:
	@echo "Running Phase 13.1 Demo: Identity Routing + People UI..."
	go test -v ./internal/demo_phase13_1_identity_routing_web/...

# Phase 14 Demo: Circle Policies + Preference Learning
# Reference: docs/ADR/ADR-0030-phase14-policy-learning.md
demo-phase14:
	@echo "Running Phase 14 Demo: Policy Learning..."
	go test -v ./internal/demo_phase14_policy_learning/...

# Phase 15 Demo: Household Approvals + Intersections
# Reference: docs/ADR/ADR-0031-phase15-household-approvals.md
demo-phase15:
	@echo "Running Phase 15 Demo: Household Approvals..."
	go test -v ./internal/demo_phase15_household_approvals/...

# Phase 16 Demo: Notification Projection
# Reference: docs/ADR/ADR-0032-phase16-notification-projection.md
demo-phase16:
	@echo "Running Phase 16 Demo: Notification Projection..."
	go test -v ./internal/demo_phase16_notifications/...

# Phase 18 Demo: Product Language System
# Reference: docs/ADR/ADR-0034-phase18-product-language-and-web-shell.md
demo-phase18:
	@echo "Running Phase 18 Demo: Product Language System..."
	go test -v ./internal/demo_phase18_product_language/...

# =============================================================================
# Web Server Targets
# =============================================================================
# These are convenience targets for running the QuantumLife web server.
# The server supports graceful shutdown via SIGINT/SIGTERM.
# Reference: docs/ADR/ADR-0023-phase6-quiet-loop-web.md

# Run web server in real mode (no mock data)
web:
	@echo "Starting QuantumLife Web on :8080 (real mode)..."
	go run ./cmd/quantumlife-web -mock=false

# Run web server with mock data
web-mock:
	@echo "Starting QuantumLife Web on :8080 (mock mode)..."
	go run ./cmd/quantumlife-web -mock=true

# Run web server in demo mode (deterministic seed)
# Reference: docs/GUIDED_DEMO_SCRIPT_V1.md
web-demo:
	@echo "Starting QuantumLife Web on :8080 (demo mode)..."
	go run ./cmd/quantumlife-web -mock=true

# Run web server in app mode (alias for real mode)
web-app:
	@echo "Starting QuantumLife Web on :8080 (app mode)..."
	go run ./cmd/quantumlife-web -mock=false

# Stop whatever is listening on :8080
# Safe to run even if nothing is listening (will not fail)
web-stop:
	@echo "Stopping process on :8080..."
	@lsof -ti :8080 | xargs -r kill 2>/dev/null || true
	@echo "Done."

# Check if :8080 is bound
# Always returns 0 (safe for scripts)
web-status:
	@echo "Checking :8080 status..."
	@if lsof -i :8080 >/dev/null 2>&1; then \
		echo "Port 8080: BOUND"; \
		lsof -i :8080; \
	else \
		echo "Port 8080: FREE"; \
	fi
