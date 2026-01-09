// Package enforcementaudit provides Phase 44.2: Enforcement Wiring Audit.
//
// This engine audits and proves that HOLD-only constraints actually bind
// the runtime. It inspects the wiring manifest and runs behavioral probes.
//
// CRITICAL INVARIANTS:
//   - Wiring Audit: Verifies all enforcement wrappers are connected.
//   - Behavioral Audit: Verifies clamping actually works via probes.
//   - Hash-only storage. No raw identifiers.
//   - Deterministic: same inputs + clock => same hashes and outcomes.
//   - NO time.Now() - clock injection required.
//   - NO goroutines.
//
// Reference: docs/ADR/ADR-0082-phase44-2-enforcement-wiring-audit.md
package enforcementaudit

import (
	"time"

	"quantumlife/internal/enforcementclamp"
	ea "quantumlife/pkg/domain/enforcementaudit"
)

// ============================================================================
// Interfaces
// ============================================================================

// AuditStore stores audit runs.
type AuditStore interface {
	AppendRun(run ea.AuditRun) error
	GetLatestRun() *ea.AuditRun
	ListRuns() []ea.AuditRun
}

// AckStore stores audit acknowledgments.
type AckStore interface {
	AppendAck(ack ea.AuditAck) error
	IsAcked(runHash string) bool
}

// Clock provides time injection.
type Clock interface {
	Now() time.Time
}

// ============================================================================
// Probe Inputs
// ============================================================================

// ProbeInputs contains inputs for behavioral probes.
// These are abstract enums only, no raw data.
type ProbeInputs struct {
	HasDelegatedHolding       bool
	HasTrustTransfer          bool
	SimulatedCandidateKinds   []string // Abstract kinds: "surface", "interrupt_candidate"
	SimulatedPressureOutcomes []string // Abstract outcomes: "hold", "queue_proof"
}

// DefaultProbeInputs returns default probe inputs for testing.
func DefaultProbeInputs() ProbeInputs {
	return ProbeInputs{
		HasDelegatedHolding:       true,
		HasTrustTransfer:          true,
		SimulatedCandidateKinds:   []string{"surface", "interrupt_candidate", "deliver", "execute"},
		SimulatedPressureOutcomes: []string{"hold", "queue_proof", "no_effect"},
	}
}

// ============================================================================
// Engine
// ============================================================================

// Engine manages enforcement wiring audits.
type Engine struct {
	auditStore AuditStore
	ackStore   AckStore
	clk        Clock
	clamp      *enforcementclamp.Engine
}

// NewEngine creates a new enforcement audit engine.
func NewEngine(auditStore AuditStore, ackStore AckStore, clk Clock) *Engine {
	return &Engine{
		auditStore: auditStore,
		ackStore:   ackStore,
		clk:        clk,
		clamp:      enforcementclamp.NewEngine(),
	}
}

// ============================================================================
// Time Helpers
// ============================================================================

// GetCurrentPeriodKey returns the current hour bucket from the clock.
// Format: "YYYY-MM-DD-HH"
func (e *Engine) GetCurrentPeriodKey() string {
	return e.clk.Now().UTC().Format("2006-01-02-15")
}

// ============================================================================
// Audit Execution
// ============================================================================

// RunAudit executes a complete audit run.
func (e *Engine) RunAudit(manifest EnforcementManifest, probes ProbeInputs) *ea.AuditRun {
	periodKey := e.GetCurrentPeriodKey()

	// Collect all checks
	checks := make([]ea.AuditCheck, 0, ea.MaxChecksPerRun)

	// 1. Wiring audit checks
	wiringChecks := e.runWiringAudit(manifest)
	checks = append(checks, wiringChecks...)

	// 2. Behavioral probe checks
	behaviorChecks := e.runBehavioralProbes(probes)
	checks = append(checks, behaviorChecks...)

	// Limit to max checks
	if len(checks) > ea.MaxChecksPerRun {
		checks = checks[:ea.MaxChecksPerRun]
	}

	// Determine overall status
	status := ea.StatusPass
	for _, check := range checks {
		if check.Status == ea.StatusFail && check.Severity == ea.SeverityCritical {
			status = ea.StatusFail
			break
		}
	}

	// Build run
	run := &ea.AuditRun{
		PeriodKey: periodKey,
		Status:    status,
		Checks:    checks,
	}

	// Compute critical fails bucket
	criticalCount := run.CountCriticalFails()
	run.CriticalFailsBucket = ea.ComputeCriticalFailsBucket(criticalCount)

	// Compute run hash
	run.RunHash = run.ComputeHash()

	// Persist
	if e.auditStore != nil {
		_ = e.auditStore.AppendRun(*run)
	}

	return run
}

// ============================================================================
// Wiring Audit
// ============================================================================

// runWiringAudit checks the manifest for missing components.
func (e *Engine) runWiringAudit(manifest EnforcementManifest) []ea.AuditCheck {
	checks := make([]ea.AuditCheck, 0)

	// Check each component
	components := []struct {
		applied   bool
		component string
		target    ea.AuditTargetKind
	}{
		{manifest.PressureGateApplied, "pressure_gate", ea.TargetPressurePipeline},
		{manifest.DelegatedHoldingApplied, "delegated_holding", ea.TargetPressurePipeline},
		{manifest.TrustTransferApplied, "trust_transfer", ea.TargetPressurePipeline},
		{manifest.InterruptPreviewApplied, "interrupt_preview", ea.TargetInterruptPipeline},
		{manifest.DeliveryOrchestratorUsesClamp, "delivery_orchestrator", ea.TargetDeliveryPipeline},
		{manifest.TimeWindowAdapterApplied, "timewindow_adapter", ea.TargetTimeWindowPipeline},
		{manifest.CommerceExcluded, "commerce_filter", ea.TargetPressurePipeline},
		{manifest.ClampWrapperRegistered, "clamp_wrapper", ea.TargetPressurePipeline},
	}

	for _, c := range components {
		check := e.buildWiringCheck(c.applied, c.component, c.target)
		checks = append(checks, check)
	}

	return checks
}

// buildWiringCheck builds a single wiring check.
func (e *Engine) buildWiringCheck(applied bool, component string, target ea.AuditTargetKind) ea.AuditCheck {
	status := ea.StatusPass
	checkKind := ea.CheckContractApplied
	severity := ea.SeverityInfo

	if !applied {
		status = ea.StatusFail
		checkKind = ea.CheckContractNotApplied
		severity = ea.SeverityCritical
	}

	evidence := component + "|" + string(target) + "|" + status.CanonicalString()

	return ea.AuditCheck{
		Target:       target,
		Check:        checkKind,
		Status:       status,
		Severity:     severity,
		Component:    component,
		EvidenceHash: ea.ComputeEvidenceHash(evidence),
	}
}

// ============================================================================
// Behavioral Probes
// ============================================================================

// runBehavioralProbes runs deterministic probes to verify clamping.
func (e *Engine) runBehavioralProbes(probes ProbeInputs) []ea.AuditCheck {
	checks := make([]ea.AuditCheck, 0)

	// Only run probes if contracts are active
	if !probes.HasDelegatedHolding && !probes.HasTrustTransfer {
		return checks
	}

	// Probe: SURFACE must be clamped to HOLD
	for _, kind := range probes.SimulatedCandidateKinds {
		if enforcementclamp.IsForbiddenDecision(kind) {
			check := e.probeClampingWorks(kind, probes)
			checks = append(checks, check)
		}
	}

	return checks
}

// probeClampingWorks verifies that a forbidden decision gets clamped.
func (e *Engine) probeClampingWorks(forbiddenKind string, probes ProbeInputs) ea.AuditCheck {
	// Build probe input
	input := enforcementclamp.ClampInput{
		CircleIDHash:    ea.ComputeEvidenceHash("probe_circle"),
		RawDecisionKind: forbiddenKind,
		RawReasonBucket: "probe",
		ContractsSummary: enforcementclamp.ContractsSummary{
			HasHoldOnlyContract: probes.HasDelegatedHolding,
			HasTransferContract: probes.HasTrustTransfer,
		},
	}

	// Run clamp
	output := e.clamp.ClampOutcome(input)

	// Verify clamping happened
	status := ea.StatusPass
	severity := ea.SeverityInfo

	if !output.WasClamped {
		status = ea.StatusFail
		severity = ea.SeverityCritical
	}

	// Verify result is HOLD or QUEUE_PROOF (not forbidden)
	if output.ClampedDecisionKind != ea.ClampedHold && output.ClampedDecisionKind != ea.ClampedQueueProof {
		status = ea.StatusFail
		severity = ea.SeverityCritical
	}

	evidence := forbiddenKind + "|clamped_to|" + output.ClampedDecisionKind.CanonicalString()

	return ea.AuditCheck{
		Target:       ea.TargetPressurePipeline,
		Check:        ea.CheckContractApplied,
		Status:       status,
		Severity:     severity,
		Component:    "clamp_wrapper",
		EvidenceHash: ea.ComputeEvidenceHash(evidence),
	}
}

// ============================================================================
// Query Methods
// ============================================================================

// GetLatestRun returns the latest audit run.
func (e *Engine) GetLatestRun() *ea.AuditRun {
	if e.auditStore == nil {
		return nil
	}
	return e.auditStore.GetLatestRun()
}

// IsLatestRunPassing checks if the latest run passed.
func (e *Engine) IsLatestRunPassing() bool {
	run := e.GetLatestRun()
	return run != nil && run.Status == ea.StatusPass
}

// ============================================================================
// Acknowledgment
// ============================================================================

// AcknowledgeRun acknowledges an audit run.
func (e *Engine) AcknowledgeRun(runHash string) (*ea.AuditAck, error) {
	periodKey := e.GetCurrentPeriodKey()

	ack := &ea.AuditAck{
		RunHash:   runHash,
		PeriodKey: periodKey,
	}
	ack.AckHash = ack.ComputeHash()

	if e.ackStore != nil {
		if err := e.ackStore.AppendAck(*ack); err != nil {
			return nil, err
		}
	}

	return ack, nil
}

// IsRunAcked checks if a run has been acknowledged.
func (e *Engine) IsRunAcked(runHash string) bool {
	if e.ackStore == nil {
		return false
	}
	return e.ackStore.IsAcked(runHash)
}

// ============================================================================
// Proof Page Building
// ============================================================================

// BuildProofPage builds the proof page for the latest audit run.
func (e *Engine) BuildProofPage() *ea.AuditProofPage {
	run := e.GetLatestRun()
	if run == nil {
		// No audit run yet
		page := ea.NewDefaultProofPage()
		page.PeriodKey = e.GetCurrentPeriodKey()
		page.Lines = []string{
			"No audit has been run yet.",
			"Run an audit to verify enforcement.",
		}
		page.PageHash = page.ComputeHash()
		return page
	}

	return ea.BuildProofPageFromRun(run)
}

// ============================================================================
// Validation Helpers
// ============================================================================

// ValidateManifestComplete checks if the manifest is complete and returns checks.
func ValidateManifestComplete(manifest EnforcementManifest) (bool, []string) {
	missing := manifest.MissingComponents()
	return len(missing) == 0, missing
}
