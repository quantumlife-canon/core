// Package coverageplan provides the Phase 47: Pack Coverage Realization engine.
//
// This package connects Phase 46 Marketplace Packs to actual observer capabilities.
// Coverage realization expands OBSERVERS and SCANNERS only. It NEVER grants permission,
// NEVER changes interrupt policy, NEVER changes delivery, NEVER changes execution.
//
// Track B: Expand observers, not actions.
//
// CRITICAL: No time.Now() in this package - clock is injected.
// CRITICAL: No goroutines in this package.
// CRITICAL: All operations are pure and deterministic.
//
// Reference: docs/ADR/ADR-0085-phase47-pack-coverage-realization.md
package coverageplan

import (
	"sort"

	domain "quantumlife/pkg/domain/coverageplan"
	marketplace "quantumlife/pkg/domain/marketplace"
)

// Clock is a function that returns the current period key (YYYY-MM-DD).
type Clock func() string

// PackProvider provides installed packs for a circle.
type PackProvider interface {
	// InstalledPacks returns all installed packs for a circle.
	InstalledPacks(circleIDHash string) []marketplace.PackInstallRecord
}

// ExistingCoverageProvider provides the last coverage plan for a circle.
type ExistingCoverageProvider interface {
	// LastPlan returns the last coverage plan for a circle, if any.
	LastPlan(circleIDHash string) (domain.CoveragePlan, bool)
}

// Engine provides pure, deterministic coverage plan operations.
// All methods are side-effect free.
type Engine struct {
	clock Clock
}

// NewEngine creates a new coverage plan engine with injected clock.
func NewEngine(clock Clock) *Engine {
	return &Engine{
		clock: clock,
	}
}

// PackCapabilityMapping defines which pack slugs enable which capabilities.
// This is a static, hardcoded allowlist.
type PackCapabilityMapping struct {
	PackSlug     string
	Source       domain.CoverageSourceKind // Empty if not source-specific
	Capabilities []domain.CoverageCapability
}

// packMappings is the static pack-to-capability mapping.
// Unknown pack slugs are ignored.
var packMappings = []PackCapabilityMapping{
	{
		PackSlug:     "core-gmail-receipts",
		Source:       domain.SourceGmail,
		Capabilities: []domain.CoverageCapability{domain.CapReceiptObserver, domain.CapCommerceObserver},
	},
	{
		PackSlug:     "core-finance-commerce",
		Source:       domain.SourceFinanceTrueLayer,
		Capabilities: []domain.CoverageCapability{domain.CapFinanceCommerceObserver, domain.CapCommerceObserver},
	},
	{
		PackSlug:     "core-pressure",
		Source:       "", // Not source-specific
		Capabilities: []domain.CoverageCapability{domain.CapPressureMap, domain.CapTimeWindowSources},
	},
	{
		PackSlug:     "core-device-hints",
		Source:       domain.SourceDeviceNotification,
		Capabilities: []domain.CoverageCapability{domain.CapNotificationMetadata},
	},
	// Phase 46 default packs that map to capabilities
	{
		PackSlug:     "commerce-observer",
		Source:       domain.SourceGmail,
		Capabilities: []domain.CoverageCapability{domain.CapCommerceObserver},
	},
	{
		PackSlug:     "inbox-enrichment",
		Source:       domain.SourceGmail,
		Capabilities: []domain.CoverageCapability{domain.CapReceiptObserver, domain.CapCommerceObserver},
	},
	{
		PackSlug:     "calendar-awareness",
		Source:       "", // Not source-specific
		Capabilities: []domain.CoverageCapability{domain.CapTimeWindowSources},
	},
}

// packSlugFromHash looks up a pack slug from its hash.
// Returns empty string if not found.
func packSlugFromHash(slugHash string) string {
	// Check against known pack slugs
	knownSlugs := []string{
		"core-gmail-receipts",
		"core-finance-commerce",
		"core-pressure",
		"core-device-hints",
		"family-friends",
		"essential-services",
		"quiet-marketing",
		"commerce-observer",
		"calendar-awareness",
		"inbox-enrichment",
	}

	for _, slug := range knownSlugs {
		if marketplace.HashString(slug) == slugHash {
			return slug
		}
	}
	return ""
}

// getMappingForSlug returns the mapping for a pack slug, or nil if unknown.
func getMappingForSlug(slug string) *PackCapabilityMapping {
	for i := range packMappings {
		if packMappings[i].PackSlug == slug {
			return &packMappings[i]
		}
	}
	return nil
}

// BuildPlan builds a coverage plan from installed packs.
// Unknown pack slugs are ignored (not errors).
// Commerce capability NEVER implies interrupts - just observation.
func (e *Engine) BuildPlan(circleIDHash string, installed []marketplace.PackInstallRecord) domain.CoveragePlan {
	periodKey := e.clock()

	// Collect capabilities by source
	sourceMap := make(map[domain.CoverageSourceKind]map[domain.CoverageCapability]bool)
	allCaps := make(map[domain.CoverageCapability]bool)

	for _, install := range installed {
		if install.Status != marketplace.PackStatusInstalled {
			continue
		}

		// Look up the pack slug from hash
		slug := packSlugFromHash(install.PackSlugHash)
		if slug == "" {
			continue // Unknown pack, ignore
		}

		// Find the capability mapping
		mapping := getMappingForSlug(slug)
		if mapping == nil {
			continue // No mapping for this pack, ignore
		}

		// Add capabilities
		for _, cap := range mapping.Capabilities {
			allCaps[cap] = true

			if mapping.Source != "" {
				if sourceMap[mapping.Source] == nil {
					sourceMap[mapping.Source] = make(map[domain.CoverageCapability]bool)
				}
				sourceMap[mapping.Source][cap] = true
			}
		}
	}

	// Build source plans
	sources := make([]domain.CoverageSourcePlan, 0)
	for source, caps := range sourceMap {
		capList := make([]domain.CoverageCapability, 0, len(caps))
		for cap := range caps {
			capList = append(capList, cap)
		}
		sources = append(sources, domain.CoverageSourcePlan{
			Source:  source,
			Enabled: domain.NormalizeCapabilities(capList),
		})
	}
	sources = domain.NormalizeSources(sources)

	// Build all capabilities list
	capList := make([]domain.CoverageCapability, 0, len(allCaps))
	for cap := range allCaps {
		capList = append(capList, cap)
	}
	capList = domain.NormalizeCapabilities(capList)

	plan := domain.CoveragePlan{
		CircleIDHash: circleIDHash,
		PeriodKey:    periodKey,
		Sources:      sources,
		Capabilities: capList,
	}
	plan.PlanHash = plan.ComputePlanHash()

	return plan
}

// DiffPlans computes the delta between two coverage plans.
func (e *Engine) DiffPlans(prev, next domain.CoveragePlan) domain.CoverageDelta {
	prevCaps := make(map[domain.CoverageCapability]bool)
	for _, cap := range prev.Capabilities {
		prevCaps[cap] = true
	}

	nextCaps := make(map[domain.CoverageCapability]bool)
	for _, cap := range next.Capabilities {
		nextCaps[cap] = true
	}

	added := make([]domain.CoverageCapability, 0)
	removed := make([]domain.CoverageCapability, 0)
	unchanged := make([]domain.CoverageCapability, 0)

	// Find added and unchanged
	for cap := range nextCaps {
		if prevCaps[cap] {
			unchanged = append(unchanged, cap)
		} else {
			added = append(added, cap)
		}
	}

	// Find removed
	for cap := range prevCaps {
		if !nextCaps[cap] {
			removed = append(removed, cap)
		}
	}

	delta := domain.CoverageDelta{
		Added:     domain.NormalizeCapabilities(added),
		Removed:   domain.NormalizeCapabilities(removed),
		Unchanged: domain.NormalizeCapabilities(unchanged),
	}
	delta.DeltaHash = delta.ComputeDeltaHash()

	return delta
}

// BuildProofPage builds the coverage proof page.
// CRITICAL: Never shows pack IDs - only capability labels.
func (e *Engine) BuildProofPage(circleIDHash string, prev *domain.CoveragePlan, next domain.CoveragePlan, delta domain.CoverageDelta) domain.CoverageProofPage {
	// Build calm copy
	var lines []string
	if delta.IsEmpty() {
		lines = []string{"Nothing changed."}
	} else if delta.HasAdditions() && !delta.HasRemovals() {
		lines = []string{"Coverage widened quietly."}
	} else if !delta.HasAdditions() && delta.HasRemovals() {
		lines = []string{"Some coverage was narrowed."}
	} else {
		lines = []string{"Coverage was adjusted."}
	}

	// Build display labels for added/removed
	added := make([]string, len(delta.Added))
	for i, cap := range delta.Added {
		added[i] = cap.DisplayLabel()
	}
	sort.Strings(added)

	removed := make([]string, len(delta.Removed))
	for i, cap := range delta.Removed {
		removed[i] = cap.DisplayLabel()
	}
	sort.Strings(removed)

	statusHash := domain.ComputeProofStatusHash(next.PlanHash, delta.DeltaHash)

	return domain.CoverageProofPage{
		Title:      "Coverage Proof",
		Lines:      lines,
		Added:      added,
		Removed:    removed,
		StatusHash: statusHash,
		PlanHash:   next.PlanHash,
		DeltaHash:  delta.DeltaHash,
	}
}

// BuildCue builds the coverage proof cue.
// Cue appears only if delta.Added is non-empty AND not acked.
func (e *Engine) BuildCue(page domain.CoverageProofPage, acked bool) domain.CoverageProofCue {
	// Cue available if there are additions and not acked
	available := len(page.Added) > 0 && !acked

	text := ""
	if available {
		text = "A new lens was added - quietly."
	}

	statusHash := domain.ComputeCueStatusHash(page.PlanHash, available)

	return domain.CoverageProofCue{
		Available:  available,
		Text:       text,
		Path:       "/proof/coverage",
		StatusHash: statusHash,
	}
}

// ShouldShowCue determines if the coverage cue should be shown.
// This integrates with the single-whisper priority chain at LOW priority.
// Priority order (highest to lowest):
//   1. Shadow receipt cue
//   2. Reality check cue
//   3. First minutes cue
//   4. ... other higher-priority cues ...
//   5. Coverage cue (LOW priority)
//
// Returns true if coverage cue should be shown given other cue states.
func (e *Engine) ShouldShowCue(
	coverageCueAvailable bool,
	shadowReceiptCueShown bool,
	realityCueShown bool,
	firstMinutesCueShown bool,
	marketplaceCueShown bool,
) bool {
	// Don't show if coverage cue is not available
	if !coverageCueAvailable {
		return false
	}

	// Don't show if higher priority cues are shown
	if shadowReceiptCueShown {
		return false
	}
	if realityCueShown {
		return false
	}
	if firstMinutesCueShown {
		return false
	}
	if marketplaceCueShown {
		return false
	}

	return true
}

// BuildAck builds a coverage proof acknowledgment.
func (e *Engine) BuildAck(circleIDHash, periodKey string, kind domain.CoverageProofAckKind) domain.CoverageProofAck {
	ack := domain.CoverageProofAck{
		CircleIDHash: circleIDHash,
		PeriodKey:    periodKey,
		AckKind:      kind,
	}
	ack.StatusHash = ack.ComputeStatusHash()
	return ack
}

// EmptyPlan returns an empty coverage plan for a circle.
func (e *Engine) EmptyPlan(circleIDHash string) domain.CoveragePlan {
	periodKey := e.clock()
	plan := domain.CoveragePlan{
		CircleIDHash: circleIDHash,
		PeriodKey:    periodKey,
		Sources:      []domain.CoverageSourcePlan{},
		Capabilities: []domain.CoverageCapability{},
	}
	plan.PlanHash = plan.ComputePlanHash()
	return plan
}

// GetCapabilitiesForSource returns the enabled capabilities for a source.
func (e *Engine) GetCapabilitiesForSource(plan domain.CoveragePlan, source domain.CoverageSourceKind) []domain.CoverageCapability {
	for _, sp := range plan.Sources {
		if sp.Source == source {
			return sp.Enabled
		}
	}
	return []domain.CoverageCapability{}
}

// IsCapabilityEnabled checks if a specific capability is enabled in a plan.
func (e *Engine) IsCapabilityEnabled(plan domain.CoveragePlan, cap domain.CoverageCapability) bool {
	return plan.HasCapability(cap)
}
