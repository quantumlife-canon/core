// Package demo_family_calendar provides demo output formatting.
package demo_family_calendar

import (
	"fmt"
	"strings"
	"time"
)

// londonLocation is the Europe/London timezone for friendly display.
var londonLocation *time.Location

func init() {
	var err error
	londonLocation, err = time.LoadLocation("Europe/London")
	if err != nil {
		londonLocation = time.UTC
	}
}

// PrintResult prints the demo result in a formatted way.
func PrintResult(result *Result) {
	fmt.Println()
	printDivider("VERTICAL SLICE v5: Real Calendar Read Demo")
	fmt.Println()

	// Print mode and status
	printSection("RUN MODE")
	fmt.Printf("  Mode: %s\n", result.Mode)
	if result.UsingMock {
		fmt.Println("  ╔════════════════════════════════════════════════════════╗")
		fmt.Println("  ║  RUNNING WITH MOCK PROVIDER                            ║")
		fmt.Println("  ║  (No Google/Microsoft credentials configured)          ║")
		fmt.Println("  ╚════════════════════════════════════════════════════════╝")
	}
	fmt.Println()

	// Print providers
	printSection("PROVIDERS USED")
	for _, p := range result.ProvidersUsed {
		icon := "✓"
		if p == "mock" {
			icon = "◉"
		}
		fmt.Printf("  %s %s\n", icon, p)
	}
	fmt.Println()

	// Print intersection info
	printSection("INTERSECTION")
	fmt.Printf("  Intersection ID:  %s\n", result.IntersectionID)
	fmt.Printf("  Contract Version: %s\n", result.ContractVersion)
	fmt.Printf("  Scopes Used:      calendar:read (READ-ONLY)\n")
	fmt.Println()

	// Print authorization
	printSection("AUTHORIZATION PROOF")
	fmt.Printf("  Proof ID: %s\n", result.AuthorizationProofID)
	fmt.Printf("  Trace ID: %s\n", result.TraceID)
	fmt.Println()

	// Print time range
	printSection("TIME RANGE QUERIED")
	fmt.Printf("  From: %s\n", formatTime(result.TimeRange.Start))
	fmt.Printf("  To:   %s\n", formatTime(result.TimeRange.End))
	fmt.Println()

	// Print events summary
	printSection("EVENTS FETCHED")
	fmt.Printf("  Total Events: %d\n", result.EventsFetched)
	for provider, count := range result.EventsByProvider {
		fmt.Printf("  - %s: %d events\n", provider, count)
	}
	fmt.Println()

	// Print events list
	if len(result.Events) > 0 {
		printSection("CALENDAR EVENTS")
		maxEvents := 10
		if len(result.Events) < maxEvents {
			maxEvents = len(result.Events)
		}
		for i := 0; i < maxEvents; i++ {
			evt := result.Events[i]
			fmt.Printf("  [%d] %s\n", i+1, evt.Title)
			fmt.Printf("      Time: %s - %s\n",
				formatTimeShort(evt.StartTime),
				formatTimeShort(evt.EndTime))
			if evt.Location != "" {
				fmt.Printf("      Location: %s\n", evt.Location)
			}
		}
		if len(result.Events) > maxEvents {
			fmt.Printf("  ... and %d more events\n", len(result.Events)-maxEvents)
		}
		fmt.Println()
	}

	// Print free slots
	printSection("FREE SLOTS FOUND")
	fmt.Printf("  Total Free Slots: %d\n", result.FreeSlotsFound)
	if len(result.FreeSlots) > 0 {
		fmt.Println("  Top 3 Suggestions:")
		for i, slot := range result.FreeSlots {
			fmt.Printf("  [%d] %s - %s (%.0f min)\n",
				i+1,
				formatTimeShort(slot.Start),
				formatTimeShort(slot.End),
				slot.Duration.Minutes())
		}
	}
	fmt.Println()

	// Print audit trace summary
	printSection("AUDIT TRACE SUMMARY")
	fmt.Printf("  Total Audit Entries: %d\n", len(result.AuditEntries))
	eventTypes := make(map[string]int)
	for _, entry := range result.AuditEntries {
		eventTypes[entry.EventType]++
	}
	fmt.Println("  Event Types:")
	for eventType, count := range eventTypes {
		fmt.Printf("    - %s: %d\n", eventType, count)
	}
	fmt.Println()

	// Print detailed audit entries
	if len(result.AuditEntries) > 0 {
		printSection("AUDIT ENTRIES (Chronological)")
		for i, entry := range result.AuditEntries {
			fmt.Printf("  [%d] %s\n", i+1, entry.EventType)
			fmt.Printf("      Action: %s, Outcome: %s\n", entry.Action, entry.Outcome)
			if entry.AuthorizationProofID != "" {
				fmt.Printf("      Auth Proof: %s\n", truncate(entry.AuthorizationProofID, 30))
			}
		}
		fmt.Println()
	}

	// Print final status
	printDivider("DEMO COMPLETE")
	if result.Success {
		fmt.Println("  Status: SUCCESS")
		fmt.Println("  - All reads completed successfully")
		fmt.Println("  - NO external writes performed")
		fmt.Println("  - Audit trail recorded")
	} else {
		fmt.Printf("  Status: FAILED\n")
		fmt.Printf("  Error: %s\n", result.Error)
	}
	fmt.Println()
}

// printDivider prints a section divider.
func printDivider(title string) {
	line := strings.Repeat("═", 60)
	fmt.Printf("╔%s╗\n", line)
	padding := (60 - len(title)) / 2
	fmt.Printf("║%s%s%s║\n",
		strings.Repeat(" ", padding),
		title,
		strings.Repeat(" ", 60-len(title)-padding))
	fmt.Printf("╚%s╝\n", line)
}

// printSection prints a section header.
func printSection(title string) {
	fmt.Printf("── %s ──\n", title)
}

// formatTime formats a time for display in London timezone.
func formatTime(t time.Time) string {
	return t.In(londonLocation).Format("Mon 02 Jan 2006 15:04 MST")
}

// formatTimeShort formats a time shortly for display.
func formatTimeShort(t time.Time) string {
	return t.In(londonLocation).Format("15:04")
}

// truncate truncates a string to max length.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
