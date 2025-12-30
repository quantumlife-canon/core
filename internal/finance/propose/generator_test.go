// Package propose_test provides tests for the proposal generator.
package propose_test

import (
	"testing"
	"time"

	"quantumlife/internal/finance/propose"
	"quantumlife/pkg/primitives/finance"
)

// mockDismissalStore is a test implementation of DismissalStore.
type mockDismissalStore struct {
	dismissed     map[string]bool
	lastGenerated map[string]time.Time
}

func newMockDismissalStore() *mockDismissalStore {
	return &mockDismissalStore{
		dismissed:     make(map[string]bool),
		lastGenerated: make(map[string]time.Time),
	}
}

func (s *mockDismissalStore) IsDismissed(fingerprint string) bool {
	return s.dismissed[fingerprint]
}

func (s *mockDismissalStore) GetLastGenerated(fingerprint string) *time.Time {
	if t, ok := s.lastGenerated[fingerprint]; ok {
		return &t
	}
	return nil
}

func (s *mockDismissalStore) RecordGenerated(fingerprint string, at time.Time) {
	s.lastGenerated[fingerprint] = at
}

func TestGenerator_Generate(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	clockFunc := func() time.Time { return now }
	idCounter := 0
	idGen := func() string {
		idCounter++
		return "id-" + string(rune('0'+idCounter))
	}

	store := newMockDismissalStore()
	gen := propose.NewGenerator(propose.DefaultConfig(), store, clockFunc, idGen)

	// Create notable observations
	observations := []finance.FinancialObservation{
		{
			ObservationID: "obs-1",
			OwnerType:     "circle",
			OwnerID:       "circle-1",
			Type:          finance.ObservationCategoryShift,
			Title:         "Groceries spending",
			Description:   "Groceries spending increased this month.",
			Category:      "Groceries",
			Severity:      finance.SeverityNotable,
			WindowStart:   now.AddDate(0, 0, -30),
			WindowEnd:     now,
			Fingerprint:   "fp-obs-1",
			Reason:        "Spending exceeded threshold",
		},
	}

	batch := gen.Generate("circle", "circle-1", observations, "trace-123")

	if batch == nil {
		t.Fatal("expected batch, got nil")
	}

	if len(batch.Proposals) == 0 {
		t.Error("expected at least one proposal")
	}

	// Verify proposal structure
	if len(batch.Proposals) > 0 {
		p := batch.Proposals[0]

		if p.OwnerType != "circle" {
			t.Errorf("OwnerType = %q, want %q", p.OwnerType, "circle")
		}

		if p.OwnerID != "circle-1" {
			t.Errorf("OwnerID = %q, want %q", p.OwnerID, "circle-1")
		}

		if p.Status != finance.StatusActive {
			t.Errorf("Status = %q, want %q", p.Status, finance.StatusActive)
		}

		// Verify mandatory disclaimers
		if p.Disclaimers.Informational == "" {
			t.Error("Disclaimers.Informational should not be empty")
		}
		if p.Disclaimers.NoAction == "" {
			t.Error("Disclaimers.NoAction should not be empty")
		}
		if p.Disclaimers.Dismissible == "" {
			t.Error("Disclaimers.Dismissible should not be empty")
		}
	}
}

func TestGenerator_SilencePolicy_NoObservations(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	clockFunc := func() time.Time { return now }
	idGen := func() string { return "id-test" }

	store := newMockDismissalStore()
	gen := propose.NewGenerator(propose.DefaultConfig(), store, clockFunc, idGen)

	// Empty observations should trigger silence
	batch := gen.Generate("circle", "circle-1", nil, "trace-123")

	if !batch.SilenceApplied {
		t.Error("expected silence to be applied for empty observations")
	}

	if batch.SilenceReason == "" {
		t.Error("expected silence reason to be set")
	}

	if len(batch.Proposals) != 0 {
		t.Errorf("expected 0 proposals, got %d", len(batch.Proposals))
	}
}

func TestGenerator_SilencePolicy_OnlyInfoSeverity(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	clockFunc := func() time.Time { return now }
	idGen := func() string { return "id-test" }

	store := newMockDismissalStore()
	gen := propose.NewGenerator(propose.DefaultConfig(), store, clockFunc, idGen)

	// Only info-level observations (not notable/significant)
	observations := []finance.FinancialObservation{
		{
			ObservationID: "obs-1",
			OwnerType:     "circle",
			OwnerID:       "circle-1",
			Type:          finance.ObservationBalanceChange,
			Severity:      finance.SeverityInfo, // Not notable
			Fingerprint:   "fp-info",
		},
	}

	batch := gen.Generate("circle", "circle-1", observations, "trace-123")

	if !batch.SilenceApplied {
		t.Error("expected silence for info-only observations")
	}

	if len(batch.Proposals) != 0 {
		t.Errorf("expected 0 proposals for info-only observations, got %d", len(batch.Proposals))
	}
}

func TestGenerator_DismissedProposalSuppressed(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	clockFunc := func() time.Time { return now }
	idGen := func() string { return "id-test" }

	store := newMockDismissalStore()
	gen := propose.NewGenerator(propose.DefaultConfig(), store, clockFunc, idGen)

	// Create observation with known fingerprint
	observations := []finance.FinancialObservation{
		{
			ObservationID: "obs-1",
			OwnerType:     "circle",
			OwnerID:       "circle-1",
			Type:          finance.ObservationCategoryShift,
			Category:      "Groceries",
			Severity:      finance.SeverityNotable,
			Fingerprint:   "fp-dismissed",
		},
	}

	// First generation should create proposal
	batch1 := gen.Generate("circle", "circle-1", observations, "trace-1")
	if len(batch1.Proposals) == 0 {
		t.Fatal("expected proposal from first generation")
	}

	// Mark the fingerprint as dismissed
	fp := batch1.Proposals[0].Fingerprint
	store.dismissed[fp] = true

	// Second generation should suppress the proposal
	batch2 := gen.Generate("circle", "circle-1", observations, "trace-2")

	if len(batch2.Proposals) != 0 {
		t.Errorf("expected 0 proposals after dismissal, got %d", len(batch2.Proposals))
	}

	if batch2.SuppressedCount == 0 {
		t.Error("expected suppressed count > 0")
	}
}

func TestGenerator_RecentProposalSuppressed(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	clockFunc := func() time.Time { return now }
	idGen := func() string { return "id-test" }

	config := propose.DefaultConfig()
	config.MinIntervalDays = 7 // 7 day minimum interval

	store := newMockDismissalStore()
	gen := propose.NewGenerator(config, store, clockFunc, idGen)

	observations := []finance.FinancialObservation{
		{
			ObservationID: "obs-1",
			OwnerType:     "circle",
			OwnerID:       "circle-1",
			Type:          finance.ObservationCategoryShift,
			Category:      "Groceries",
			Severity:      finance.SeverityNotable,
			Fingerprint:   "fp-recent",
			Reason:        "Test reason",
		},
	}

	// First generation
	batch1 := gen.Generate("circle", "circle-1", observations, "trace-1")
	if len(batch1.Proposals) == 0 {
		t.Fatal("expected proposal from first generation")
	}

	// Immediately try again (should be suppressed due to min interval)
	batch2 := gen.Generate("circle", "circle-1", observations, "trace-2")

	if len(batch2.Proposals) != 0 {
		t.Errorf("expected 0 proposals due to recent generation, got %d", len(batch2.Proposals))
	}
}

func TestGenerator_BatchLimit(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	clockFunc := func() time.Time { return now }
	idCounter := 0
	idGen := func() string {
		idCounter++
		return "id-" + string(rune('0'+idCounter))
	}

	config := propose.DefaultConfig()
	config.MaxProposalsPerBatch = 2

	store := newMockDismissalStore()
	gen := propose.NewGenerator(config, store, clockFunc, idGen)

	// Create many notable observations
	var observations []finance.FinancialObservation
	categories := []string{"Groceries", "Gas", "Entertainment", "Utilities", "Shopping"}
	for _, cat := range categories {
		observations = append(observations, finance.FinancialObservation{
			ObservationID: "obs-" + cat,
			OwnerType:     "circle",
			OwnerID:       "circle-1",
			Type:          finance.ObservationCategoryShift,
			Category:      cat,
			Severity:      finance.SeverityNotable,
			Fingerprint:   "fp-" + cat,
			Reason:        "Test reason for " + cat,
		})
	}

	batch := gen.Generate("circle", "circle-1", observations, "trace-123")

	if len(batch.Proposals) > config.MaxProposalsPerBatch {
		t.Errorf("expected at most %d proposals, got %d",
			config.MaxProposalsPerBatch, len(batch.Proposals))
	}
}

func TestStandardDisclaimers(t *testing.T) {
	d := finance.StandardDisclaimers()

	// Verify all required disclaimers are present
	if d.Informational == "" {
		t.Error("Informational disclaimer is required")
	}

	if d.NoAction == "" {
		t.Error("NoAction disclaimer is required")
	}

	if d.Dismissible == "" {
		t.Error("Dismissible disclaimer is required")
	}

	// Verify language is neutral (no urgency, fear, shame, authority)
	forbiddenPatterns := []string{
		"must", "should", "need to", "have to",
		"urgent", "immediately", "act now",
		"warning", "alert", "danger",
		"excessive", "overspending", "bad",
	}

	allText := d.Informational + d.NoAction + d.Dismissible
	for _, pattern := range forbiddenPatterns {
		// Simple case-insensitive check
		if containsIgnoreCase(allText, pattern) {
			t.Errorf("disclaimer contains forbidden pattern %q", pattern)
		}
	}
}

func containsIgnoreCase(s, pattern string) bool {
	s = toLower(s)
	pattern = toLower(pattern)
	for i := 0; i <= len(s)-len(pattern); i++ {
		if s[i:i+len(pattern)] == pattern {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}
