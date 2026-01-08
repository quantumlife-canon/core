// Package delegatedholding provides domain types for Phase 42: Delegated Holding Contracts.
//
// Delegated Holding Contracts allow circles to explicitly set time-bounded, revocable
// agreements that bias the system toward HOLD for specific pressure scopes.
//
// CRITICAL INVARIANTS:
//   - NO execution. NO delivery. NO interrupts. Only HOLD bias.
//   - Hash-only persistence. No raw identifiers, timestamps, or amounts.
//   - Deterministic: same inputs + clock => same hashes and outcomes.
//   - No goroutines. No time.Now() - clock injection only.
//   - Bounded retention: 30 days OR 200 records max, FIFO eviction.
//   - Trust baseline required before contract creation.
//   - One active contract per circle at a time.
//
// Reference: docs/ADR/ADR-0079-phase42-delegated-holding-contracts.md
package delegatedholding

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"quantumlife/pkg/domain/externalpressure"
)

// ═══════════════════════════════════════════════════════════════════════════
// Enums
// ═══════════════════════════════════════════════════════════════════════════

// DelegationScope represents what type of pressure the contract applies to.
type DelegationScope string

const (
	// ScopeHuman matches pressure from human circles (individuals, communications).
	ScopeHuman DelegationScope = "human"
	// ScopeInstitution matches pressure from institution circles (companies, services).
	ScopeInstitution DelegationScope = "institution"
)

// AllDelegationScopes returns all scopes in deterministic order.
func AllDelegationScopes() []DelegationScope {
	return []DelegationScope{ScopeHuman, ScopeInstitution}
}

// Validate checks if the scope is valid.
func (s DelegationScope) Validate() error {
	switch s {
	case ScopeHuman, ScopeInstitution:
		return nil
	default:
		return fmt.Errorf("invalid delegation scope: %s", s)
	}
}

// CanonicalString returns the canonical string representation.
func (s DelegationScope) CanonicalString() string {
	return string(s)
}

// DisplayText returns human-readable text for the scope.
func (s DelegationScope) DisplayText() string {
	switch s {
	case ScopeHuman:
		return "People"
	case ScopeInstitution:
		return "Institutions"
	default:
		return "Unknown"
	}
}

// DelegationAction represents what happens when pressure matches the contract.
type DelegationAction string

const (
	// ActionHoldSilently suppresses from all surfaces, no proof queued.
	ActionHoldSilently DelegationAction = "hold_silently"
	// ActionQueueProof suppresses from surfaces but queues proof for later.
	ActionQueueProof DelegationAction = "queue_proof"
)

// AllDelegationActions returns all actions in deterministic order.
func AllDelegationActions() []DelegationAction {
	return []DelegationAction{ActionHoldSilently, ActionQueueProof}
}

// Validate checks if the action is valid.
func (a DelegationAction) Validate() error {
	switch a {
	case ActionHoldSilently, ActionQueueProof:
		return nil
	default:
		return fmt.Errorf("invalid delegation action: %s", a)
	}
}

// CanonicalString returns the canonical string representation.
func (a DelegationAction) CanonicalString() string {
	return string(a)
}

// DisplayText returns human-readable text for the action.
func (a DelegationAction) DisplayText() string {
	switch a {
	case ActionHoldSilently:
		return "Hold silently"
	case ActionQueueProof:
		return "Queue proof"
	default:
		return "Unknown"
	}
}

// DelegationDuration represents how long the contract is active.
type DelegationDuration string

const (
	// DurationHour expires after 1 hour bucket.
	DurationHour DelegationDuration = "hour"
	// DurationDay expires after 1 day bucket.
	DurationDay DelegationDuration = "day"
	// DurationTrip requires manual revocation; auto-expires after 7 days.
	DurationTrip DelegationDuration = "trip"
)

// AllDelegationDurations returns all durations in deterministic order.
func AllDelegationDurations() []DelegationDuration {
	return []DelegationDuration{DurationHour, DurationDay, DurationTrip}
}

// Validate checks if the duration is valid.
func (d DelegationDuration) Validate() error {
	switch d {
	case DurationHour, DurationDay, DurationTrip:
		return nil
	default:
		return fmt.Errorf("invalid delegation duration: %s", d)
	}
}

// CanonicalString returns the canonical string representation.
func (d DelegationDuration) CanonicalString() string {
	return string(d)
}

// DisplayText returns human-readable text for the duration.
func (d DelegationDuration) DisplayText() string {
	switch d {
	case DurationHour:
		return "One hour"
	case DurationDay:
		return "One day"
	case DurationTrip:
		return "Until revoked"
	default:
		return "Unknown"
	}
}

// BucketCount returns the number of time buckets for expiry calculation.
// For trip, returns 7 (max 7 days).
func (d DelegationDuration) BucketCount() int {
	switch d {
	case DurationHour:
		return 1
	case DurationDay:
		return 24 // 24 hours
	case DurationTrip:
		return 168 // 7 days * 24 hours
	default:
		return 0
	}
}

// DelegationState represents the current state of a contract.
type DelegationState string

const (
	// StateActive means the contract is currently in effect.
	StateActive DelegationState = "active"
	// StateExpired means the contract has passed its duration.
	StateExpired DelegationState = "expired"
	// StateRevoked means the circle explicitly revoked the contract.
	StateRevoked DelegationState = "revoked"
)

// AllDelegationStates returns all states in deterministic order.
func AllDelegationStates() []DelegationState {
	return []DelegationState{StateActive, StateExpired, StateRevoked}
}

// Validate checks if the state is valid.
func (s DelegationState) Validate() error {
	switch s {
	case StateActive, StateExpired, StateRevoked:
		return nil
	default:
		return fmt.Errorf("invalid delegation state: %s", s)
	}
}

// CanonicalString returns the canonical string representation.
func (s DelegationState) CanonicalString() string {
	return string(s)
}

// DisplayText returns human-readable text for the state.
func (s DelegationState) DisplayText() string {
	switch s {
	case StateActive:
		return "Active"
	case StateExpired:
		return "Expired"
	case StateRevoked:
		return "Revoked"
	default:
		return "Unknown"
	}
}

// ApplyResultKind represents the outcome of applying a contract to pressure.
type ApplyResultKind string

const (
	// ResultNoEffect means the contract does not apply to this pressure.
	ResultNoEffect ApplyResultKind = "no_effect"
	// ResultHold means the contract applies and pressure should be held.
	ResultHold ApplyResultKind = "hold"
	// ResultQueueProof means the contract applies and proof should be queued.
	ResultQueueProof ApplyResultKind = "queue_proof"
)

// AllApplyResultKinds returns all result kinds in deterministic order.
func AllApplyResultKinds() []ApplyResultKind {
	return []ApplyResultKind{ResultNoEffect, ResultHold, ResultQueueProof}
}

// Validate checks if the result kind is valid.
func (r ApplyResultKind) Validate() error {
	switch r {
	case ResultNoEffect, ResultHold, ResultQueueProof:
		return nil
	default:
		return fmt.Errorf("invalid apply result kind: %s", r)
	}
}

// CanonicalString returns the canonical string representation.
func (r ApplyResultKind) CanonicalString() string {
	return string(r)
}

// DecisionReasonBucket represents reasons for eligibility decisions.
type DecisionReasonBucket string

const (
	// ReasonOK means eligible to create a contract.
	ReasonOK DecisionReasonBucket = "ok"
	// ReasonTrustMissing means no trust baseline established.
	ReasonTrustMissing DecisionReasonBucket = "trust_missing"
	// ReasonInterruptPreviewActive means interrupt preview is active.
	ReasonInterruptPreviewActive DecisionReasonBucket = "interrupt_preview_active"
	// ReasonActiveContractExists means a contract already exists.
	ReasonActiveContractExists DecisionReasonBucket = "active_contract_exists"
)

// AllDecisionReasonBuckets returns all reason buckets in deterministic order.
func AllDecisionReasonBuckets() []DecisionReasonBucket {
	return []DecisionReasonBucket{
		ReasonOK, ReasonTrustMissing, ReasonInterruptPreviewActive, ReasonActiveContractExists,
	}
}

// Validate checks if the reason bucket is valid.
func (r DecisionReasonBucket) Validate() error {
	switch r {
	case ReasonOK, ReasonTrustMissing, ReasonInterruptPreviewActive, ReasonActiveContractExists:
		return nil
	default:
		return fmt.Errorf("invalid decision reason bucket: %s", r)
	}
}

// CanonicalString returns the canonical string representation.
func (r DecisionReasonBucket) CanonicalString() string {
	return string(r)
}

// DisplayText returns human-readable text for the reason.
func (r DecisionReasonBucket) DisplayText() string {
	switch r {
	case ReasonOK:
		return "Ready"
	case ReasonTrustMissing:
		return "Trust baseline required"
	case ReasonInterruptPreviewActive:
		return "Interrupt preview is active"
	case ReasonActiveContractExists:
		return "A contract is already active"
	default:
		return "Unknown"
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Core Types
// ═══════════════════════════════════════════════════════════════════════════

// MaxRetentionDays is the maximum retention period for contracts.
const MaxRetentionDays = 30

// MaxRecords is the maximum number of contract records to store.
const MaxRecords = 200

// MaxTripDays is the maximum duration for a trip contract.
const MaxTripDays = 7

// DelegatedHoldingContract represents an active holding agreement.
// CRITICAL: Contains only hashes, enums, and buckets - no raw identifiers.
type DelegatedHoldingContract struct {
	// ContractIDHash is the deterministic hash of the contract.
	ContractIDHash string

	// CircleIDHash identifies the circle this contract belongs to.
	CircleIDHash string

	// Scope determines what type of pressure this contract applies to.
	Scope DelegationScope

	// MaxHorizon is the maximum pressure horizon this contract handles.
	// Pressure with horizon > MaxHorizon is not affected.
	MaxHorizon externalpressure.PressureHorizon

	// MaxMagnitude is the maximum pressure magnitude this contract handles.
	// Pressure with magnitude > MaxMagnitude is not affected.
	MaxMagnitude externalpressure.PressureMagnitude

	// Action determines what happens when pressure matches.
	Action DelegationAction

	// Duration determines how long the contract is active.
	Duration DelegationDuration

	// State is the current state of the contract.
	State DelegationState

	// PeriodKey is the bucketed period when this contract was created.
	// Format: "YYYY-MM-DD-HH" (hour bucket, not timestamp)
	PeriodKey string

	// StatusHash is a deterministic hash of the contract for verification.
	StatusHash string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (c *DelegatedHoldingContract) CanonicalString() string {
	return fmt.Sprintf("DHC|v1|%s|%s|%s|%s|%s|%s|%s|%s|%s",
		c.ContractIDHash,
		c.CircleIDHash,
		c.Scope.CanonicalString(),
		c.MaxHorizon,
		c.MaxMagnitude,
		c.Action.CanonicalString(),
		c.Duration.CanonicalString(),
		c.State.CanonicalString(),
		c.PeriodKey,
	)
}

// ComputeHash computes a deterministic hash of the contract.
func (c *DelegatedHoldingContract) ComputeHash() string {
	h := sha256.Sum256([]byte(c.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the contract is valid.
func (c *DelegatedHoldingContract) Validate() error {
	if c.ContractIDHash == "" {
		return fmt.Errorf("missing contract_id_hash")
	}
	if c.CircleIDHash == "" {
		return fmt.Errorf("missing circle_id_hash")
	}
	if err := c.Scope.Validate(); err != nil {
		return err
	}
	if err := c.MaxHorizon.Validate(); err != nil {
		return err
	}
	if err := c.MaxMagnitude.Validate(); err != nil {
		return err
	}
	if err := c.Action.Validate(); err != nil {
		return err
	}
	if err := c.Duration.Validate(); err != nil {
		return err
	}
	if err := c.State.Validate(); err != nil {
		return err
	}
	if c.PeriodKey == "" {
		return fmt.Errorf("missing period_key")
	}
	return nil
}

// DelegationInputs captures inputs needed to check contract eligibility.
type DelegationInputs struct {
	// CircleIDHash identifies the circle.
	CircleIDHash string

	// HasTrustBaseline indicates if a trust baseline exists (from Phase 20).
	HasTrustBaseline bool

	// HasActiveInterruptPreview indicates if interrupt preview is active (from Phase 34).
	HasActiveInterruptPreview bool

	// ExistingActiveContract indicates if an active contract already exists.
	ExistingActiveContract bool
}

// CanonicalString returns the pipe-delimited canonical form.
func (i *DelegationInputs) CanonicalString() string {
	return fmt.Sprintf("DHI|v1|%s|%t|%t|%t",
		i.CircleIDHash,
		i.HasTrustBaseline,
		i.HasActiveInterruptPreview,
		i.ExistingActiveContract,
	)
}

// CreateContractInput contains all inputs to create a new contract.
type CreateContractInput struct {
	// CircleIDHash identifies the circle.
	CircleIDHash string

	// Scope determines what type of pressure this contract applies to.
	Scope DelegationScope

	// Action determines what happens when pressure matches.
	Action DelegationAction

	// Duration determines how long the contract is active.
	Duration DelegationDuration

	// MaxHorizon is the maximum pressure horizon this contract handles.
	MaxHorizon externalpressure.PressureHorizon

	// MaxMagnitude is the maximum pressure magnitude this contract handles.
	MaxMagnitude externalpressure.PressureMagnitude

	// NowBucket is the current time bucket from injected clock.
	// Format: "YYYY-MM-DD-HH" (hour bucket)
	NowBucket string
}

// CanonicalString returns the pipe-delimited canonical form.
func (i *CreateContractInput) CanonicalString() string {
	return fmt.Sprintf("CCI|v1|%s|%s|%s|%s|%s|%s|%s",
		i.CircleIDHash,
		i.Scope.CanonicalString(),
		i.Action.CanonicalString(),
		i.Duration.CanonicalString(),
		i.MaxHorizon,
		i.MaxMagnitude,
		i.NowBucket,
	)
}

// ComputeContractIDHash computes the deterministic contract ID hash.
func (i *CreateContractInput) ComputeContractIDHash() string {
	h := sha256.Sum256([]byte(i.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the input is valid.
func (i *CreateContractInput) Validate() error {
	if i.CircleIDHash == "" {
		return fmt.Errorf("missing circle_id_hash")
	}
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if err := i.Action.Validate(); err != nil {
		return err
	}
	if err := i.Duration.Validate(); err != nil {
		return err
	}
	if err := i.MaxHorizon.Validate(); err != nil {
		return err
	}
	if err := i.MaxMagnitude.Validate(); err != nil {
		return err
	}
	if i.NowBucket == "" {
		return fmt.Errorf("missing now_bucket")
	}
	return nil
}

// RevokeContractInput contains inputs to revoke a contract.
type RevokeContractInput struct {
	// CircleIDHash identifies the circle.
	CircleIDHash string

	// ContractIDHash identifies the contract to revoke.
	ContractIDHash string

	// NowBucket is the current time bucket from injected clock.
	NowBucket string
}

// CanonicalString returns the pipe-delimited canonical form.
func (i *RevokeContractInput) CanonicalString() string {
	return fmt.Sprintf("RCI|v1|%s|%s|%s",
		i.CircleIDHash,
		i.ContractIDHash,
		i.NowBucket,
	)
}

// ComputeRevocationHash computes a deterministic hash of the revocation.
func (i *RevokeContractInput) ComputeRevocationHash() string {
	h := sha256.Sum256([]byte(i.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the input is valid.
func (i *RevokeContractInput) Validate() error {
	if i.CircleIDHash == "" {
		return fmt.Errorf("missing circle_id_hash")
	}
	if i.ContractIDHash == "" {
		return fmt.Errorf("missing contract_id_hash")
	}
	if i.NowBucket == "" {
		return fmt.Errorf("missing now_bucket")
	}
	return nil
}

// DelegationProof represents proof of a delegation for display.
// CRITICAL: Contains only hashes, enums, and buckets - no raw identifiers.
type DelegationProof struct {
	// ContractIDHash identifies the contract (prefix shown only).
	ContractIDHash string

	// State is the current state.
	State DelegationState

	// Scope is the delegation scope.
	Scope DelegationScope

	// CreatedBucket is when the contract was created.
	CreatedBucket string

	// StatusHash is a deterministic hash for verification.
	StatusHash string
}

// CanonicalString returns the pipe-delimited canonical form.
func (p *DelegationProof) CanonicalString() string {
	return fmt.Sprintf("DP|v1|%s|%s|%s|%s",
		p.ContractIDHash,
		p.State.CanonicalString(),
		p.Scope.CanonicalString(),
		p.CreatedBucket,
	)
}

// ComputeHash computes a deterministic hash of the proof.
func (p *DelegationProof) ComputeHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the proof is valid.
func (p *DelegationProof) Validate() error {
	if p.ContractIDHash == "" {
		return fmt.Errorf("missing contract_id_hash")
	}
	if err := p.State.Validate(); err != nil {
		return err
	}
	if err := p.Scope.Validate(); err != nil {
		return err
	}
	if p.CreatedBucket == "" {
		return fmt.Errorf("missing created_bucket")
	}
	if p.StatusHash == "" {
		return fmt.Errorf("missing status_hash")
	}
	return nil
}

// EligibilityDecision represents the result of checking contract eligibility.
type EligibilityDecision struct {
	// Allowed indicates if a contract can be created.
	Allowed bool

	// Reason is the reason for the decision.
	Reason DecisionReasonBucket
}

// CanonicalString returns the pipe-delimited canonical form.
func (d *EligibilityDecision) CanonicalString() string {
	return fmt.Sprintf("ED|v1|%t|%s", d.Allowed, d.Reason.CanonicalString())
}

// HoldingDecision represents the result of applying a contract to pressure.
type HoldingDecision struct {
	// Result is the outcome of applying the contract.
	Result ApplyResultKind

	// ContractIDHash is the contract that was applied (if any).
	ContractIDHash string
}

// CanonicalString returns the pipe-delimited canonical form.
func (d *HoldingDecision) CanonicalString() string {
	return fmt.Sprintf("HD|v1|%s|%s", d.Result.CanonicalString(), d.ContractIDHash)
}

// PressureInput represents pressure for contract matching.
// CRITICAL: Contains only abstract buckets from externalpressure.
type PressureInput struct {
	// CircleIDHash identifies the circle.
	CircleIDHash string

	// CircleKind indicates if this is sovereign or external.
	CircleKind externalpressure.CircleKind

	// Horizon is the pressure horizon.
	Horizon externalpressure.PressureHorizon

	// Magnitude is the pressure magnitude.
	Magnitude externalpressure.PressureMagnitude

	// Category is the pressure category.
	Category externalpressure.PressureCategory
}

// CanonicalString returns the pipe-delimited canonical form.
func (p *PressureInput) CanonicalString() string {
	return fmt.Sprintf("PI|v1|%s|%s|%s|%s|%s",
		p.CircleIDHash,
		p.CircleKind,
		p.Horizon,
		p.Magnitude,
		p.Category,
	)
}

// Revocation represents a contract revocation record.
type Revocation struct {
	// CircleIDHash identifies the circle.
	CircleIDHash string

	// ContractIDHash identifies the revoked contract.
	ContractIDHash string

	// RevocationHash is a deterministic hash of the revocation.
	RevocationHash string

	// PeriodKey is when the revocation occurred.
	PeriodKey string
}

// CanonicalString returns the pipe-delimited canonical form.
func (r *Revocation) CanonicalString() string {
	return fmt.Sprintf("REV|v1|%s|%s|%s|%s",
		r.CircleIDHash,
		r.ContractIDHash,
		r.RevocationHash,
		r.PeriodKey,
	)
}

// ComputeHash computes a deterministic hash of the revocation.
func (r *Revocation) ComputeHash() string {
	h := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// ═══════════════════════════════════════════════════════════════════════════
// Page Types for Web UI
// ═══════════════════════════════════════════════════════════════════════════

// DelegatePage represents the /delegate page data.
type DelegatePage struct {
	// Title is the page title.
	Title string

	// Lines contains explanatory text lines.
	Lines []string

	// HasActiveContract indicates if a contract is active.
	HasActiveContract bool

	// ActiveContract is the current active contract (if any).
	ActiveContract *DelegatedHoldingContract

	// CanCreate indicates if a new contract can be created.
	CanCreate bool

	// BlockedReason is why creation is blocked (if not CanCreate).
	BlockedReason string

	// CreatePath is the form action for creating a contract.
	CreatePath string

	// RevokePath is the form action for revoking a contract.
	RevokePath string

	// ProofPath is the link to the proof page.
	ProofPath string

	// BackLink is the link to go back.
	BackLink string

	// StatusHash is a deterministic hash for verification.
	StatusHash string
}

// CanonicalString returns the pipe-delimited canonical form.
func (p *DelegatePage) CanonicalString() string {
	var b strings.Builder
	b.WriteString("DPAGE|v1|")
	b.WriteString(p.Title)
	b.WriteString("|")
	b.WriteString(fmt.Sprintf("%t", p.HasActiveContract))
	b.WriteString("|")
	b.WriteString(fmt.Sprintf("%t", p.CanCreate))
	b.WriteString("|")
	b.WriteString(p.BlockedReason)
	if p.ActiveContract != nil {
		b.WriteString("|")
		b.WriteString(p.ActiveContract.ContractIDHash)
	}
	return b.String()
}

// ComputeHash computes a deterministic hash of the page.
func (p *DelegatePage) ComputeHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// DelegateProofPage represents the /proof/delegate page data.
type DelegateProofPage struct {
	// Title is the page title.
	Title string

	// Lines contains explanatory text lines.
	Lines []string

	// HasContract indicates if any contract exists.
	HasContract bool

	// IsActive indicates if the contract is active.
	IsActive bool

	// Scope is the contract scope.
	Scope DelegationScope

	// Action is the contract action.
	Action DelegationAction

	// Duration is the contract duration.
	Duration DelegationDuration

	// CreatedBucket is when the contract was created.
	CreatedBucket string

	// ContractHashPrefix is the first 8 chars of the contract hash.
	ContractHashPrefix string

	// StatusHashPrefix is the first 8 chars of the status hash.
	StatusHashPrefix string

	// BackLink is the link to go back.
	BackLink string
}

// CanonicalString returns the pipe-delimited canonical form.
func (p *DelegateProofPage) CanonicalString() string {
	return fmt.Sprintf("DPPAGE|v1|%s|%t|%t|%s|%s|%s|%s",
		p.Title,
		p.HasContract,
		p.IsActive,
		p.Scope.CanonicalString(),
		p.Action.CanonicalString(),
		p.Duration.CanonicalString(),
		p.CreatedBucket,
	)
}

// ComputeHash computes a deterministic hash of the page.
func (p *DelegateProofPage) ComputeHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// DefaultDelegateTitle is the standard page title.
const DefaultDelegateTitle = "Held, by agreement."

// DefaultProofTitle is the standard proof page title.
const DefaultProofTitle = "Agreement, kept."

// NewDefaultDelegatePage creates an empty delegate page.
func NewDefaultDelegatePage() *DelegatePage {
	page := &DelegatePage{
		Title:      DefaultDelegateTitle,
		Lines:      []string{"No holding agreement is active."},
		CanCreate:  false,
		CreatePath: "/delegate/create",
		RevokePath: "/delegate/revoke",
		ProofPath:  "/proof/delegate",
		BackLink:   "/today",
	}
	page.StatusHash = page.ComputeHash()
	return page
}

// NewDefaultProofPage creates an empty proof page.
func NewDefaultProofPage() *DelegateProofPage {
	return &DelegateProofPage{
		Title:       DefaultProofTitle,
		Lines:       []string{"No delegation recorded."},
		HasContract: false,
		BackLink:    "/delegate",
	}
}

// BuildProofFromContract creates a proof page from a contract.
func BuildProofFromContract(contract *DelegatedHoldingContract) *DelegateProofPage {
	if contract == nil {
		return NewDefaultProofPage()
	}

	contractPrefix := contract.ContractIDHash
	if len(contractPrefix) > 8 {
		contractPrefix = contractPrefix[:8]
	}

	statusPrefix := contract.StatusHash
	if len(statusPrefix) > 8 {
		statusPrefix = statusPrefix[:8]
	}

	page := &DelegateProofPage{
		Title:              DefaultProofTitle,
		Lines:              []string{"A holding agreement exists."},
		HasContract:        true,
		IsActive:           contract.State == StateActive,
		Scope:              contract.Scope,
		Action:             contract.Action,
		Duration:           contract.Duration,
		CreatedBucket:      contract.PeriodKey,
		ContractHashPrefix: contractPrefix,
		StatusHashPrefix:   statusPrefix,
		BackLink:           "/delegate",
	}

	return page
}
