// Package view - daily view module.
//
// DailyView represents the daily "home truth" for the user.
// It aggregates obligations and determines NeedsYou state.
//
// CRITICAL: Deterministic computation. Same inputs = same output.
// CRITICAL: Uses canonical string hashing (NOT JSON).
//
// Reference: docs/ADR/ADR-0019-phase2-obligation-extraction.md
package view

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/obligation"
)

// DailyView represents the daily state for a user.
// This is the "home truth" that determines NeedsYou.
type DailyView struct {
	// DateKey is the UTC date (YYYY-MM-DD)
	DateKey string

	// ComputedAt is when this view was computed
	ComputedAt time.Time

	// Circles summarizes each circle
	Circles map[identity.EntityID]*CircleDailySummary

	// Obligations is the sorted list of all obligations
	Obligations []*obligation.Obligation

	// NeedsYou is the headline truth
	NeedsYou bool

	// NeedsYouReasons explains why NeedsYou is true (max 3)
	NeedsYouReasons []string

	// Hash is the deterministic hash of this view
	Hash string

	// ObligationsHash is the hash of just obligations
	ObligationsHash string
}

// CircleDailySummary summarizes a circle for the daily view.
type CircleDailySummary struct {
	CircleID   identity.EntityID
	CircleName string

	// Counts
	UnreadEmails      int
	UpcomingEvents    int
	PendingTx         int
	ObligationCount   int
	HighRegretCount   int // Obligations with regret >= threshold
	TodayHorizonCount int // Obligations due today

	// Top reasons (max 3)
	TopReasons []string

	// Next due item
	NextDueBy *time.Time
}

// NeedsYouConfig configures the NeedsYou computation.
type NeedsYouConfig struct {
	// RegretThreshold is the minimum regret score to trigger NeedsYou
	RegretThreshold float64

	// AttentionHorizons are horizons that trigger NeedsYou
	AttentionHorizons []obligation.AttentionHorizon

	// MaxReasons is max reasons to show
	MaxReasons int
}

// DefaultNeedsYouConfig returns sensible defaults.
func DefaultNeedsYouConfig() NeedsYouConfig {
	return NeedsYouConfig{
		RegretThreshold: 0.5,
		AttentionHorizons: []obligation.AttentionHorizon{
			obligation.HorizonToday,
			obligation.Horizon24h,
		},
		MaxReasons: 3,
	}
}

// DailyViewBuilder constructs a DailyView.
type DailyViewBuilder struct {
	dateKey     string
	computedAt  time.Time
	circles     map[identity.EntityID]*CircleDailySummary
	obligations []*obligation.Obligation
	config      NeedsYouConfig
}

// NewDailyViewBuilder creates a new builder.
func NewDailyViewBuilder(computedAt time.Time, config NeedsYouConfig) *DailyViewBuilder {
	return &DailyViewBuilder{
		dateKey:    computedAt.UTC().Format("2006-01-02"),
		computedAt: computedAt,
		circles:    make(map[identity.EntityID]*CircleDailySummary),
		config:     config,
	}
}

// AddCircle adds or updates a circle summary.
func (b *DailyViewBuilder) AddCircle(circleID identity.EntityID, circleName string) *CircleDailySummary {
	summary, exists := b.circles[circleID]
	if !exists {
		summary = &CircleDailySummary{
			CircleID:   circleID,
			CircleName: circleName,
		}
		b.circles[circleID] = summary
	}
	return summary
}

// SetObligations sets the obligations (will be sorted).
func (b *DailyViewBuilder) SetObligations(obligs []*obligation.Obligation) {
	b.obligations = obligs
}

// Build creates the DailyView with computed NeedsYou and hash.
func (b *DailyViewBuilder) Build() *DailyView {
	// Sort obligations deterministically
	obligation.SortObligations(b.obligations)

	// Compute per-circle summaries
	for _, oblig := range b.obligations {
		summary := b.circles[oblig.CircleID]
		if summary == nil {
			continue
		}

		summary.ObligationCount++

		if oblig.RegretScore >= b.config.RegretThreshold {
			summary.HighRegretCount++
		}

		if oblig.Horizon == obligation.HorizonToday || oblig.Horizon == obligation.Horizon24h {
			summary.TodayHorizonCount++
		}

		// Track next due
		if oblig.DueBy != nil {
			if summary.NextDueBy == nil || oblig.DueBy.Before(*summary.NextDueBy) {
				summary.NextDueBy = oblig.DueBy
			}
		}

		// Add to top reasons (max 3)
		if len(summary.TopReasons) < 3 && oblig.Reason != "" {
			summary.TopReasons = append(summary.TopReasons, oblig.Reason)
		}
	}

	// Compute NeedsYou
	needsYou, reasons := b.computeNeedsYou()

	// Compute hashes
	obligHash := obligation.ComputeObligationsHash(b.obligations)
	viewHash := b.computeHash(needsYou, obligHash)

	return &DailyView{
		DateKey:         b.dateKey,
		ComputedAt:      b.computedAt,
		Circles:         b.circles,
		Obligations:     b.obligations,
		NeedsYou:        needsYou,
		NeedsYouReasons: reasons,
		Hash:            viewHash,
		ObligationsHash: obligHash,
	}
}

// computeNeedsYou determines if NeedsYou is true.
func (b *DailyViewBuilder) computeNeedsYou() (bool, []string) {
	var reasons []string

	// Check for high-regret obligations within attention horizons
	horizonSet := make(map[obligation.AttentionHorizon]bool)
	for _, h := range b.config.AttentionHorizons {
		horizonSet[h] = true
	}

	for _, oblig := range b.obligations {
		// Must be in attention horizon
		if !horizonSet[oblig.Horizon] {
			continue
		}

		// Must meet regret threshold
		if oblig.RegretScore < b.config.RegretThreshold {
			continue
		}

		// This obligation triggers NeedsYou
		if len(reasons) < b.config.MaxReasons {
			reasons = append(reasons, oblig.Reason)
		}
	}

	needsYou := len(reasons) > 0
	return needsYou, reasons
}

// computeHash generates deterministic hash using canonical strings.
func (b *DailyViewBuilder) computeHash(needsYou bool, obligHash string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("date:%s", b.dateKey))
	parts = append(parts, fmt.Sprintf("computed:%d", b.computedAt.Unix()))
	parts = append(parts, fmt.Sprintf("needs_you:%t", needsYou))
	parts = append(parts, fmt.Sprintf("oblighash:%s", obligHash))

	// Circle summaries in sorted order
	circleIDs := make([]identity.EntityID, 0, len(b.circles))
	for id := range b.circles {
		circleIDs = append(circleIDs, id)
	}
	sort.Slice(circleIDs, func(i, j int) bool {
		return circleIDs[i] < circleIDs[j]
	})

	for _, id := range circleIDs {
		summary := b.circles[id]
		parts = append(parts, fmt.Sprintf("circle:%s:%s:%d:%d",
			id, summary.CircleName, summary.ObligationCount, summary.HighRegretCount))
	}

	canonical := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// NothingNeedsYou returns true if NeedsYou is false.
func (d *DailyView) NothingNeedsYou() bool {
	return !d.NeedsYou
}

// GetCircleSummary returns the summary for a circle.
func (d *DailyView) GetCircleSummary(circleID identity.EntityID) *CircleDailySummary {
	return d.Circles[circleID]
}

// GetObligationsByCircle returns obligations for a specific circle.
func (d *DailyView) GetObligationsByCircle(circleID identity.EntityID) []*obligation.Obligation {
	return obligation.FilterByCircle(d.Obligations, circleID)
}

// GetHighRegretObligations returns obligations with regret >= threshold.
func (d *DailyView) GetHighRegretObligations(threshold float64) []*obligation.Obligation {
	return obligation.FilterByMinRegret(d.Obligations, threshold)
}

// GetTodayObligations returns obligations due today or within 24h.
func (d *DailyView) GetTodayObligations() []*obligation.Obligation {
	return obligation.FilterByHorizon(d.Obligations,
		obligation.HorizonToday, obligation.Horizon24h)
}

// CanonicalString returns the canonical string representation.
func (d *DailyView) CanonicalString() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("daily_view:%s", d.DateKey))
	parts = append(parts, fmt.Sprintf("computed:%d", d.ComputedAt.Unix()))
	parts = append(parts, fmt.Sprintf("needs_you:%t", d.NeedsYou))
	parts = append(parts, fmt.Sprintf("obligation_count:%d", len(d.Obligations)))
	parts = append(parts, fmt.Sprintf("circle_count:%d", len(d.Circles)))
	parts = append(parts, fmt.Sprintf("hash:%s", d.Hash))

	return strings.Join(parts, "|")
}
