// Package shadowview provides the shadow receipt viewer for Phase 21.
//
// Phase 21: Unified Onboarding + Shadow Receipt Viewer
//
// CRITICAL INVARIANTS:
//   - Shows ONLY abstract buckets and hashes - never raw content
//   - No goroutines. No time.Now().
//   - Stdlib only.
//   - Read-only projection from existing receipts
//
// Reference: docs/ADR/ADR-0051-phase21-onboarding-modes-shadow-receipt-viewer.md
package shadowview

import (
	"quantumlife/pkg/domain/shadowllm"
)

// ShadowReceiptPage contains all data needed to render the shadow receipt proof page.
//
// CRITICAL: Contains ONLY abstract buckets and hashes.
// CRITICAL: No raw content, no identifiable information.
type ShadowReceiptPage struct {
	// HasReceipt indicates if a receipt exists.
	HasReceipt bool

	// Source section
	Source SourceSection

	// Observation section
	Observation ObservationSection

	// Confidence section
	Confidence ConfidenceSection

	// Restraint section
	Restraint RestraintSection

	// Calibration section (Phase 19.4+)
	Calibration CalibrationSection

	// TrustAnchor section
	TrustAnchor TrustAnchorSection

	// ReceiptHash is the full receipt hash for dismissal tracking.
	ReceiptHash string
}

// SourceSection describes connected sources abstractly.
type SourceSection struct {
	// Statement describes what sources are connected.
	Statement string

	// IsConnected indicates if any real source is connected.
	IsConnected bool
}

// ObservationSection describes what was observed.
type ObservationSection struct {
	// Magnitude indicates how much activity (nothing/a_few/several).
	Magnitude string

	// Categories lists abstract categories observed.
	Categories []string

	// Horizon indicates urgency bucket.
	Horizon string

	// Statement is a human-readable summary.
	Statement string
}

// ConfidenceSection describes observation confidence.
type ConfidenceSection struct {
	// Bucket is the confidence level (low/medium/high).
	Bucket string

	// Statement confirms observation-only nature.
	Statement string
}

// RestraintSection lists what was NOT done.
type RestraintSection struct {
	// NoActionsTaken indicates no actions were executed.
	NoActionsTaken bool

	// NoDraftsCreated indicates no drafts were generated.
	NoDraftsCreated bool

	// NoNotificationsSent indicates no notifications were sent.
	NoNotificationsSent bool

	// NoRulesPromoted indicates no rules were promoted to production.
	NoRulesPromoted bool

	// Statements are the explicit negatives.
	Statements []string
}

// CalibrationSection shows calibration status (Phase 19.4+).
type CalibrationSection struct {
	// HasCalibration indicates if calibration data exists.
	HasCalibration bool

	// AgreementBucket indicates agreement level (match/partial/conflict/none).
	AgreementBucket string

	// VoteUsefulness indicates if vote was recorded.
	VoteUsefulness string

	// Statement describes calibration status.
	Statement string
}

// TrustAnchorSection provides the proof hash.
type TrustAnchorSection struct {
	// PeriodLabel is the abstract time period (e.g., "today").
	PeriodLabel string

	// ReceiptHash is the SHA256 hash of the receipt.
	ReceiptHash string

	// Statement confirms append-only nature.
	Statement string
}

// Magnitude bucket display values.
const (
	MagnitudeDisplayNothing = "nothing"
	MagnitudeDisplayAFew    = "a few"
	MagnitudeDisplaySeveral = "several"
)

// HorizonDisplayText returns human-readable horizon.
func HorizonDisplayText(h shadowllm.Horizon) string {
	switch h {
	case shadowllm.HorizonNow:
		return "now"
	case shadowllm.HorizonSoon:
		return "soon"
	case shadowllm.HorizonLater:
		return "later"
	case shadowllm.HorizonSomeday:
		return "someday"
	default:
		return "unknown"
	}
}

// MagnitudeDisplayText returns human-readable magnitude.
func MagnitudeDisplayText(m shadowllm.MagnitudeBucket) string {
	switch m {
	case shadowllm.MagnitudeNothing:
		return MagnitudeDisplayNothing
	case shadowllm.MagnitudeAFew:
		return MagnitudeDisplayAFew
	case shadowllm.MagnitudeSeveral:
		return MagnitudeDisplaySeveral
	default:
		return "unknown"
	}
}

// ConfidenceDisplayText returns human-readable confidence.
func ConfidenceDisplayText(c shadowllm.ConfidenceBucket) string {
	switch c {
	case shadowllm.ConfidenceLow:
		return "low"
	case shadowllm.ConfidenceMed:
		return "medium"
	case shadowllm.ConfidenceHigh:
		return "high"
	default:
		return "unknown"
	}
}

// CategoryDisplayText returns human-readable category.
func CategoryDisplayText(c shadowllm.AbstractCategory) string {
	switch c {
	case shadowllm.CategoryMoney:
		return "money"
	case shadowllm.CategoryTime:
		return "time"
	case shadowllm.CategoryPeople:
		return "people"
	case shadowllm.CategoryWork:
		return "work"
	case shadowllm.CategoryHome:
		return "home"
	case shadowllm.CategoryHealth:
		return "health"
	case shadowllm.CategoryFamily:
		return "family"
	case shadowllm.CategorySchool:
		return "school"
	default:
		return "unknown"
	}
}
