// Command quantumlife-daemon is the main QuantumLife service daemon.
//
// Usage:
//   quantumlife-daemon                    # Show status
//   quantumlife-daemon --demo-calendar-suggest  # Run calendar suggest demo
//
// Reference: docs/TECHNOLOGY_SELECTION_V1.md §13 Implementation Checklist
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"quantumlife/internal/demo"
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
	demoCalendarSuggest := flag.Bool("demo-calendar-suggest", false, "Run the calendar suggest demo (suggest-only mode)")
	flag.Parse()

	fmt.Print(banner)

	if *demoCalendarSuggest {
		runDemoCalendarSuggest()
		return
	}

	// Default: show status
	fmt.Println("Runtime Layers:")
	fmt.Println("  - Circle Runtime         [in-memory impl available]")
	fmt.Println("  - Intersection Runtime   [in-memory impl available]")
	fmt.Println("  - Authority & Policy     [skeleton]")
	fmt.Println("  - Negotiation Engine     [skeleton]")
	fmt.Println("  - Action Execution       [skeleton]")
	fmt.Println("  - Memory Layer           [in-memory impl available]")
	fmt.Println("  - Audit & Governance     [in-memory impl available]")
	fmt.Println("  - Orchestrator           [suggest-only impl available]")
	fmt.Println()
	fmt.Println("Available demos:")
	fmt.Println("  --demo-calendar-suggest  Run calendar suggestion demo")
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
