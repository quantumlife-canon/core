// Package vendorcontract provides domain types for Phase 49: Vendor Reality Contracts.
//
// This package defines vendor-declared caps that can only REDUCE pressure.
// Contracts integrate via a single choke-point clamp function.
//
// CRITICAL: No time.Now() in this package - clock must be injected.
// CRITICAL: No goroutines in this package.
// CRITICAL: Contracts can only reduce pressure, never increase it.
// CRITICAL: Commerce vendors capped at SURFACE_ONLY regardless of declaration.
//
// Reference: docs/ADR/ADR-0087-phase49-vendor-reality-contracts.md
package vendorcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// ============================================================================
// ContractScope Enum
// ============================================================================

// ContractScope represents the scope of a vendor contract.
type ContractScope string

const (
	ScopeCommerce    ContractScope = "scope_commerce"
	ScopeInstitution ContractScope = "scope_institution"
	ScopeHealth      ContractScope = "scope_health"
	ScopeTransport   ContractScope = "scope_transport"
	ScopeUnknown     ContractScope = "scope_unknown"
)

// Validate checks if the ContractScope is valid.
func (s ContractScope) Validate() error {
	switch s {
	case ScopeCommerce, ScopeInstitution, ScopeHealth, ScopeTransport, ScopeUnknown:
		return nil
	default:
		return errors.New("invalid contract scope")
	}
}

// CanonicalString returns the canonical string representation.
func (s ContractScope) CanonicalString() string {
	return string(s)
}

// String returns the string representation.
func (s ContractScope) String() string {
	return string(s)
}

// AllContractScopes returns all valid scopes in stable order.
func AllContractScopes() []ContractScope {
	return []ContractScope{
		ScopeCommerce,
		ScopeHealth,
		ScopeInstitution,
		ScopeTransport,
		ScopeUnknown,
	}
}

// ============================================================================
// PressureAllowance Enum
// ============================================================================

// PressureAllowance represents the maximum pressure level a vendor allows.
// Ordering: allow_hold_only < allow_surface_only < allow_interrupt_candidate
type PressureAllowance string

const (
	AllowHoldOnly           PressureAllowance = "allow_hold_only"
	AllowSurfaceOnly        PressureAllowance = "allow_surface_only"
	AllowInterruptCandidate PressureAllowance = "allow_interrupt_candidate"
)

// Validate checks if the PressureAllowance is valid.
func (p PressureAllowance) Validate() error {
	switch p {
	case AllowHoldOnly, AllowSurfaceOnly, AllowInterruptCandidate:
		return nil
	default:
		return errors.New("invalid pressure allowance")
	}
}

// CanonicalString returns the canonical string representation.
func (p PressureAllowance) CanonicalString() string {
	return string(p)
}

// String returns the string representation.
func (p PressureAllowance) String() string {
	return string(p)
}

// Level returns the numeric level for comparison.
// allow_hold_only=0, allow_surface_only=1, allow_interrupt_candidate=2
func (p PressureAllowance) Level() int {
	switch p {
	case AllowHoldOnly:
		return 0
	case AllowSurfaceOnly:
		return 1
	case AllowInterruptCandidate:
		return 2
	default:
		return 0 // Unknown defaults to most restrictive
	}
}

// AllPressureAllowances returns all valid allowances in order.
func AllPressureAllowances() []PressureAllowance {
	return []PressureAllowance{
		AllowHoldOnly,
		AllowSurfaceOnly,
		AllowInterruptCandidate,
	}
}

// ============================================================================
// FrequencyBucket Enum
// ============================================================================

// FrequencyBucket represents how often vendor pressure is allowed.
type FrequencyBucket string

const (
	FreqPerDay   FrequencyBucket = "freq_per_day"
	FreqPerWeek  FrequencyBucket = "freq_per_week"
	FreqPerEvent FrequencyBucket = "freq_per_event"
)

// Validate checks if the FrequencyBucket is valid.
func (f FrequencyBucket) Validate() error {
	switch f {
	case FreqPerDay, FreqPerWeek, FreqPerEvent:
		return nil
	default:
		return errors.New("invalid frequency bucket")
	}
}

// CanonicalString returns the canonical string representation.
func (f FrequencyBucket) CanonicalString() string {
	return string(f)
}

// String returns the string representation.
func (f FrequencyBucket) String() string {
	return string(f)
}

// AllFrequencyBuckets returns all valid buckets in stable order.
func AllFrequencyBuckets() []FrequencyBucket {
	return []FrequencyBucket{
		FreqPerDay,
		FreqPerEvent,
		FreqPerWeek,
	}
}

// ============================================================================
// EmergencyBucket Enum
// ============================================================================

// EmergencyBucket represents what kinds of emergencies can escalate.
type EmergencyBucket string

const (
	EmergencyNone            EmergencyBucket = "emergency_none"
	EmergencyHumanOnly       EmergencyBucket = "emergency_human_only"
	EmergencyInstitutionOnly EmergencyBucket = "emergency_institution_only"
)

// Validate checks if the EmergencyBucket is valid.
func (e EmergencyBucket) Validate() error {
	switch e {
	case EmergencyNone, EmergencyHumanOnly, EmergencyInstitutionOnly:
		return nil
	default:
		return errors.New("invalid emergency bucket")
	}
}

// CanonicalString returns the canonical string representation.
func (e EmergencyBucket) CanonicalString() string {
	return string(e)
}

// String returns the string representation.
func (e EmergencyBucket) String() string {
	return string(e)
}

// AllEmergencyBuckets returns all valid buckets in stable order.
func AllEmergencyBuckets() []EmergencyBucket {
	return []EmergencyBucket{
		EmergencyHumanOnly,
		EmergencyInstitutionOnly,
		EmergencyNone,
	}
}

// ============================================================================
// DeclaredByKind Enum
// ============================================================================

// DeclaredByKind represents who declared the contract.
type DeclaredByKind string

const (
	DeclaredVendorSelf  DeclaredByKind = "declared_vendor_self"
	DeclaredRegulator   DeclaredByKind = "declared_regulator"
	DeclaredMarketplace DeclaredByKind = "declared_marketplace"
)

// Validate checks if the DeclaredByKind is valid.
func (d DeclaredByKind) Validate() error {
	switch d {
	case DeclaredVendorSelf, DeclaredRegulator, DeclaredMarketplace:
		return nil
	default:
		return errors.New("invalid declared by kind")
	}
}

// CanonicalString returns the canonical string representation.
func (d DeclaredByKind) CanonicalString() string {
	return string(d)
}

// String returns the string representation.
func (d DeclaredByKind) String() string {
	return string(d)
}

// AllDeclaredByKinds returns all valid kinds in stable order.
func AllDeclaredByKinds() []DeclaredByKind {
	return []DeclaredByKind{
		DeclaredMarketplace,
		DeclaredRegulator,
		DeclaredVendorSelf,
	}
}

// ============================================================================
// ContractStatus Enum
// ============================================================================

// ContractStatus represents the status of a vendor contract.
type ContractStatus string

const (
	StatusActive  ContractStatus = "status_active"
	StatusRevoked ContractStatus = "status_revoked"
)

// Validate checks if the ContractStatus is valid.
func (c ContractStatus) Validate() error {
	switch c {
	case StatusActive, StatusRevoked:
		return nil
	default:
		return errors.New("invalid contract status")
	}
}

// CanonicalString returns the canonical string representation.
func (c ContractStatus) CanonicalString() string {
	return string(c)
}

// String returns the string representation.
func (c ContractStatus) String() string {
	return string(c)
}

// AllContractStatuses returns all valid statuses in stable order.
func AllContractStatuses() []ContractStatus {
	return []ContractStatus{
		StatusActive,
		StatusRevoked,
	}
}

// ============================================================================
// ContractReasonBucket Enum
// ============================================================================

// ContractReasonBucket represents the reason for a contract outcome.
type ContractReasonBucket string

const (
	ReasonOK             ContractReasonBucket = "reason_ok"
	ReasonInvalid        ContractReasonBucket = "reason_invalid"
	ReasonCommerceCapped ContractReasonBucket = "reason_commerce_capped"
	ReasonNoPower        ContractReasonBucket = "reason_no_power"
	ReasonRejected       ContractReasonBucket = "reason_rejected"
)

// Validate checks if the ContractReasonBucket is valid.
func (r ContractReasonBucket) Validate() error {
	switch r {
	case ReasonOK, ReasonInvalid, ReasonCommerceCapped, ReasonNoPower, ReasonRejected:
		return nil
	default:
		return errors.New("invalid contract reason bucket")
	}
}

// CanonicalString returns the canonical string representation.
func (r ContractReasonBucket) CanonicalString() string {
	return string(r)
}

// String returns the string representation.
func (r ContractReasonBucket) String() string {
	return string(r)
}

// AllContractReasonBuckets returns all valid reasons in stable order.
func AllContractReasonBuckets() []ContractReasonBucket {
	return []ContractReasonBucket{
		ReasonCommerceCapped,
		ReasonInvalid,
		ReasonNoPower,
		ReasonOK,
		ReasonRejected,
	}
}

// ============================================================================
// VendorContract Struct
// ============================================================================

// VendorContract represents a vendor's declared pressure constraints.
// CRITICAL: This can only REDUCE pressure, never increase it.
type VendorContract struct {
	VendorCircleHash   string            // SHA256 hash of vendor circle ID
	Scope              ContractScope     // What type of vendor
	AllowedPressure    PressureAllowance // Maximum pressure level
	MaxFrequency       FrequencyBucket   // How often pressure is allowed
	EmergencyException EmergencyBucket   // What emergencies can escalate
	DeclaredBy         DeclaredByKind    // Who declared the contract
	PeriodKey          string            // Period key (YYYY-MM-DD)
}

// Validate checks if the VendorContract is valid.
func (c VendorContract) Validate() error {
	if c.VendorCircleHash == "" {
		return errors.New("vendor circle hash is required")
	}
	if len(c.VendorCircleHash) != 64 {
		return errors.New("vendor circle hash must be 64 hex chars")
	}
	if err := c.Scope.Validate(); err != nil {
		return err
	}
	if err := c.AllowedPressure.Validate(); err != nil {
		return err
	}
	if err := c.MaxFrequency.Validate(); err != nil {
		return err
	}
	if err := c.EmergencyException.Validate(); err != nil {
		return err
	}
	if err := c.DeclaredBy.Validate(); err != nil {
		return err
	}
	if c.PeriodKey == "" {
		return errors.New("period key is required")
	}
	return nil
}

// CanonicalString returns a pipe-delimited canonical string.
// Fields are in stable order for deterministic hashing.
func (c VendorContract) CanonicalString() string {
	return c.VendorCircleHash + "|" +
		c.Scope.CanonicalString() + "|" +
		c.AllowedPressure.CanonicalString() + "|" +
		c.MaxFrequency.CanonicalString() + "|" +
		c.EmergencyException.CanonicalString() + "|" +
		c.DeclaredBy.CanonicalString() + "|" +
		c.PeriodKey
}

// ComputeContractHash computes the deterministic hash of this contract.
func (c VendorContract) ComputeContractHash() string {
	return HashContractString(c.CanonicalString())
}

// ============================================================================
// VendorContractRecord Struct
// ============================================================================

// VendorContractRecord represents a stored contract record.
// CRITICAL: Hash-only storage, no raw vendor strings.
type VendorContractRecord struct {
	ContractHash     string            // SHA256 of contract
	VendorCircleHash string            // SHA256 of vendor circle ID
	Scope            ContractScope     // Scope of contract
	EffectiveCap     PressureAllowance // Effective cap after commerce rules
	Status           ContractStatus    // Active or revoked
	CreatedAtBucket  string            // Period key when created (no raw timestamp)
	PeriodKey        string            // Period key for this record
}

// Validate checks if the VendorContractRecord is valid.
func (r VendorContractRecord) Validate() error {
	if r.ContractHash == "" {
		return errors.New("contract hash is required")
	}
	if r.VendorCircleHash == "" {
		return errors.New("vendor circle hash is required")
	}
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if err := r.EffectiveCap.Validate(); err != nil {
		return err
	}
	if err := r.Status.Validate(); err != nil {
		return err
	}
	if r.CreatedAtBucket == "" {
		return errors.New("created at bucket is required")
	}
	if r.PeriodKey == "" {
		return errors.New("period key is required")
	}
	return nil
}

// CanonicalString returns a pipe-delimited canonical string.
func (r VendorContractRecord) CanonicalString() string {
	return r.ContractHash + "|" +
		r.VendorCircleHash + "|" +
		r.Scope.CanonicalString() + "|" +
		r.EffectiveCap.CanonicalString() + "|" +
		r.Status.CanonicalString() + "|" +
		r.CreatedAtBucket + "|" +
		r.PeriodKey
}

// ============================================================================
// VendorContractOutcome Struct
// ============================================================================

// VendorContractOutcome represents the result of processing a contract.
type VendorContractOutcome struct {
	Accepted     bool                 // Whether the contract was accepted
	EffectiveCap PressureAllowance    // The effective cap (may be reduced)
	Reason       ContractReasonBucket // Reason for the outcome
}

// CanonicalString returns a pipe-delimited canonical string.
func (o VendorContractOutcome) CanonicalString() string {
	accepted := "false"
	if o.Accepted {
		accepted = "true"
	}
	return accepted + "|" +
		o.EffectiveCap.CanonicalString() + "|" +
		o.Reason.CanonicalString()
}

// ============================================================================
// VendorContractProofLine Struct
// ============================================================================

// VendorContractProofLine represents an abstract proof line for display.
// CRITICAL: No vendor names, URLs, or identifiers.
type VendorContractProofLine struct {
	VendorCircleHash string            // SHA256 hash of vendor circle
	Scope            ContractScope     // Contract scope
	EffectiveCap     PressureAllowance // Effective pressure cap
	PeriodKey        string            // Period key
	ProofHash        string            // Hash of proof line
}

// Validate checks if the VendorContractProofLine is valid.
func (p VendorContractProofLine) Validate() error {
	if p.VendorCircleHash == "" {
		return errors.New("vendor circle hash is required")
	}
	if err := p.Scope.Validate(); err != nil {
		return err
	}
	if err := p.EffectiveCap.Validate(); err != nil {
		return err
	}
	if p.PeriodKey == "" {
		return errors.New("period key is required")
	}
	if p.ProofHash == "" {
		return errors.New("proof hash is required")
	}
	return nil
}

// CanonicalString returns a pipe-delimited canonical string (without ProofHash).
func (p VendorContractProofLine) CanonicalString() string {
	return p.VendorCircleHash + "|" +
		p.Scope.CanonicalString() + "|" +
		p.EffectiveCap.CanonicalString() + "|" +
		p.PeriodKey
}

// ComputeProofHash computes the hash of this proof line.
func (p VendorContractProofLine) ComputeProofHash() string {
	return HashContractString(p.CanonicalString())
}

// ============================================================================
// VendorProofAck Struct
// ============================================================================

// VendorProofAckKind represents acknowledgment type.
type VendorProofAckKind string

const (
	VendorAckViewed    VendorProofAckKind = "vendor_ack_viewed"
	VendorAckDismissed VendorProofAckKind = "vendor_ack_dismissed"
)

// Validate checks if the VendorProofAckKind is valid.
func (k VendorProofAckKind) Validate() error {
	switch k {
	case VendorAckViewed, VendorAckDismissed:
		return nil
	default:
		return errors.New("invalid vendor proof ack kind")
	}
}

// CanonicalString returns the canonical string representation.
func (k VendorProofAckKind) CanonicalString() string {
	return string(k)
}

// String returns the string representation.
func (k VendorProofAckKind) String() string {
	return string(k)
}

// VendorProofAck represents an acknowledgment of vendor proof.
type VendorProofAck struct {
	VendorCircleHash string             // SHA256 hash of vendor circle
	PeriodKey        string             // Period key
	AckKind          VendorProofAckKind // Type of acknowledgment
	StatusHash       string             // SHA256 hash of ack state
}

// Validate checks if the VendorProofAck is valid.
func (a VendorProofAck) Validate() error {
	if a.VendorCircleHash == "" {
		return errors.New("vendor circle hash is required")
	}
	if a.PeriodKey == "" {
		return errors.New("period key is required")
	}
	if err := a.AckKind.Validate(); err != nil {
		return err
	}
	if a.StatusHash == "" {
		return errors.New("status hash is required")
	}
	return nil
}

// CanonicalString returns a pipe-delimited canonical string.
func (a VendorProofAck) CanonicalString() string {
	return a.VendorCircleHash + "|" +
		a.PeriodKey + "|" +
		a.AckKind.CanonicalString()
}

// ComputeStatusHash computes the SHA256 hash of the canonical ack string.
func (a VendorProofAck) ComputeStatusHash() string {
	return HashContractString(a.CanonicalString())
}

// ============================================================================
// VendorProofPage Struct
// ============================================================================

// VendorProofPage represents the UI model for vendor contract proof.
type VendorProofPage struct {
	Title      string                    // Page title
	Lines      []string                  // Calm copy lines
	ProofLines []VendorContractProofLine // Abstract proof lines
	StatusHash string                    // Status hash for page
}

// VendorProofCue represents the whisper cue for vendor contracts.
type VendorProofCue struct {
	Available  bool   // Whether the cue should be shown
	Text       string // Cue text (whisper style)
	Path       string // Path to proof page
	StatusHash string // Hash of cue state
}

// ============================================================================
// Hashing Helpers
// ============================================================================

// HashContractString computes SHA256 hash of a string and returns hex-encoded result.
func HashContractString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ComputeProofPageStatusHash computes the status hash for a proof page.
func ComputeProofPageStatusHash(proofLines []VendorContractProofLine) string {
	if len(proofLines) == 0 {
		return HashContractString("vendor_proof|empty")
	}
	canonical := "vendor_proof"
	for _, line := range proofLines {
		canonical += "|" + line.ProofHash
	}
	return HashContractString(canonical)
}

// ComputeVendorCueStatusHash computes the status hash for a cue.
func ComputeVendorCueStatusHash(contractCount int, available bool) string {
	availStr := "hidden"
	if available {
		availStr = "shown"
	}
	return HashContractString("vendor_cue|" + string(rune('0'+contractCount)) + "|" + availStr)
}

// ============================================================================
// Bounded Retention Constants
// ============================================================================

const (
	MaxVendorContractRecords = 200
	MaxVendorContractDays    = 30
	MaxVendorProofAckRecords = 200
	MaxVendorProofAckDays    = 30
)
