// Package mode provides system mode derivation for Phase 21.
//
// Phase 21: Unified Onboarding + Shadow Receipt Viewer
//
// CRITICAL INVARIANTS:
//   - Mode is derived, not configured - purely read-only
//   - No goroutines. No time.Now().
//   - Stdlib only.
//   - Mode derivation must be deterministic
//
// Reference: docs/ADR/ADR-0051-phase21-onboarding-modes-shadow-receipt-viewer.md
package mode

// Mode represents the current system mode.
// This is a derived value, not a configuration setting.
type Mode string

const (
	// ModeDemo indicates no real connections and stub shadow provider.
	// This is the default state for new circles or when testing.
	ModeDemo Mode = "demo"

	// ModeConnected indicates Gmail is connected but no shadow receipts exist.
	// Circle has connected real data but shadow analysis hasn't run yet.
	ModeConnected Mode = "connected"

	// ModeShadow indicates shadow receipts exist for the current period.
	// The system is actively observing and producing abstract analysis.
	ModeShadow Mode = "shadow"
)

// DisplayText returns human-readable text for the mode.
func (m Mode) DisplayText() string {
	switch m {
	case ModeDemo:
		return "Demo"
	case ModeConnected:
		return "Connected"
	case ModeShadow:
		return "Shadow"
	default:
		return "Unknown"
	}
}

// Description returns a brief description of what the mode means.
func (m Mode) Description() string {
	switch m {
	case ModeDemo:
		return "Using demonstration data"
	case ModeConnected:
		return "Connected to real sources"
	case ModeShadow:
		return "Observing quietly"
	default:
		return ""
	}
}

// IsReal returns true if the mode represents real data connection.
func (m Mode) IsReal() bool {
	return m == ModeConnected || m == ModeShadow
}

// ModeIndicator contains the mode and its display properties.
type ModeIndicator struct {
	// Mode is the derived system mode.
	Mode Mode

	// DisplayText is the human-readable mode name.
	DisplayText string

	// Description is a brief explanation.
	Description string

	// IsReal indicates if connected to real data.
	IsReal bool
}

// NewModeIndicator creates a mode indicator from a derived mode.
func NewModeIndicator(m Mode) ModeIndicator {
	return ModeIndicator{
		Mode:        m,
		DisplayText: m.DisplayText(),
		Description: m.Description(),
		IsReal:      m.IsReal(),
	}
}
