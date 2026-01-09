// Package proofhub provides the engine for Phase 52: Proof Hub + Connected Status.
//
// CRITICAL INVARIANTS:
// - NO POWER: This engine is observation/proof only. It MUST NOT change any
//   runtime behavior (no execution, no delivery, no polling, no goroutines).
// - PURE FUNCTIONS: All operations are deterministic.
// - CLOCK INJECTION: Time buckets are passed in via injected clock interface.
// - HASH-ONLY: Only produces hash-safe outputs.
// - NO NETWORK: No external calls. No storage writes. Pure transformation.
//
// Reference: docs/ADR/ADR-0090-phase52-proof-hub-connected-status.md
package proofhub

import (
	"time"

	domain "quantumlife/pkg/domain/proofhub"
)

// ============================================================================
// Clock Interface
// ============================================================================

// Clock provides the current time for period key derivation.
type Clock interface {
	Now() time.Time
}

// ============================================================================
// Store Adapters (read-only interfaces)
// ============================================================================

// ConnectionStatusReader reads connection status (hash-only).
type ConnectionStatusReader interface {
	// IsGmailConnected returns true if Gmail is connected for the circle.
	IsGmailConnected(circleIDHash string) bool
	// IsTrueLayerConnected returns true if TrueLayer is connected for the circle.
	IsTrueLayerConnected(circleIDHash string) bool
	// IsDeviceRegistered returns true if a device is registered for the circle.
	IsDeviceRegistered(circleIDHash string) bool
}

// SyncReceiptReader reads sync receipt status (bucket-only).
type SyncReceiptReader interface {
	// GmailLastSyncBucket returns the Gmail sync recency bucket.
	GmailLastSyncBucket(circleIDHash string, now time.Time) domain.SyncRecencyBucket
	// GmailNoticedMagnitude returns the Gmail noticed magnitude bucket.
	GmailNoticedMagnitude(circleIDHash string) domain.MagnitudeBucket
	// TrueLayerLastSyncBucket returns the TrueLayer sync recency bucket.
	TrueLayerLastSyncBucket(circleIDHash string, now time.Time) domain.SyncRecencyBucket
	// TrueLayerNoticedMagnitude returns the TrueLayer noticed magnitude bucket.
	TrueLayerNoticedMagnitude(circleIDHash string) domain.MagnitudeBucket
}

// ShadowHealthReader reads shadow provider health (bucket-only).
type ShadowHealthReader interface {
	// ShadowProviderKind returns the shadow provider kind bucket.
	ShadowProviderKind() string
	// ShadowRealAllowed returns true if real shadow is allowed.
	ShadowRealAllowed() bool
	// ShadowHealthStatus returns the shadow health status.
	ShadowHealthStatus() domain.ProviderStatus
}

// TransparencyLogReader reads transparency log status (bucket-only).
type TransparencyLogReader interface {
	// TransparencyLinesMagnitude returns the magnitude of transparency log lines.
	TransparencyLinesMagnitude(periodKey string) domain.MagnitudeBucket
	// LastLedgerPeriodBucket returns the recency bucket for the last ledger entry.
	LastLedgerPeriodBucket(now time.Time) domain.SyncRecencyBucket
}

// EnforcementReader reads enforcement audit status (bucket-only).
type EnforcementReader interface {
	// EnforcementAuditRecent returns true if there was a recent enforcement audit.
	EnforcementAuditRecent(circleIDHash string, now time.Time) bool
	// InterruptPolicyConfigured returns true if interrupt policy is configured.
	InterruptPolicyConfigured(circleIDHash string) bool
}

// AckReader reads proof hub acknowledgments.
type AckReader interface {
	// IsDismissed returns true if the proof hub was dismissed for this period+status.
	IsDismissed(circleIDHash, periodKey, statusHash string) bool
	// LastAckedStatusHash returns the last acked status hash for the period.
	LastAckedStatusHash(circleIDHash, periodKey string) (string, bool)
}

// ============================================================================
// Stub Implementations (for when stores don't exist)
// ============================================================================

// StubConnectionReader returns defaults when no connection store exists.
type StubConnectionReader struct{}

func (s StubConnectionReader) IsGmailConnected(circleIDHash string) bool     { return false }
func (s StubConnectionReader) IsTrueLayerConnected(circleIDHash string) bool { return false }
func (s StubConnectionReader) IsDeviceRegistered(circleIDHash string) bool   { return false }

// StubSyncReceiptReader returns defaults when no sync receipt store exists.
type StubSyncReceiptReader struct{}

func (s StubSyncReceiptReader) GmailLastSyncBucket(circleIDHash string, now time.Time) domain.SyncRecencyBucket {
	return domain.SyncNever
}
func (s StubSyncReceiptReader) GmailNoticedMagnitude(circleIDHash string) domain.MagnitudeBucket {
	return domain.MagNothing
}
func (s StubSyncReceiptReader) TrueLayerLastSyncBucket(circleIDHash string, now time.Time) domain.SyncRecencyBucket {
	return domain.SyncNever
}
func (s StubSyncReceiptReader) TrueLayerNoticedMagnitude(circleIDHash string) domain.MagnitudeBucket {
	return domain.MagNothing
}

// StubShadowHealthReader returns defaults when no shadow health exists.
type StubShadowHealthReader struct{}

func (s StubShadowHealthReader) ShadowProviderKind() string              { return "unknown" }
func (s StubShadowHealthReader) ShadowRealAllowed() bool                 { return false }
func (s StubShadowHealthReader) ShadowHealthStatus() domain.ProviderStatus { return domain.StatusUnknown }

// StubTransparencyLogReader returns defaults when no transparency log exists.
type StubTransparencyLogReader struct{}

func (s StubTransparencyLogReader) TransparencyLinesMagnitude(periodKey string) domain.MagnitudeBucket {
	return domain.MagNothing
}
func (s StubTransparencyLogReader) LastLedgerPeriodBucket(now time.Time) domain.SyncRecencyBucket {
	return domain.SyncNever
}

// StubEnforcementReader returns defaults when no enforcement audit exists.
type StubEnforcementReader struct{}

func (s StubEnforcementReader) EnforcementAuditRecent(circleIDHash string, now time.Time) bool {
	return false
}
func (s StubEnforcementReader) InterruptPolicyConfigured(circleIDHash string) bool {
	return false
}

// StubAckReader returns defaults when no ack store exists.
type StubAckReader struct{}

func (s StubAckReader) IsDismissed(circleIDHash, periodKey, statusHash string) bool {
	return false
}
func (s StubAckReader) LastAckedStatusHash(circleIDHash, periodKey string) (string, bool) {
	return "", false
}

// ============================================================================
// Engine
// ============================================================================

// Engine builds proof hub inputs and pages.
// It is stateless and produces deterministic output.
type Engine struct {
	clk                Clock
	connectionReader   ConnectionStatusReader
	syncReceiptReader  SyncReceiptReader
	shadowHealthReader ShadowHealthReader
	transparencyReader TransparencyLogReader
	enforcementReader  EnforcementReader
	ackReader          AckReader
}

// EngineOption configures the engine.
type EngineOption func(*Engine)

// WithConnectionReader sets the connection reader.
func WithConnectionReader(r ConnectionStatusReader) EngineOption {
	return func(e *Engine) { e.connectionReader = r }
}

// WithSyncReceiptReader sets the sync receipt reader.
func WithSyncReceiptReader(r SyncReceiptReader) EngineOption {
	return func(e *Engine) { e.syncReceiptReader = r }
}

// WithShadowHealthReader sets the shadow health reader.
func WithShadowHealthReader(r ShadowHealthReader) EngineOption {
	return func(e *Engine) { e.shadowHealthReader = r }
}

// WithTransparencyReader sets the transparency log reader.
func WithTransparencyReader(r TransparencyLogReader) EngineOption {
	return func(e *Engine) { e.transparencyReader = r }
}

// WithEnforcementReader sets the enforcement reader.
func WithEnforcementReader(r EnforcementReader) EngineOption {
	return func(e *Engine) { e.enforcementReader = r }
}

// WithAckReader sets the ack reader.
func WithAckReader(r AckReader) EngineOption {
	return func(e *Engine) { e.ackReader = r }
}

// New creates a new proof hub engine with injected clock.
func New(clk Clock, opts ...EngineOption) *Engine {
	e := &Engine{
		clk:                clk,
		connectionReader:   StubConnectionReader{},
		syncReceiptReader:  StubSyncReceiptReader{},
		shadowHealthReader: StubShadowHealthReader{},
		transparencyReader: StubTransparencyLogReader{},
		enforcementReader:  StubEnforcementReader{},
		ackReader:          StubAckReader{},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// ============================================================================
// Input Building
// ============================================================================

// BuildInputs gathers inputs from existing stores.
func (e *Engine) BuildInputs(circleIDHash string) domain.ProofHubInputs {
	now := e.clk.Now()
	periodKey := ComputePeriodKey(now)

	return domain.ProofHubInputs{
		CircleIDHash: circleIDHash,
		NowPeriodKey: periodKey,

		GmailConnected:        e.connectionReader.IsGmailConnected(circleIDHash),
		GmailLastSyncBucket:   e.syncReceiptReader.GmailLastSyncBucket(circleIDHash, now),
		GmailNoticedMagnitude: e.syncReceiptReader.GmailNoticedMagnitude(circleIDHash),

		TrueLayerConnected:        e.connectionReader.IsTrueLayerConnected(circleIDHash),
		TrueLayerLastSyncBucket:   e.syncReceiptReader.TrueLayerLastSyncBucket(circleIDHash, now),
		TrueLayerNoticedMagnitude: e.syncReceiptReader.TrueLayerNoticedMagnitude(circleIDHash),

		ShadowProviderKind: e.shadowHealthReader.ShadowProviderKind(),
		ShadowRealAllowed:  e.shadowHealthReader.ShadowRealAllowed(),
		ShadowHealthStatus: e.shadowHealthReader.ShadowHealthStatus(),

		DeviceRegistered: e.connectionReader.IsDeviceRegistered(circleIDHash),

		TransparencyLinesMagnitude: e.transparencyReader.TransparencyLinesMagnitude(periodKey),
		LastLedgerPeriodBucket:     e.transparencyReader.LastLedgerPeriodBucket(now),
		EnforcementAuditRecent:     e.enforcementReader.EnforcementAuditRecent(circleIDHash, now),
		InterruptPolicyConfigured:  e.enforcementReader.InterruptPolicyConfigured(circleIDHash),
	}
}

// ============================================================================
// Page Building
// ============================================================================

// BuildPage builds a proof hub page from inputs.
func (e *Engine) BuildPage(inputs domain.ProofHubInputs) domain.ProofHubPage {
	statusHash := domain.HashProofHubStatus(inputs)

	sections := []domain.ProofHubSection{
		e.buildIdentitySection(inputs),
		e.buildConnectionsSection(inputs),
		e.buildSyncSection(inputs),
		e.buildShadowSection(inputs),
		e.buildLedgerSection(inputs),
		e.buildInvariantsSection(),
	}

	// Sort for determinism
	domain.SortSections(sections)

	page := domain.ProofHubPage{
		Title:      "Proof, quietly.",
		PeriodKey:  inputs.NowPeriodKey,
		StatusHash: statusHash,
		Sections:   sections,
	}

	return page
}

func (e *Engine) buildIdentitySection(inputs domain.ProofHubInputs) domain.ProofHubSection {
	// Show only short hash prefix (first 16 chars)
	shortHash := inputs.CircleIDHash
	if len(shortHash) > 16 {
		shortHash = shortHash[:16]
	}

	badges := []domain.ProofHubBadge{
		{Label: "Circle", Kind: "identity", Value: shortHash},
	}
	domain.SortBadges(badges)

	return domain.ProofHubSection{
		Kind:   domain.SectionIdentity,
		Title:  "Identity",
		Badges: badges,
		Lines:  []string{"Your circle, quietly verified."},
	}
}

func (e *Engine) buildConnectionsSection(inputs domain.ProofHubInputs) domain.ProofHubSection {
	badges := []domain.ProofHubBadge{
		{Label: "Gmail", Kind: "connection", Value: connectStatusValue(inputs.GmailConnected)},
		{Label: "TrueLayer", Kind: "connection", Value: connectStatusValue(inputs.TrueLayerConnected)},
		{Label: "Device", Kind: "connection", Value: connectStatusValue(inputs.DeviceRegistered)},
	}
	domain.SortBadges(badges)

	return domain.ProofHubSection{
		Kind:   domain.SectionConnections,
		Title:  "Connections",
		Badges: badges,
		Lines:  []string{"What's connected, nothing more."},
	}
}

func (e *Engine) buildSyncSection(inputs domain.ProofHubInputs) domain.ProofHubSection {
	badges := []domain.ProofHubBadge{
		{Label: "Gmail sync", Kind: "recency", Value: inputs.GmailLastSyncBucket.String()},
		{Label: "Gmail activity", Kind: "magnitude", Value: inputs.GmailNoticedMagnitude.String()},
		{Label: "TrueLayer sync", Kind: "recency", Value: inputs.TrueLayerLastSyncBucket.String()},
		{Label: "TrueLayer activity", Kind: "magnitude", Value: inputs.TrueLayerNoticedMagnitude.String()},
	}
	domain.SortBadges(badges)

	return domain.ProofHubSection{
		Kind:   domain.SectionSync,
		Title:  "Sync",
		Badges: badges,
		Lines:  []string{"Activity buckets, never counts."},
	}
}

func (e *Engine) buildShadowSection(inputs domain.ProofHubInputs) domain.ProofHubSection {
	badges := []domain.ProofHubBadge{
		{Label: "Provider", Kind: "shadow", Value: inputs.ShadowProviderKind},
		{Label: "Real allowed", Kind: "shadow", Value: boolToYesNo(inputs.ShadowRealAllowed)},
		{Label: "Health", Kind: "shadow", Value: inputs.ShadowHealthStatus.String()},
	}
	domain.SortBadges(badges)

	return domain.ProofHubSection{
		Kind:   domain.SectionShadow,
		Title:  "Shadow",
		Badges: badges,
		Lines:  []string{"Shadow provider status, quietly."},
	}
}

func (e *Engine) buildLedgerSection(inputs domain.ProofHubInputs) domain.ProofHubSection {
	badges := []domain.ProofHubBadge{
		{Label: "Ledger entries", Kind: "magnitude", Value: inputs.TransparencyLinesMagnitude.String()},
		{Label: "Ledger recency", Kind: "recency", Value: inputs.LastLedgerPeriodBucket.String()},
		{Label: "Audit recent", Kind: "status", Value: boolToYesNo(inputs.EnforcementAuditRecent)},
		{Label: "Interrupt policy", Kind: "status", Value: boolToYesNo(inputs.InterruptPolicyConfigured)},
	}
	domain.SortBadges(badges)

	return domain.ProofHubSection{
		Kind:   domain.SectionLedger,
		Title:  "Ledger",
		Badges: badges,
		Lines:  []string{"Transparency, quietly recorded."},
	}
}

func (e *Engine) buildInvariantsSection() domain.ProofHubSection {
	return domain.ProofHubSection{
		Kind:   domain.SectionInvariants,
		Title:  "Invariants",
		Badges: []domain.ProofHubBadge{},
		Lines: []string{
			"No background execution.",
			"Hash-only storage.",
			"Silence is default.",
		},
	}
}

func connectStatusValue(connected bool) string {
	if connected {
		return domain.ConnectYes.String()
	}
	return domain.ConnectNo.String()
}

func boolToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// ============================================================================
// Cue Building
// ============================================================================

// BuildCue builds the proof hub cue.
func (e *Engine) BuildCue(page domain.ProofHubPage, dismissed bool) domain.ProofHubCue {
	if dismissed {
		return domain.ProofHubCue{Available: false}
	}

	return domain.ProofHubCue{
		Available: true,
		Text:      "Proof is available — quietly.",
		Path:      "/proof/hub",
	}
}

// ShouldShowCue determines if the cue should be shown.
func (e *Engine) ShouldShowCue(circleIDHash, periodKey, statusHash string) bool {
	// Check if dismissed for this period+status
	if e.ackReader.IsDismissed(circleIDHash, periodKey, statusHash) {
		return false
	}

	// Check if status hash differs from last acked
	lastAcked, exists := e.ackReader.LastAckedStatusHash(circleIDHash, periodKey)
	if exists && lastAcked == statusHash {
		return false
	}

	return true
}

// ============================================================================
// Helper Functions
// ============================================================================

// ComputePeriodKey computes the period key from a time (ISO week format).
func ComputePeriodKey(t time.Time) string {
	year, week := t.ISOWeek()
	return formatWeek(year, week)
}

func formatWeek(year, week int) string {
	if week < 10 {
		return string(rune('0'+year/1000)) + string(rune('0'+(year/100)%10)) + string(rune('0'+(year/10)%10)) + string(rune('0'+year%10)) + "-W0" + string(rune('0'+week))
	}
	return string(rune('0'+year/1000)) + string(rune('0'+(year/100)%10)) + string(rune('0'+(year/10)%10)) + string(rune('0'+year%10)) + "-W" + string(rune('0'+week/10)) + string(rune('0'+week%10))
}
