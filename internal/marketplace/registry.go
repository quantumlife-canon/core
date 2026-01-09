// Package marketplace provides the Phase 46: Circle Registry + Packs engine.
// This is a meaning-only + observer-intent-only layer.
// Packs MUST NOT grant permission to SURFACE/INTERRUPT/DELIVER/EXECUTE.
package marketplace

import (
	"sort"
	"sync"

	domain "quantumlife/pkg/domain/marketplace"
)

// Registry holds the catalog of available pack templates.
// Thread-safe, read-mostly structure.
type Registry struct {
	mu       sync.RWMutex
	packs    map[domain.PackSlug]domain.PackTemplate
	slugList []domain.PackSlug // Sorted for stable iteration
}

// NewRegistry creates a new pack registry.
func NewRegistry() *Registry {
	return &Registry{
		packs:    make(map[domain.PackSlug]domain.PackTemplate),
		slugList: make([]domain.PackSlug, 0),
	}
}

// Register adds a pack template to the registry.
// Returns error if pack is invalid or slug already exists.
func (r *Registry) Register(pack domain.PackTemplate) error {
	if err := pack.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Compute version hash if not set
	if pack.VersionHash == "" {
		pack.VersionHash = pack.ComputeVersionHash()
	}

	// Add or update pack
	_, exists := r.packs[pack.Slug]
	r.packs[pack.Slug] = pack

	if !exists {
		r.slugList = append(r.slugList, pack.Slug)
		sort.Slice(r.slugList, func(i, j int) bool {
			return r.slugList[i] < r.slugList[j]
		})
	}

	return nil
}

// Get returns a pack template by slug.
func (r *Registry) Get(slug domain.PackSlug) (domain.PackTemplate, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pack, exists := r.packs[slug]
	return pack, exists
}

// GetByHash returns a pack template by slug hash.
func (r *Registry) GetByHash(slugHash string) (domain.PackTemplate, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, pack := range r.packs {
		if domain.HashString(pack.Slug.CanonicalString()) == slugHash {
			return pack, true
		}
	}
	return domain.PackTemplate{}, false
}

// List returns all pack templates in stable order.
func (r *Registry) List() []domain.PackTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.PackTemplate, 0, len(r.slugList))
	for _, slug := range r.slugList {
		if pack, exists := r.packs[slug]; exists {
			result = append(result, pack)
		}
	}
	return result
}

// ListPublic returns all public pack templates in stable order.
func (r *Registry) ListPublic() []domain.PackTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.PackTemplate, 0)
	for _, slug := range r.slugList {
		if pack, exists := r.packs[slug]; exists {
			if pack.Visibility == domain.PackVisibilityPublic {
				result = append(result, pack)
			}
		}
	}
	return result
}

// ListByKind returns pack templates of a specific kind.
func (r *Registry) ListByKind(kind domain.PackKind) []domain.PackTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.PackTemplate, 0)
	for _, slug := range r.slugList {
		if pack, exists := r.packs[slug]; exists {
			if pack.Kind == kind {
				result = append(result, pack)
			}
		}
	}
	return result
}

// ListByTier returns pack templates of a specific tier.
func (r *Registry) ListByTier(tier domain.PackTier) []domain.PackTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.PackTemplate, 0)
	for _, slug := range r.slugList {
		if pack, exists := r.packs[slug]; exists {
			if pack.Tier == tier {
				result = append(result, pack)
			}
		}
	}
	return result
}

// Count returns the number of registered packs.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.packs)
}

// Contains checks if a pack is registered.
func (r *Registry) Contains(slug domain.PackSlug) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.packs[slug]
	return exists
}

// DefaultRegistry returns a registry with curated default packs.
func DefaultRegistry() *Registry {
	r := NewRegistry()

	// Family & Friends pack - semantics for human circles
	_ = r.Register(domain.PackTemplate{
		Slug:        "family-friends",
		Kind:        domain.PackKindSemantics,
		Tier:        domain.PackTierCurated,
		Visibility:  domain.PackVisibilityPublic,
		Title:       "Family & Friends",
		Description: "Default semantics for human circles: high necessity, human-waiting urgency.",
		SemanticsPresets: []domain.SemanticsPreset{
			{
				CirclePatternHash: domain.HashString("pattern:human:*"),
				SemanticKind:      "semantic_human",
				UrgencyModel:      "urgency_human_waiting",
				NecessityLevel:    "necessity_high",
			},
		},
		Effect: domain.EffectNoPower,
	})

	// Essential Services pack
	_ = r.Register(domain.PackTemplate{
		Slug:        "essential-services",
		Kind:        domain.PackKindSemantics,
		Tier:        domain.PackTierCurated,
		Visibility:  domain.PackVisibilityPublic,
		Title:       "Essential Services",
		Description: "Semantics for essential services like banks and healthcare: hard deadline urgency.",
		SemanticsPresets: []domain.SemanticsPreset{
			{
				CirclePatternHash: domain.HashString("pattern:service:essential:*"),
				SemanticKind:      "semantic_service_essential",
				UrgencyModel:      "urgency_hard_deadline",
				NecessityLevel:    "necessity_high",
			},
		},
		Effect: domain.EffectNoPower,
	})

	// Quiet Marketing pack
	_ = r.Register(domain.PackTemplate{
		Slug:        "quiet-marketing",
		Kind:        domain.PackKindSemantics,
		Tier:        domain.PackTierCurated,
		Visibility:  domain.PackVisibilityPublic,
		Title:       "Quiet Marketing",
		Description: "Semantics for marketing/promotional circles: low necessity, never interrupt.",
		SemanticsPresets: []domain.SemanticsPreset{
			{
				CirclePatternHash: domain.HashString("pattern:service:optional:*"),
				SemanticKind:      "semantic_service_optional",
				UrgencyModel:      "urgency_never_interrupt",
				NecessityLevel:    "necessity_low",
			},
		},
		Effect: domain.EffectNoPower,
	})

	// Commerce Observer pack
	_ = r.Register(domain.PackTemplate{
		Slug:        "commerce-observer",
		Kind:        domain.PackKindObserverBinding,
		Tier:        domain.PackTierCurated,
		Visibility:  domain.PackVisibilityPublic,
		Title:       "Commerce Observer",
		Description: "Intent to observe commerce-related circles for transaction patterns.",
		ObserverBindings: []domain.ObserverBinding{
			{
				CirclePatternHash: domain.HashString("pattern:commerce:*"),
				ObserverSlug:      "commerce-observer",
				BindingKind:       domain.BindingKindObserveOnly,
				Effect:            domain.EffectNoPower,
			},
		},
		Effect: domain.EffectNoPower,
	})

	// Calendar Awareness pack
	_ = r.Register(domain.PackTemplate{
		Slug:        "calendar-awareness",
		Kind:        domain.PackKindCombined,
		Tier:        domain.PackTierCurated,
		Visibility:  domain.PackVisibilityPublic,
		Title:       "Calendar Awareness",
		Description: "Semantics and observer intents for calendar-related circles.",
		SemanticsPresets: []domain.SemanticsPreset{
			{
				CirclePatternHash: domain.HashString("pattern:calendar:*"),
				SemanticKind:      "semantic_service_essential",
				UrgencyModel:      "urgency_time_window",
				NecessityLevel:    "necessity_medium",
			},
		},
		ObserverBindings: []domain.ObserverBinding{
			{
				CirclePatternHash: domain.HashString("pattern:calendar:*"),
				ObserverSlug:      "calendar-observer",
				BindingKind:       domain.BindingKindAnnotate,
				Effect:            domain.EffectNoPower,
			},
		},
		Effect: domain.EffectNoPower,
	})

	// Inbox Enrichment pack
	_ = r.Register(domain.PackTemplate{
		Slug:        "inbox-enrichment",
		Kind:        domain.PackKindObserverBinding,
		Tier:        domain.PackTierCurated,
		Visibility:  domain.PackVisibilityPublic,
		Title:       "Inbox Enrichment",
		Description: "Intent to enrich inbox circles with metadata annotations.",
		ObserverBindings: []domain.ObserverBinding{
			{
				CirclePatternHash: domain.HashString("pattern:inbox:*"),
				ObserverSlug:      "inbox-enricher",
				BindingKind:       domain.BindingKindEnrich,
				Effect:            domain.EffectNoPower,
			},
		},
		Effect: domain.EffectNoPower,
	})

	return r
}
