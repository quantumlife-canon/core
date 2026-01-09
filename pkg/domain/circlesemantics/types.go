// Package circlesemantics provides domain types for Phase 45: Circle Semantics & Necessity Declaration.
// This is a meaning-only layer - semantics do NOT grant permission or enable actions.
package circlesemantics

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

// CircleSemanticKind represents what kind of entity a circle is.
type CircleSemanticKind string

const (
	SemanticHuman                CircleSemanticKind = "semantic_human"
	SemanticInstitution          CircleSemanticKind = "semantic_institution"
	SemanticServiceEssential     CircleSemanticKind = "semantic_service_essential"
	SemanticServiceTransactional CircleSemanticKind = "semantic_service_transactional"
	SemanticServiceOptional      CircleSemanticKind = "semantic_service_optional"
	SemanticUnknown              CircleSemanticKind = "semantic_unknown"
)

// Validate checks if the CircleSemanticKind is valid.
func (k CircleSemanticKind) Validate() error {
	switch k {
	case SemanticHuman, SemanticInstitution, SemanticServiceEssential,
		SemanticServiceTransactional, SemanticServiceOptional, SemanticUnknown:
		return nil
	default:
		return fmt.Errorf("invalid CircleSemanticKind: %s", k)
	}
}

// CanonicalString returns the canonical string representation.
func (k CircleSemanticKind) CanonicalString() string {
	return string(k)
}

// String returns the string representation.
func (k CircleSemanticKind) String() string {
	return string(k)
}

// AllCircleSemanticKinds returns all valid kinds in stable order.
func AllCircleSemanticKinds() []CircleSemanticKind {
	return []CircleSemanticKind{
		SemanticHuman,
		SemanticInstitution,
		SemanticServiceEssential,
		SemanticServiceTransactional,
		SemanticServiceOptional,
		SemanticUnknown,
	}
}

// UrgencyModel represents what urgency style is valid for a circle.
type UrgencyModel string

const (
	UrgencyNeverInterrupt UrgencyModel = "urgency_never_interrupt"
	UrgencyHardDeadline   UrgencyModel = "urgency_hard_deadline"
	UrgencyHumanWaiting   UrgencyModel = "urgency_human_waiting"
	UrgencyTimeWindow     UrgencyModel = "urgency_time_window"
	UrgencySoftReminder   UrgencyModel = "urgency_soft_reminder"
	UrgencyUnknown        UrgencyModel = "urgency_unknown"
)

// Validate checks if the UrgencyModel is valid.
func (u UrgencyModel) Validate() error {
	switch u {
	case UrgencyNeverInterrupt, UrgencyHardDeadline, UrgencyHumanWaiting,
		UrgencyTimeWindow, UrgencySoftReminder, UrgencyUnknown:
		return nil
	default:
		return fmt.Errorf("invalid UrgencyModel: %s", u)
	}
}

// CanonicalString returns the canonical string representation.
func (u UrgencyModel) CanonicalString() string {
	return string(u)
}

// String returns the string representation.
func (u UrgencyModel) String() string {
	return string(u)
}

// AllUrgencyModels returns all valid urgency models in stable order.
func AllUrgencyModels() []UrgencyModel {
	return []UrgencyModel{
		UrgencyNeverInterrupt,
		UrgencyHardDeadline,
		UrgencyHumanWaiting,
		UrgencyTimeWindow,
		UrgencySoftReminder,
		UrgencyUnknown,
	}
}

// NecessityLevel represents how essential a circle is (meaning only).
type NecessityLevel string

const (
	NecessityLow     NecessityLevel = "necessity_low"
	NecessityMedium  NecessityLevel = "necessity_medium"
	NecessityHigh    NecessityLevel = "necessity_high"
	NecessityUnknown NecessityLevel = "necessity_unknown"
)

// Validate checks if the NecessityLevel is valid.
func (n NecessityLevel) Validate() error {
	switch n {
	case NecessityLow, NecessityMedium, NecessityHigh, NecessityUnknown:
		return nil
	default:
		return fmt.Errorf("invalid NecessityLevel: %s", n)
	}
}

// CanonicalString returns the canonical string representation.
func (n NecessityLevel) CanonicalString() string {
	return string(n)
}

// String returns the string representation.
func (n NecessityLevel) String() string {
	return string(n)
}

// AllNecessityLevels returns all valid necessity levels in stable order.
func AllNecessityLevels() []NecessityLevel {
	return []NecessityLevel{
		NecessityLow,
		NecessityMedium,
		NecessityHigh,
		NecessityUnknown,
	}
}

// SemanticsProvenance represents who/what declared the semantics.
type SemanticsProvenance string

const (
	ProvenanceUserDeclared     SemanticsProvenance = "provenance_user_declared"
	ProvenanceDerivedRules     SemanticsProvenance = "provenance_derived_rules"
	ProvenanceImportedConnector SemanticsProvenance = "provenance_imported_connector"
)

// Validate checks if the SemanticsProvenance is valid.
func (p SemanticsProvenance) Validate() error {
	switch p {
	case ProvenanceUserDeclared, ProvenanceDerivedRules, ProvenanceImportedConnector:
		return nil
	default:
		return fmt.Errorf("invalid SemanticsProvenance: %s", p)
	}
}

// CanonicalString returns the canonical string representation.
func (p SemanticsProvenance) CanonicalString() string {
	return string(p)
}

// String returns the string representation.
func (p SemanticsProvenance) String() string {
	return string(p)
}

// SemanticsEffect represents what power the semantics grant.
// In Phase 45, this MUST always be EffectNoPower.
type SemanticsEffect string

const (
	// EffectNoPower is the ONLY allowed value in Phase 45.
	// Semantics provide meaning but do NOT grant permission.
	EffectNoPower SemanticsEffect = "effect_no_power"
)

// Validate checks if the SemanticsEffect is valid.
// In Phase 45, only EffectNoPower is valid.
func (e SemanticsEffect) Validate() error {
	if e == EffectNoPower {
		return nil
	}
	return fmt.Errorf("invalid SemanticsEffect: %s (only effect_no_power allowed)", e)
}

// CanonicalString returns the canonical string representation.
func (e SemanticsEffect) CanonicalString() string {
	return string(e)
}

// String returns the string representation.
func (e SemanticsEffect) String() string {
	return string(e)
}

// Allowed notes bucket values.
const (
	NotesBucketNone    = "none"
	NotesBucketUserSet = "user_set"
	NotesBucketDerived = "derived"
)

// AllowedNotesBuckets contains the valid notes bucket values.
var AllowedNotesBuckets = map[string]bool{
	NotesBucketNone:    true,
	NotesBucketUserSet: true,
	NotesBucketDerived: true,
}

// ValidateNotesBucket checks if a notes bucket value is allowed.
func ValidateNotesBucket(bucket string) error {
	if AllowedNotesBuckets[bucket] {
		return nil
	}
	return fmt.Errorf("invalid NotesBucket: %s (allowed: none, user_set, derived)", bucket)
}

// CircleSemanticsKey identifies a circle for semantics lookup.
type CircleSemanticsKey struct {
	CircleIDHash string // REQUIRED - already hashed circle identifier
}

// CanonicalString returns the canonical string representation.
func (k CircleSemanticsKey) CanonicalString() string {
	return k.CircleIDHash
}

// Validate checks if the key is valid.
func (k CircleSemanticsKey) Validate() error {
	if k.CircleIDHash == "" {
		return errors.New("CircleIDHash is required")
	}
	return nil
}

// CircleSemantics represents the semantic meaning of a circle.
type CircleSemantics struct {
	Kind        CircleSemanticKind
	Urgency     UrgencyModel
	Necessity   NecessityLevel
	Provenance  SemanticsProvenance
	Effect      SemanticsEffect // MUST be EffectNoPower always
	NotesBucket string          // allowed: "none" | "user_set" | "derived"
}

// Validate checks if the CircleSemantics is valid.
func (s CircleSemantics) Validate() error {
	if err := s.Kind.Validate(); err != nil {
		return err
	}
	if err := s.Urgency.Validate(); err != nil {
		return err
	}
	if err := s.Necessity.Validate(); err != nil {
		return err
	}
	if err := s.Provenance.Validate(); err != nil {
		return err
	}
	if err := s.Effect.Validate(); err != nil {
		return err
	}
	if s.NotesBucket != "" {
		if err := ValidateNotesBucket(s.NotesBucket); err != nil {
			return err
		}
	}
	return nil
}

// CanonicalStringV1 returns a pipe-delimited canonical string in stable field order.
func (s CircleSemantics) CanonicalStringV1() string {
	notesBucket := s.NotesBucket
	if notesBucket == "" {
		notesBucket = NotesBucketNone
	}
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		s.Kind.CanonicalString(),
		s.Urgency.CanonicalString(),
		s.Necessity.CanonicalString(),
		s.Provenance.CanonicalString(),
		s.Effect.CanonicalString(),
		notesBucket,
	)
}

// SemanticsRecord represents a stored semantics record.
type SemanticsRecord struct {
	PeriodKey    string          // daily bucket key (YYYY-MM-DD)
	CircleIDHash string          // hashed circle identifier
	SemanticsHash string         // sha256(CircleSemantics.CanonicalStringV1())
	StatusHash   string          // sha256(PeriodKey|CircleIDHash|SemanticsHash)
	Semantics    CircleSemantics // abstract-only, no names/ids
}

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (r SemanticsRecord) CanonicalStringV1() string {
	return fmt.Sprintf("%s|%s|%s|%s",
		r.PeriodKey,
		r.CircleIDHash,
		r.SemanticsHash,
		r.Semantics.CanonicalStringV1(),
	)
}

// Validate checks if the record is valid.
func (r SemanticsRecord) Validate() error {
	if r.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	if r.CircleIDHash == "" {
		return errors.New("CircleIDHash is required")
	}
	if r.SemanticsHash == "" {
		return errors.New("SemanticsHash is required")
	}
	if r.StatusHash == "" {
		return errors.New("StatusHash is required")
	}
	return r.Semantics.Validate()
}

// SemanticsChange represents a change between semantics states.
type SemanticsChange struct {
	BeforeHash string // may be empty for new records
	AfterHash  string // may be empty for cleared records
	ChangeKind string // "created"|"updated"|"cleared"|"no_change"
}

// Allowed change kinds.
const (
	ChangeKindCreated  = "created"
	ChangeKindUpdated  = "updated"
	ChangeKindCleared  = "cleared"
	ChangeKindNoChange = "no_change"
)

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (c SemanticsChange) CanonicalStringV1() string {
	return fmt.Sprintf("%s|%s|%s", c.BeforeHash, c.AfterHash, c.ChangeKind)
}

// Validate checks if the change is valid.
func (c SemanticsChange) Validate() error {
	switch c.ChangeKind {
	case ChangeKindCreated, ChangeKindUpdated, ChangeKindCleared, ChangeKindNoChange:
		return nil
	default:
		return fmt.Errorf("invalid ChangeKind: %s", c.ChangeKind)
	}
}

// SemanticsSettingsPage represents the UI model for the settings page.
type SemanticsSettingsPage struct {
	Title      string
	Lines      []string
	Items      []SemanticsSettingsItem
	StatusHash string
}

// SemanticsSettingsItem represents a single circle's settings in the UI.
type SemanticsSettingsItem struct {
	CircleIDHash     string
	Current          CircleSemantics
	AllowedKinds     []CircleSemanticKind
	AllowedUrgency   []UrgencyModel
	AllowedNecessity []NecessityLevel
}

// SemanticsProofPage represents the UI model for the proof page.
type SemanticsProofPage struct {
	Title      string
	Lines      []string
	Entries    []SemanticsProofEntry
	StatusHash string
}

// SemanticsProofEntry represents a single entry in the proof page.
type SemanticsProofEntry struct {
	CircleIDHash  string
	SemanticsHash string
	Kind          CircleSemanticKind
	Urgency       UrgencyModel
	Necessity     NecessityLevel
	Provenance    SemanticsProvenance
	Effect        SemanticsEffect
}

// SemanticsCue represents the whisper cue for semantics.
type SemanticsCue struct {
	Available  bool
	Text       string // "You can name what kind of thing this is."
	Path       string // "/settings/semantics"
	StatusHash string
}

// SemanticsProofAck represents an acknowledgment of the proof page.
type SemanticsProofAck struct {
	PeriodKey  string
	StatusHash string
	AckKind    string // "viewed"|"dismissed"
}

// Allowed ack kinds.
const (
	AckKindViewed    = "viewed"
	AckKindDismissed = "dismissed"
)

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (a SemanticsProofAck) CanonicalStringV1() string {
	return fmt.Sprintf("%s|%s|%s", a.PeriodKey, a.StatusHash, a.AckKind)
}

// Validate checks if the ack is valid.
func (a SemanticsProofAck) Validate() error {
	if a.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	if a.StatusHash == "" {
		return errors.New("StatusHash is required")
	}
	switch a.AckKind {
	case AckKindViewed, AckKindDismissed:
		return nil
	default:
		return fmt.Errorf("invalid AckKind: %s", a.AckKind)
	}
}

// HashString computes SHA256 hash of a string and returns hex-encoded result.
func HashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ComputeSemanticsHash computes the hash of a CircleSemantics.
func ComputeSemanticsHash(s CircleSemantics) string {
	return HashString(s.CanonicalStringV1())
}

// ComputeStatusHash computes the status hash for a record.
func ComputeStatusHash(periodKey, circleIDHash, semanticsHash string) string {
	canonical := fmt.Sprintf("%s|%s|%s", periodKey, circleIDHash, semanticsHash)
	return HashString(canonical)
}

// MaxDisplayEntries is the maximum number of entries to display in UI.
const MaxDisplayEntries = 25

// CircleCountBucket returns a bucketed count description.
func CircleCountBucket(count int) string {
	if count == 0 {
		return "nothing"
	}
	if count <= 3 {
		return "a_few"
	}
	return "several"
}
