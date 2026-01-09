package marketplace

import (
	"fmt"
	"sort"

	domain "quantumlife/pkg/domain/marketplace"
)

// Engine provides pure deterministic logic for marketplace operations.
// All methods are deterministic given the same inputs.
// No time.Now() calls - clock is injected.
// No goroutines - single-threaded execution.
type Engine struct {
	clock func() string // Returns period key (YYYY-MM-DD)
}

// NewEngine creates a new marketplace engine with injected clock.
func NewEngine(clock func() string) *Engine {
	return &Engine{clock: clock}
}

// MarketplaceInputs contains the inputs for building marketplace pages.
type MarketplaceInputs struct {
	AvailablePacks  []domain.PackTemplate
	InstalledPacks  []domain.PackInstallRecord
	RemovedPacks    []domain.PackRemovalRecord
}

// BuildHomePage builds the marketplace home page model.
func (e *Engine) BuildHomePage(inputs MarketplaceInputs) domain.MarketplaceHomePage {
	// Build installed slugs map for quick lookup
	installedSlugs := make(map[string]domain.PackInstallRecord)
	for _, rec := range inputs.InstalledPacks {
		if rec.Status == domain.PackStatusInstalled {
			installedSlugs[rec.PackSlugHash] = rec
		}
	}

	// Build available cards
	availableCards := make([]domain.PackCard, 0)
	installedCards := make([]domain.PackCard, 0)

	for _, pack := range inputs.AvailablePacks {
		slugHash := domain.HashString(pack.Slug.CanonicalString())
		card := domain.PackCard{
			SlugHash:    slugHash,
			Title:       pack.Title,
			Description: pack.Description,
			Kind:        pack.Kind,
			Tier:        pack.Tier,
			VersionHash: pack.VersionHash,
			Effect:      domain.EffectNoPower, // CRITICAL: Always enforced
		}

		if _, installed := installedSlugs[slugHash]; installed {
			card.Status = domain.PackStatusInstalled
			installedCards = append(installedCards, card)
		} else {
			card.Status = domain.PackStatusAvailable
			availableCards = append(availableCards, card)
		}
	}

	// Sort for stable output
	sort.Slice(availableCards, func(i, j int) bool {
		return availableCards[i].SlugHash < availableCards[j].SlugHash
	})
	sort.Slice(installedCards, func(i, j int) bool {
		return installedCards[i].SlugHash < installedCards[j].SlugHash
	})

	// Limit display
	if len(availableCards) > domain.MaxDisplayPacks {
		availableCards = availableCards[:domain.MaxDisplayPacks]
	}
	if len(installedCards) > domain.MaxDisplayPacks {
		installedCards = installedCards[:domain.MaxDisplayPacks]
	}

	// Build lines
	lines := []string{
		"Browse packs to customize circle semantics and observer intents.",
		"Packs provide meaning only - they do not grant permission.",
	}

	// Compute status hash
	statusHash := e.computeHomePageHash(availableCards, installedCards)

	return domain.MarketplaceHomePage{
		Title:          "Marketplace",
		Lines:          lines,
		AvailablePacks: availableCards,
		InstalledPacks: installedCards,
		StatusHash:     statusHash,
	}
}

// BuildDetailPage builds the detail page for a specific pack.
func (e *Engine) BuildDetailPage(pack domain.PackTemplate, installed bool) domain.PackDetailPage {
	slugHash := domain.HashString(pack.Slug.CanonicalString())

	card := domain.PackCard{
		SlugHash:    slugHash,
		Title:       pack.Title,
		Description: pack.Description,
		Kind:        pack.Kind,
		Tier:        pack.Tier,
		VersionHash: pack.VersionHash,
		Effect:      domain.EffectNoPower,
	}

	if installed {
		card.Status = domain.PackStatusInstalled
	} else {
		card.Status = domain.PackStatusAvailable
	}

	// Build preset displays
	presets := make([]domain.SemanticsPresetDisplay, 0, len(pack.SemanticsPresets))
	for _, p := range pack.SemanticsPresets {
		presets = append(presets, domain.SemanticsPresetDisplay{
			PatternHash:    p.CirclePatternHash,
			SemanticKind:   p.SemanticKind,
			UrgencyModel:   p.UrgencyModel,
			NecessityLevel: p.NecessityLevel,
		})
	}

	// Build binding displays
	bindings := make([]domain.ObserverBindingDisplay, 0, len(pack.ObserverBindings))
	for _, b := range pack.ObserverBindings {
		bindings = append(bindings, domain.ObserverBindingDisplay{
			PatternHash:  b.CirclePatternHash,
			ObserverSlug: b.ObserverSlug,
			BindingKind:  b.BindingKind,
			Effect:       domain.EffectNoPower, // CRITICAL: Always enforced
		})
	}

	lines := []string{
		fmt.Sprintf("Pack: %s", pack.Title),
		pack.Description,
		"This pack provides meaning only - it does not grant permission.",
	}

	statusHash := domain.HashString(fmt.Sprintf("%s|%s|%v", slugHash, pack.VersionHash, installed))

	return domain.PackDetailPage{
		Title:            pack.Title,
		Lines:            lines,
		Pack:             card,
		SemanticsPresets: presets,
		ObserverBindings: bindings,
		CanInstall:       !installed,
		CanRemove:        installed,
		StatusHash:       statusHash,
	}
}

// BuildInstallIntent creates an install intent for a pack.
// CRITICAL: Effect is ALWAYS EffectNoPower.
func (e *Engine) BuildInstallIntent(pack domain.PackTemplate) domain.PackInstallIntent {
	return domain.PackInstallIntent{
		PackSlugHash: domain.HashString(pack.Slug.CanonicalString()),
		VersionHash:  pack.VersionHash,
		Effect:       domain.EffectNoPower, // CRITICAL: Always enforced
	}
}

// ApplyInstallIntent applies an install intent and returns a record.
// CRITICAL: Effect is ALWAYS EffectNoPower.
func (e *Engine) ApplyInstallIntent(intent domain.PackInstallIntent) domain.PackInstallRecord {
	periodKey := e.clock()

	return domain.PackInstallRecord{
		PeriodKey:    periodKey,
		PackSlugHash: intent.PackSlugHash,
		VersionHash:  intent.VersionHash,
		StatusHash:   domain.ComputeStatusHash(periodKey, intent.PackSlugHash, intent.VersionHash),
		Status:       domain.PackStatusInstalled,
		Effect:       domain.EffectNoPower, // CRITICAL: Always enforced
	}
}

// BuildRemovalRecord creates a removal record for a pack.
func (e *Engine) BuildRemovalRecord(packSlugHash, versionHash string) domain.PackRemovalRecord {
	periodKey := e.clock()

	return domain.PackRemovalRecord{
		PeriodKey:    periodKey,
		PackSlugHash: packSlugHash,
		VersionHash:  versionHash,
		StatusHash:   domain.ComputeRemovalStatusHash(periodKey, packSlugHash),
	}
}

// BuildProofPage builds the proof page showing all installed packs.
func (e *Engine) BuildProofPage(inputs MarketplaceInputs) domain.MarketplaceProofPage {
	// Build installed lines
	installed := make([]domain.InstalledProofLine, 0)
	for _, rec := range inputs.InstalledPacks {
		if rec.Status != domain.PackStatusInstalled {
			continue
		}

		// Find the pack template to get counts
		presetsCount := 0
		bindingsCount := 0
		for _, pack := range inputs.AvailablePacks {
			if domain.HashString(pack.Slug.CanonicalString()) == rec.PackSlugHash {
				presetsCount = len(pack.SemanticsPresets)
				bindingsCount = len(pack.ObserverBindings)
				break
			}
		}

		installed = append(installed, domain.InstalledProofLine{
			PackSlugHash:  rec.PackSlugHash,
			VersionHash:   rec.VersionHash,
			InstalledDate: rec.PeriodKey,
			PresetsCount:  presetsCount,
			BindingsCount: bindingsCount,
			Effect:        domain.EffectNoPower, // CRITICAL: Always enforced
		})
	}

	// Build removed lines
	removed := make([]domain.RemovedProofLine, 0)
	for _, rec := range inputs.RemovedPacks {
		removed = append(removed, domain.RemovedProofLine{
			PackSlugHash: rec.PackSlugHash,
			VersionHash:  rec.VersionHash,
			RemovedDate:  rec.PeriodKey,
		})
	}

	// Sort for stable output
	sort.Slice(installed, func(i, j int) bool {
		return installed[i].PackSlugHash < installed[j].PackSlugHash
	})
	sort.Slice(removed, func(i, j int) bool {
		return removed[i].PackSlugHash < removed[j].PackSlugHash
	})

	lines := []string{
		"This shows all installed and removed packs.",
		"Packs provide meaning and observer intents only.",
		"No pack grants permission to surface, interrupt, deliver, or execute.",
	}

	statusHash := e.computeProofPageHash(installed, removed)

	return domain.MarketplaceProofPage{
		Title:          "Marketplace Proof",
		Lines:          lines,
		InstalledPacks: installed,
		RemovedPacks:   removed,
		StatusHash:     statusHash,
	}
}

// ComputeCue computes the marketplace cue for the whisper.
func (e *Engine) ComputeCue(inputs MarketplaceInputs) domain.MarketplaceCue {
	// Show cue if no packs installed
	if len(inputs.InstalledPacks) == 0 {
		return domain.MarketplaceCue{
			Available:  true,
			Text:       "Explore packs for your circles.",
			Path:       "/marketplace",
			StatusHash: domain.HashString("cue:marketplace:available"),
		}
	}

	return domain.MarketplaceCue{
		Available:  false,
		StatusHash: domain.HashString("cue:marketplace:hidden"),
	}
}

// IsPackInstalled checks if a pack is currently installed.
func (e *Engine) IsPackInstalled(packSlugHash string, installed []domain.PackInstallRecord) bool {
	for _, rec := range installed {
		if rec.PackSlugHash == packSlugHash && rec.Status == domain.PackStatusInstalled {
			return true
		}
	}
	return false
}

// computeHomePageHash computes a hash for the home page state.
func (e *Engine) computeHomePageHash(available, installed []domain.PackCard) string {
	canonical := fmt.Sprintf("home|%d|%d", len(available), len(installed))
	for _, c := range available {
		canonical += "|" + c.SlugHash
	}
	for _, c := range installed {
		canonical += "|" + c.SlugHash
	}
	return domain.HashString(canonical)
}

// computeProofPageHash computes a hash for the proof page state.
func (e *Engine) computeProofPageHash(installed []domain.InstalledProofLine, removed []domain.RemovedProofLine) string {
	canonical := fmt.Sprintf("proof|%d|%d", len(installed), len(removed))
	for _, l := range installed {
		canonical += "|" + l.PackSlugHash
	}
	for _, l := range removed {
		canonical += "|" + l.PackSlugHash
	}
	return domain.HashString(canonical)
}
