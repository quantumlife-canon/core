// Package demo provides demo-specific components for the suggest-only vertical slice.
package demo

import (
	"context"
	"fmt"
	"time"

	auditImpl "quantumlife/internal/audit/impl_inmem"
	"quantumlife/internal/circle"
	circleImpl "quantumlife/internal/circle/impl_inmem"
	intImpl "quantumlife/internal/intersection/impl_inmem"
	memoryImpl "quantumlife/internal/memory/impl_inmem"
	"quantumlife/internal/orchestrator"
	orchImpl "quantumlife/internal/orchestrator/impl_inmem"
	"quantumlife/pkg/primitives"
)

// DemoResult contains the output of a demo run.
type DemoResult struct {
	CircleID    string
	TraceID     string
	Suggestions []orchImpl.Suggestion
	AuditLog    []AuditEntry
	Success     bool
	Error       error
}

// AuditEntry is a simplified audit entry for demo output.
type AuditEntry struct {
	ID        string
	EventType string
	Action    string
	Outcome   string
	TraceID   string
	Timestamp time.Time
}

// Runner executes the demo scenario.
type Runner struct {
	circleRuntime *circleImpl.Runtime
	intRuntime    *intImpl.Runtime
	auditStore    *auditImpl.Store
	memoryStore   *memoryImpl.Store
	calendar      *MockCalendar
	orchestrator  *orchImpl.SuggestOnlyOrchestrator
}

// NewRunner creates a new demo runner with all components wired together.
func NewRunner() *Runner {
	// Create in-memory stores
	auditStore := auditImpl.NewStore()
	memoryStore := memoryImpl.NewStore()
	circleRuntime := circleImpl.NewRuntime()
	intRuntime := intImpl.NewRuntime()

	// Create mock calendar
	calendar := NewMockCalendar()

	// Create suggestion engine
	suggestionEngine := NewDeterministicSuggestionEngine(calendar)

	// Create orchestrator
	orch := orchImpl.NewSuggestOnlyOrchestrator(orchImpl.Config{
		AuditLogger:      auditStore,
		MemoryUpdater:    memoryStore,
		IntDiscoverer:    intRuntime,
		SuggestionEngine: suggestionEngine,
	})

	return &Runner{
		circleRuntime: circleRuntime,
		intRuntime:    intRuntime,
		auditStore:    auditStore,
		memoryStore:   memoryStore,
		calendar:      calendar,
		orchestrator:  orch,
	}
}

// Run executes the demo scenario.
func (r *Runner) Run(ctx context.Context) (*DemoResult, error) {
	result := &DemoResult{}

	// Step 1: Create a root circle
	cir, err := r.circleRuntime.Create(ctx, circle.CreateRequest{
		TenantID: "demo-tenant",
	})
	if err != nil {
		result.Error = fmt.Errorf("failed to create circle: %w", err)
		return result, result.Error
	}
	result.CircleID = cir.ID

	// Step 2: Create loop context
	traceID := primitives.LoopTraceID(fmt.Sprintf("trace-%d", time.Now().UnixNano()))
	result.TraceID = string(traceID)

	loopCtx := primitives.LoopContext{
		TraceID:        traceID,
		IssuerCircleID: cir.ID,
		CreatedAt:      time.Now(),
		RiskClass:      primitives.RiskLow,
		AutonomyMode:   "suggest_only",
		CurrentStep:    primitives.StepIntent,
		Metadata: map[string]string{
			"demo": "calendar-suggest",
		},
	}

	// Step 3: Create intent
	intent := orchestrator.Intent{
		ID:             fmt.Sprintf("intent-%d", time.Now().UnixNano()),
		IssuerCircleID: cir.ID,
		Type:           "calendar_analysis",
		Description:    "Analyze calendar and suggest activities for free time slots",
		Parameters: map[string]string{
			"scope":        "personal",
			"min_duration": "1h",
			"categories":   "family,leisure,personal",
		},
		CreatedAt: time.Now(),
	}

	// Step 4: Execute the suggest-only loop
	loopResult, err := r.orchestrator.ExecuteLoop(ctx, loopCtx, intent)
	if err != nil {
		result.Error = fmt.Errorf("loop execution failed: %w", err)
		return result, result.Error
	}

	// Extract suggestions from result
	result.Suggestions = orchImpl.GetSuggestions(loopResult)
	result.Success = loopResult.Success

	// Get audit log entries
	auditEntries := r.auditStore.GetAllEntries()
	for _, entry := range auditEntries {
		traceIDFromMeta := ""
		if entry.Metadata != nil {
			traceIDFromMeta = entry.Metadata["trace_id"]
		}
		result.AuditLog = append(result.AuditLog, AuditEntry{
			ID:        entry.ID,
			EventType: entry.EventType,
			Action:    entry.Action,
			Outcome:   entry.Outcome,
			TraceID:   traceIDFromMeta,
			Timestamp: entry.Timestamp,
		})
	}

	return result, nil
}

// PrintResult prints the demo result in a formatted way.
func PrintResult(result *DemoResult) {
	fmt.Println("============================================================")
	fmt.Println("  QuantumLife Demo: Calendar Suggest-Only Mode")
	fmt.Println("============================================================")
	fmt.Println()

	fmt.Printf("Circle ID: %s\n", result.CircleID)
	fmt.Printf("Trace ID:  %s\n", result.TraceID)
	fmt.Printf("Status:    %s\n", map[bool]string{true: "SUCCESS", false: "FAILED"}[result.Success])
	fmt.Println()

	fmt.Println("------------------------------------------------------------")
	fmt.Println("  SUGGESTIONS (read-only, no actions executed)")
	fmt.Println("------------------------------------------------------------")
	fmt.Println()

	for i, sug := range result.Suggestions {
		fmt.Printf("Suggestion %d:\n", i+1)
		fmt.Printf("  Time Slot:   %s\n", sug.TimeSlot)
		fmt.Printf("  Description: %s\n", sug.Description)
		fmt.Printf("  Why:         %s\n", sug.Explanation)
		fmt.Printf("  Category:    %s\n", sug.Category)
		fmt.Println()
	}

	fmt.Println("------------------------------------------------------------")
	fmt.Println("  AUDIT LOG (trace of all loop steps)")
	fmt.Println("------------------------------------------------------------")
	fmt.Println()

	for _, entry := range result.AuditLog {
		fmt.Printf("[%s] %s\n", entry.ID, entry.EventType)
		fmt.Printf("         Action: %s | Outcome: %s\n", entry.Action, entry.Outcome)
		fmt.Printf("         Trace:  %s\n", entry.TraceID)
		fmt.Println()
	}

	fmt.Println("============================================================")
	fmt.Println("  Demo completed. No external actions were executed.")
	fmt.Println("  All output is suggestions only.")
	fmt.Println("============================================================")
}

