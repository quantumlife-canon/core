// Package demo_phase48_market_signal provides demonstration tests for Phase 48.
//
// Phase 48: Market Signal Binding (Non-Extractive Marketplace v1)
//
// This phase binds unmet necessities to available marketplace packs WITHOUT:
// - Recommendations
// - Nudges
// - Ranking
// - Persuasion
// - Execution
//
// This is signal exposure only, not a funnel.
//
// Reference: docs/ADR/ADR-0086-phase48-market-signal-binding.md
package demo_phase48_market_signal

import (
	"testing"
	"time"

	"quantumlife/internal/marketsignal"
	"quantumlife/internal/persist"
	domain "quantumlife/pkg/domain/marketsignal"
)

// ============================================================================
// Domain Type Tests
// ============================================================================

func TestMarketSignalKind_Validate(t *testing.T) {
	tests := []struct {
		name    string
		kind    domain.MarketSignalKind
		wantErr bool
	}{
		{"valid coverage_gap", domain.MarketSignalCoverageGap, false},
		{"invalid kind", domain.MarketSignalKind("invalid"), true},
		{"empty kind", domain.MarketSignalKind(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.kind.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMarketSignalEffect_Validate(t *testing.T) {
	tests := []struct {
		name    string
		effect  domain.MarketSignalEffect
		wantErr bool
	}{
		{"valid effect_no_power", domain.EffectNoPower, false},
		{"invalid effect", domain.MarketSignalEffect("effect_power"), true},
		{"empty effect", domain.MarketSignalEffect(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.effect.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMarketSignalVisibility_Validate(t *testing.T) {
	tests := []struct {
		name       string
		visibility domain.MarketSignalVisibility
		wantErr    bool
	}{
		{"valid proof_only", domain.VisibilityProofOnly, false},
		{"invalid visibility", domain.MarketSignalVisibility("push"), true},
		{"empty visibility", domain.MarketSignalVisibility(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.visibility.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNecessityKind_Validate(t *testing.T) {
	tests := []struct {
		name    string
		kind    domain.NecessityKind
		wantErr bool
	}{
		{"valid high", domain.NecessityKindHigh, false},
		{"valid medium", domain.NecessityKindMedium, false},
		{"valid low", domain.NecessityKindLow, false},
		{"valid unknown", domain.NecessityKindUnknown, false},
		{"invalid kind", domain.NecessityKind("invalid"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.kind.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCoverageGapKind_Validate(t *testing.T) {
	tests := []struct {
		name    string
		kind    domain.CoverageGapKind
		wantErr bool
	}{
		{"valid no_observer", domain.GapNoObserver, false},
		{"valid partial_cover", domain.GapPartialCover, false},
		{"invalid kind", domain.CoverageGapKind("invalid"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.kind.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMarketProofAckKind_Validate(t *testing.T) {
	tests := []struct {
		name    string
		kind    domain.MarketProofAckKind
		wantErr bool
	}{
		{"valid viewed", domain.AckViewed, false},
		{"valid dismissed", domain.AckDismissed, false},
		{"invalid kind", domain.MarketProofAckKind("invalid"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.kind.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMarketSignal_Validate(t *testing.T) {
	validSignal := domain.MarketSignal{
		SignalID:      "test-signal-id",
		CircleHash:    "test-circle-hash",
		NecessityKind: domain.NecessityKindHigh,
		CoverageGap:   domain.GapNoObserver,
		PackIDHash:    "test-pack-hash",
		Kind:          domain.MarketSignalCoverageGap,
		Effect:        domain.EffectNoPower,
		Visibility:    domain.VisibilityProofOnly,
		PeriodKey:     "2025-01-15",
	}

	t.Run("valid signal", func(t *testing.T) {
		if err := validSignal.Validate(); err != nil {
			t.Errorf("valid signal should not error: %v", err)
		}
	})

	t.Run("missing signal ID", func(t *testing.T) {
		s := validSignal
		s.SignalID = ""
		if err := s.Validate(); err == nil {
			t.Error("should error on missing SignalID")
		}
	})

	t.Run("missing circle hash", func(t *testing.T) {
		s := validSignal
		s.CircleHash = ""
		if err := s.Validate(); err == nil {
			t.Error("should error on missing CircleHash")
		}
	})

	t.Run("missing pack ID hash", func(t *testing.T) {
		s := validSignal
		s.PackIDHash = ""
		if err := s.Validate(); err == nil {
			t.Error("should error on missing PackIDHash")
		}
	})

	t.Run("missing period key", func(t *testing.T) {
		s := validSignal
		s.PeriodKey = ""
		if err := s.Validate(); err == nil {
			t.Error("should error on missing PeriodKey")
		}
	})

	t.Run("invalid effect", func(t *testing.T) {
		s := validSignal
		s.Effect = domain.MarketSignalEffect("invalid")
		if err := s.Validate(); err == nil {
			t.Error("should error on invalid effect")
		}
	})
}

func TestMarketSignal_CanonicalString(t *testing.T) {
	signal := domain.MarketSignal{
		SignalID:      "test-id",
		CircleHash:    "circle-hash",
		NecessityKind: domain.NecessityKindHigh,
		CoverageGap:   domain.GapNoObserver,
		PackIDHash:    "pack-hash",
		Kind:          domain.MarketSignalCoverageGap,
		Effect:        domain.EffectNoPower,
		Visibility:    domain.VisibilityProofOnly,
		PeriodKey:     "2025-01-15",
	}

	canonical := signal.CanonicalString()
	if canonical == "" {
		t.Error("canonical string should not be empty")
	}

	// Verify deterministic
	canonical2 := signal.CanonicalString()
	if canonical != canonical2 {
		t.Error("canonical string should be deterministic")
	}
}

func TestMarketSignal_ComputeSignalID(t *testing.T) {
	signal := domain.MarketSignal{
		CircleHash:    "circle-hash",
		NecessityKind: domain.NecessityKindHigh,
		CoverageGap:   domain.GapNoObserver,
		PackIDHash:    "pack-hash",
		Kind:          domain.MarketSignalCoverageGap,
		Effect:        domain.EffectNoPower,
		Visibility:    domain.VisibilityProofOnly,
		PeriodKey:     "2025-01-15",
	}

	id := signal.ComputeSignalID()
	if id == "" {
		t.Error("computed signal ID should not be empty")
	}

	// Verify deterministic
	id2 := signal.ComputeSignalID()
	if id != id2 {
		t.Error("computed signal ID should be deterministic")
	}
}

func TestMarketProofAck_Validate(t *testing.T) {
	validAck := domain.MarketProofAck{
		CircleHash: "circle-hash",
		PeriodKey:  "2025-01-15",
		AckKind:    domain.AckDismissed,
		StatusHash: "status-hash",
	}

	t.Run("valid ack", func(t *testing.T) {
		if err := validAck.Validate(); err != nil {
			t.Errorf("valid ack should not error: %v", err)
		}
	})

	t.Run("missing circle hash", func(t *testing.T) {
		a := validAck
		a.CircleHash = ""
		if err := a.Validate(); err == nil {
			t.Error("should error on missing CircleHash")
		}
	})

	t.Run("missing period key", func(t *testing.T) {
		a := validAck
		a.PeriodKey = ""
		if err := a.Validate(); err == nil {
			t.Error("should error on missing PeriodKey")
		}
	})

	t.Run("missing status hash", func(t *testing.T) {
		a := validAck
		a.StatusHash = ""
		if err := a.Validate(); err == nil {
			t.Error("should error on missing StatusHash")
		}
	})
}

func TestNormalizeSignals_Deduplication(t *testing.T) {
	signal := domain.MarketSignal{
		SignalID:      "same-id",
		CircleHash:    "circle",
		NecessityKind: domain.NecessityKindHigh,
		CoverageGap:   domain.GapNoObserver,
		PackIDHash:    "pack",
		Kind:          domain.MarketSignalCoverageGap,
		Effect:        domain.EffectNoPower,
		Visibility:    domain.VisibilityProofOnly,
		PeriodKey:     "2025-01-15",
	}

	signals := []domain.MarketSignal{signal, signal, signal}
	normalized := domain.NormalizeSignals(signals)

	if len(normalized) != 1 {
		t.Errorf("expected 1 signal after dedup, got %d", len(normalized))
	}
}

func TestNormalizeSignals_Sorting(t *testing.T) {
	signals := []domain.MarketSignal{
		{SignalID: "z-signal"},
		{SignalID: "a-signal"},
		{SignalID: "m-signal"},
	}

	normalized := domain.NormalizeSignals(signals)

	if normalized[0].SignalID != "a-signal" {
		t.Error("signals should be sorted by SignalID")
	}
	if normalized[1].SignalID != "m-signal" {
		t.Error("signals should be sorted by SignalID")
	}
	if normalized[2].SignalID != "z-signal" {
		t.Error("signals should be sorted by SignalID")
	}
}

// ============================================================================
// Engine Tests
// ============================================================================

func TestEngine_GenerateSignals_NoNecessity(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	engine := marketsignal.NewEngine(clock)

	input := marketsignal.MarketSignalInput{
		Necessities:    []marketsignal.NecessityDeclaration{},
		CoveragePlan:   marketsignal.CoveragePlanView{},
		AvailablePacks: []marketsignal.AvailablePack{},
		PeriodKey:      "2025-01-15",
	}

	result := engine.GenerateSignals(input)

	if len(result.Signals) != 0 {
		t.Errorf("expected no signals without necessity, got %d", len(result.Signals))
	}
}

func TestEngine_GenerateSignals_LowNecessity_NoSignal(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	engine := marketsignal.NewEngine(clock)

	input := marketsignal.MarketSignalInput{
		Necessities: []marketsignal.NecessityDeclaration{
			{CircleHash: "circle1", NecessityKind: domain.NecessityKindLow},
		},
		CoveragePlan: marketsignal.CoveragePlanView{CircleHash: "circle1", Capabilities: []string{}},
		AvailablePacks: []marketsignal.AvailablePack{
			{PackSlugHash: "pack1", Capabilities: []marketsignal.PackCapability{{PackSlugHash: "pack1", Capabilities: []string{"cap1"}}}},
		},
		PeriodKey: "2025-01-15",
	}

	result := engine.GenerateSignals(input)

	// Low necessity should not generate signals
	if len(result.Signals) != 0 {
		t.Errorf("expected no signals for low necessity, got %d", len(result.Signals))
	}
}

func TestEngine_GenerateSignals_HighNecessity_WithGap(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	engine := marketsignal.NewEngine(clock)

	input := marketsignal.MarketSignalInput{
		Necessities: []marketsignal.NecessityDeclaration{
			{CircleHash: "circle1", NecessityKind: domain.NecessityKindHigh},
		},
		CoveragePlan: marketsignal.CoveragePlanView{CircleHash: "circle1", Capabilities: []string{}},
		AvailablePacks: []marketsignal.AvailablePack{
			{PackSlugHash: "pack1", Capabilities: []marketsignal.PackCapability{{PackSlugHash: "pack1", Capabilities: []string{"cap1"}}}},
		},
		PeriodKey: "2025-01-15",
	}

	result := engine.GenerateSignals(input)

	if len(result.Signals) == 0 {
		t.Error("expected signal for high necessity with coverage gap")
	}

	// Verify signal properties
	for _, sig := range result.Signals {
		if sig.Effect != domain.EffectNoPower {
			t.Error("signal must have effect_no_power")
		}
		if sig.Visibility != domain.VisibilityProofOnly {
			t.Error("signal must have proof_only visibility")
		}
		if sig.Kind != domain.MarketSignalCoverageGap {
			t.Error("signal must be coverage_gap kind")
		}
	}
}

func TestEngine_GenerateSignals_MediumNecessity_WithGap(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	engine := marketsignal.NewEngine(clock)

	input := marketsignal.MarketSignalInput{
		Necessities: []marketsignal.NecessityDeclaration{
			{CircleHash: "circle1", NecessityKind: domain.NecessityKindMedium},
		},
		CoveragePlan: marketsignal.CoveragePlanView{CircleHash: "circle1", Capabilities: []string{}},
		AvailablePacks: []marketsignal.AvailablePack{
			{PackSlugHash: "pack1", Capabilities: []marketsignal.PackCapability{{PackSlugHash: "pack1", Capabilities: []string{"cap1"}}}},
		},
		PeriodKey: "2025-01-15",
	}

	result := engine.GenerateSignals(input)

	if len(result.Signals) == 0 {
		t.Error("expected signal for medium necessity with coverage gap")
	}
}

func TestEngine_GenerateSignals_NoCoverageGap_NoSignal(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	engine := marketsignal.NewEngine(clock)

	input := marketsignal.MarketSignalInput{
		Necessities: []marketsignal.NecessityDeclaration{
			{CircleHash: "circle1", NecessityKind: domain.NecessityKindHigh},
		},
		// Coverage already has the capability
		CoveragePlan: marketsignal.CoveragePlanView{CircleHash: "circle1", Capabilities: []string{"cap1"}},
		AvailablePacks: []marketsignal.AvailablePack{
			{PackSlugHash: "pack1", Capabilities: []marketsignal.PackCapability{{PackSlugHash: "pack1", Capabilities: []string{"cap1"}}}},
		},
		PeriodKey: "2025-01-15",
	}

	result := engine.GenerateSignals(input)

	// No gap = no signals (silence is default)
	if len(result.Signals) != 0 {
		t.Errorf("expected no signals when coverage is complete, got %d", len(result.Signals))
	}
}

func TestEngine_GenerateSignals_MaxThreePerCircle(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	engine := marketsignal.NewEngine(clock)

	// Create 5 available packs that could fill gaps
	packs := make([]marketsignal.AvailablePack, 5)
	for i := 0; i < 5; i++ {
		packs[i] = marketsignal.AvailablePack{
			PackSlugHash: domain.HashString("pack" + string(rune('a'+i))),
			Capabilities: []marketsignal.PackCapability{
				{PackSlugHash: domain.HashString("pack" + string(rune('a'+i))), Capabilities: []string{"cap" + string(rune('1'+i))}},
			},
		}
	}

	input := marketsignal.MarketSignalInput{
		Necessities: []marketsignal.NecessityDeclaration{
			{CircleHash: "circle1", NecessityKind: domain.NecessityKindHigh},
		},
		CoveragePlan:   marketsignal.CoveragePlanView{CircleHash: "circle1", Capabilities: []string{}},
		AvailablePacks: packs,
		PeriodKey:      "2025-01-15",
	}

	result := engine.GenerateSignals(input)

	// Should be capped at 3 signals per circle
	if len(result.Signals) > 3 {
		t.Errorf("expected max 3 signals per circle, got %d", len(result.Signals))
	}
}

func TestEngine_GenerateSignals_DeterministicOrder(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	engine := marketsignal.NewEngine(clock)

	input := marketsignal.MarketSignalInput{
		Necessities: []marketsignal.NecessityDeclaration{
			{CircleHash: "circle1", NecessityKind: domain.NecessityKindHigh},
		},
		CoveragePlan: marketsignal.CoveragePlanView{CircleHash: "circle1", Capabilities: []string{}},
		AvailablePacks: []marketsignal.AvailablePack{
			{PackSlugHash: "pack-z", Capabilities: []marketsignal.PackCapability{{PackSlugHash: "pack-z", Capabilities: []string{"cap1"}}}},
			{PackSlugHash: "pack-a", Capabilities: []marketsignal.PackCapability{{PackSlugHash: "pack-a", Capabilities: []string{"cap2"}}}},
		},
		PeriodKey: "2025-01-15",
	}

	result1 := engine.GenerateSignals(input)
	result2 := engine.GenerateSignals(input)

	if len(result1.Signals) != len(result2.Signals) {
		t.Error("signal generation should be deterministic")
	}

	for i := range result1.Signals {
		if result1.Signals[i].SignalID != result2.Signals[i].SignalID {
			t.Error("signal order should be deterministic")
		}
	}
}

func TestEngine_BuildProofPage_NoSignals(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	engine := marketsignal.NewEngine(clock)

	page := engine.BuildProofPage([]domain.MarketSignal{})

	if page.Title == "" {
		t.Error("proof page should have title")
	}
	if len(page.Lines) == 0 {
		t.Error("proof page should have copy lines")
	}
	if len(page.Signals) != 0 {
		t.Error("proof page should have no signals")
	}
}

func TestEngine_BuildProofPage_WithSignals(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	engine := marketsignal.NewEngine(clock)

	signals := []domain.MarketSignal{
		{
			SignalID:      "test-id",
			CircleHash:    "circle",
			NecessityKind: domain.NecessityKindHigh,
			CoverageGap:   domain.GapNoObserver,
			PackIDHash:    "pack",
			Kind:          domain.MarketSignalCoverageGap,
			Effect:        domain.EffectNoPower,
			Visibility:    domain.VisibilityProofOnly,
			PeriodKey:     "2025-01-15",
		},
	}

	page := engine.BuildProofPage(signals)

	if len(page.Signals) != 1 {
		t.Errorf("expected 1 signal display, got %d", len(page.Signals))
	}

	// Verify no recommendation language in copy
	for _, line := range page.Lines {
		if containsRecommendation(line) {
			t.Errorf("proof page should not contain recommendation language: %s", line)
		}
	}
}

func TestEngine_BuildCue_NoSignals(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	engine := marketsignal.NewEngine(clock)

	cue := engine.BuildCue([]domain.MarketSignal{}, false)

	if cue.Available {
		t.Error("cue should not be available without signals")
	}
}

func TestEngine_BuildCue_WithSignals(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	engine := marketsignal.NewEngine(clock)

	signals := []domain.MarketSignal{
		{SignalID: "test-id"},
	}

	cue := engine.BuildCue(signals, false)

	if !cue.Available {
		t.Error("cue should be available with signals")
	}
	if cue.Path != "/proof/market" {
		t.Errorf("expected path /proof/market, got %s", cue.Path)
	}

	// Verify no recommendation language in cue text
	if containsRecommendation(cue.Text) {
		t.Errorf("cue text should not contain recommendation language: %s", cue.Text)
	}
}

func TestEngine_BuildCue_Dismissed(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	engine := marketsignal.NewEngine(clock)

	signals := []domain.MarketSignal{
		{SignalID: "test-id"},
	}

	cue := engine.BuildCue(signals, true) // dismissed

	if cue.Available {
		t.Error("cue should not be available when dismissed")
	}
}

// ============================================================================
// Store Tests
// ============================================================================

func TestMarketSignalStore_AppendAndList(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	store := persist.NewMarketSignalStore(clock)

	signal := domain.MarketSignal{
		SignalID:      "test-id",
		CircleHash:    "circle1",
		NecessityKind: domain.NecessityKindHigh,
		CoverageGap:   domain.GapNoObserver,
		PackIDHash:    "pack1",
		Kind:          domain.MarketSignalCoverageGap,
		Effect:        domain.EffectNoPower,
		Visibility:    domain.VisibilityProofOnly,
		PeriodKey:     "2025-01-15",
	}

	if err := store.AppendSignal(signal); err != nil {
		t.Fatalf("failed to append signal: %v", err)
	}

	if store.Count() != 1 {
		t.Errorf("expected count 1, got %d", store.Count())
	}

	signals := store.ListByCirclePeriod("circle1", "2025-01-15")
	if len(signals) != 1 {
		t.Errorf("expected 1 signal, got %d", len(signals))
	}
}

func TestMarketSignalStore_Deduplication(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	store := persist.NewMarketSignalStore(clock)

	signal := domain.MarketSignal{
		SignalID:      "same-id",
		CircleHash:    "circle1",
		NecessityKind: domain.NecessityKindHigh,
		CoverageGap:   domain.GapNoObserver,
		PackIDHash:    "pack1",
		Kind:          domain.MarketSignalCoverageGap,
		Effect:        domain.EffectNoPower,
		Visibility:    domain.VisibilityProofOnly,
		PeriodKey:     "2025-01-15",
	}

	// Append same signal multiple times
	store.AppendSignal(signal)
	store.AppendSignal(signal)
	store.AppendSignal(signal)

	if store.Count() != 1 {
		t.Errorf("expected count 1 after dedup, got %d", store.Count())
	}
}

func TestMarketSignalStore_ListByPeriod(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	store := persist.NewMarketSignalStore(clock)

	signal1 := domain.MarketSignal{
		SignalID:  "id1",
		PeriodKey: "2025-01-15",
	}
	signal2 := domain.MarketSignal{
		SignalID:  "id2",
		PeriodKey: "2025-01-16",
	}

	store.AppendSignal(signal1)
	store.AppendSignal(signal2)

	signals := store.ListByPeriod("2025-01-15")
	if len(signals) != 1 {
		t.Errorf("expected 1 signal for period, got %d", len(signals))
	}
}

func TestMarketProofAckStore_AppendAndCheck(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	store := persist.NewMarketProofAckStore(clock)

	ack := domain.MarketProofAck{
		CircleHash: "circle1",
		PeriodKey:  "2025-01-15",
		AckKind:    domain.AckDismissed,
		StatusHash: "status-hash",
	}

	if err := store.AppendAck(ack); err != nil {
		t.Fatalf("failed to append ack: %v", err)
	}

	if !store.IsProofDismissed("circle1", "2025-01-15") {
		t.Error("proof should be dismissed")
	}

	if store.IsProofDismissed("circle2", "2025-01-15") {
		t.Error("proof for different circle should not be dismissed")
	}
}

func TestMarketProofAckStore_ViewedDoesNotDismiss(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC) }
	store := persist.NewMarketProofAckStore(clock)

	ack := domain.MarketProofAck{
		CircleHash: "circle1",
		PeriodKey:  "2025-01-15",
		AckKind:    domain.AckViewed, // viewed, not dismissed
		StatusHash: "status-hash",
	}

	store.AppendAck(ack)

	if store.IsProofDismissed("circle1", "2025-01-15") {
		t.Error("viewed should not mark as dismissed")
	}
}

// ============================================================================
// Invariant Tests
// ============================================================================

func TestInvariant_EffectAlwaysNoPower(t *testing.T) {
	// Verify that MarketSignalEffect only allows EffectNoPower
	if err := domain.EffectNoPower.Validate(); err != nil {
		t.Error("EffectNoPower should be valid")
	}

	// Any other effect should be invalid
	if err := domain.MarketSignalEffect("effect_power").Validate(); err == nil {
		t.Error("only effect_no_power should be valid")
	}
}

func TestInvariant_VisibilityAlwaysProofOnly(t *testing.T) {
	// Verify that MarketSignalVisibility only allows VisibilityProofOnly
	if err := domain.VisibilityProofOnly.Validate(); err != nil {
		t.Error("VisibilityProofOnly should be valid")
	}

	// Any other visibility should be invalid
	if err := domain.MarketSignalVisibility("push").Validate(); err == nil {
		t.Error("only proof_only visibility should be valid")
	}
}

func TestInvariant_MaxSignalsPerCircle(t *testing.T) {
	if domain.MaxSignalsPerCirclePeriod != 3 {
		t.Errorf("MaxSignalsPerCirclePeriod should be 3, got %d", domain.MaxSignalsPerCirclePeriod)
	}
}

func TestInvariant_BoundedRetention(t *testing.T) {
	if domain.MaxMarketSignalRecords != 200 {
		t.Errorf("MaxMarketSignalRecords should be 200, got %d", domain.MaxMarketSignalRecords)
	}
	if domain.MaxMarketSignalDays != 30 {
		t.Errorf("MaxMarketSignalDays should be 30, got %d", domain.MaxMarketSignalDays)
	}
}

// Helper function to check for recommendation language
func containsRecommendation(text string) bool {
	forbidden := []string{
		"recommend",
		"should buy",
		"should install",
		"don't miss",
		"limited time",
		"featured",
		"promoted",
		"best choice",
		"top pick",
	}
	for _, word := range forbidden {
		if len(text) > 0 && len(word) > 0 {
			for i := 0; i <= len(text)-len(word); i++ {
				match := true
				for j := 0; j < len(word); j++ {
					if text[i+j] != word[j] && text[i+j] != word[j]-32 && text[i+j] != word[j]+32 {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
		}
	}
	return false
}
