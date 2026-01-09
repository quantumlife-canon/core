// Package circlesemantics provides the engine for Phase 45: Circle Semantics & Necessity Declaration.
// This engine provides meaning-only semantics - it does NOT grant permission or enable actions.
// IMPORTANT: This package MUST NOT import or be imported by decision/delivery/execution packages.
package circlesemantics

import (
	"sort"
	"time"

	domain "quantumlife/pkg/domain/circlesemantics"
)

// Clock abstracts time for deterministic testing.
type Clock interface {
	Now() time.Time
}

// SemanticsInputs contains inputs for deriving default semantics.
type SemanticsInputs struct {
	CircleIDHashes          []string
	CircleTypes             map[string]string // values: "human"|"institution"|"commerce"|"unknown"
	HasGmail                bool
	HasTrueLayer            bool
	HasCommerceObservations bool
}

// AuditStore interface for checking proof dismissal.
type AckStore interface {
	IsProofDismissed(periodKey string) bool
}

// RecordStore interface for semantics records.
type RecordStore interface {
	GetLatest(circleIDHash string) (domain.SemanticsRecord, bool)
	ListLatestAll() []domain.SemanticsRecord
	ListByPeriod(periodKey string) []domain.SemanticsRecord
}

// Engine provides semantics operations.
// This is a pure, deterministic engine with no side effects.
type Engine struct {
	clock Clock
}

// NewEngine creates a new semantics engine with injected clock.
func NewEngine(clock Clock) *Engine {
	return &Engine{clock: clock}
}

// CircleType constants matching existing buckets.
const (
	CircleTypeHuman       = "human"
	CircleTypeInstitution = "institution"
	CircleTypeCommerce    = "commerce"
	CircleTypeUnknown     = "unknown"
)

// DeriveDefaultSemantics derives default semantics for a circle based on its type.
// Rules are deterministic and do NOT grant any power.
func (e *Engine) DeriveDefaultSemantics(circleIDHash string, circleType string) domain.CircleSemantics {
	// All derived semantics have effect_no_power - meaning only, no permission
	base := domain.CircleSemantics{
		Effect:      domain.EffectNoPower,
		Provenance:  domain.ProvenanceDerivedRules,
		NotesBucket: domain.NotesBucketDerived,
	}

	switch circleType {
	case CircleTypeCommerce:
		// Commerce: optional service, never interrupt, low necessity
		base.Kind = domain.SemanticServiceOptional
		base.Urgency = domain.UrgencyNeverInterrupt
		base.Necessity = domain.NecessityLow

	case CircleTypeHuman:
		// Human: human semantic, human-waiting urgency model, medium necessity
		base.Kind = domain.SemanticHuman
		base.Urgency = domain.UrgencyHumanWaiting
		base.Necessity = domain.NecessityMedium

	case CircleTypeInstitution:
		// Institution: institution semantic, hard deadline urgency, high necessity
		base.Kind = domain.SemanticInstitution
		base.Urgency = domain.UrgencyHardDeadline
		base.Necessity = domain.NecessityHigh

	default:
		// Unknown: all unknown values
		base.Kind = domain.SemanticUnknown
		base.Urgency = domain.UrgencyUnknown
		base.Necessity = domain.NecessityUnknown
	}

	return base
}

// DeriveAllDefaults derives default semantics for all circles in inputs.
func (e *Engine) DeriveAllDefaults(inputs SemanticsInputs) map[string]domain.CircleSemantics {
	result := make(map[string]domain.CircleSemantics)
	for _, circleIDHash := range inputs.CircleIDHashes {
		circleType := inputs.CircleTypes[circleIDHash]
		if circleType == "" {
			circleType = CircleTypeUnknown
		}
		result[circleIDHash] = e.DeriveDefaultSemantics(circleIDHash, circleType)
	}
	return result
}

// BuildSettingsPage builds the settings page model.
// Shows circle_id_hash only (no other identifiers).
// Items are stable-sorted by CircleIDHash.
func (e *Engine) BuildSettingsPage(inputs SemanticsInputs, existingRecords []domain.SemanticsRecord) domain.SemanticsSettingsPage {
	// Build map of existing records by circle ID hash
	existingMap := make(map[string]domain.CircleSemantics)
	for _, rec := range existingRecords {
		existingMap[rec.CircleIDHash] = rec.Semantics
	}

	// Derive defaults for all circles
	defaults := e.DeriveAllDefaults(inputs)

	// Build items
	items := make([]domain.SemanticsSettingsItem, 0, len(inputs.CircleIDHashes))
	for _, circleIDHash := range inputs.CircleIDHashes {
		current, exists := existingMap[circleIDHash]
		if !exists {
			current = defaults[circleIDHash]
		}

		item := domain.SemanticsSettingsItem{
			CircleIDHash:     circleIDHash,
			Current:          current,
			AllowedKinds:     domain.AllCircleSemanticKinds(),
			AllowedUrgency:   domain.AllUrgencyModels(),
			AllowedNecessity: domain.AllNecessityLevels(),
		}
		items = append(items, item)
	}

	// Stable sort by CircleIDHash
	sort.Slice(items, func(i, j int) bool {
		return items[i].CircleIDHash < items[j].CircleIDHash
	})

	// Cap display to max entries
	displayItems := items
	if len(displayItems) > domain.MaxDisplayEntries {
		displayItems = displayItems[:domain.MaxDisplayEntries]
	}

	// Compute status hash
	statusCanonical := e.computeSettingsStatusCanonical(displayItems)
	statusHash := domain.HashString(statusCanonical)

	return domain.SemanticsSettingsPage{
		Title: "Circle Semantics",
		Lines: []string{
			"Name what each source means to you.",
			"This helps explain priorities but does not change behavior.",
		},
		Items:      displayItems,
		StatusHash: statusHash,
	}
}

func (e *Engine) computeSettingsStatusCanonical(items []domain.SemanticsSettingsItem) string {
	result := "settings"
	for _, item := range items {
		result += "|" + item.CircleIDHash + "|" + item.Current.CanonicalStringV1()
	}
	return result
}

// ApplyUserDeclaration applies a user's semantic declaration.
// Enforces provenance_user_declared and effect_no_power.
// Returns the new record, change info, and any error.
func (e *Engine) ApplyUserDeclaration(
	circleIDHash string,
	desired domain.CircleSemantics,
	previous *domain.CircleSemantics,
) (domain.SemanticsRecord, domain.SemanticsChange, error) {
	// Enforce provenance_user_declared for user saves
	desired.Provenance = domain.ProvenanceUserDeclared

	// CRITICAL: Always enforce effect_no_power regardless of input
	desired.Effect = domain.EffectNoPower

	// Validate and sanitize notes bucket
	if !domain.AllowedNotesBuckets[desired.NotesBucket] {
		desired.NotesBucket = domain.NotesBucketNone
	}
	if desired.NotesBucket == "" {
		desired.NotesBucket = domain.NotesBucketUserSet
	}

	// Validate the sanitized semantics
	if err := desired.Validate(); err != nil {
		return domain.SemanticsRecord{}, domain.SemanticsChange{}, err
	}

	// Compute period key from clock
	periodKey := e.clock.Now().Format("2006-01-02")

	// Compute hashes
	semanticsHash := domain.ComputeSemanticsHash(desired)
	statusHash := domain.ComputeStatusHash(periodKey, circleIDHash, semanticsHash)

	// Build record
	record := domain.SemanticsRecord{
		PeriodKey:     periodKey,
		CircleIDHash:  circleIDHash,
		SemanticsHash: semanticsHash,
		StatusHash:    statusHash,
		Semantics:     desired,
	}

	// Compute change
	var change domain.SemanticsChange
	if previous == nil {
		change = domain.SemanticsChange{
			BeforeHash: "",
			AfterHash:  semanticsHash,
			ChangeKind: domain.ChangeKindCreated,
		}
	} else {
		beforeHash := domain.ComputeSemanticsHash(*previous)
		if beforeHash == semanticsHash {
			change = domain.SemanticsChange{
				BeforeHash: beforeHash,
				AfterHash:  semanticsHash,
				ChangeKind: domain.ChangeKindNoChange,
			}
		} else {
			change = domain.SemanticsChange{
				BeforeHash: beforeHash,
				AfterHash:  semanticsHash,
				ChangeKind: domain.ChangeKindUpdated,
			}
		}
	}

	return record, change, nil
}

// BuildProofPage builds the proof page model.
// Entries are stable-sorted by CircleIDHash.
func (e *Engine) BuildProofPage(allRecordsForPeriod []domain.SemanticsRecord) domain.SemanticsProofPage {
	// Sort records by CircleIDHash
	sorted := make([]domain.SemanticsRecord, len(allRecordsForPeriod))
	copy(sorted, allRecordsForPeriod)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CircleIDHash < sorted[j].CircleIDHash
	})

	// Cap display to max entries
	displayRecords := sorted
	if len(displayRecords) > domain.MaxDisplayEntries {
		displayRecords = displayRecords[:domain.MaxDisplayEntries]
	}

	// Build entries
	entries := make([]domain.SemanticsProofEntry, 0, len(displayRecords))
	for _, rec := range displayRecords {
		entry := domain.SemanticsProofEntry{
			CircleIDHash:  rec.CircleIDHash,
			SemanticsHash: rec.SemanticsHash,
			Kind:          rec.Semantics.Kind,
			Urgency:       rec.Semantics.Urgency,
			Necessity:     rec.Semantics.Necessity,
			Provenance:    rec.Semantics.Provenance,
			Effect:        rec.Semantics.Effect,
		}
		entries = append(entries, entry)
	}

	// Compute status hash
	statusCanonical := e.computeProofStatusCanonical(entries)
	statusHash := domain.HashString(statusCanonical)

	return domain.SemanticsProofPage{
		Title: "Semantics Proof",
		Lines: []string{
			"These are meanings you set or we derived.",
			"Meanings do not grant permission.",
		},
		Entries:    entries,
		StatusHash: statusHash,
	}
}

func (e *Engine) computeProofStatusCanonical(entries []domain.SemanticsProofEntry) string {
	result := "proof"
	for _, entry := range entries {
		result += "|" + entry.CircleIDHash + "|" + entry.SemanticsHash
	}
	return result
}

// ComputeCue computes the semantics cue (whisper).
// Returns a cue only if:
// - There exists at least one circle with semantic_unknown
// - AND user has connected something (gmail or truelayer)
// - AND proof not dismissed this period
func (e *Engine) ComputeCue(inputs SemanticsInputs, records []domain.SemanticsRecord, ackStore AckStore) domain.SemanticsCue {
	periodKey := e.clock.Now().Format("2006-01-02")

	// Check if dismissed this period
	if ackStore != nil && ackStore.IsProofDismissed(periodKey) {
		return domain.SemanticsCue{
			Available:  false,
			StatusHash: domain.HashString("cue|dismissed|" + periodKey),
		}
	}

	// Check if connected
	hasConnected := inputs.HasGmail || inputs.HasTrueLayer

	if !hasConnected {
		return domain.SemanticsCue{
			Available:  false,
			StatusHash: domain.HashString("cue|not_connected|" + periodKey),
		}
	}

	// Check for unknown semantics
	hasUnknown := false

	// First check existing records
	for _, rec := range records {
		if rec.Semantics.Kind == domain.SemanticUnknown {
			hasUnknown = true
			break
		}
	}

	// Also check derived defaults for circles without records
	if !hasUnknown {
		recordMap := make(map[string]bool)
		for _, rec := range records {
			recordMap[rec.CircleIDHash] = true
		}

		for _, circleIDHash := range inputs.CircleIDHashes {
			if !recordMap[circleIDHash] {
				circleType := inputs.CircleTypes[circleIDHash]
				if circleType == "" || circleType == CircleTypeUnknown {
					hasUnknown = true
					break
				}
			}
		}
	}

	if !hasUnknown {
		return domain.SemanticsCue{
			Available:  false,
			StatusHash: domain.HashString("cue|no_unknown|" + periodKey),
		}
	}

	return domain.SemanticsCue{
		Available:  true,
		Text:       "You can name what kind of thing this is.",
		Path:       "/settings/semantics",
		StatusHash: domain.HashString("cue|available|" + periodKey),
	}
}

// GetPeriodKey returns the current period key from the clock.
func (e *Engine) GetPeriodKey() string {
	return e.clock.Now().Format("2006-01-02")
}

// CreateProofAck creates a proof acknowledgment.
func (e *Engine) CreateProofAck(ackKind string, statusHash string) (domain.SemanticsProofAck, error) {
	periodKey := e.clock.Now().Format("2006-01-02")

	ack := domain.SemanticsProofAck{
		PeriodKey:  periodKey,
		StatusHash: statusHash,
		AckKind:    ackKind,
	}

	if err := ack.Validate(); err != nil {
		return domain.SemanticsProofAck{}, err
	}

	return ack, nil
}

// ParseSemanticsFromForm parses semantics from form values.
func ParseSemanticsFromForm(kind, urgency, necessity string) (domain.CircleSemantics, error) {
	k := domain.CircleSemanticKind(kind)
	if err := k.Validate(); err != nil {
		return domain.CircleSemantics{}, err
	}

	u := domain.UrgencyModel(urgency)
	if err := u.Validate(); err != nil {
		return domain.CircleSemantics{}, err
	}

	n := domain.NecessityLevel(necessity)
	if err := n.Validate(); err != nil {
		return domain.CircleSemantics{}, err
	}

	return domain.CircleSemantics{
		Kind:        k,
		Urgency:     u,
		Necessity:   n,
		Provenance:  domain.ProvenanceUserDeclared,
		Effect:      domain.EffectNoPower, // Always enforced
		NotesBucket: domain.NotesBucketUserSet,
	}, nil
}
