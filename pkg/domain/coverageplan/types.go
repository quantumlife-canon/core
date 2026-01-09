// Package coverageplan provides domain types for Phase 47: Pack Coverage Realization.
//
// This package connects Phase 46 Marketplace Packs to actual observer capabilities.
// Coverage realization expands OBSERVERS and SCANNERS only. It NEVER grants permission,
// NEVER changes interrupt policy, NEVER changes delivery, NEVER changes execution.
//
// Track B: Expand observers, not actions.
//
// CRITICAL: No time.Now() in this package - clock must be injected.
// CRITICAL: No goroutines in this package.
// CRITICAL: Capabilities are a fixed vocabulary - no free text.
//
// Reference: docs/ADR/ADR-0085-phase47-pack-coverage-realization.md
package coverageplan

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
)

// CoverageSourceKind represents the source of coverage data.
type CoverageSourceKind string

const (
	SourceGmail              CoverageSourceKind = "source_gmail"
	SourceFinanceTrueLayer   CoverageSourceKind = "source_finance_truelayer"
	SourceDeviceNotification CoverageSourceKind = "source_device_notification"
)

// Validate checks if the CoverageSourceKind is valid.
func (s CoverageSourceKind) Validate() error {
	switch s {
	case SourceGmail, SourceFinanceTrueLayer, SourceDeviceNotification:
		return nil
	default:
		return fmt.Errorf("invalid CoverageSourceKind: %s", s)
	}
}

// CanonicalString returns the canonical string representation.
func (s CoverageSourceKind) CanonicalString() string {
	return string(s)
}

// String returns the string representation.
func (s CoverageSourceKind) String() string {
	return string(s)
}

// AllCoverageSourceKinds returns all valid source kinds in stable order.
func AllCoverageSourceKinds() []CoverageSourceKind {
	return []CoverageSourceKind{
		SourceDeviceNotification,
		SourceFinanceTrueLayer,
		SourceGmail,
	}
}

// CoverageCapability represents a specific observer capability.
// CRITICAL: This is a fixed vocabulary - no free text allowed.
type CoverageCapability string

const (
	CapReceiptObserver        CoverageCapability = "cap_receipt_observer"
	CapCommerceObserver       CoverageCapability = "cap_commerce_observer"
	CapFinanceCommerceObserver CoverageCapability = "cap_finance_commerce_observer"
	CapPressureMap            CoverageCapability = "cap_pressure_map"
	CapTimeWindowSources      CoverageCapability = "cap_timewindow_sources"
	CapNotificationMetadata   CoverageCapability = "cap_notification_metadata"
)

// Validate checks if the CoverageCapability is valid.
func (c CoverageCapability) Validate() error {
	switch c {
	case CapReceiptObserver, CapCommerceObserver, CapFinanceCommerceObserver,
		CapPressureMap, CapTimeWindowSources, CapNotificationMetadata:
		return nil
	default:
		return fmt.Errorf("invalid CoverageCapability: %s", c)
	}
}

// CanonicalString returns the canonical string representation.
func (c CoverageCapability) CanonicalString() string {
	return string(c)
}

// String returns the string representation.
func (c CoverageCapability) String() string {
	return string(c)
}

// DisplayLabel returns a user-friendly label for UI display.
func (c CoverageCapability) DisplayLabel() string {
	switch c {
	case CapReceiptObserver:
		return "Receipt scanning"
	case CapCommerceObserver:
		return "Commerce patterns"
	case CapFinanceCommerceObserver:
		return "Finance observation"
	case CapPressureMap:
		return "Pressure mapping"
	case CapTimeWindowSources:
		return "Time-window analysis"
	case CapNotificationMetadata:
		return "Notification metadata"
	default:
		return string(c)
	}
}

// AllCoverageCapabilities returns all valid capabilities in stable order.
func AllCoverageCapabilities() []CoverageCapability {
	return []CoverageCapability{
		CapCommerceObserver,
		CapFinanceCommerceObserver,
		CapNotificationMetadata,
		CapPressureMap,
		CapReceiptObserver,
		CapTimeWindowSources,
	}
}

// CoverageChangeKind represents what happened to a capability.
type CoverageChangeKind string

const (
	ChangeAdded     CoverageChangeKind = "change_added"
	ChangeRemoved   CoverageChangeKind = "change_removed"
	ChangeUnchanged CoverageChangeKind = "change_unchanged"
)

// Validate checks if the CoverageChangeKind is valid.
func (k CoverageChangeKind) Validate() error {
	switch k {
	case ChangeAdded, ChangeRemoved, ChangeUnchanged:
		return nil
	default:
		return fmt.Errorf("invalid CoverageChangeKind: %s", k)
	}
}

// CanonicalString returns the canonical string representation.
func (k CoverageChangeKind) CanonicalString() string {
	return string(k)
}

// String returns the string representation.
func (k CoverageChangeKind) String() string {
	return string(k)
}

// CoverageProofAckKind represents acknowledgment type.
type CoverageProofAckKind string

const (
	AckViewed    CoverageProofAckKind = "ack_viewed"
	AckDismissed CoverageProofAckKind = "ack_dismissed"
)

// Validate checks if the CoverageProofAckKind is valid.
func (k CoverageProofAckKind) Validate() error {
	switch k {
	case AckViewed, AckDismissed:
		return nil
	default:
		return fmt.Errorf("invalid CoverageProofAckKind: %s", k)
	}
}

// CanonicalString returns the canonical string representation.
func (k CoverageProofAckKind) CanonicalString() string {
	return string(k)
}

// String returns the string representation.
func (k CoverageProofAckKind) String() string {
	return string(k)
}

// CoverageSourcePlan represents the capabilities enabled for a single source.
type CoverageSourcePlan struct {
	Source  CoverageSourceKind   // The coverage source
	Enabled []CoverageCapability // Capabilities enabled for this source (sorted)
}

// Validate checks if the CoverageSourcePlan is valid.
func (p CoverageSourcePlan) Validate() error {
	if err := p.Source.Validate(); err != nil {
		return err
	}
	for i, cap := range p.Enabled {
		if err := cap.Validate(); err != nil {
			return fmt.Errorf("Enabled[%d]: %w", i, err)
		}
	}
	return nil
}

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (p CoverageSourcePlan) CanonicalStringV1() string {
	sorted := NormalizeCapabilities(p.Enabled)
	caps := ""
	for i, cap := range sorted {
		if i > 0 {
			caps += ","
		}
		caps += cap.CanonicalString()
	}
	return fmt.Sprintf("%s|[%s]", p.Source.CanonicalString(), caps)
}

// CoveragePlan represents the full coverage plan for a circle.
type CoveragePlan struct {
	CircleIDHash string               // SHA256 hash of circle ID
	PeriodKey    string               // Period key (YYYY-MM-DD)
	Sources      []CoverageSourcePlan // Per-source coverage plans (sorted by source)
	Capabilities []CoverageCapability // All enabled capabilities (sorted, deduped)
	PlanHash     string               // SHA256 hash of canonical plan string
}

// Validate checks if the CoveragePlan is valid.
func (p CoveragePlan) Validate() error {
	if p.CircleIDHash == "" {
		return errors.New("CircleIDHash is required")
	}
	if p.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	for i, sp := range p.Sources {
		if err := sp.Validate(); err != nil {
			return fmt.Errorf("Sources[%d]: %w", i, err)
		}
	}
	for i, cap := range p.Capabilities {
		if err := cap.Validate(); err != nil {
			return fmt.Errorf("Capabilities[%d]: %w", i, err)
		}
	}
	return nil
}

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (p CoveragePlan) CanonicalStringV1() string {
	sourcesStr := ""
	for i, sp := range p.Sources {
		if i > 0 {
			sourcesStr += ";"
		}
		sourcesStr += sp.CanonicalStringV1()
	}
	capsStr := ""
	for i, cap := range p.Capabilities {
		if i > 0 {
			capsStr += ","
		}
		capsStr += cap.CanonicalString()
	}
	return fmt.Sprintf("%s|%s|[%s]|[%s]",
		p.CircleIDHash,
		p.PeriodKey,
		sourcesStr,
		capsStr,
	)
}

// ComputePlanHash computes the SHA256 hash of the canonical plan string.
func (p CoveragePlan) ComputePlanHash() string {
	return HashString(p.CanonicalStringV1())
}

// HasCapability checks if a capability is enabled.
func (p CoveragePlan) HasCapability(cap CoverageCapability) bool {
	for _, c := range p.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// CoverageDelta represents the difference between two coverage plans.
type CoverageDelta struct {
	Added     []CoverageCapability // Capabilities added (sorted)
	Removed   []CoverageCapability // Capabilities removed (sorted)
	Unchanged []CoverageCapability // Capabilities unchanged (sorted)
	DeltaHash string               // SHA256 hash of canonical delta string
}

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (d CoverageDelta) CanonicalStringV1() string {
	addedStr := ""
	for i, cap := range d.Added {
		if i > 0 {
			addedStr += ","
		}
		addedStr += cap.CanonicalString()
	}
	removedStr := ""
	for i, cap := range d.Removed {
		if i > 0 {
			removedStr += ","
		}
		removedStr += cap.CanonicalString()
	}
	unchangedStr := ""
	for i, cap := range d.Unchanged {
		if i > 0 {
			unchangedStr += ","
		}
		unchangedStr += cap.CanonicalString()
	}
	return fmt.Sprintf("added:[%s]|removed:[%s]|unchanged:[%s]",
		addedStr, removedStr, unchangedStr)
}

// ComputeDeltaHash computes the SHA256 hash of the canonical delta string.
func (d CoverageDelta) ComputeDeltaHash() string {
	return HashString(d.CanonicalStringV1())
}

// IsEmpty returns true if there are no changes.
func (d CoverageDelta) IsEmpty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0
}

// HasAdditions returns true if capabilities were added.
func (d CoverageDelta) HasAdditions() bool {
	return len(d.Added) > 0
}

// HasRemovals returns true if capabilities were removed.
func (d CoverageDelta) HasRemovals() bool {
	return len(d.Removed) > 0
}

// CoverageProofPage represents the UI model for coverage proof.
type CoverageProofPage struct {
	Title      string   // Page title
	Lines      []string // Calm copy lines
	Added      []string // Added capability display labels
	Removed    []string // Removed capability display labels
	StatusHash string   // SHA256 hash of page state
	PlanHash   string   // Hash of current plan
	DeltaHash  string   // Hash of delta
}

// CoverageProofCue represents the whisper cue for coverage changes.
type CoverageProofCue struct {
	Available  bool   // Whether the cue should be shown
	Text       string // Cue text (whisper style)
	Path       string // Path to proof page
	StatusHash string // Hash of cue state
}

// CoverageProofAck represents an acknowledgment of coverage proof.
type CoverageProofAck struct {
	CircleIDHash string               // SHA256 hash of circle ID
	PeriodKey    string               // Period key (YYYY-MM-DD)
	AckKind      CoverageProofAckKind // Type of acknowledgment
	StatusHash   string               // SHA256 hash of ack state
}

// Validate checks if the CoverageProofAck is valid.
func (a CoverageProofAck) Validate() error {
	if a.CircleIDHash == "" {
		return errors.New("CircleIDHash is required")
	}
	if a.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	if err := a.AckKind.Validate(); err != nil {
		return err
	}
	if a.StatusHash == "" {
		return errors.New("StatusHash is required")
	}
	return nil
}

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (a CoverageProofAck) CanonicalStringV1() string {
	return fmt.Sprintf("%s|%s|%s",
		a.CircleIDHash,
		a.PeriodKey,
		a.AckKind.CanonicalString(),
	)
}

// ComputeStatusHash computes the SHA256 hash of the canonical ack string.
func (a CoverageProofAck) ComputeStatusHash() string {
	return HashString(a.CanonicalStringV1())
}

// HashString computes SHA256 hash of a string and returns hex-encoded result.
func HashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// NormalizeCapabilities sorts and deduplicates capabilities.
func NormalizeCapabilities(caps []CoverageCapability) []CoverageCapability {
	if len(caps) == 0 {
		return []CoverageCapability{}
	}

	// Deduplicate
	seen := make(map[CoverageCapability]bool)
	result := make([]CoverageCapability, 0, len(caps))
	for _, cap := range caps {
		if !seen[cap] {
			seen[cap] = true
			result = append(result, cap)
		}
	}

	// Sort lexicographically
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})

	return result
}

// NormalizeSources sorts source plans by source kind.
func NormalizeSources(sources []CoverageSourcePlan) []CoverageSourcePlan {
	if len(sources) == 0 {
		return []CoverageSourcePlan{}
	}

	// Deep copy
	result := make([]CoverageSourcePlan, len(sources))
	for i, sp := range sources {
		result[i] = CoverageSourcePlan{
			Source:  sp.Source,
			Enabled: NormalizeCapabilities(sp.Enabled),
		}
	}

	// Sort by source kind
	sort.Slice(result, func(i, j int) bool {
		return result[i].Source < result[j].Source
	})

	return result
}

// ComputeProofStatusHash computes the status hash for a proof page.
func ComputeProofStatusHash(planHash, deltaHash string) string {
	return HashString(fmt.Sprintf("proof|%s|%s", planHash, deltaHash))
}

// ComputeCueStatusHash computes the status hash for a cue.
func ComputeCueStatusHash(planHash string, available bool) string {
	availStr := "hidden"
	if available {
		availStr = "shown"
	}
	return HashString(fmt.Sprintf("cue|%s|%s", planHash, availStr))
}

// Bounded retention constants.
const (
	MaxCoveragePlanRecords  = 200
	MaxCoveragePlanDays     = 30
	MaxCoverageProofAckRecords = 200
	MaxCoverageProofAckDays = 30
)
