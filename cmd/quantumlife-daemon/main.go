// Command quantumlife-daemon is the main QuantumLife service daemon.
//
// Usage:
//
//	quantumlife-daemon                              # Show status
//	quantumlife-daemon --demo-calendar-suggest      # Run calendar suggest demo (v1)
//	quantumlife-daemon --demo-family-invite-suggest # Run family invite demo (v2)
//	quantumlife-daemon --demo-family-negotiate-commit # Run negotiation demo (v3)
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
