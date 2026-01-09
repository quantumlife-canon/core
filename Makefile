# QuantumLife Makefile
#
# Reference:
#   - docs/QUANTUMLIFE_CANON_V1.md (meaning)
#   - docs/TECHNICAL_SPLIT_V1.md (boundaries)
#   - docs/TECHNOLOGY_SELECTION_V1.md (technology)
#
# Guardrails enforce Canon invariants at build time.

.PHONY: all build test fmt lint vet guardrails ci clean help ingest-once demo-phase2 demo-phase3 demo-phase4 demo-phase5 demo-phase6 demo-phase7 demo-phase8 demo-phase9 demo-phase10 demo-phase11 demo-phase12 demo-phase13 demo-phase13-1 demo-phase14 demo-phase15 demo-phase16 demo-phase18 demo-phase18-2 demo-phase18-3 demo-phase18-4 demo-phase18-5 demo-phase18-6 demo-phase18-9 demo-phase19-shadow demo-phase19-1 demo-phase19-4 demo-phase19-real-keys-smoke demo-phase20 demo-phase26A demo-phase26B demo-phase26C demo-phase29 demo-phase31-4 demo-phase42 demo-phase43 demo-phase44 web web-mock web-demo web-app web-stop web-status run-real-shadow check-real-shadow-config check-today-quietly check-held check-quiet-shift check-proof check-connection-onboarding check-shadow-mode check-shadow-diff check-real-gmail-quiet check-trust-accrual check-journey check-first-minutes check-reality-check check-truelayer-finance-mirror check-external-pressure check-delegated-holding check-held-proof check-trust-transfer ios-open ios-build ios-test ios-clean

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
	@echo "  make demo-phase18-2 - Run Phase 18.2 today quietly demo"
	@echo "  make demo-phase18-3 - Run Phase 18.3 held projection demo"
	@echo "  make demo-phase18-4 - Run Phase 18.4 quiet shift demo"
	@echo "  make demo-phase18-5 - Run Phase 18.5 quiet proof demo"
	@echo "  make demo-phase18-6 - Run Phase 18.6 first connect demo"
	@echo "  make demo-phase18-7 - Run Phase 18.7 mirror proof demo"
	@echo "  make demo-phase18-8 - Run Phase 18.8 OAuth Gmail demo"
	@echo "  make demo-phase18-9 - Run Phase 18.9 quiet verification demo"
	@echo "  make demo-phase19-shadow - Run Phase 19 shadow mode demo"
	@echo "  make demo-phase19-1 - Run Phase 19.1 real Gmail connection demo"
	@echo "  make demo-phase19-2 - Run Phase 19.2 shadow mode demo"
	@echo "  make demo-phase19-3 - Run Phase 19.3 Azure shadow provider demo"
	@echo "  make demo-phase19-3b - Run Phase 19.3b Go Real Azure + Embeddings demo"
	@echo "  make demo-phase19-3c - Run Phase 19.3c Real Azure Chat Shadow demo"
	@echo "  make demo-phase19-4 - Run Phase 19.4 shadow diff + calibration demo"
	@echo "  make demo-phase19-5 - Run Phase 19.5 shadow gating demo"
	@echo "  make demo-phase19-6 - Run Phase 19.6 rule pack export demo"
	@echo "  make demo-phase20 - Run Phase 20 trust accrual demo"
	@echo "  make demo-phase21 - Run Phase 21 onboarding/shadow receipt demo"
	@echo "  make demo-phase22 - Run Phase 22 quiet inbox mirror demo"
	@echo "  make demo-phase23 - Run Phase 23 gentle invitation demo"
	@echo "  make demo-phase24 - Run Phase 24 first action demo"
	@echo "  make demo-phase25 - Run Phase 25 undoable execution demo"
	@echo "  make demo-phase26A - Run Phase 26A guided journey demo"
	@echo "  make demo-phase26B - Run Phase 26B first minutes proof demo"
	@echo "  make demo-phase26C - Run Phase 26C connected reality check demo"
	@echo "  make demo-phase27 - Run Phase 27 real shadow receipt demo"
	@echo "  make demo-phase28 - Run Phase 28 trust kept demo"
	@echo "  make demo-phase29 - Run Phase 29 TrueLayer finance mirror demo"
	@echo "  make demo-phase30A - Run Phase 30A identity + replay demo"
	@echo "  make demo-phase31  - Run Phase 31 commerce observers demo"
	@echo "  make demo-phase31-1 - Run Phase 31.1 gmail receipt observers demo"
	@echo "  make demo-phase31-2 - Run Phase 31.2 commerce from finance demo"
	@echo "  make demo-phase31-3b - Run Phase 31.3b TrueLayer real sync demo"
	@echo "  make demo-phase31-4 - Run Phase 31.4 external pressure circles demo"
	@echo "  make demo-phase42 - Run Phase 42 delegated holding contracts demo"
	@echo "  make demo-phase43 - Run Phase 43 held under agreement proof ledger demo"
	@echo "  make demo-phase44 - Run Phase 44 cross-circle trust transfer (HOLD-only) demo"
	@echo "  make demo-phase55 - Run Phase 55 observer consent activation UI demo"
	@echo ""
	@echo "Web Server:"
	@echo "  make web          - Run web server on :8080 (real mode)"
	@echo "  make web-mock     - Run web server on :8080 with mock data"
	@echo "  make web-demo     - Run web server on :8080 in demo mode"
	@echo "  make web-app      - Run web server on :8080 in app mode"
	@echo "  make web-stop     - Stop whatever is listening on :8080"
	@echo "  make web-status   - Check if :8080 is bound"
	@echo "  make run-real-shadow - Run web with real Azure shadow provider"
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
	@echo "  make check-today-quietly - Check today quietly constraints (Phase 18.2)"
	@echo "  make check-held - Check held projection constraints (Phase 18.3)"
	@echo "  make check-quiet-shift - Check quiet shift constraints (Phase 18.4)"
	@echo "  make check-real-gmail-quiet - Check real Gmail quiet constraints (Phase 19.1)"
	@echo "  make check-shadow-mode - Check shadow mode constraints (Phase 19.2)"
	@echo "  make check-shadow-azure - Check Azure shadow constraints (Phase 19.3)"
	@echo "  make check-go-real-azure - Check go real azure constraints (Phase 19.3b)"
	@echo "  make check-shadow-real-chat - Check shadow real chat constraints (Phase 19.3c)"
	@echo "  make check-real-shadow-config - Check real shadow config (Phase 19.3)"
	@echo "  make check-shadow-diff - Check shadow diff constraints (Phase 19.4)"
	@echo "  make check-shadow-gating - Check shadow gating constraints (Phase 19.5)"
	@echo "  make check-rulepack-export - Check rule pack export constraints (Phase 19.6)"
	@echo "  make check-trust-accrual - Check trust accrual constraints (Phase 20)"
	@echo "  make check-phase25 - Check undoable execution constraints (Phase 25)"
	@echo "  make check-journey - Check guided journey constraints (Phase 26A)"
	@echo "  make check-first-minutes - Check first minutes proof constraints (Phase 26B)"
	@echo "  make check-reality-check - Check connected reality check constraints (Phase 26C)"
	@echo "  make check-shadow-receipt-primary - Check shadow receipt primary constraints (Phase 27)"
	@echo "  make check-trust-kept - Check trust kept constraints (Phase 28)"
	@echo "  make check-truelayer-finance-mirror - Check TrueLayer finance mirror constraints (Phase 29)"
	@echo "  make check-identity-replay - Check identity + replay constraints (Phase 30A)"
	@echo "  make check-commerce-observer - Check commerce observer constraints (Phase 31)"
	@echo "  make check-receipt-observer - Check receipt observer constraints (Phase 31.1)"
	@echo "  make check-commerce-from-finance - Check commerce from finance constraints (Phase 31.2)"
	@echo "  make check-external-pressure - Check external pressure constraints (Phase 31.4)"
	@echo "  make check-delegated-holding - Check delegated holding constraints (Phase 42)"
	@echo "  make check-held-proof - Check held proof constraints (Phase 43)"
	@echo "  make check-trust-transfer - Check trust transfer constraints (Phase 44)"
	@echo ""
	@echo "iOS (Phase 19):"
	@echo "  make ios-open   - Open iOS project in Xcode (macOS only)"
	@echo "  make ios-build  - Build iOS project (macOS only)"
	@echo "  make ios-test   - Run iOS tests (macOS only)"
	@echo "  make ios-clean  - Clean iOS build artifacts"
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

check-today-quietly:
	@echo "Checking today quietly constraints (Phase 18.2)..."
	@./scripts/guardrails/today_quietly_enforced.sh

check-held:
	@echo "Checking held projection constraints (Phase 18.3)..."
	@./scripts/guardrails/held_projection_enforced.sh

check-quiet-shift:
	@echo "Checking quiet shift constraints (Phase 18.4)..."
	@./scripts/guardrails/quiet_shift_enforced.sh

check-proof:
	@echo "Checking proof constraints (Phase 18.5)..."
	@./scripts/guardrails/proof_enforced.sh

check-connection-onboarding:
	@echo "Checking connection onboarding constraints (Phase 18.6)..."
	@./scripts/guardrails/connection_onboarding_enforced.sh

check-mirror-proof:
	@echo "Checking mirror proof constraints (Phase 18.7)..."
	@./scripts/guardrails/mirror_proof_enforced.sh

check-oauth-gmail:
	@echo "Checking OAuth Gmail read-only constraints (Phase 18.8)..."
	@./scripts/guardrails/oauth_gmail_readonly_enforced.sh

check-quiet-verification:
	@echo "Checking quiet verification constraints (Phase 18.9)..."
	@./scripts/guardrails/quiet_verification_enforced.sh

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

# Phase 18.2 Demo: Today, quietly
# Reference: Phase 18.2 specification
demo-phase18-2:
	@echo "Running Phase 18.2 Demo: Today, quietly..."
	go test -v ./internal/demo_phase18_2_today_quietly/...

# Phase 18.3 Demo: Held, not shown
# Reference: docs/ADR/ADR-0035-phase18-3-proof-of-care.md
demo-phase18-3:
	@echo "Running Phase 18.3 Demo: Held, not shown..."
	go test -v ./internal/demo_phase18_3_held/...

# Phase 18.4 Demo: Quiet Shift
# Reference: docs/ADR/ADR-0036-phase18-4-quiet-shift.md
demo-phase18-4:
	@echo "Running Phase 18.4 Demo: Quiet Shift..."
	go test -v ./internal/demo_phase18_4_quiet_shift/...

# Phase 18.5 Demo: Quiet Proof
# Reference: docs/ADR/ADR-0037-phase18-5-quiet-proof.md
demo-phase18-5:
	@echo "Running Phase 18.5 Demo: Quiet Proof..."
	go test -v ./internal/demo_phase18_5_proof/...

# Phase 18.6 Demo: First Connect
# Reference: docs/ADR/ADR-0038-phase18-6-first-connect.md
demo-phase18-6:
	@echo "Running Phase 18.6 Demo: First Connect..."
	go test -v ./internal/demo_phase18_6_first_connect/...

# Phase 18.7 Demo: Mirror Proof
# Reference: docs/ADR/ADR-0039-phase18-7-mirror-proof.md
demo-phase18-7:
	@echo "Running Phase 18.7 Demo: Mirror Proof..."
	go test -v ./internal/demo_phase18_7_mirror/...

# Phase 18.8 Demo: OAuth Gmail Read-Only
# Reference: docs/ADR/ADR-0041-phase18-8-real-oauth-gmail-readonly.md
demo-phase18-8:
	@echo "Running Phase 18.8 Demo: OAuth Gmail Read-Only..."
	go test -v ./internal/demo_phase18_8_oauth_gmail/...

# Phase 18.9 Demo: Quiet Verification
# Reference: docs/ADR/ADR-0042-phase18-9-real-data-quiet-verification.md
demo-phase18-9:
	@echo "Running Phase 18.9 Demo: Quiet Verification..."
	go test -v ./internal/demo_phase18_9_quiet_verification/...

# Phase 19 Demo: LLM Shadow-Mode Contract
# Reference: docs/ADR/ADR-0043-phase19-shadow-mode-contract.md
demo-phase19-shadow:
	@echo "Running Phase 19 Demo: Shadow Mode Contract..."
	go test -v ./internal/demo_phase19_shadow_contract/...

# Phase 19.1 Demo: Real Gmail Connection (You-only)
# Reference: Phase 19.1 specification
demo-phase19-1:
	@echo "Running Phase 19.1 Demo: Real Gmail Connection..."
	go test -v ./internal/demo_phase19_1_real_gmail_quiet/...

# Check real Gmail quiet constraints (Phase 19.1)
check-real-gmail-quiet:
	@echo "Checking real Gmail quiet constraints (Phase 19.1)..."
	@./scripts/guardrails/real_gmail_quiet_enforced.sh

# Phase 19.2 Demo: Shadow Mode Contract
# Reference: docs/ADR/ADR-0043-phase19-2-shadow-mode-contract.md
demo-phase19-2:
	@echo "Running Phase 19.2 Demo: Shadow Mode Contract..."
	go test -v ./internal/demo_phase19_2_shadow_mode/...

demo-phase19-3:
	@echo "Running Phase 19.3 Demo: Azure OpenAI Shadow Provider..."
	go test -v ./internal/demo_phase19_3_azure_shadow/...

# Check shadow mode constraints (Phase 19.2)
check-shadow-mode:
	@echo "Checking shadow mode constraints (Phase 19.2)..."
	@./scripts/guardrails/shadow_mode_enforced.sh

# Check Azure shadow provider constraints (Phase 19.3)
check-shadow-azure:
	@echo "Checking Azure shadow provider constraints (Phase 19.3)..."
	@./scripts/guardrails/shadow_azure_enforced.sh

# Phase 19.3b Demo: Go Real Azure + Embeddings
# Reference: docs/ADR/ADR-0049-phase19-3b-go-real-azure-and-embeddings.md
demo-phase19-3b:
	@echo "Running Phase 19.3b Demo: Go Real Azure + Embeddings..."
	go test -v ./internal/demo_phase19_3b_go_real/...

# Check go real azure constraints (Phase 19.3b)
check-go-real-azure:
	@echo "Checking go real azure constraints (Phase 19.3b)..."
	@./scripts/guardrails/go_real_azure_enforced.sh

# Phase 19.3c Demo: Real Azure Chat Shadow Run
# Reference: docs/ADR/ADR-0050-phase19-3c-real-azure-chat-shadow.md
demo-phase19-3c:
	@echo "Running Phase 19.3c Demo: Real Azure Chat Shadow Run..."
	go test -v ./internal/demo_phase19_3c_real_chat_shadow/...

# Check shadow real chat constraints (Phase 19.3c)
check-shadow-real-chat:
	@echo "Checking shadow real chat constraints (Phase 19.3c)..."
	@./scripts/guardrails/shadow_real_chat_enforced.sh

# Phase 19.4 Demo: Shadow Diff + Calibration
# Reference: docs/ADR/ADR-0045-phase19-4-shadow-diff-calibration.md
demo-phase19-4:
	@echo "Running Phase 19.4 Demo: Shadow Diff + Calibration..."
	go test -v ./internal/demo_phase19_4_shadow_diff/...

# Check shadow diff constraints (Phase 19.4)
check-shadow-diff:
	@echo "Checking shadow diff constraints (Phase 19.4)..."
	@./scripts/guardrails/shadow_diff_enforced.sh

# Phase 19.5 Demo: Shadow Gating + Promotion Candidates
# Reference: docs/ADR/ADR-0046-phase19-5-shadow-gating-and-promotion-candidates.md
demo-phase19-5:
	@echo "Running Phase 19.5 Demo: Shadow Gating + Promotion Candidates..."
	go test -v ./internal/demo_phase19_5_shadow_gating/...

# Check shadow gating constraints (Phase 19.5)
check-shadow-gating:
	@echo "Checking shadow gating constraints (Phase 19.5)..."
	@./scripts/guardrails/shadow_gating_enforced.sh

# Phase 19.6 Demo: Rule Pack Export (Promotion Pipeline)
# Reference: docs/ADR/ADR-0047-phase19-6-rulepack-export.md
demo-phase19-6:
	@echo "Running Phase 19.6 Demo: Rule Pack Export..."
	go test -v ./internal/demo_phase19_6_rulepack_export/...

# Check rule pack export constraints (Phase 19.6)
check-rulepack-export:
	@echo "Checking rule pack export constraints (Phase 19.6)..."
	@./scripts/guardrails/rulepack_export_enforced.sh

# Phase 20 Demo: Trust Accrual Layer (Proof Over Time)
# Reference: docs/ADR/ADR-0048-phase20-trust-accrual-layer.md
demo-phase20:
	@echo "Running Phase 20 Demo: Trust Accrual Layer..."
	go test -v ./internal/demo_phase20_trust_accrual/...

# Check trust accrual constraints (Phase 20)
check-trust-accrual:
	@echo "Checking trust accrual constraints (Phase 20)..."
	@./scripts/guardrails/trust_accrual_enforced.sh

# Phase 21 Demo: Unified Onboarding + Shadow Receipt Viewer
# Reference: docs/ADR/ADR-0051-phase21-onboarding-modes-shadow-receipt-viewer.md
demo-phase21:
	@echo "Running Phase 21 Demo: Unified Onboarding + Shadow Receipt Viewer..."
	go test -v ./internal/demo_phase21_onboarding_shadow_receipt/...

# Check Phase 21 onboarding/shadowview constraints
check-phase21:
	@echo "Checking Phase 21 constraints..."
	@./scripts/guardrails/phase21_onboarding_shadow_receipt_enforced.sh

# Phase 22 Demo: Quiet Inbox Mirror (First Real Value Moment)
# Reference: docs/ADR/ADR-0052-phase22-quiet-inbox-mirror.md
demo-phase22:
	@echo "Running Phase 22 Demo: Quiet Inbox Mirror..."
	go test -v ./internal/demo_phase22_quiet_inbox_mirror/...

# Check Phase 22 quiet inbox mirror constraints
check-phase22:
	@echo "Checking Phase 22 constraints..."
	@./scripts/guardrails/quiet_inbox_mirror_enforced.sh

# Phase 23 Demo: Gentle Action Invitation (Trust-Preserving)
# Reference: docs/ADR/ADR-0053-phase23-gentle-invitation.md
demo-phase23:
	@echo "Running Phase 23 Demo: Gentle Action Invitation..."
	go test -v ./internal/demo_phase23_gentle_invitation/...

# Check Phase 23 gentle invitation constraints
check-phase23:
	@echo "Checking Phase 23 constraints..."
	@./scripts/guardrails/gentle_invitation_enforced.sh

# Run Phase 24 demo: First Reversible Real Action
# Reference: docs/ADR/ADR-0054-phase24-first-reversible-action.md
demo-phase24:
	@echo "Running Phase 24 Demo: First Reversible Real Action..."
	go test -v ./internal/demo_phase24_first_action/...

# Check Phase 24 first action constraints
check-phase24:
	@echo "Checking Phase 24 constraints..."
	@./scripts/guardrails/first_action_enforced.sh

# Run Phase 25 demo: First Undoable Execution
# Reference: docs/ADR/ADR-0055-phase25-first-undoable-execution.md
demo-phase25:
	@echo "Running Phase 25 Demo: First Undoable Execution..."
	go test -v ./internal/demo_phase25_first_undoable_execution/...

# Check Phase 25 undoable execution constraints
check-phase25:
	@echo "Checking Phase 25 constraints..."
	@./scripts/guardrails/undoable_execution_enforced.sh

# Run Phase 26A demo: Guided Journey
# Reference: docs/ADR/ADR-0056-phase26A-guided-journey.md
demo-phase26A:
	@echo "Running Phase 26A Demo: Guided Journey..."
	go test -v ./internal/demo_phase26A_guided_journey/...

# Check Phase 26A guided journey constraints
check-journey:
	@echo "Checking Phase 26A constraints..."
	@./scripts/guardrails/journey_enforced.sh

# Run Phase 26B demo: First Minutes Proof
# Reference: docs/ADR/ADR-0056-phase26B-first-five-minutes-proof.md
demo-phase26B:
	@echo "Running Phase 26B Demo: First Minutes Proof..."
	go test -v ./internal/demo_phase26B_first_minutes/...

# Check Phase 26B first minutes proof constraints
check-first-minutes:
	@echo "Checking Phase 26B constraints..."
	@./scripts/guardrails/first_minutes_enforced.sh

# Run Phase 26C demo: Connected Reality Check
# Reference: docs/ADR/ADR-0057-phase26C-connected-reality-check.md
demo-phase26C:
	@echo "Running Phase 26C Demo: Connected Reality Check..."
	go test -v ./internal/demo_phase26C_reality_check/...

# Check Phase 26C connected reality check constraints
check-reality-check:
	@echo "Checking Phase 26C constraints..."
	@./scripts/guardrails/reality_check_enforced.sh

# Run Phase 27 demo: Real Shadow Receipt (Primary Proof)
# Reference: docs/ADR/ADR-0058-phase27-real-shadow-receipt-primary-proof.md
demo-phase27:
	@echo "Running Phase 27 Demo: Real Shadow Receipt (Primary Proof)..."
	go test -v ./internal/demo_phase27_shadow_receipt/...

# Check Phase 27 shadow receipt primary constraints
check-shadow-receipt-primary:
	@echo "Checking Phase 27 constraints..."
	@./scripts/guardrails/shadow_receipt_primary_enforced.sh

# Run Phase 28 demo: Trust Kept — First Real Act, Then Silence
# Reference: docs/ADR/ADR-0059-phase28-trust-kept.md
demo-phase28:
	@echo "Running Phase 28 Demo: Trust Kept..."
	go test -v ./internal/demo_phase28_trust_kept/...

# Check Phase 28 trust kept constraints
check-trust-kept:
	@echo "Checking Phase 28 constraints..."
	@./scripts/guardrails/trust_kept_enforced.sh

# Run Phase 29 demo: TrueLayer Read-Only Connect + Finance Mirror Proof
# Reference: docs/ADR/ADR-0060-phase29-truelayer-readonly-finance-mirror.md
demo-phase29:
	@echo "Running Phase 29 Demo: TrueLayer Finance Mirror..."
	go test -v ./internal/demo_phase29_truelayer_finance_mirror/...

# Check Phase 29 TrueLayer finance mirror constraints
check-truelayer-finance-mirror:
	@echo "Checking Phase 29 constraints..."
	@./scripts/guardrails/truelayer_finance_mirror_enforced.sh

# Run Phase 30A demo: Identity + Replay
# Reference: docs/ADR/ADR-0061-phase30A-identity-and-replay.md
demo-phase30A:
	@echo "Running Phase 30A Demo: Identity + Replay..."
	go test -v ./internal/demo_phase30A_identity_replay/...

# Check Phase 30A Identity + Replay constraints
check-identity-replay:
	@echo "Checking Phase 30A constraints..."
	@./scripts/guardrails/identity_replay_enforced.sh

# Run Phase 31 demo: Commerce Observers
# Reference: docs/ADR/ADR-0062-phase31-commerce-observers.md
demo-phase31:
	@echo "Running Phase 31 Demo: Commerce Observers..."
	go test -v ./internal/demo_phase31_commerce_observer/...

# Check Phase 31 Commerce Observer constraints
check-commerce-observer:
	@echo "Checking Phase 31 constraints..."
	@./scripts/guardrails/commerce_observer_enforced.sh

# Phase 31.1: Gmail Receipt Observers (Email -> CommerceSignals)
# Reference: docs/ADR/ADR-0063-phase31-1-gmail-receipt-observers.md
demo-phase31-1:
	@echo "Running Phase 31.1 Demo: Gmail Receipt Observers..."
	go test -v ./internal/demo_phase31_1_gmail_receipt_observer/...

# Check Phase 31.1 Receipt Observer constraints
check-receipt-observer:
	@echo "Checking Phase 31.1 constraints..."
	@./scripts/guardrails/receipt_observer_enforced.sh

# Phase 31.2: Commerce from Finance (TrueLayer -> CommerceSignals)
# Reference: docs/ADR/ADR-0064-phase31-2-commerce-from-finance.md
demo-phase31-2:
	@echo "Running Phase 31.2 Demo: Commerce from Finance..."
	go test -v ./internal/demo_phase31_2_finance_commerce_observer/...

# Check Phase 31.2 Commerce from Finance constraints
check-commerce-from-finance:
	@echo "Checking Phase 31.2 constraints..."
	@./scripts/guardrails/commerce_from_finance_enforced.sh

# Phase 31.3: Real Finance Only (No Mock Path)
# Reference: docs/ADR/ADR-0065-phase31-3-real-finance-only.md
demo-phase31-3:
	@echo "Running Phase 31.3 Demo: Real Finance Only..."
	go test -v ./internal/demo_phase31_3_real_finance_only/...

# Check Phase 31.3 Real Finance Only constraints
check-real-finance-only:
	@echo "Checking Phase 31.3 constraints..."
	@./scripts/guardrails/commerce_real_finance_only_enforced.sh

# Phase 31.3b: Real TrueLayer Sync (Accounts + Transactions → Finance Mirror + Commerce Observer)
# Reference: docs/ADR/ADR-0066-phase31-3b-truelayer-real-sync.md
demo-phase31-3b:
	@echo "Running Phase 31.3b Demo: TrueLayer Real Sync..."
	go test -v ./internal/demo_phase31_3b_truelayer_real_sync/...

# Check Phase 31.3b TrueLayer Real Sync constraints
check-truelayer-real-sync:
	@echo "Checking Phase 31.3b constraints..."
	@./scripts/guardrails/truelayer_real_sync_enforced.sh

# Phase 31.4: External Pressure Circles + Intersection Pressure Map
# Reference: docs/ADR/ADR-0067-phase31-4-external-pressure-circles.md
demo-phase31-4:
	@echo "Running Phase 31.4 Demo: External Pressure Circles..."
	go test -v ./internal/demo_phase31_4_external_pressure/...

# Check Phase 31.4 External Pressure constraints
check-external-pressure:
	@echo "Checking Phase 31.4 constraints..."
	@./scripts/guardrails/external_pressure_enforced.sh

# Run Phase 32 Demo: Pressure Decision Gate
demo-phase32:
	@echo "Running Phase 32 Demo: Pressure Decision Gate..."
	go test -v ./internal/demo_phase32_pressure_decision/...

# Check Phase 32 Pressure Decision Gate constraints
check-pressure-decision-gate:
	@echo "Checking Phase 32 constraints..."
	@./scripts/guardrails/pressure_decision_gate_enforced.sh

# Run Phase 33 Demo: Interrupt Permission Contract
demo-phase33:
	@echo "Running Phase 33 Demo: Interrupt Permission Contract..."
	go test -v ./internal/demo_phase33_interrupt_permission/...

# Check Phase 33 Interrupt Permission Contract constraints
check-interrupt-permission:
	@echo "Checking Phase 33 constraints..."
	@./scripts/guardrails/interrupt_permission_enforced.sh

# Run Phase 34 Demo: Permitted Interrupt Preview
demo-phase34:
	@echo "Running Phase 34 Demo: Permitted Interrupt Preview..."
	go test -v ./internal/demo_phase34_interrupt_preview/...

# Check Phase 34 Permitted Interrupt Preview constraints
check-interrupt-preview:
	@echo "Checking Phase 34 constraints..."
	@./scripts/guardrails/interrupt_preview_enforced.sh

# Run Phase 35 Demo: Push Transport
demo-phase35:
	@echo "Running Phase 35 Demo: Push Transport..."
	go test -v ./internal/demo_phase35_push_transport/...

# Check Phase 35 Push Transport constraints
check-push-transport:
	@echo "Checking Phase 35 constraints..."
	@./scripts/guardrails/push_transport_enforced.sh

# Run Phase 35b Demo: APNs Sealed Secret Boundary
demo-phase35b:
	@echo "Running Phase 35b Demo: APNs Sealed Secret Boundary..."
	go test -v ./internal/demo_phase35b_apns_transport/...

# Check Phase 35b APNs Sealed Secret constraints
check-apns-sealed:
	@echo "Checking Phase 35b constraints..."
	@./scripts/guardrails/apns_sealed_secret_enforced.sh

# Run Phase 36 Demo: Interrupt Delivery Orchestrator
demo-phase36:
	@echo "Running Phase 36 Demo: Interrupt Delivery Orchestrator..."
	go test -v ./internal/demo_phase36_interrupt_delivery/...

# Check Phase 36 Interrupt Delivery constraints
check-interrupt-delivery:
	@echo "Checking Phase 36 constraints..."
	@./scripts/guardrails/interrupt_delivery_enforced.sh

# Run Phase 37 Demo: Device Registration + Deep-Link
demo-phase37:
	@echo "Running Phase 37 Demo: Device Registration + Deep-Link..."
	go test -v ./internal/demo_phase37_device_registration/...

# Check Phase 37 Device Registration constraints
check-device-registration:
	@echo "Checking Phase 37 constraints..."
	@./scripts/guardrails/device_registration_enforced.sh

# Run Phase 38 Demo: Mobile Notification Metadata Observer
demo-phase38:
	@echo "Running Phase 38 Demo: Mobile Notification Metadata Observer..."
	go test -v ./internal/demo_phase38_notification_observer/...

# Check Phase 38 Notification Observer constraints
check-notification-observer:
	@echo "Checking Phase 38 constraints..."
	@./scripts/guardrails/notification_observer_enforced.sh

# Run Phase 39 Demo: Attention Envelopes
demo-phase39:
	@echo "Running Phase 39 Demo: Attention Envelopes..."
	go test -v ./internal/demo_phase39_attention_envelope/...

# Check Phase 39 Attention Envelope constraints
check-attention-envelope:
	@echo "Checking Phase 39 constraints..."
	@./scripts/guardrails/attention_envelope_enforced.sh

# Run Phase 40 Demo: Time-Window Pressure Sources
demo-phase40:
	@echo "Running Phase 40 Demo: Time-Window Pressure Sources..."
	go test -v ./internal/demo_phase40_timewindow_sources/...

# Check Phase 40 Time Window constraints
check-timewindow-sources:
	@echo "Checking Phase 40 constraints..."
	@./scripts/guardrails/timewindow_pressure_sources_enforced.sh

# Run Phase 41 Demo: Live Interrupt Loop (APNs)
demo-phase41:
	@echo "Running Phase 41 Demo: Live Interrupt Loop (APNs)..."
	go test -v ./internal/demo_phase41_interrupt_rehearsal/...

# Check Phase 41 Interrupt Rehearsal constraints
check-interrupt-rehearsal:
	@echo "Checking Phase 41 constraints..."
	@./scripts/guardrails/interrupt_rehearsal_enforced.sh

# Run Phase 42 Demo: Delegated Holding Contracts
demo-phase42:
	@echo "Running Phase 42 Demo: Delegated Holding Contracts..."
	go test -v ./internal/demo_phase42_delegated_holding/...

# Check Phase 42 Delegated Holding constraints
check-delegated-holding:
	@echo "Checking Phase 42 constraints..."
	@./scripts/guardrails/delegated_holding_enforced.sh

# Run Phase 43 Demo: Held Under Agreement Proof Ledger
# Reference: docs/ADR/ADR-0080-phase43-held-under-agreement-proof-ledger.md
demo-phase43:
	@echo "Running Phase 43 Demo: Held Under Agreement Proof Ledger..."
	go test -v ./internal/demo_phase43_held_proof/...

# Check Phase 43 Held Proof constraints
check-held-proof:
	@echo "Checking Phase 43 constraints..."
	@./scripts/guardrails/held_proof_enforced.sh

# Run Phase 44 Demo: Cross-Circle Trust Transfer (HOLD-only)
# Reference: docs/ADR/ADR-0081-phase44-cross-circle-trust-transfer-hold-only.md
demo-phase44:
	@echo "Running Phase 44 Demo: Cross-Circle Trust Transfer (HOLD-only)..."
	go test -v ./internal/demo_phase44_trust_transfer/...

# Check Phase 44 Trust Transfer constraints
check-trust-transfer:
	@echo "Checking Phase 44 constraints..."
	@./scripts/guardrails/trust_transfer_enforced.sh

# Run Phase 44.2 Demo: Enforcement Wiring Audit
# Reference: docs/ADR/ADR-0082-phase44-2-enforcement-wiring-audit.md
demo-phase44-2:
	@echo "Running Phase 44.2 Demo: Enforcement Wiring Audit..."
	go test -v ./internal/demo_phase44_2_enforcement_audit/...

# Check Phase 44.2 Enforcement Wiring constraints
check-enforcement-wiring:
	@echo "Checking Phase 44.2 constraints..."
	@./scripts/guardrails/enforcement_wiring_enforced.sh

# Run Phase 45 Demo: Circle Semantics & Necessity Declaration
# Reference: docs/ADR/ADR-0083-phase45-circle-semantics-necessity.md
demo-phase45:
	@echo "Running Phase 45 Demo: Circle Semantics..."
	go test -v ./internal/demo_phase45_circle_semantics/...

# Check Phase 45 Circle Semantics constraints
check-circle-semantics:
	@echo "Checking Phase 45 constraints..."
	@./scripts/guardrails/circle_semantics_enforced.sh

# Run Phase 46 Demo: Circle Registry + Packs (Marketplace v0)
# Reference: docs/ADR/ADR-0084-phase46-circle-registry-packs.md
demo-phase46:
	@echo "Running Phase 46 Demo: Marketplace..."
	go test -v ./internal/demo_phase46_marketplace/...

# Check Phase 46 Marketplace constraints
check-marketplace:
	@echo "Checking Phase 46 constraints..."
	@./scripts/guardrails/marketplace_enforced.sh

# Run Phase 47 Demo: Coverage Realization
demo-phase47:
	@echo "Running Phase 47 Demo: Coverage Realization..."
	go test -v ./internal/demo_phase47_coverage_realization/...

# Check Phase 47 Coverage Realization constraints
check-coverage-realization:
	@echo "Checking Phase 47 constraints..."
	@./scripts/guardrails/coverage_realization_enforced.sh

# Run Phase 48 Demo: Market Signal Binding
# Reference: docs/ADR/ADR-0086-phase48-market-signal-binding.md
demo-phase48:
	@echo "Running Phase 48 Demo: Market Signal Binding..."
	go test -v ./internal/demo_phase48_market_signal/...

# Check Phase 48 Market Signal Binding constraints
check-market-signal:
	@echo "Checking Phase 48 constraints..."
	@./scripts/guardrails/market_signal_enforced.sh

# =============================================================================
# Phase 49: Vendor Reality Contracts Demo
# =============================================================================
# Demonstrates HOLD-first, clamp-only vendor contracts.
# Commerce vendors are always capped at SURFACE_ONLY.
# Contracts can only reduce pressure, never increase it.
# Reference: docs/ADR/ADR-0087-phase49-vendor-reality-contracts.md
demo-phase49:
	@echo "Running Phase 49 Demo: Vendor Reality Contracts..."
	go test -v ./internal/demo_phase49_vendor_contracts/...

# Check Phase 49 Vendor Reality Contracts constraints
check-vendor-contracts:
	@echo "Checking Phase 49 constraints..."
	@./scripts/guardrails/vendor_reality_contracts_enforced.sh

# =============================================================================
# Phase 50: Signed Vendor Claims + Pack Manifests
# =============================================================================
# Provides Ed25519-based authenticity primitives for vendor claims and pack manifests.
# This is authenticity-only - NO power to change decisions, outcomes, or delivery.
# Hash-only storage: only fingerprints stored, never raw keys or signatures.
# Reference: docs/ADR/ADR-0088-phase50-signed-vendor-claims-and-pack-manifests.md
demo-phase50:
	@echo "Running Phase 50 Demo: Signed Vendor Claims + Pack Manifests..."
	go test -v ./internal/demo_phase50_signed_claims/...

# Check Phase 50 Signed Claims constraints
check-signed-claims:
	@echo "Checking Phase 50 constraints..."
	@./scripts/guardrails/signed_claims_enforced.sh

# =============================================================================
# Phase 51: Transparency Log / Claim Ledger
# =============================================================================
# Append-only, hash-only, observation/proof-only ledger for signed claims/manifests.
# NO POWER: Does not affect decisions, outcomes, or delivery.
# Reference: docs/ADR/ADR-0089-phase51-transparency-log-claim-ledger.md
demo-phase51:
	@echo "Running Phase 51 Demo: Transparency Log..."
	go test -v ./internal/demo_phase51_transparency_log/...

# Check Phase 51 Transparency Log constraints
check-transparency-log:
	@echo "Checking Phase 51 constraints..."
	@./scripts/guardrails/transparency_log_enforced.sh

# =============================================================================
# Phase 52: Proof Hub + Connected Status
# =============================================================================
# Single proof hub page showing connected status abstractly.
# NO POWER: Observation/proof only, no execution or delivery.
# Reference: docs/ADR/ADR-0090-phase52-proof-hub-connected-status.md
demo-phase52:
	@echo "Running Phase 52 Demo: Proof Hub..."
	go test -v ./internal/demo_phase52_proof_hub/...

# Check Phase 52 Proof Hub constraints
check-proof-hub:
	@echo "Checking Phase 52 constraints..."
	@./scripts/guardrails/proof_hub_enforced.sh

# Run Phase 53 Urgency Resolution demo tests
# Phase 53: Urgency Resolution Layer - cap-only, clamp-only, no execution, no delivery.
# Reference: docs/ADR/ADR-0091-phase53-urgency-resolution-layer.md
demo-phase53:
	@echo "Running Phase 53 Demo: Urgency Resolution..."
	go test -v ./internal/demo_phase53_urgency_resolution/...

# Check Phase 53 Urgency Resolution constraints
check-urgency-resolution:
	@echo "Checking Phase 53 constraints..."
	@./scripts/guardrails/urgency_resolution_enforced.sh

# Run Phase 54 Urgency Delivery Binding demo tests
# Phase 54: Urgency → Delivery Binding - POST-triggered, no background execution.
# Reference: docs/ADR/ADR-0092-phase54-urgency-delivery-binding.md
demo-phase54:
	@echo "Running Phase 54 Demo: Urgency Delivery Binding..."
	go test -v ./internal/demo_phase54_urgency_delivery_binding/...

# Check Phase 54 Urgency Delivery Binding constraints
check-urgency-delivery-binding:
	@echo "Checking Phase 54 constraints..."
	@./scripts/guardrails/urgency_delivery_binding_enforced.sh

# =============================================================================
# Phase 55: Observer Consent Activation UI
# =============================================================================
# Provides explicit POST-only consent controls for observer capabilities.
# Uses existing Coverage Plan mechanism - does NOT add new power.
# Consent controls ONLY what may be observed, never what may be done.
# Hash-only storage, bounded retention (200 records OR 30 days).
# Reference: docs/ADR/ADR-0092-phase55-observer-consent-activation-ui.md
demo-phase55:
	@echo "Running Phase 55 Demo: Observer Consent Activation UI..."
	go test -v ./internal/demo_phase55_observer_consent/...

# Check Phase 55 Observer Consent constraints
check-observer-consent:
	@echo "Checking Phase 55 constraints..."
	@./scripts/guardrails/observer_consent_enforced.sh

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

# =============================================================================
# Real Shadow Provider Targets (Phase 19.3)
# =============================================================================
# Reference: docs/REAL_KEYS_LOCAL_RUNBOOK_V1.md
#
# CRITICAL: Requires Azure OpenAI credentials in environment variables.
# See docs/REAL_KEYS_LOCAL_RUNBOOK_V1.md for setup instructions.

# Run web server with real Azure shadow provider
# Requires: AZURE_OPENAI_ENDPOINT, AZURE_OPENAI_DEPLOYMENT, AZURE_OPENAI_API_KEY
run-real-shadow:
	@echo "Starting QuantumLife Web with real shadow provider on :8080..."
	@echo "Checking Azure OpenAI configuration..."
	@if [ -z "$$AZURE_OPENAI_ENDPOINT" ]; then \
		echo "ERROR: AZURE_OPENAI_ENDPOINT not set"; \
		echo "See: docs/REAL_KEYS_LOCAL_RUNBOOK_V1.md"; \
		exit 1; \
	fi
	@if [ -z "$$AZURE_OPENAI_DEPLOYMENT" ]; then \
		echo "ERROR: AZURE_OPENAI_DEPLOYMENT not set"; \
		echo "See: docs/REAL_KEYS_LOCAL_RUNBOOK_V1.md"; \
		exit 1; \
	fi
	@if [ -z "$$AZURE_OPENAI_API_KEY" ]; then \
		echo "ERROR: AZURE_OPENAI_API_KEY not set"; \
		echo "See: docs/REAL_KEYS_LOCAL_RUNBOOK_V1.md"; \
		exit 1; \
	fi
	@echo "Azure config verified (endpoint, deployment, key present)"
	@echo ""
	QL_SHADOW_REAL_ALLOWED=true QL_SHADOW_PROVIDER_KIND=azure_openai QL_SHADOW_MODE=observe go run ./cmd/quantumlife-web -mock=false

# Check real shadow configuration (without running)
check-real-shadow-config:
	@echo "Checking real shadow provider configuration..."
	@echo ""
	@echo "Environment variables:"
	@if [ -n "$$AZURE_OPENAI_ENDPOINT" ]; then \
		echo "  AZURE_OPENAI_ENDPOINT: [SET]"; \
	else \
		echo "  AZURE_OPENAI_ENDPOINT: [NOT SET] (required)"; \
	fi
	@if [ -n "$$AZURE_OPENAI_DEPLOYMENT" ]; then \
		echo "  AZURE_OPENAI_DEPLOYMENT: $$AZURE_OPENAI_DEPLOYMENT"; \
	else \
		echo "  AZURE_OPENAI_DEPLOYMENT: [NOT SET] (required)"; \
	fi
	@if [ -n "$$AZURE_OPENAI_API_KEY" ]; then \
		echo "  AZURE_OPENAI_API_KEY: [SET]"; \
	else \
		echo "  AZURE_OPENAI_API_KEY: [NOT SET] (required)"; \
	fi
	@if [ -n "$$AZURE_OPENAI_API_VERSION" ]; then \
		echo "  AZURE_OPENAI_API_VERSION: $$AZURE_OPENAI_API_VERSION"; \
	else \
		echo "  AZURE_OPENAI_API_VERSION: [NOT SET] (defaults to 2024-02-15-preview)"; \
	fi
	@echo ""
	@if [ -n "$$AZURE_OPENAI_ENDPOINT" ] && [ -n "$$AZURE_OPENAI_DEPLOYMENT" ] && [ -n "$$AZURE_OPENAI_API_KEY" ]; then \
		echo "Status: READY - All required variables set"; \
		echo "Run: make run-real-shadow"; \
	else \
		echo "Status: NOT READY - Missing required variables"; \
		echo "See: docs/REAL_KEYS_LOCAL_RUNBOOK_V1.md"; \
	fi

# Phase 19.3: Real keys smoke tests (does not call network)
demo-phase19-real-keys-smoke:
	@echo "Running Phase 19.3 Demo: Real Keys Smoke Tests..."
	go test -v ./internal/demo_phase19_real_keys_smoke/...

# =============================================================================
# iOS Targets (Phase 19)
# =============================================================================
# Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
#
# CRITICAL: Requires macOS with Xcode installed.
# These targets will fail on non-macOS systems.

# Open iOS project in Xcode
ios-open:
	@echo "Opening QuantumLife iOS project..."
	@if [ -d "ios/QuantumLife/QuantumLife.xcodeproj" ]; then \
		open ios/QuantumLife/QuantumLife.xcodeproj; \
	else \
		echo "ERROR: iOS project not found at ios/QuantumLife/"; \
		exit 1; \
	fi

# Build iOS project (requires Xcode)
ios-build:
	@echo "Building QuantumLife iOS..."
	@if command -v xcodebuild >/dev/null 2>&1; then \
		xcodebuild -project ios/QuantumLife/QuantumLife.xcodeproj \
			-scheme QuantumLife \
			-destination 'platform=iOS Simulator,name=iPhone 15' \
			-configuration Debug \
			build; \
	else \
		echo "ERROR: xcodebuild not found. Install Xcode on macOS."; \
		exit 1; \
	fi

# Run iOS tests (requires Xcode)
ios-test:
	@echo "Running QuantumLife iOS tests..."
	@if command -v xcodebuild >/dev/null 2>&1; then \
		xcodebuild -project ios/QuantumLife/QuantumLife.xcodeproj \
			-scheme QuantumLife \
			-destination 'platform=iOS Simulator,name=iPhone 15' \
			-configuration Debug \
			test; \
	else \
		echo "ERROR: xcodebuild not found. Install Xcode on macOS."; \
		exit 1; \
	fi

# Clean iOS build artifacts
ios-clean:
	@echo "Cleaning QuantumLife iOS..."
	@if command -v xcodebuild >/dev/null 2>&1; then \
		xcodebuild -project ios/QuantumLife/QuantumLife.xcodeproj \
			-scheme QuantumLife \
			clean; \
	else \
		echo "Skipping xcodebuild clean (not on macOS)"; \
	fi
	@rm -rf ios/QuantumLife/build
	@echo "Done."
