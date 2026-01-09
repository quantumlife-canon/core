// Package enforcementaudit provides Phase 44.2: Enforcement Wiring Audit types.
//
// This package defines types for auditing and proving that HOLD-only constraints
// actually bind the runtime. It ensures no pipeline can "escape" enforcement.
//
// CRITICAL INVARIANTS:
//   - Hash-only storage. No raw identifiers.
//   - All checks deterministic with canonical strings.
//   - Max 12 checks per audit run.
//   - Bounded retention: 30 days OR 100 records.
//   - No goroutines. Clock injection required.
//   - Components must be from allowlist.
//
// Reference: docs/ADR/ADR-0082-phase44-2-enforcement-wiring-audit.md
package enforcementaudit

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ============================================================================
// Constants
// ============================================================================

const (
	// MaxChecksPerRun is the maximum number of checks per audit run.
	MaxChecksPerRun = 12

	// MaxRetentionDays is the maximum retention period.
	MaxRetentionDays = 30

	// MaxRecords is the maximum number of records to retain.
	MaxRecords = 100

	// Version is the current schema version.
	Version = "v1.0.0"
)

// ============================================================================
// Enums: AuditTargetKind
// ============================================================================

// AuditTargetKind represents the kind of pipeline being audited.
type AuditTargetKind string

const (
	TargetPressurePipeline         AuditTargetKind = "pressure_pipeline"
	TargetInterruptPipeline        AuditTargetKind = "interrupt_pipeline"
	TargetDeliveryPipeline         AuditTargetKind = "delivery_pipeline"
	TargetActionInvitationPipeline AuditTargetKind = "action_invitation_pipeline"
	TargetTimeWindowPipeline       AuditTargetKind = "timewindow_pipeline"
)

// Validate validates the AuditTargetKind.
func (k AuditTargetKind) Validate() error {
	switch k {
	case TargetPressurePipeline, TargetInterruptPipeline, TargetDeliveryPipeline,
		TargetActionInvitationPipeline, TargetTimeWindowPipeline:
		return nil
	default:
		return errors.New("invalid audit target kind")
	}
}

// CanonicalString returns the canonical string representation.
func (k AuditTargetKind) CanonicalString() string {
	return string(k)
}

// AllTargetKinds returns all valid target kinds in canonical order.
func AllTargetKinds() []AuditTargetKind {
	return []AuditTargetKind{
		TargetPressurePipeline,
		TargetInterruptPipeline,
		TargetDeliveryPipeline,
		TargetActionInvitationPipeline,
		TargetTimeWindowPipeline,
	}
}

// ============================================================================
// Enums: AuditCheckKind
// ============================================================================

// AuditCheckKind represents the kind of check performed.
type AuditCheckKind string

const (
	CheckContractApplied    AuditCheckKind = "contract_applied"
	CheckContractNotApplied AuditCheckKind = "contract_not_applied"
	CheckContractMisapplied AuditCheckKind = "contract_misapplied"
	CheckContractConflict   AuditCheckKind = "contract_conflict"
)

// Validate validates the AuditCheckKind.
func (k AuditCheckKind) Validate() error {
	switch k {
	case CheckContractApplied, CheckContractNotApplied, CheckContractMisapplied, CheckContractConflict:
		return nil
	default:
		return errors.New("invalid audit check kind")
	}
}

// CanonicalString returns the canonical string representation.
func (k AuditCheckKind) CanonicalString() string {
	return string(k)
}

// ============================================================================
// Enums: AuditStatus
// ============================================================================

// AuditStatus represents the status of an audit check or run.
type AuditStatus string

const (
	StatusPass AuditStatus = "pass"
	StatusFail AuditStatus = "fail"
)

// Validate validates the AuditStatus.
func (s AuditStatus) Validate() error {
	switch s {
	case StatusPass, StatusFail:
		return nil
	default:
		return errors.New("invalid audit status")
	}
}

// CanonicalString returns the canonical string representation.
func (s AuditStatus) CanonicalString() string {
	return string(s)
}

// ============================================================================
// Enums: AuditSeverity
// ============================================================================

// AuditSeverity represents the severity of an audit finding.
type AuditSeverity string

const (
	SeverityInfo     AuditSeverity = "info"
	SeverityWarn     AuditSeverity = "warn"
	SeverityCritical AuditSeverity = "critical"
)

// Validate validates the AuditSeverity.
func (s AuditSeverity) Validate() error {
	switch s {
	case SeverityInfo, SeverityWarn, SeverityCritical:
		return nil
	default:
		return errors.New("invalid audit severity")
	}
}

// CanonicalString returns the canonical string representation.
func (s AuditSeverity) CanonicalString() string {
	return string(s)
}

// ============================================================================
// Enums: ClampedDecisionKind
// ============================================================================

// ClampedDecisionKind represents the result of clamping a decision.
type ClampedDecisionKind string

const (
	ClampedNoEffect    ClampedDecisionKind = "no_effect"
	ClampedHold        ClampedDecisionKind = "hold"
	ClampedQueueProof  ClampedDecisionKind = "queue_proof"
	// NOTE: SURFACE, INTERRUPT_CANDIDATE, DELIVER, EXECUTE are NEVER returned
	// when a HOLD-only contract is active. This is the core invariant.
)

// Validate validates the ClampedDecisionKind.
func (k ClampedDecisionKind) Validate() error {
	switch k {
	case ClampedNoEffect, ClampedHold, ClampedQueueProof:
		return nil
	default:
		return errors.New("invalid clamped decision kind")
	}
}

// CanonicalString returns the canonical string representation.
func (k ClampedDecisionKind) CanonicalString() string {
	return string(k)
}

// ============================================================================
// Allowed Components
// ============================================================================

// AllowedComponents is the allowlist of component identifiers.
var AllowedComponents = map[string]bool{
	"pressure_gate":        true,
	"interrupt_preview":    true,
	"delivery_orchestrator": true,
	"action_invitation":    true,
	"timewindow_adapter":   true,
	"delegated_holding":    true,
	"trust_transfer":       true,
	"envelope_adapter":     true,
	"interrupt_policy":     true,
	"commerce_filter":      true,
	"manifest_builder":     true,
	"clamp_wrapper":        true,
}

// IsAllowedComponent checks if a component is in the allowlist.
func IsAllowedComponent(component string) bool {
	return AllowedComponents[component]
}

// ============================================================================
// Core Structs: AuditCheck
// ============================================================================

// AuditCheck represents a single audit check.
type AuditCheck struct {
	Target       AuditTargetKind
	Check        AuditCheckKind
	Status       AuditStatus
	Severity     AuditSeverity
	Component    string // Must be from AllowedComponents
	EvidenceHash string // SHA256 of canonical evidence line
}

// Validate validates the AuditCheck.
func (c *AuditCheck) Validate() error {
	if err := c.Target.Validate(); err != nil {
		return fmt.Errorf("target: %w", err)
	}
	if err := c.Check.Validate(); err != nil {
		return fmt.Errorf("check: %w", err)
	}
	if err := c.Status.Validate(); err != nil {
		return fmt.Errorf("status: %w", err)
	}
	if err := c.Severity.Validate(); err != nil {
		return fmt.Errorf("severity: %w", err)
	}
	if c.Component == "" {
		return errors.New("component required")
	}
	if !IsAllowedComponent(c.Component) {
		return errors.New("component not in allowlist")
	}
	if c.EvidenceHash == "" {
		return errors.New("evidence_hash required")
	}
	if len(c.EvidenceHash) != 64 {
		return errors.New("evidence_hash must be 64 hex chars")
	}
	return nil
}

// CanonicalString returns the canonical string representation.
// Format: v1|target|check|status|severity|component|evidence_hash
func (c *AuditCheck) CanonicalString() string {
	return fmt.Sprintf("v1|%s|%s|%s|%s|%s|%s",
		c.Target.CanonicalString(),
		c.Check.CanonicalString(),
		c.Status.CanonicalString(),
		c.Severity.CanonicalString(),
		c.Component,
		c.EvidenceHash,
	)
}

// ComputeHash computes the SHA256 hash of the canonical string.
func (c *AuditCheck) ComputeHash() string {
	h := sha256.Sum256([]byte(c.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// ============================================================================
// Core Structs: AuditRun
// ============================================================================

// AuditRun represents a complete audit run.
type AuditRun struct {
	PeriodKey          string       // Format: YYYY-MM-DD-HH
	Status             AuditStatus
	CriticalFailsBucket string      // Magnitude bucket: "0", "1", "2-5", "6+"
	Checks             []AuditCheck // Bounded: max 12 checks
	RunHash            string       // SHA256 of canonical string
}

// Validate validates the AuditRun.
func (r *AuditRun) Validate() error {
	if r.PeriodKey == "" {
		return errors.New("period_key required")
	}
	if err := r.Status.Validate(); err != nil {
		return fmt.Errorf("status: %w", err)
	}
	if len(r.Checks) > MaxChecksPerRun {
		return fmt.Errorf("too many checks: %d > %d", len(r.Checks), MaxChecksPerRun)
	}
	for i, check := range r.Checks {
		if err := check.Validate(); err != nil {
			return fmt.Errorf("check[%d]: %w", i, err)
		}
	}
	return nil
}

// CanonicalString returns the canonical string representation.
func (r *AuditRun) CanonicalString() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("v1|%s|%s|%s|%d",
		r.PeriodKey,
		r.Status.CanonicalString(),
		r.CriticalFailsBucket,
		len(r.Checks),
	))

	// Sort checks by canonical string for determinism
	sortedChecks := make([]AuditCheck, len(r.Checks))
	copy(sortedChecks, r.Checks)
	sort.Slice(sortedChecks, func(i, j int) bool {
		return sortedChecks[i].CanonicalString() < sortedChecks[j].CanonicalString()
	})

	for _, check := range sortedChecks {
		sb.WriteString("|")
		sb.WriteString(check.CanonicalString())
	}

	return sb.String()
}

// ComputeHash computes the SHA256 hash of the canonical string.
func (r *AuditRun) ComputeHash() string {
	h := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// CountCriticalFails counts checks with critical severity and fail status.
func (r *AuditRun) CountCriticalFails() int {
	count := 0
	for _, check := range r.Checks {
		if check.Severity == SeverityCritical && check.Status == StatusFail {
			count++
		}
	}
	return count
}

// ComputeCriticalFailsBucket computes the magnitude bucket for critical fails.
func ComputeCriticalFailsBucket(count int) string {
	switch {
	case count == 0:
		return "0"
	case count == 1:
		return "1"
	case count >= 2 && count <= 5:
		return "2-5"
	default:
		return "6+"
	}
}

// ============================================================================
// Core Structs: AuditProofPage
// ============================================================================

// AuditProofPage represents the proof page for an audit run.
type AuditProofPage struct {
	PeriodKey string
	Status    AuditStatus
	Lines     []string // Calm, no blame, no identifiers
	RunHash   string
	PageHash  string
}

// Validate validates the AuditProofPage.
func (p *AuditProofPage) Validate() error {
	if p.PeriodKey == "" {
		return errors.New("period_key required")
	}
	if err := p.Status.Validate(); err != nil {
		return fmt.Errorf("status: %w", err)
	}
	return nil
}

// CanonicalString returns the canonical string representation.
func (p *AuditProofPage) CanonicalString() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("v1|%s|%s|%s|%d",
		p.PeriodKey,
		p.Status.CanonicalString(),
		p.RunHash,
		len(p.Lines),
	))
	for _, line := range p.Lines {
		sb.WriteString("|")
		sb.WriteString(line)
	}
	return sb.String()
}

// ComputeHash computes the SHA256 hash of the canonical string.
func (p *AuditProofPage) ComputeHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// ============================================================================
// Core Structs: AuditAck
// ============================================================================

// AuditAck represents an acknowledgment of an audit run.
type AuditAck struct {
	RunHash   string
	PeriodKey string
	AckHash   string
}

// Validate validates the AuditAck.
func (a *AuditAck) Validate() error {
	if a.RunHash == "" {
		return errors.New("run_hash required")
	}
	if a.PeriodKey == "" {
		return errors.New("period_key required")
	}
	return nil
}

// CanonicalString returns the canonical string representation.
func (a *AuditAck) CanonicalString() string {
	return fmt.Sprintf("v1|%s|%s", a.RunHash, a.PeriodKey)
}

// ComputeHash computes the SHA256 hash of the canonical string.
func (a *AuditAck) ComputeHash() string {
	h := sha256.Sum256([]byte(a.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// ============================================================================
// Proof Page Builders
// ============================================================================

// DefaultProofLines returns the default calm lines for a passing audit.
var DefaultProofLines = []string{
	"Enforcement is wired.",
	"Holding agreements cannot be bypassed.",
	"No actions were enabled.",
}

// FailProofLines returns calm lines for a failing audit.
var FailProofLines = []string{
	"Enforcement audit detected issues.",
	"Some pipelines may need review.",
	"No actions were enabled.",
}

// NewDefaultProofPage creates a new default proof page.
func NewDefaultProofPage() *AuditProofPage {
	return &AuditProofPage{
		Status: StatusPass,
		Lines:  DefaultProofLines,
	}
}

// BuildProofPageFromRun builds a proof page from an audit run.
func BuildProofPageFromRun(run *AuditRun) *AuditProofPage {
	lines := DefaultProofLines
	if run.Status == StatusFail {
		lines = FailProofLines
	}

	page := &AuditProofPage{
		PeriodKey: run.PeriodKey,
		Status:    run.Status,
		Lines:     lines,
		RunHash:   run.RunHash,
	}
	page.PageHash = page.ComputeHash()
	return page
}

// ============================================================================
// Evidence Hash Helper
// ============================================================================

// ComputeEvidenceHash computes a SHA256 hash from evidence text.
func ComputeEvidenceHash(evidence string) string {
	h := sha256.Sum256([]byte(evidence))
	return hex.EncodeToString(h[:])
}
