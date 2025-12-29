// Package impl_inmem provides an in-memory implementation of the orchestrator.
// This is for demo and testing purposes only.
//
// This orchestrator runs the Irreducible Loop with in-memory components.
// It is designed for the "suggest-only" demo mode.
package impl_inmem

import (
	"context"
	"fmt"
	"time"

	"quantumlife/internal/audit"
	"quantumlife/internal/intersection"
	"quantumlife/internal/memory"
	"quantumlife/internal/orchestrator"
	"quantumlife/pkg/primitives"
)

// SuggestOnlyOrchestrator implements the LoopOrchestrator for suggest-only mode.
// This orchestrator NEVER executes real actions. It produces suggestions only.
//
// CRITICAL: This implementation skips the Action step and goes directly to
// Settlement with a "suggested" outcome. No external systems are touched.
type SuggestOnlyOrchestrator struct {
	auditLogger    audit.LoopEventEmitter
	memoryUpdater  memory.LoopMemoryUpdater
	intDiscoverer  intersection.LoopDiscoverer

	// suggestionEngine produces deterministic suggestions
	suggestionEngine SuggestionEngine

	// loopStatus tracks active loops
	loopStatus map[primitives.LoopTraceID]*orchestrator.LoopStatus
}

// SuggestionEngine produces suggestions deterministically.
// This is a control-plane component that may use LLM/SLM in production,
// but uses deterministic logic for the demo.
type SuggestionEngine interface {
	// GenerateSuggestions produces suggestions from the given context.
	GenerateSuggestions(ctx context.Context, loopCtx primitives.LoopContext, data interface{}) ([]Suggestion, error)
}

// Suggestion represents a suggested action (NOT executed).
type Suggestion struct {
	ID          string
	Description string
	Explanation string // Why this suggestion was made
	Category    string
	Priority    int
	TimeSlot    string
}

// Config contains configuration for the orchestrator.
type Config struct {
	AuditLogger      audit.LoopEventEmitter
	MemoryUpdater    memory.LoopMemoryUpdater
	IntDiscoverer    intersection.LoopDiscoverer
	SuggestionEngine SuggestionEngine
}

// NewSuggestOnlyOrchestrator creates a new suggest-only orchestrator.
func NewSuggestOnlyOrchestrator(cfg Config) *SuggestOnlyOrchestrator {
	return &SuggestOnlyOrchestrator{
		auditLogger:      cfg.AuditLogger,
		memoryUpdater:    cfg.MemoryUpdater,
		intDiscoverer:    cfg.IntDiscoverer,
		suggestionEngine: cfg.SuggestionEngine,
		loopStatus:       make(map[primitives.LoopTraceID]*orchestrator.LoopStatus),
	}
}

// ExecuteLoop runs a suggest-only loop from intent to memory update.
// This implementation produces suggestions but does NOT execute any actions.
func (o *SuggestOnlyOrchestrator) ExecuteLoop(ctx context.Context, loopCtx primitives.LoopContext, intent orchestrator.Intent) (*orchestrator.LoopResult, error) {
	startedAt := time.Now()

	result := &orchestrator.LoopResult{
		TraceID:   loopCtx.TraceID,
		StartedAt: startedAt,
	}

	// Initialize loop status
	o.loopStatus[loopCtx.TraceID] = &orchestrator.LoopStatus{
		TraceID:      loopCtx.TraceID,
		CurrentStep:  primitives.StepIntent,
		State:        orchestrator.LoopStateActive,
		StepStatuses: make(map[primitives.LoopStep]orchestrator.StepStatus),
		LastUpdated:  time.Now(),
	}

	// Step 1: Intent
	loopCtx = loopCtx.WithStep(primitives.StepIntent)
	if err := o.auditLogger.EmitStepStarted(ctx, loopCtx, "intent"); err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepIntent, err)
	}

	intentResult := &orchestrator.IntentResult{
		IntentID:       intent.ID,
		Classification: intent.Type,
		SuggestedAction: "suggest_activities",
		RequiredScopes: []string{"calendar:read"},
		ProcessedAt:    time.Now(),
	}
	result.IntentResult = intentResult

	if err := o.auditLogger.EmitStepCompleted(ctx, loopCtx, "intent", "classified as "+intent.Type); err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepIntent, err)
	}

	// Step 2: Intersection Discovery
	loopCtx = loopCtx.WithStep(primitives.StepIntersectionDiscovery)
	if err := o.auditLogger.EmitStepStarted(ctx, loopCtx, "intersection_discovery"); err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepIntersectionDiscovery, err)
	}

	discoveryResult, err := o.intDiscoverer.DiscoverForLoop(ctx, loopCtx, intersection.DiscoveryCriteria{
		IssuerCircleID: loopCtx.IssuerCircleID,
		RequiredScopes: intentResult.RequiredScopes,
		PreferExisting: true,
	})
	if err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepIntersectionDiscovery, err)
	}

	result.DiscoveryResult = &orchestrator.DiscoveryResult{
		IntersectionID:  discoveryResult.IntersectionID,
		IsNew:           discoveryResult.IsNew,
		ContractVersion: discoveryResult.ContractVersion,
		AvailableScopes: discoveryResult.AvailableScopes,
		DiscoveredAt:    time.Now(),
	}
	loopCtx = loopCtx.WithIntersection(discoveryResult.IntersectionID)

	if err := o.auditLogger.EmitStepCompleted(ctx, loopCtx, "intersection_discovery", "found "+discoveryResult.IntersectionID); err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepIntersectionDiscovery, err)
	}

	// Step 3: Authority Negotiation (auto-granted for demo with read-only scope)
	loopCtx = loopCtx.WithStep(primitives.StepAuthorityNegotiation)
	if err := o.auditLogger.EmitStepStarted(ctx, loopCtx, "authority_negotiation"); err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepAuthorityNegotiation, err)
	}

	// For demo: auto-grant read-only authority
	result.AuthorityResult = &orchestrator.AuthorityResult{
		Granted:       true,
		GrantID:       fmt.Sprintf("grant-%s", loopCtx.TraceID),
		GrantedScopes: []string{"calendar:read"},
		Conditions:    []string{"read_only", "suggest_only"},
		NegotiatedAt:  time.Now(),
	}

	if err := o.auditLogger.EmitStepCompleted(ctx, loopCtx, "authority_negotiation", "granted read-only"); err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepAuthorityNegotiation, err)
	}

	// Step 4: Commitment (commit to suggestion generation only)
	loopCtx = loopCtx.WithStep(primitives.StepCommitment)
	if err := o.auditLogger.EmitStepStarted(ctx, loopCtx, "commitment"); err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepCommitment, err)
	}

	result.Commitment = &orchestrator.Commitment{
		ID:               fmt.Sprintf("commit-%s", loopCtx.TraceID),
		ActionType:       "suggest_only",
		ActionParameters: intent.Parameters,
		AuthorityGrantID: result.AuthorityResult.GrantID,
		IdempotencyKey:   string(loopCtx.TraceID),
		CommittedAt:      time.Now(),
	}

	if err := o.auditLogger.EmitStepCompleted(ctx, loopCtx, "commitment", "committed to suggest_only"); err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepCommitment, err)
	}

	// Step 5: Action (SUGGEST ONLY - no real execution)
	loopCtx = loopCtx.WithStep(primitives.StepAction)
	if err := o.auditLogger.EmitStepStarted(ctx, loopCtx, "action"); err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepAction, err)
	}

	// Generate suggestions (NOT real actions)
	suggestions, err := o.suggestionEngine.GenerateSuggestions(ctx, loopCtx, intent.Parameters)
	if err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepAction, err)
	}

	// Package suggestions as action result
	resultData := make(map[string]string)
	for i, s := range suggestions {
		resultData[fmt.Sprintf("suggestion_%d_desc", i)] = s.Description
		resultData[fmt.Sprintf("suggestion_%d_why", i)] = s.Explanation
		resultData[fmt.Sprintf("suggestion_%d_slot", i)] = s.TimeSlot
	}
	resultData["suggestion_count"] = fmt.Sprintf("%d", len(suggestions))
	resultData["mode"] = "suggest_only"

	now := time.Now()
	result.ActionResult = &orchestrator.ActionResult{
		ActionID:     fmt.Sprintf("action-%s", loopCtx.TraceID),
		CommitmentID: result.Commitment.ID,
		Success:      true,
		ResultCode:   "SUGGESTIONS_GENERATED",
		ResultData:   resultData,
		StartedAt:    now,
		CompletedAt:  now,
	}

	if err := o.auditLogger.EmitStepCompleted(ctx, loopCtx, "action", fmt.Sprintf("generated %d suggestions", len(suggestions))); err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepAction, err)
	}

	// Step 6: Settlement
	loopCtx = loopCtx.WithStep(primitives.StepSettlement)
	if err := o.auditLogger.EmitStepStarted(ctx, loopCtx, "settlement"); err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepSettlement, err)
	}

	result.SettlementResult = &orchestrator.SettlementResult{
		SettlementID: fmt.Sprintf("settle-%s", loopCtx.TraceID),
		ActionID:     result.ActionResult.ActionID,
		Status:       orchestrator.SettlementComplete,
		SettledAt:    time.Now(),
	}

	if err := o.auditLogger.EmitStepCompleted(ctx, loopCtx, "settlement", "settled as suggest_only"); err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepSettlement, err)
	}

	// Step 7: Memory Update
	loopCtx = loopCtx.WithStep(primitives.StepMemoryUpdate)
	if err := o.auditLogger.EmitStepStarted(ctx, loopCtx, "memory_update"); err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepMemoryUpdate, err)
	}

	memResult, err := o.memoryUpdater.RecordLoopOutcome(ctx, loopCtx, memory.LoopOutcome{
		TraceID:      string(loopCtx.TraceID),
		Success:      true,
		FinalStep:    string(primitives.StepMemoryUpdate),
		IntentID:     intent.ID,
		ActionID:     result.ActionResult.ActionID,
		SettlementID: result.SettlementResult.SettlementID,
		Metadata:     resultData,
	})
	if err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepMemoryUpdate, err)
	}

	result.MemoryResult = &orchestrator.MemoryResult{
		RecordID: memResult.RecordID,
		TraceID:  loopCtx.TraceID,
		StoredAt: time.Now(),
	}

	if err := o.auditLogger.EmitStepCompleted(ctx, loopCtx, "memory_update", "recorded outcome"); err != nil {
		return o.failLoop(ctx, loopCtx, result, primitives.StepMemoryUpdate, err)
	}

	// Complete the loop
	result.FinalStep = primitives.StepMemoryUpdate
	result.Success = true
	result.CompletedAt = time.Now()

	if err := o.auditLogger.EmitLoopCompleted(ctx, loopCtx, true, "suggest-only loop completed"); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to emit loop completed event: %v\n", err)
	}

	// Update loop status
	o.loopStatus[loopCtx.TraceID].State = orchestrator.LoopStateCompleted
	o.loopStatus[loopCtx.TraceID].CurrentStep = primitives.StepMemoryUpdate

	return result, nil
}

// ResumeLoop resumes a paused or failed loop (not implemented for demo).
func (o *SuggestOnlyOrchestrator) ResumeLoop(ctx context.Context, loopCtx primitives.LoopContext, fromStep primitives.LoopStep) (*orchestrator.LoopResult, error) {
	return nil, fmt.Errorf("resume not implemented for demo")
}

// AbortLoop aborts an in-progress loop.
func (o *SuggestOnlyOrchestrator) AbortLoop(ctx context.Context, traceID primitives.LoopTraceID, reason string) error {
	status, ok := o.loopStatus[traceID]
	if !ok {
		return fmt.Errorf("loop not found: %s", traceID)
	}

	status.State = orchestrator.LoopStateAborted

	loopCtx := primitives.LoopContext{
		TraceID:     traceID,
		CurrentStep: status.CurrentStep,
	}

	return o.auditLogger.EmitLoopAborted(ctx, loopCtx, reason)
}

// GetLoopStatus returns the current status of a loop.
func (o *SuggestOnlyOrchestrator) GetLoopStatus(ctx context.Context, traceID primitives.LoopTraceID) (*orchestrator.LoopStatus, error) {
	if status, ok := o.loopStatus[traceID]; ok {
		return status, nil
	}
	return nil, fmt.Errorf("loop not found: %s", traceID)
}

// GetSuggestions retrieves the suggestions from a completed loop result.
func GetSuggestions(result *orchestrator.LoopResult) []Suggestion {
	if result == nil || result.ActionResult == nil {
		return nil
	}

	var suggestions []Suggestion
	data := result.ActionResult.ResultData

	count := 0
	if countStr, ok := data["suggestion_count"]; ok {
		fmt.Sscanf(countStr, "%d", &count)
	}

	for i := 0; i < count; i++ {
		suggestions = append(suggestions, Suggestion{
			ID:          fmt.Sprintf("sug-%d", i),
			Description: data[fmt.Sprintf("suggestion_%d_desc", i)],
			Explanation: data[fmt.Sprintf("suggestion_%d_why", i)],
			TimeSlot:    data[fmt.Sprintf("suggestion_%d_slot", i)],
		})
	}

	return suggestions
}

// failLoop handles loop failure at any step.
func (o *SuggestOnlyOrchestrator) failLoop(ctx context.Context, loopCtx primitives.LoopContext, result *orchestrator.LoopResult, step primitives.LoopStep, err error) (*orchestrator.LoopResult, error) {
	result.FinalStep = step
	result.Success = false
	result.FailureStep = &step
	result.FailureReason = err.Error()
	result.CompletedAt = time.Now()

	o.auditLogger.EmitStepFailed(ctx, loopCtx, string(step), err.Error())
	o.auditLogger.EmitLoopCompleted(ctx, loopCtx, false, err.Error())

	if status, ok := o.loopStatus[loopCtx.TraceID]; ok {
		status.State = orchestrator.LoopStateFailed
	}

	return result, err
}

// Verify interface compliance at compile time.
var _ orchestrator.LoopOrchestrator = (*SuggestOnlyOrchestrator)(nil)
