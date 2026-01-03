// Package invitation provides the Gentle Action Invitation engine.
//
// Phase 23: Gentle Action Invitation (Trust-Preserving)
//
// This engine decides if an invitation is eligible and selects exactly
// one invitation kind based on abstract inputs.
//
// CRITICAL INVARIANTS:
//   - Max one invitation per period
//   - Not shown unless trust baseline exists
//   - Not shown if dismissed this period
//   - Never auto-execute
//   - Never create urgency
//   - No goroutines. No time.Now() - clock injection only.
//   - Stdlib only.
//
// Reference: docs/ADR/ADR-0053-phase23-gentle-invitation.md
package invitation

import (
	"time"

	"quantumlife/pkg/domain/invitation"
)

// Engine computes gentle action invitations.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new invitation engine.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{clock: clock}
}

// TrustInputs contains abstract trust-related inputs.
// CRITICAL: No identifiers, no raw data.
type TrustInputs struct {
	// HasQuietMirrorSummary indicates a mirror summary exists.
	HasQuietMirrorSummary bool

	// QuietMirrorMagnitude is the abstract magnitude from mirror.
	QuietMirrorMagnitude string

	// HasHeldSummary indicates held items exist.
	HasHeldSummary bool

	// HeldMagnitude is the abstract magnitude of held items.
	HeldMagnitude string

	// HasTrustAccrual indicates trust has been accrued.
	HasTrustAccrual bool

	// TrustScore is an abstract trust level (0-1).
	TrustScore float64

	// HasShadowReceipt indicates shadow observation exists.
	HasShadowReceipt bool
}

// ComputeEligibility builds an eligibility check from abstract inputs.
func (e *Engine) ComputeEligibility(
	circleID string,
	hasGmailConnection bool,
	hasSyncReceipt bool,
	trustInputs *TrustInputs,
	dismissedThisPeriod bool,
	acceptedThisPeriod bool,
) *invitation.InvitationEligibility {
	now := e.clock()
	period := invitation.NewInvitationPeriod(now.Format("2006-01-02"))

	eligibility := &invitation.InvitationEligibility{
		CircleID:            circleID,
		HasGmailConnection:  hasGmailConnection,
		HasSyncReceipt:      hasSyncReceipt,
		Period:              period,
		DismissedThisPeriod: dismissedThisPeriod,
		AcceptedThisPeriod:  acceptedThisPeriod,
	}

	if trustInputs != nil {
		eligibility.HasQuietMirrorViewed = trustInputs.HasQuietMirrorSummary
		eligibility.HasTrustBaseline = trustInputs.HasTrustAccrual && trustInputs.TrustScore > 0
		eligibility.HasShadowReceipt = trustInputs.HasShadowReceipt
		eligibility.HeldMagnitude = trustInputs.HeldMagnitude
	}

	return eligibility
}

// Compute produces an InvitationSummary if eligible, nil otherwise.
func (e *Engine) Compute(eligibility *invitation.InvitationEligibility) *invitation.InvitationSummary {
	if eligibility == nil {
		return nil
	}

	if !eligibility.IsEligible() {
		return nil
	}

	// Select invitation kind based on held magnitude
	kind := e.selectKind(eligibility)

	return &invitation.InvitationSummary{
		CircleID:   eligibility.CircleID,
		Period:     eligibility.Period,
		Kind:       kind,
		Text:       kind.DisplayText(),
		WhisperCue: "If you ever want, you can decide what should happen next.",
		SourceHash: eligibility.Hash(),
	}
}

// selectKind chooses the invitation kind based on context.
// This is deterministic - same inputs produce same kind.
func (e *Engine) selectKind(eligibility *invitation.InvitationEligibility) invitation.InvitationKind {
	// If there are held items, offer to keep holding
	if eligibility.HeldMagnitude != "" && eligibility.HeldMagnitude != "nothing" {
		return invitation.KindHoldContinue
	}

	// If shadow observations exist, offer review
	if eligibility.HasShadowReceipt {
		return invitation.KindReviewOnce
	}

	// Default: offer notification preference
	return invitation.KindNotifyNextTime
}

// BuildPage creates the UI page data from a summary.
func (e *Engine) BuildPage(summary *invitation.InvitationSummary) *invitation.InvitationPage {
	if summary == nil {
		return invitation.NewEmptyPage()
	}
	return invitation.NewInvitationPage(summary)
}

// BuildWhisperCue creates the optional whisper cue for /today.
// CRITICAL: Only one cue, dismissable, never pushed.
type WhisperCue struct {
	// Show indicates if the whisper should be shown.
	Show bool

	// Text is the whisper text.
	Text string

	// Link is the destination URL.
	Link string
}

// BuildWhisperCue creates a whisper cue if eligible.
func (e *Engine) BuildWhisperCue(summary *invitation.InvitationSummary) *WhisperCue {
	if summary == nil {
		return &WhisperCue{Show: false}
	}

	return &WhisperCue{
		Show: true,
		Text: summary.WhisperCue,
		Link: "/invite",
	}
}

// CurrentPeriod returns the current invitation period.
func (e *Engine) CurrentPeriod() invitation.InvitationPeriod {
	now := e.clock()
	return invitation.NewInvitationPeriod(now.Format("2006-01-02"))
}
