// Command quantumlife-daemon is the main QuantumLife service daemon.
//
// Usage:
//
//	quantumlife-daemon                              # Show status
//	quantumlife-daemon --demo-calendar-suggest      # Run calendar suggest demo (v1)
//	quantumlife-daemon --demo-family-invite-suggest # Run family invite demo (v2)
//	quantumlife-daemon --demo-family-negotiate-commit # Run negotiation demo (v3)
//	quantumlife-daemon --demo-family-finance         # Run family finance demo (v8.6)
//	quantumlife-daemon --demo-v9-dryrun-execution   # Run v9 dry-run execution demo
//	quantumlife-daemon --demo-v9-guarded-execution # Run v9 guarded execution demo
//	quantumlife-daemon --demo-v9-execute-tiny-payment # Run v9.3 real payment demo
//	quantumlife-daemon --demo-v9-multiparty-tiny-payment # Run v9.4 multi-party demo
//	quantumlife-daemon --demo-v9-multiparty-execute-tiny-payment-real # Run v9.5 real multi-party demo
//	quantumlife-daemon --demo-v9-idempotency-replay-defense # Run v9.6 idempotency demo
//	quantumlife-daemon --demo-v9-caps-rate-limit           # Run v9.11 caps + rate limit demo
//
// Reference: docs/TECHNOLOGY_SELECTION_V1.md §13 Implementation Checklist
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"quantumlife/internal/demo"
	"quantumlife/internal/demo_family"
	"quantumlife/internal/demo_family_calendar"
	"quantumlife/internal/demo_family_negotiate"
	"quantumlife/internal/demo_family_simulate"
	"quantumlife/internal/demo_v911_caps"
	"quantumlife/internal/demo_v95_multiparty_real"
	"quantumlife/internal/demo_v96_idempotency"
	"quantumlife/internal/demo_v9_dryrun"
	"quantumlife/internal/demo_v9_execute"
	"quantumlife/internal/demo_v9_guarded"
	"quantumlife/internal/demo_v9_multiparty"
	"quantumlife/pkg/primitives"
)

const banner = `
╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║   QuantumLife Daemon v0.0.1                                   ║
║                                                               ║
║   Status: Demo Mode Available                                 ║
║                                                               ║
║   Canon: docs/QUANTUMLIFE_CANON_V1.md                         ║
║   Tech:  docs/TECHNOLOGY_SELECTION_V1.md                      ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
`

func main() {
	// Parse flags
	demoCalendarSuggest := flag.Bool("demo-calendar-suggest", false, "Run the calendar suggest demo (suggest-only mode, v1)")
	demoFamilyInviteSuggest := flag.Bool("demo-family-invite-suggest", false, "Run the family invite demo (suggest-only mode, v2)")
	demoFamilyNegotiateCommit := flag.Bool("demo-family-negotiate-commit", false, "Run the negotiation + commitment demo (suggest-only mode, v3)")
	demoFamilySimulateAction := flag.Bool("demo-family-simulate-action", false, "Run the simulate action demo (simulate mode, v4)")
	demoFamilyRealCalendarRead := flag.Bool("demo-family-real-calendar-read", false, "Run the real calendar read demo (simulate mode, v5)")
	demoFamilyFinance := flag.Bool("demo-family-finance", false, "Run the family finance demo (v8.6)")
	demoV9DryrunExecution := flag.Bool("demo-v9-dryrun-execution", false, "Run the v9 dry-run execution demo (no real money moves)")
	demoV9GuardedExecution := flag.Bool("demo-v9-guarded-execution", false, "Run the v9 guarded execution demo (adapter blocks)")
	demoV9ExecuteTinyPayment := flag.Bool("demo-v9-execute-tiny-payment", false, "Run the v9.3 real payment demo (REAL MONEY MAY MOVE)")
	demoV9MultipartyTinyPayment := flag.Bool("demo-v9-multiparty-tiny-payment", false, "Run the v9.4 multi-party payment demo (SIMULATED)")
	demoV95MultipartyReal := flag.Bool("demo-v9-multiparty-execute-tiny-payment-real", false, "Run the v9.5 real multi-party payment demo (SANDBOX ONLY)")
	demoV96Idempotency := flag.Bool("demo-v9-idempotency-replay-defense", false, "Run the v9.6 idempotency and replay defense demo")
	demoV911CapsRateLimit := flag.Bool("demo-v9-caps-rate-limit", false, "Run the v9.11 caps and rate limit demo")
	flag.Parse()

	fmt.Print(banner)

	if *demoCalendarSuggest {
		runDemoCalendarSuggest()
		return
	}

	if *demoFamilyInviteSuggest {
		runDemoFamilyInviteSuggest()
		return
	}

	if *demoFamilyNegotiateCommit {
		runDemoFamilyNegotiateCommit()
		return
	}

	if *demoFamilySimulateAction {
		runDemoFamilySimulateAction()
		return
	}

	if *demoFamilyRealCalendarRead {
		runDemoFamilyRealCalendarRead()
		return
	}

	if *demoFamilyFinance {
		runDemoFamilyFinance()
		return
	}

	if *demoV9DryrunExecution {
		runDemoV9DryrunExecution()
		return
	}

	if *demoV9GuardedExecution {
		runDemoV9GuardedExecution()
		return
	}

	if *demoV9ExecuteTinyPayment {
		runDemoV9ExecuteTinyPayment()
		return
	}

	if *demoV9MultipartyTinyPayment {
		runDemoV9MultipartyTinyPayment()
		return
	}

	if *demoV95MultipartyReal {
		runDemoV95MultipartyReal()
		return
	}

	if *demoV96Idempotency {
		runDemoV96Idempotency()
		return
	}

	if *demoV911CapsRateLimit {
		runDemoV911CapsRateLimit()
		return
	}

	// Default: show status
	fmt.Println("Runtime Layers:")
	fmt.Println("  - Circle Runtime         [in-memory impl available]")
	fmt.Println("  - Intersection Runtime   [in-memory impl available]")
	fmt.Println("  - Authority & Policy     [skeleton]")
	fmt.Println("  - Negotiation Engine     [in-memory impl available]")
	fmt.Println("  - Action Execution       [skeleton]")
	fmt.Println("  - Memory Layer           [in-memory impl available]")
	fmt.Println("  - Audit & Governance     [in-memory impl available]")
	fmt.Println("  - Orchestrator           [suggest-only impl available]")
	fmt.Println()
	fmt.Println("Available demos:")
	fmt.Println("  --demo-calendar-suggest          Single circle calendar suggestions (v1)")
	fmt.Println("  --demo-family-invite-suggest     Family intersection with invite tokens (v2)")
	fmt.Println("  --demo-family-negotiate-commit   Full negotiation loop + commitment (v3)")
	fmt.Println("  --demo-family-simulate-action    Simulated execution pipeline (v4)")
	fmt.Println("  --demo-family-real-calendar-read Real calendar read with OAuth (v5)")
	fmt.Println("  --demo-family-finance            Family financial intersections (v8.6)")
	fmt.Println("  --demo-v9-dryrun-execution       v9 dry-run financial execution (NO REAL MONEY)")
	fmt.Println("  --demo-v9-guarded-execution      v9 guarded execution with adapter (NO REAL MONEY)")
	fmt.Println("  --demo-v9-execute-tiny-payment   v9.3 real payment execution (REAL MONEY MAY MOVE)")
	fmt.Println("  --demo-v9-multiparty-tiny-payment v9.4 multi-party payment (SIMULATED)")
	fmt.Println("  --demo-v9-multiparty-execute-tiny-payment-real v9.5 real multi-party (SANDBOX)")
	fmt.Println("  --demo-v9-idempotency-replay-defense v9.6 idempotency + replay defense")
	fmt.Println("  --demo-v9-caps-rate-limit            v9.11 daily caps + rate limits")
	fmt.Println()
	fmt.Println("Run with --help for more options.")

	os.Exit(0)
}

// runDemoCalendarSuggest runs the calendar suggest demo.
func runDemoCalendarSuggest() {
	fmt.Println()
	fmt.Println("Running Calendar Suggest Demo...")
	fmt.Println("This demo uses SUGGEST-ONLY mode: no external actions are executed.")
	fmt.Println()

	runner := demo.NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		fmt.Printf("Demo failed: %v\n", err)
		os.Exit(1)
	}

	demo.PrintResult(result)
}

// runDemoFamilyInviteSuggest runs the family invite demo.
func runDemoFamilyInviteSuggest() {
	fmt.Println()
	fmt.Println("Running Family Intersection Demo (Vertical Slice v2)...")
	fmt.Println("This demo uses SUGGEST-ONLY mode: no external actions are executed.")
	fmt.Println()

	runner := demo_family.NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		fmt.Printf("Demo failed: %v\n", err)
		os.Exit(1)
	}

	demo_family.PrintResult(result)
}

// runDemoFamilyNegotiateCommit runs the family negotiation + commitment demo.
func runDemoFamilyNegotiateCommit() {
	fmt.Println()
	fmt.Println("Running Family Negotiation Demo (Vertical Slice v3)...")
	fmt.Println("This demo uses SUGGEST-ONLY mode: no external actions are executed.")
	fmt.Println("Demonstrates: Proposal -> Counterproposal -> Accept -> Finalize -> Commit")
	fmt.Println()

	runner := demo_family_negotiate.NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		fmt.Printf("Demo failed: %v\n", err)
		os.Exit(1)
	}

	demo_family_negotiate.PrintResult(result)
}

// runDemoFamilySimulateAction runs the family simulate action demo.
func runDemoFamilySimulateAction() {
	fmt.Println()
	fmt.Println("Running Family Simulate Action Demo (Vertical Slice v4)...")
	fmt.Println("This demo uses SIMULATE mode: deterministic execution, no external writes.")
	fmt.Println("Demonstrates: Commitment -> Action -> Auth -> Simulate -> Settle -> Memory")
	fmt.Println()

	runner := demo_family_simulate.NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		fmt.Printf("Demo failed: %v\n", err)
		os.Exit(1)
	}

	demo_family_simulate.PrintResult(result)
}

// runDemoFamilyRealCalendarRead runs the real calendar read demo.
func runDemoFamilyRealCalendarRead() {
	fmt.Println()
	fmt.Println("Running Family Real Calendar Read Demo (Vertical Slice v5)...")
	fmt.Println("This demo uses SIMULATE mode: reads from real calendars, NO external writes.")
	fmt.Println("Demonstrates: OAuth -> Token Mint -> Calendar Read -> Free Slots -> Audit")
	fmt.Println()
	fmt.Println("Required env vars for real providers:")
	fmt.Println("  Google:    GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET")
	fmt.Println("  Microsoft: MICROSOFT_CLIENT_ID, MICROSOFT_CLIENT_SECRET, MICROSOFT_TENANT_ID")
	fmt.Println()
	fmt.Println("Or use CLI auth flow: quantumlife-cli auth google --circle <id> --redirect <uri>")
	fmt.Println()

	// Try to create runner with persistence (for CLI auth flow support)
	runner, err := demo_family_calendar.NewRunnerWithPersistence(primitives.ModeSimulate)
	if err != nil {
		// Fall back to regular runner
		runner = demo_family_calendar.NewRunner()
	}

	result, err := runner.Run(context.Background())

	if err != nil {
		fmt.Printf("Demo failed: %v\n", err)
		os.Exit(1)
	}

	demo_family_calendar.PrintResult(result)
}

// runDemoFamilyFinance runs the family financial intersections demo.
func runDemoFamilyFinance() {
	fmt.Println()
	fmt.Println("Running Family Financial Intersections Demo (v8.6)...")
	fmt.Println("This demo uses READ + PROPOSE mode: no execution, no payments.")
	fmt.Println("Demonstrates: Shared Views -> Symmetry Proof -> Neutral Proposals")
	fmt.Println()

	printer := demo_family.NewPrinter()
	demo := demo_family.NewFamilyFinanceDemo(printer)

	if err := demo.Run(); err != nil {
		fmt.Printf("Demo failed: %v\n", err)
		os.Exit(1)
	}
}

// runDemoV9DryrunExecution runs the v9 dry-run execution demo.
// CRITICAL: This is DRY-RUN ONLY. NO REAL MONEY MOVES.
func runDemoV9DryrunExecution() {
	fmt.Println()
	fmt.Println("Running v9 Dry-Run Execution Demo...")
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  CRITICAL: DRY-RUN MODE                                       ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  NO REAL MONEY MOVES. NO PROVIDER WRITE CALLS.                ║")
	fmt.Println("║  Settlement outcome ALWAYS: blocked, revoked, expired, or     ║")
	fmt.Println("║  aborted - NEVER settled_successfully.                        ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  This slice proves execution safety without risk.             ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Demonstrates:")
	fmt.Println("  1. Intent creation with explicit amount, recipient, currency")
	fmt.Println("  2. Sealed ExecutionEnvelope with action hash")
	fmt.Println("  3. Approval request with neutral language")
	fmt.Println("  4. Approval verification bound to action hash")
	fmt.Println("  5. Revocation window with circle-initiated revocation")
	fmt.Println("  6. Affirmative validity check (6 conditions)")
	fmt.Println("  7. Execution blocked/aborted (NEVER succeeds)")
	fmt.Println("  8. Settlement recorded as non-success")
	fmt.Println("  9. Complete audit trail reconstruction")
	fmt.Println()

	runner := demo_v9_dryrun.NewRunner()
	result, err := runner.Run()

	if err != nil {
		fmt.Printf("Demo failed: %v\n", err)
		os.Exit(1)
	}

	demo_v9_dryrun.PrintResult(result)
}

// runDemoV9GuardedExecution runs the v9 guarded execution demo.
// CRITICAL: This uses GUARDED adapters. NO REAL MONEY MOVES.
func runDemoV9GuardedExecution() {
	fmt.Println()
	fmt.Println("Running v9 Guarded Execution Demo...")
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  CRITICAL: GUARDED ADAPTER MODE                               ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  NO REAL MONEY MOVES. Adapter ALWAYS blocks execution.        ║")
	fmt.Println("║  Settlement outcome ALWAYS: blocked, revoked, expired, or     ║")
	fmt.Println("║  aborted - NEVER settled_successfully.                        ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  This slice proves execution pipeline structure is correct    ║")
	fmt.Println("║  while guaranteeing safety.                                   ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Demonstrates:")
	fmt.Println("  1. Intent creation with explicit amount, recipient, currency")
	fmt.Println("  2. Sealed ExecutionEnvelope with action hash")
	fmt.Println("  3. Approval request with neutral language")
	fmt.Println("  4. Revocation window expires without revocation")
	fmt.Println("  5. Adapter.Prepare() validates envelope")
	fmt.Println("  6. Adapter.Execute() invoked - BLOCKED by guardrail")
	fmt.Println("  7. Settlement recorded as blocked (not succeeded)")
	fmt.Println("  8. Complete audit trail with adapter events")
	fmt.Println()

	runner := demo_v9_guarded.NewRunner()
	result, err := runner.Run()

	if err != nil {
		fmt.Printf("Demo failed: %v\n", err)
		os.Exit(1)
	}

	demo_v9_guarded.PrintResult(result)
}

// runDemoV9ExecuteTinyPayment runs the v9.3 real payment execution demo.
// CRITICAL: This demo may move REAL MONEY (in sandbox mode).
// It must be minimal, constrained, auditable, interruptible, and boring.
func runDemoV9ExecuteTinyPayment() {
	fmt.Println()
	fmt.Println("Running v9.3 Real Payment Execution Demo...")
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  v9.3: SINGLE-PARTY REAL FINANCIAL EXECUTION                  ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  This is the FIRST slice where money may actually move.       ║")
	fmt.Println("║  It must be minimal, constrained, auditable, interruptible,   ║")
	fmt.Println("║  and boring.                                                  ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  HARD SAFETY CONSTRAINTS:                                     ║")
	fmt.Println("║  1) Provider: TrueLayer ONLY                                  ║")
	fmt.Println("║  2) Cap: £1.00 (100 pence)                                    ║")
	fmt.Println("║  3) Pre-defined payees only (no free-text)                    ║")
	fmt.Println("║  4) Explicit per-action approval                              ║")
	fmt.Println("║  5) Forced pause before execution                             ║")
	fmt.Println("║  6) No retries - failures require new approval                ║")
	fmt.Println("║  7) Full audit trail                                          ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Demonstrates:")
	fmt.Println("  1. Intent creation with explicit amount, recipient, currency")
	fmt.Println("  2. Sealed ExecutionEnvelope with action hash")
	fmt.Println("  3. Approval request with neutral language")
	fmt.Println("  4. Explicit --approve flag requirement")
	fmt.Println("  5. Cap enforcement (£1.00 max)")
	fmt.Println("  6. Pre-defined payee validation")
	fmt.Println("  7. Forced pause before provider call")
	fmt.Println("  8. TrueLayer payment creation (sandbox)")
	fmt.Println("  9. Settlement recorded as succeeded (if provider confirms)")
	fmt.Println(" 10. Complete audit trail with receipt")
	fmt.Println()

	runner := demo_v9_execute.NewRunner()
	result, err := runner.Run()

	if err != nil {
		fmt.Printf("Demo failed: %v\n", err)
		os.Exit(1)
	}

	demo_v9_execute.PrintResult(result)
}

// runDemoV9MultipartyTinyPayment runs the v9.4 multi-party payment demo.
// CRITICAL: This demo is SIMULATED. NO REAL MONEY MOVES.
// It demonstrates multi-party approval requirements for shared money.
func runDemoV9MultipartyTinyPayment() {
	fmt.Println()
	fmt.Println("Running v9.4 Multi-Party Payment Demo...")
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  v9.4: MULTI-PARTY FINANCIAL EXECUTION                        ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  SIMULATED MODE - NO REAL MONEY MOVES                         ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  This slice extends v9.3 with multi-party approval for shared ║")
	fmt.Println("║  money in intersections. All v9.3 constraints remain in force.║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  MULTI-PARTY REQUIREMENTS:                                    ║")
	fmt.Println("║  1) Threshold approvals per contract ApprovalPolicy           ║")
	fmt.Println("║  2) Symmetry: all approvers see identical bundle              ║")
	fmt.Println("║  3) Neutrality: no coercive language in approval requests     ║")
	fmt.Println("║  4) Single-use: approvals cannot be reused                    ║")
	fmt.Println("║  5) Expiry: approvals verified at execution time              ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Demonstrates:")
	fmt.Println("  1. Family intersection with 2/2 approval policy")
	fmt.Println("  2. ApprovalBundle creation with deterministic hash")
	fmt.Println("  3. Symmetry proof (all approvers received identical content)")
	fmt.Println("  4. Neutral language verification")
	fmt.Println("  5. Threshold verification (2 of 2 required)")
	fmt.Println("  6. Multi-party gate blocks insufficient approvals")
	fmt.Println("  7. Mock connector executes with Simulated=true")
	fmt.Println("  8. Complete audit trail with v9.4 events")
	fmt.Println()

	runner := demo_v9_multiparty.NewRunner()
	results, err := runner.Run()

	if err != nil {
		fmt.Printf("Demo failed: %v\n", err)
		os.Exit(1)
	}

	for _, result := range results {
		demo_v9_multiparty.PrintResult(result)
	}
}

// runDemoV95MultipartyReal runs the v9.5 real multi-party payment demo.
// CRITICAL: This demo uses SANDBOX mode only. TrueLayer sandbox or mock fallback.
// It demonstrates strengthened presentation semantics and provider selection.
func runDemoV95MultipartyReal() {
	fmt.Println()
	fmt.Println("Running v9.5 Real Multi-Party Payment Demo...")
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  v9.5: REAL MULTI-PARTY SANDBOX EXECUTION                     ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  SANDBOX ONLY - TrueLayer sandbox or mock fallback            ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  This slice extends v9.4 with:                                ║")
	fmt.Println("║  1) Strengthened presentation: bundle MUST be presented to    ║")
	fmt.Println("║     each approver BEFORE their approval can be accepted       ║")
	fmt.Println("║  2) Provider selection: TrueLayer (sandbox) or mock           ║")
	fmt.Println("║  3) Revocation during forced pause: abort BEFORE provider     ║")
	fmt.Println("║  4) Attempt tracking: exactly one trace finalization          ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  All v9.3/v9.4 constraints remain in force:                   ║")
	fmt.Println("║  - £1.00 cap, predefined payees only, forced pause            ║")
	fmt.Println("║  - No retries, multi-party threshold, symmetry verification   ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Demonstrates:")
	fmt.Println("  1. PresentationGate verifies bundle was presented")
	fmt.Println("  2. Approval rejected if no presentation record exists")
	fmt.Println("  3. Provider selection (TrueLayer sandbox vs mock)")
	fmt.Println("  4. Revocation during forced pause aborts execution")
	fmt.Println("  5. Sandbox enforcement (live TrueLayer rejected)")
	fmt.Println("  6. Attempt tracking prevents audit duplication")
	fmt.Println("  7. Complete audit trail with v9.5 events")
	fmt.Println()

	runner := demo_v95_multiparty_real.NewRunner()
	results, err := runner.Run()

	if err != nil {
		fmt.Printf("Demo failed: %v\n", err)
		os.Exit(1)
	}

	for _, result := range results {
		demo_v95_multiparty_real.PrintResult(result)
	}
}

// runDemoV96Idempotency runs the v9.6 idempotency and replay defense demo.
// CRITICAL: This demo proves idempotency and replay defense mechanisms.
func runDemoV96Idempotency() {
	fmt.Println()
	fmt.Println("Running v9.6 Idempotency + Replay Defense Demo...")
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  v9.6: IDEMPOTENCY + REPLAY DEFENSE                           ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  PREVENTS DUPLICATE PAYMENTS AND REPLAYS VIA:                 ║")
	fmt.Println("║  1) Deterministic idempotency key (envelope + action + attempt)║")
	fmt.Println("║  2) Attempt ledger enforcing exactly-once semantics           ║")
	fmt.Println("║  3) One in-flight attempt per envelope policy                 ║")
	fmt.Println("║  4) Provider idempotency key propagation                      ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  All v9.3/v9.4/v9.5 constraints remain in force:              ║")
	fmt.Println("║  - £1.00 cap, predefined payees only, forced pause            ║")
	fmt.Println("║  - No retries, multi-party threshold, symmetry verification   ║")
	fmt.Println("║  - Bundle MUST be presented before approval accepted          ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Demonstrates:")
	fmt.Println("  1. Double-invoke same attempt ID -> blocked (replay)")
	fmt.Println("  2. Two attempts while first in-flight -> blocked (inflight)")
	fmt.Println("  3. After terminal settle -> replay blocked")
	fmt.Println("  4. Mock respects idempotency + MoneyMoved=false")
	fmt.Println()

	runner := demo_v96_idempotency.NewRunner()
	results, err := runner.Run()

	if err != nil {
		fmt.Printf("Demo failed: %v\n", err)
		os.Exit(1)
	}

	for _, result := range results {
		demo_v96_idempotency.PrintResult(result)
	}
}

// runDemoV911CapsRateLimit runs the v9.11 caps and rate limit demo.
// CRITICAL: This demo proves daily caps and rate limiting mechanisms.
func runDemoV911CapsRateLimit() {
	fmt.Println()
	fmt.Println("Running v9.11 Caps + Rate Limit Demo...")
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  v9.11: DAILY CAPS + RATE-LIMITED EXECUTION LEDGER            ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  PREVENTS SLOW DRAIN AND BURST EXECUTION VIA:                 ║")
	fmt.Println("║  1) Per-circle daily caps (by currency)                       ║")
	fmt.Println("║  2) Per-intersection daily caps (by currency)                 ║")
	fmt.Println("║  3) Per-payee daily caps (by currency)                        ║")
	fmt.Println("║  4) Rate limits: max attempts per day                         ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  KEY DESIGN:                                                  ║")
	fmt.Println("║  - Caps are HARD BLOCKS (no auto-split, no reduce amount)     ║")
	fmt.Println("║  - Attempts count regardless of outcome (prevents bypass)     ║")
	fmt.Println("║  - Spend only counts when money actually moves                ║")
	fmt.Println("║  - Simulated payments count attempts but NOT spend            ║")
	fmt.Println("║                                                               ║")
	fmt.Println("║  All v9.3-v9.10 constraints remain in force.                  ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Demonstrates:")
	fmt.Println("  S1: Attempt limit blocks after N attempts")
	fmt.Println("  S2: Circle daily cap blocks when spend exceeds cap")
	fmt.Println("  S3: Simulated payments don't count toward spend")
	fmt.Println("  S4: Intersection attempt limit with scope isolation")
	fmt.Println()

	runner := demo_v911_caps.NewRunner()
	results, err := runner.Run()

	if err != nil {
		fmt.Printf("Demo failed: %v\n", err)
		os.Exit(1)
	}

	for _, result := range results {
		demo_v911_caps.PrintResult(result)
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("v9.11 Demo Complete")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
