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
	"quantumlife/internal/demo_v9_dryrun"
	"quantumlife/internal/demo_v9_guarded"
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
