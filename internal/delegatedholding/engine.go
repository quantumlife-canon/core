// Package delegatedholding provides the Phase 42 engine for delegated holding contracts.
//
// CRITICAL INVARIANTS:
//   - Pure and deterministic: same inputs + clock => same outputs.
//   - NO execution. NO delivery. NO interrupts. Only HOLD bias.
//   - NO goroutines. All operations are synchronous.
//   - NO time.Now() - clock injection required.
//   - ApplyContract can ONLY return HOLD/QUEUE_PROOF/NO_EFFECT.
//   - CANNOT create SURFACE or INTERRUPT outcomes.
//   - Does NOT import pushtransport, interruptdelivery, execution, or oauth packages.
//
// Reference: docs/ADR/ADR-0079-phase42-delegated-holding-contracts.md
package delegatedholding

import (
	"time"

	dh "quantumlife/pkg/domain/delegatedholding"
	"quantumlife/pkg/domain/externalpressure"
)

// ═══════════════════════════════════════════════════════════════════════════
// Interfaces for Dependency Injection
// ═══════════════════════════════════════════════════════════════════════════

// TrustSource provides trust baseline information.
type TrustSource interface {
	// HasTrustBaseline returns true if a trust baseline exists for the circle.
	HasTrustBaseline(circleIDHash string) bool
}

// InterruptPreviewSource provides interrupt preview state.
type InterruptPreviewSource interface {
	// HasActivePreview returns true if an interrupt preview is active.
	HasActivePreview(circleIDHash string) bool
}

// ContractStore provides contract storage operations.
type ContractStore interface {
	// GetActiveContract returns the active contract for a circle, or nil.
	GetActiveContract(circleIDHash string, nowBucket string) *dh.DelegatedHoldingContract

	// UpsertActiveContract stores a contract.
	UpsertActiveContract(circleIDHash string, contract *dh.DelegatedHoldingContract, now time.Time) error

	// AppendRevocation records a contract revocation.
	AppendRevocation(circleIDHash string, contractIDHash string, nowBucket string, now time.Time) error

	// ListRecentContracts returns recent contracts for a circle.
	ListRecentContracts(circleIDHash string, limit int) []*dh.DelegatedHoldingContract
}

// Clock provides time operations.
type Clock interface {
	Now() time.Time
}

// ═══════════════════════════════════════════════════════════════════════════
// Engine
// ═══════════════════════════════════════════════════════════════════════════

// Engine provides Phase 42 delegated holding contract operations.
// CRITICAL: Pure and deterministic. No side effects except through stores.
type Engine struct {
	trustSource            TrustSource
	interruptPreviewSource InterruptPreviewSource
	contractStore          ContractStore
	clock                  Clock
}

// NewEngine creates a new Phase 42 engine.
func NewEngine(
	trustSource TrustSource,
	interruptPreviewSource InterruptPreviewSource,
	contractStore ContractStore,
	clock Clock,
) *Engine {
	return &Engine{
		trustSource:            trustSource,
		interruptPreviewSource: interruptPreviewSource,
		contractStore:          contractStore,
		clock:                  clock,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Eligibility Check
// ═══════════════════════════════════════════════════════════════════════════

// CanCreateContract checks if a new contract can be created.
// Returns a decision with allowed status and reason.
func (e *Engine) CanCreateContract(inputs dh.DelegationInputs) dh.EligibilityDecision {
	// Must have trust baseline (Phase 20)
	if !inputs.HasTrustBaseline {
		return dh.EligibilityDecision{
			Allowed: false,
			Reason:  dh.ReasonTrustMissing,
		}
	}

	// Cannot create when interrupt preview is active (Phase 34)
	if inputs.HasActiveInterruptPreview {
		return dh.EligibilityDecision{
			Allowed: false,
			Reason:  dh.ReasonInterruptPreviewActive,
		}
	}

	// Only one active contract per circle
	if inputs.ExistingActiveContract {
		return dh.EligibilityDecision{
			Allowed: false,
			Reason:  dh.ReasonActiveContractExists,
		}
	}

	return dh.EligibilityDecision{
		Allowed: true,
		Reason:  dh.ReasonOK,
	}
}

// BuildDelegationInputs builds inputs from injected sources.
func (e *Engine) BuildDelegationInputs(circleIDHash string) dh.DelegationInputs {
	nowBucket := e.computeNowBucket()

	hasTrust := false
	if e.trustSource != nil {
		hasTrust = e.trustSource.HasTrustBaseline(circleIDHash)
	}

	hasPreview := false
	if e.interruptPreviewSource != nil {
		hasPreview = e.interruptPreviewSource.HasActivePreview(circleIDHash)
	}

	hasContract := false
	if e.contractStore != nil {
		hasContract = e.contractStore.GetActiveContract(circleIDHash, nowBucket) != nil
	}

	return dh.DelegationInputs{
		CircleIDHash:              circleIDHash,
		HasTrustBaseline:          hasTrust,
		HasActiveInterruptPreview: hasPreview,
		ExistingActiveContract:    hasContract,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Contract Creation
// ═══════════════════════════════════════════════════════════════════════════

// CreateContract creates a new delegated holding contract.
// CRITICAL: Only creates the contract; does NOT persist.
func (e *Engine) CreateContract(input dh.CreateContractInput) *dh.DelegatedHoldingContract {
	if err := input.Validate(); err != nil {
		return nil
	}

	contractIDHash := input.ComputeContractIDHash()

	contract := &dh.DelegatedHoldingContract{
		ContractIDHash: contractIDHash,
		CircleIDHash:   input.CircleIDHash,
		Scope:          input.Scope,
		MaxHorizon:     input.MaxHorizon,
		MaxMagnitude:   input.MaxMagnitude,
		Action:         input.Action,
		Duration:       input.Duration,
		State:          dh.StateActive,
		PeriodKey:      input.NowBucket,
	}

	contract.StatusHash = contract.ComputeHash()
	return contract
}

// PersistContract persists a contract to the store.
func (e *Engine) PersistContract(contract *dh.DelegatedHoldingContract) error {
	if contract == nil || e.contractStore == nil {
		return nil
	}

	now := e.clock.Now()
	return e.contractStore.UpsertActiveContract(contract.CircleIDHash, contract, now)
}

// ═══════════════════════════════════════════════════════════════════════════
// State Computation
// ═══════════════════════════════════════════════════════════════════════════

// ComputeState computes the current state of a contract.
// Returns active, expired, or revoked based on nowBucket and duration.
func (e *Engine) ComputeState(contract *dh.DelegatedHoldingContract, nowBucket string) dh.DelegationState {
	if contract == nil {
		return dh.StateExpired
	}

	// Already revoked stays revoked
	if contract.State == dh.StateRevoked {
		return dh.StateRevoked
	}

	// Check expiry based on duration
	if e.isExpired(contract, nowBucket) {
		return dh.StateExpired
	}

	return dh.StateActive
}

// isExpired checks if a contract has expired based on duration and nowBucket.
func (e *Engine) isExpired(contract *dh.DelegatedHoldingContract, nowBucket string) bool {
	if contract == nil {
		return true
	}

	// Parse period keys to compare
	createdHour := parseHourBucket(contract.PeriodKey)
	nowHour := parseHourBucket(nowBucket)

	if createdHour.IsZero() || nowHour.IsZero() {
		return false // Can't determine, assume not expired
	}

	// Compute expiry time based on duration
	var expiryTime time.Time
	switch contract.Duration {
	case dh.DurationHour:
		expiryTime = createdHour.Add(time.Hour)
	case dh.DurationDay:
		expiryTime = createdHour.Add(24 * time.Hour)
	case dh.DurationTrip:
		expiryTime = createdHour.Add(7 * 24 * time.Hour) // Max 7 days
	default:
		return false
	}

	return nowHour.After(expiryTime) || nowHour.Equal(expiryTime)
}

// parseHourBucket parses a bucket string "YYYY-MM-DD-HH" to time.
func parseHourBucket(bucket string) time.Time {
	if len(bucket) < 13 {
		return time.Time{}
	}

	t, err := time.Parse("2006-01-02-15", bucket)
	if err != nil {
		return time.Time{}
	}
	return t
}

// ═══════════════════════════════════════════════════════════════════════════
// Contract Application
// ═══════════════════════════════════════════════════════════════════════════

// ApplyContract applies a contract to pressure input.
// CRITICAL: Can ONLY return NO_EFFECT, HOLD, or QUEUE_PROOF.
// CRITICAL: CANNOT return SURFACE or INTERRUPT outcomes.
func (e *Engine) ApplyContract(
	contract *dh.DelegatedHoldingContract,
	pressure dh.PressureInput,
	nowBucket string,
) dh.HoldingDecision {
	// No contract = no effect
	if contract == nil {
		return dh.HoldingDecision{
			Result:         dh.ResultNoEffect,
			ContractIDHash: "",
		}
	}

	// Contract not active = no effect
	state := e.ComputeState(contract, nowBucket)
	if state != dh.StateActive {
		return dh.HoldingDecision{
			Result:         dh.ResultNoEffect,
			ContractIDHash: contract.ContractIDHash,
		}
	}

	// Check scope match
	if !e.scopeMatches(contract.Scope, pressure) {
		return dh.HoldingDecision{
			Result:         dh.ResultNoEffect,
			ContractIDHash: contract.ContractIDHash,
		}
	}

	// Check horizon constraint
	if !e.horizonWithinMax(pressure.Horizon, contract.MaxHorizon) {
		return dh.HoldingDecision{
			Result:         dh.ResultNoEffect,
			ContractIDHash: contract.ContractIDHash,
		}
	}

	// Check magnitude constraint
	if !e.magnitudeWithinMax(pressure.Magnitude, contract.MaxMagnitude) {
		return dh.HoldingDecision{
			Result:         dh.ResultNoEffect,
			ContractIDHash: contract.ContractIDHash,
		}
	}

	// Contract applies - return appropriate result
	result := dh.ResultHold
	if contract.Action == dh.ActionQueueProof {
		result = dh.ResultQueueProof
	}

	return dh.HoldingDecision{
		Result:         result,
		ContractIDHash: contract.ContractIDHash,
	}
}

// scopeMatches checks if pressure matches the contract scope.
func (e *Engine) scopeMatches(scope dh.DelegationScope, pressure dh.PressureInput) bool {
	// External derived circles are always institution scope
	if pressure.CircleKind == externalpressure.CircleKindExternalDerived {
		return scope == dh.ScopeInstitution
	}

	// For sovereign circles, use category hints
	switch pressure.Category {
	case externalpressure.PressureCategoryDelivery,
		externalpressure.PressureCategoryRetail,
		externalpressure.PressureCategorySubscription:
		return scope == dh.ScopeInstitution
	case externalpressure.PressureCategoryTransport,
		externalpressure.PressureCategoryOther:
		// Could be either - default to matching human scope
		return scope == dh.ScopeHuman
	default:
		return false
	}
}

// horizonWithinMax checks if pressure horizon is within contract max.
func (e *Engine) horizonWithinMax(
	pressure externalpressure.PressureHorizon,
	max externalpressure.PressureHorizon,
) bool {
	// Order: soon < later < unknown
	horizonOrder := map[externalpressure.PressureHorizon]int{
		externalpressure.PressureHorizonSoon:    1,
		externalpressure.PressureHorizonLater:   2,
		externalpressure.PressureHorizonUnknown: 3,
	}

	pressureOrder, ok1 := horizonOrder[pressure]
	maxOrder, ok2 := horizonOrder[max]

	if !ok1 || !ok2 {
		return false
	}

	return pressureOrder <= maxOrder
}

// magnitudeWithinMax checks if pressure magnitude is within contract max.
func (e *Engine) magnitudeWithinMax(
	pressure externalpressure.PressureMagnitude,
	max externalpressure.PressureMagnitude,
) bool {
	// Order: nothing < a_few < several
	magnitudeOrder := map[externalpressure.PressureMagnitude]int{
		externalpressure.PressureMagnitudeNothing: 1,
		externalpressure.PressureMagnitudeAFew:    2,
		externalpressure.PressureMagnitudeSeveral: 3,
	}

	pressureOrder, ok1 := magnitudeOrder[pressure]
	maxOrder, ok2 := magnitudeOrder[max]

	if !ok1 || !ok2 {
		return false
	}

	return pressureOrder <= maxOrder
}

// ═══════════════════════════════════════════════════════════════════════════
// Contract Revocation
// ═══════════════════════════════════════════════════════════════════════════

// RevokeContract revokes an active contract.
func (e *Engine) RevokeContract(input dh.RevokeContractInput) error {
	if err := input.Validate(); err != nil {
		return err
	}

	if e.contractStore == nil {
		return nil
	}

	now := e.clock.Now()
	return e.contractStore.AppendRevocation(
		input.CircleIDHash,
		input.ContractIDHash,
		input.NowBucket,
		now,
	)
}

// ═══════════════════════════════════════════════════════════════════════════
// Page Building
// ═══════════════════════════════════════════════════════════════════════════

// BuildDelegatePage builds the /delegate page data.
func (e *Engine) BuildDelegatePage(circleIDHash string) *dh.DelegatePage {
	nowBucket := e.computeNowBucket()
	inputs := e.BuildDelegationInputs(circleIDHash)
	eligibility := e.CanCreateContract(inputs)

	var activeContract *dh.DelegatedHoldingContract
	if e.contractStore != nil {
		activeContract = e.contractStore.GetActiveContract(circleIDHash, nowBucket)
	}

	page := &dh.DelegatePage{
		Title:             dh.DefaultDelegateTitle,
		Lines:             []string{"Pre-consent to hold pressure silently."},
		HasActiveContract: activeContract != nil,
		ActiveContract:    activeContract,
		CanCreate:         eligibility.Allowed,
		BlockedReason:     eligibility.Reason.DisplayText(),
		CreatePath:        "/delegate/create",
		RevokePath:        "/delegate/revoke",
		ProofPath:         "/proof/delegate",
		BackLink:          "/today",
	}

	if activeContract != nil {
		page.Lines = []string{
			"A holding agreement is active.",
			"Matching pressure will be held.",
		}
	} else if eligibility.Allowed {
		page.Lines = []string{
			"No holding agreement is active.",
			"Create one to delegate holding.",
		}
	} else {
		page.Lines = []string{
			"No holding agreement is active.",
			eligibility.Reason.DisplayText(),
		}
	}

	page.StatusHash = page.ComputeHash()
	return page
}

// BuildProofPage builds the /proof/delegate page data.
func (e *Engine) BuildProofPage(circleIDHash string) *dh.DelegateProofPage {
	nowBucket := e.computeNowBucket()

	var contract *dh.DelegatedHoldingContract
	if e.contractStore != nil {
		// Get most recent contract (active or not)
		contracts := e.contractStore.ListRecentContracts(circleIDHash, 1)
		if len(contracts) > 0 {
			contract = contracts[0]
			// Update state
			contract.State = e.ComputeState(contract, nowBucket)
		}
	}

	return dh.BuildProofFromContract(contract)
}

// ═══════════════════════════════════════════════════════════════════════════
// Utility Methods
// ═══════════════════════════════════════════════════════════════════════════

// computeNowBucket computes the current hour bucket from clock.
func (e *Engine) computeNowBucket() string {
	if e.clock == nil {
		return ""
	}
	return e.clock.Now().UTC().Format("2006-01-02-15")
}

// GetActiveContract returns the active contract for a circle.
func (e *Engine) GetActiveContract(circleIDHash string) *dh.DelegatedHoldingContract {
	if e.contractStore == nil {
		return nil
	}
	nowBucket := e.computeNowBucket()
	return e.contractStore.GetActiveContract(circleIDHash, nowBucket)
}
