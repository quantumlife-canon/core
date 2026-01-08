// Package interruptrehearsal implements the Phase 41 Live Interrupt Loop (APNs) engine.
//
// This package provides a pure deterministic engine for rehearsal delivery.
// It orchestrates "preview→deliver→receipt" using existing Phase 32-34 pipeline outputs.
//
// CRITICAL INVARIANTS:
//   - NO goroutines. NO time.Now() - clock injection only.
//   - NO new decision logic - must reuse Phase 32→33→34 pipeline outputs.
//   - NO network calls - engine returns delivery plan; cmd/ executes it.
//   - Abstract payload only. No identifiers. No names. No merchants.
//   - Deterministic IDs/hashes: same inputs + same clock period => same hashes.
//
// Reference: docs/ADR/ADR-0078-phase41-live-interrupt-loop-apns.md
package interruptrehearsal

import (
	"time"

	ir "quantumlife/pkg/domain/interruptrehearsal"
)

// ═══════════════════════════════════════════════════════════════════════════
// Interfaces
// ═══════════════════════════════════════════════════════════════════════════

// CandidateSource provides interrupt candidates from Phase 34.
type CandidateSource interface {
	// GetInterruptPreviewCandidate returns the current interrupt preview candidate.
	// Returns candidate hash + abstract fields, or empty hash if none.
	GetInterruptPreviewCandidate(circleIDHash string, now time.Time) (candidateHash string, hasCandidate bool)
}

// PolicySource provides interrupt policy from Phase 33.
type PolicySource interface {
	// GetInterruptPolicy returns the current interrupt policy allowance.
	// Returns allowance string and max per day.
	GetInterruptPolicy(circleIDHash string, now time.Time) (allowance string, maxPerDay int, enabled bool)
}

// DeviceSource provides device registration info.
type DeviceSource interface {
	// HasRegisteredDevice checks if a device is registered.
	HasRegisteredDevice(circleIDHash string) bool

	// GetTransportKind returns the transport kind for the registered device.
	GetTransportKind(circleIDHash string) ir.TransportKind
}

// RateLimitSource provides rate limit checks.
type RateLimitSource interface {
	// CanDeliver checks if delivery is allowed (rate limit check).
	// Returns whether delivery is allowed and the reject reason if not.
	CanDeliver(circleIDHash string, periodKey string) (allowed bool, reason ir.RehearsalRejectReason)

	// GetDailyDeliveryCount returns the number of deliveries today.
	GetDailyDeliveryCount(circleIDHash string, periodKey string) int
}

// SealedStatusSource provides sealed boundary status.
type SealedStatusSource interface {
	// IsSealedReady checks if APNs sealed credentials are configured.
	IsSealedReady() bool
}

// EnvelopeSource provides attention envelope status.
type EnvelopeSource interface {
	// IsEnvelopeActive checks if an attention envelope is active.
	IsEnvelopeActive(circleIDHash string, now time.Time) bool
}

// ═══════════════════════════════════════════════════════════════════════════
// Engine
// ═══════════════════════════════════════════════════════════════════════════

// Engine is the Phase 41 rehearsal engine.
// CRITICAL: Pure deterministic engine. No side effects. No network.
type Engine struct {
	candidateSource CandidateSource
	policySource    PolicySource
	deviceSource    DeviceSource
	rateLimitSource RateLimitSource
	sealedSource    SealedStatusSource
	envelopeSource  EnvelopeSource
}

// NewEngine creates a new rehearsal engine.
func NewEngine(
	candidateSource CandidateSource,
	policySource PolicySource,
	deviceSource DeviceSource,
	rateLimitSource RateLimitSource,
	sealedSource SealedStatusSource,
	envelopeSource EnvelopeSource,
) *Engine {
	return &Engine{
		candidateSource: candidateSource,
		policySource:    policySource,
		deviceSource:    deviceSource,
		rateLimitSource: rateLimitSource,
		sealedSource:    sealedSource,
		envelopeSource:  envelopeSource,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Eligibility Evaluation
// ═══════════════════════════════════════════════════════════════════════════

// EvaluateEligibility evaluates rehearsal eligibility.
// Returns a receipt with either status_requested (eligible) or status_rejected.
// CRITICAL: No network calls. No side effects.
func (e *Engine) EvaluateEligibility(circleIDHash string, now time.Time) *ir.RehearsalReceipt {
	periodKey := formatPeriodKey(now)
	timeBucket := formatTimeBucket(now)

	// Build inputs for audit
	inputs := e.buildInputs(circleIDHash, now, periodKey)

	// Check eligibility in order of specificity
	rejectReason := e.checkEligibility(inputs)

	if rejectReason != ir.RejectNone {
		// Rejected
		receipt := &ir.RehearsalReceipt{
			Kind:             ir.RehearsalInterruptDelivery,
			Status:           ir.StatusRejected,
			RejectReason:     rejectReason,
			PeriodKey:        periodKey,
			CircleIDHash:     circleIDHash,
			CandidateHash:    inputs.CandidateHash,
			AttemptIDHash:    "", // No attempt when rejected
			TransportKind:    inputs.TransportKind,
			DeliveryBucket:   ir.DeliveryNone,
			LatencyBucket:    ir.LatencyNA,
			ErrorClassBucket: ir.ErrorClassNone,
			TimeBucket:       timeBucket,
		}
		receipt.StatusHash = receipt.ComputeStatusHash()
		return receipt
	}

	// Eligible - compute attempt ID
	attemptIDHash := ir.ComputeAttemptIDHash(circleIDHash, inputs.CandidateHash, periodKey)

	receipt := &ir.RehearsalReceipt{
		Kind:             ir.RehearsalInterruptDelivery,
		Status:           ir.StatusRequested,
		RejectReason:     ir.RejectNone,
		PeriodKey:        periodKey,
		CircleIDHash:     circleIDHash,
		CandidateHash:    inputs.CandidateHash,
		AttemptIDHash:    attemptIDHash,
		TransportKind:    inputs.TransportKind,
		DeliveryBucket:   ir.DeliveryNone, // Not yet delivered
		LatencyBucket:    ir.LatencyNA,    // Not yet measured
		ErrorClassBucket: ir.ErrorClassNone,
		TimeBucket:       timeBucket,
	}
	receipt.StatusHash = receipt.ComputeStatusHash()
	return receipt
}

// buildInputs gathers all inputs for eligibility check.
func (e *Engine) buildInputs(circleIDHash string, now time.Time, periodKey string) *ir.RehearsalInputs {
	candidateHash, hasCandidate := "", false
	if e.candidateSource != nil {
		candidateHash, hasCandidate = e.candidateSource.GetInterruptPreviewCandidate(circleIDHash, now)
	}

	allowance, maxPerDay, _ := "", 0, false
	if e.policySource != nil {
		allowance, maxPerDay, _ = e.policySource.GetInterruptPolicy(circleIDHash, now)
	}

	hasDevice := false
	transportKind := ir.TransportNone
	if e.deviceSource != nil {
		hasDevice = e.deviceSource.HasRegisteredDevice(circleIDHash)
		if hasDevice {
			transportKind = e.deviceSource.GetTransportKind(circleIDHash)
		}
	}

	dailyCount := 0
	if e.rateLimitSource != nil {
		dailyCount = e.rateLimitSource.GetDailyDeliveryCount(circleIDHash, periodKey)
	}

	sealedReady := false
	if e.sealedSource != nil {
		sealedReady = e.sealedSource.IsSealedReady()
	}

	envelopeActive := false
	if e.envelopeSource != nil {
		envelopeActive = e.envelopeSource.IsEnvelopeActive(circleIDHash, now)
	}

	if !hasCandidate {
		candidateHash = ""
	}

	return &ir.RehearsalInputs{
		CircleIDHash:       circleIDHash,
		PeriodKey:          periodKey,
		Allowance:          allowance,
		MaxPerDay:          maxPerDay,
		DailyDeliveryCount: dailyCount,
		HasDevice:          hasDevice,
		CandidateHash:      candidateHash,
		TransportKind:      transportKind,
		SealedReady:        sealedReady,
		EnvelopeActive:     envelopeActive,
		TimeBucket:         formatTimeBucket(now),
	}
}

// checkEligibility checks all eligibility requirements.
// Returns RejectNone if eligible, otherwise the reject reason.
func (e *Engine) checkEligibility(inputs *ir.RehearsalInputs) ir.RehearsalRejectReason {
	// 1. Must have a registered device
	if !inputs.HasDevice {
		return ir.RejectNoDevice
	}

	// 2. Policy must allow interrupts (allowance not empty/none)
	if inputs.Allowance == "" || inputs.Allowance == "allow_none" {
		return ir.RejectPolicyDisallows
	}

	// 3. Must have a candidate
	if inputs.CandidateHash == "" {
		return ir.RejectNoCandidate
	}

	// 4. Rate limit check
	if e.rateLimitSource != nil {
		allowed, reason := e.rateLimitSource.CanDeliver(inputs.CircleIDHash, inputs.PeriodKey)
		if !allowed {
			return reason
		}
	}

	// 5. Transport must be available
	if inputs.TransportKind == ir.TransportNone {
		return ir.RejectTransportUnavailable
	}

	// 6. For APNs, sealed credentials must be ready
	if inputs.TransportKind == ir.TransportAPNs && !inputs.SealedReady {
		return ir.RejectSealedKeyMissing
	}

	return ir.RejectNone
}

// ═══════════════════════════════════════════════════════════════════════════
// Plan Building
// ═══════════════════════════════════════════════════════════════════════════

// BuildPlan builds a delivery plan from an eligible receipt.
// CRITICAL: Returns nil if receipt is not eligible (not status_requested).
func (e *Engine) BuildPlan(receipt *ir.RehearsalReceipt) *ir.RehearsalPlan {
	if receipt == nil || receipt.Status != ir.StatusRequested {
		return nil
	}

	return &ir.RehearsalPlan{
		AttemptIDHash:  receipt.AttemptIDHash,
		TransportKind:  receipt.TransportKind,
		DeepLinkTarget: ir.DeepLinkTarget,
		PayloadTitle:   ir.PushTitle,
		PayloadBody:    ir.PushBody,
		CandidateHash:  receipt.CandidateHash,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Finalization
// ═══════════════════════════════════════════════════════════════════════════

// FinalizeAfterAttempt updates a receipt after delivery attempt.
// CRITICAL: Does not modify the input receipt; returns a new one.
func (e *Engine) FinalizeAfterAttempt(
	receipt *ir.RehearsalReceipt,
	delivered bool,
	latencyBucket ir.LatencyBucket,
	errorClassBucket ir.ErrorClassBucket,
) *ir.RehearsalReceipt {
	if receipt == nil {
		return nil
	}

	// Copy receipt
	finalized := &ir.RehearsalReceipt{
		Kind:             receipt.Kind,
		RejectReason:     receipt.RejectReason,
		PeriodKey:        receipt.PeriodKey,
		CircleIDHash:     receipt.CircleIDHash,
		CandidateHash:    receipt.CandidateHash,
		AttemptIDHash:    receipt.AttemptIDHash,
		TransportKind:    receipt.TransportKind,
		TimeBucket:       receipt.TimeBucket,
		LatencyBucket:    latencyBucket,
		ErrorClassBucket: errorClassBucket,
	}

	// Determine final status
	if delivered {
		finalized.Status = ir.StatusDelivered
		finalized.DeliveryBucket = ir.DeliveryOne
	} else if errorClassBucket != ir.ErrorClassNone {
		finalized.Status = ir.StatusFailed
		finalized.DeliveryBucket = ir.DeliveryNone
	} else {
		finalized.Status = ir.StatusAttempted
		finalized.DeliveryBucket = ir.DeliveryNone
	}

	finalized.StatusHash = finalized.ComputeStatusHash()
	return finalized
}

// ═══════════════════════════════════════════════════════════════════════════
// Proof Page Building
// ═══════════════════════════════════════════════════════════════════════════

// BuildProofPage builds a proof page from a receipt.
func (e *Engine) BuildProofPage(receipt *ir.RehearsalReceipt) *ir.RehearsalProofPage {
	if receipt == nil {
		return ir.DefaultRehearsalProofPage(formatPeriodKey(time.Time{}))
	}
	return ir.BuildProofPageFromReceipt(receipt)
}

// BuildRehearsePage builds the rehearse page with current status.
func (e *Engine) BuildRehearsePage(circleIDHash string, now time.Time) *ir.RehearsePage {
	page := ir.DefaultRehearsePage()
	periodKey := formatPeriodKey(now)

	// Get policy info
	allowance, _, enabled := "", 0, false
	if e.policySource != nil {
		allowance, _, enabled = e.policySource.GetInterruptPolicy(circleIDHash, now)
	}

	if !enabled || allowance == "" || allowance == "allow_none" {
		page.PolicyAllowanceLabel = "Interrupts off"
	} else {
		page.PolicyAllowanceLabel = allowance
	}

	// Check device
	if e.deviceSource != nil {
		page.DeviceRegistered = e.deviceSource.HasRegisteredDevice(circleIDHash)
	}

	// Check candidate
	if e.candidateSource != nil {
		_, page.CandidateAvailable = e.candidateSource.GetInterruptPreviewCandidate(circleIDHash, now)
	}

	// Determine if sending is possible
	inputs := e.buildInputs(circleIDHash, now, periodKey)
	rejectReason := e.checkEligibility(inputs)

	if rejectReason == ir.RejectNone {
		page.CanSend = true
		page.BlockedReason = ""
	} else {
		page.CanSend = false
		page.BlockedReason = rejectReason.DisplayLabel()
	}

	return page
}

// ═══════════════════════════════════════════════════════════════════════════
// Time Helpers
// ═══════════════════════════════════════════════════════════════════════════

// formatPeriodKey formats a time as a daily period key (YYYY-MM-DD).
func formatPeriodKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// formatTimeBucket formats a time as a 15-minute bucket (HH:MM).
func formatTimeBucket(t time.Time) string {
	floored := t.UTC().Truncate(15 * time.Minute)
	return floored.Format("15:04")
}

// ═══════════════════════════════════════════════════════════════════════════
// Stub Implementations
// ═══════════════════════════════════════════════════════════════════════════

// StubCandidateSource is a stub implementation for testing.
type StubCandidateSource struct {
	CandidateHash string
	HasCandidate  bool
}

// GetInterruptPreviewCandidate implements CandidateSource.
func (s *StubCandidateSource) GetInterruptPreviewCandidate(circleIDHash string, now time.Time) (string, bool) {
	return s.CandidateHash, s.HasCandidate
}

// StubPolicySource is a stub implementation for testing.
type StubPolicySource struct {
	Allowance string
	MaxPerDay int
	Enabled   bool
}

// GetInterruptPolicy implements PolicySource.
func (s *StubPolicySource) GetInterruptPolicy(circleIDHash string, now time.Time) (string, int, bool) {
	return s.Allowance, s.MaxPerDay, s.Enabled
}

// StubDeviceSource is a stub implementation for testing.
type StubDeviceSource struct {
	HasDevice     bool
	TransportKind ir.TransportKind
}

// HasRegisteredDevice implements DeviceSource.
func (s *StubDeviceSource) HasRegisteredDevice(circleIDHash string) bool {
	return s.HasDevice
}

// GetTransportKind implements DeviceSource.
func (s *StubDeviceSource) GetTransportKind(circleIDHash string) ir.TransportKind {
	return s.TransportKind
}

// StubRateLimitSource is a stub implementation for testing.
type StubRateLimitSource struct {
	Allowed      bool
	RejectReason ir.RehearsalRejectReason
	DailyCount   int
}

// CanDeliver implements RateLimitSource.
func (s *StubRateLimitSource) CanDeliver(circleIDHash string, periodKey string) (bool, ir.RehearsalRejectReason) {
	return s.Allowed, s.RejectReason
}

// GetDailyDeliveryCount implements RateLimitSource.
func (s *StubRateLimitSource) GetDailyDeliveryCount(circleIDHash string, periodKey string) int {
	return s.DailyCount
}

// StubSealedStatusSource is a stub implementation for testing.
type StubSealedStatusSource struct {
	Ready bool
}

// IsSealedReady implements SealedStatusSource.
func (s *StubSealedStatusSource) IsSealedReady() bool {
	return s.Ready
}

// StubEnvelopeSource is a stub implementation for testing.
type StubEnvelopeSource struct {
	Active bool
}

// IsEnvelopeActive implements EnvelopeSource.
func (s *StubEnvelopeSource) IsEnvelopeActive(circleIDHash string, now time.Time) bool {
	return s.Active
}
