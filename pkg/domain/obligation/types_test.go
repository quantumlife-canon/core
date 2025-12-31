package obligation

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/identity"
)

func TestNewObligationDeterministicID(t *testing.T) {
	circleID := identity.EntityID("circle-work")
	sourceEventID := "email-123"
	createdAt := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create same obligation twice
	o1 := NewObligation(circleID, sourceEventID, "email", ObligationReview, createdAt)
	o2 := NewObligation(circleID, sourceEventID, "email", ObligationReview, createdAt)

	if o1.ID != o2.ID {
		t.Errorf("Expected same ID, got %s vs %s", o1.ID, o2.ID)
	}

	// ID should be 16 hex chars
	if len(o1.ID) != 16 {
		t.Errorf("Expected 16 char ID, got %d chars: %s", len(o1.ID), o1.ID)
	}
}

func TestNewObligationDifferentInputsDifferentIDs(t *testing.T) {
	createdAt := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	o1 := NewObligation("circle-work", "email-123", "email", ObligationReview, createdAt)
	o2 := NewObligation("circle-work", "email-456", "email", ObligationReview, createdAt)
	o3 := NewObligation("circle-family", "email-123", "email", ObligationReview, createdAt)
	o4 := NewObligation("circle-work", "email-123", "email", ObligationReply, createdAt)

	ids := map[string]bool{
		o1.ID: true,
		o2.ID: true,
		o3.ID: true,
		o4.ID: true,
	}

	if len(ids) != 4 {
		t.Errorf("Expected 4 unique IDs, got %d", len(ids))
	}
}

func TestWithDueBySetsHorizon(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	o := NewObligation("circle-work", "event-1", "email", ObligationReview, now)

	// Note: HorizonToday = overdue or due now (until <= 0)
	// Horizon24h = within 24 hours
	tests := []struct {
		name    string
		dueBy   time.Time
		horizon AttentionHorizon
	}{
		{"overdue", now.Add(-1 * time.Hour), HorizonToday},
		{"due now", now, HorizonToday},
		{"within 24h", now.Add(12 * time.Hour), Horizon24h},
		{"exactly 24h", now.Add(24 * time.Hour), Horizon24h},
		{"within 7d", now.Add(3 * 24 * time.Hour), Horizon7d},
		{"far future", now.Add(30 * 24 * time.Hour), HorizonSomeday},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o.WithDueBy(tt.dueBy, now)
			if o.Horizon != tt.horizon {
				t.Errorf("Expected horizon %s, got %s", tt.horizon, o.Horizon)
			}
		})
	}
}

func TestWithScoringClamps(t *testing.T) {
	o := NewObligation("circle-work", "event-1", "email", ObligationReview, time.Now())

	// Test clamping above 1
	o.WithScoring(1.5, 2.0)
	if o.RegretScore != 1.0 {
		t.Errorf("Expected regret clamped to 1.0, got %f", o.RegretScore)
	}
	if o.Confidence != 1.0 {
		t.Errorf("Expected confidence clamped to 1.0, got %f", o.Confidence)
	}

	// Test clamping below 0
	o.WithScoring(-0.5, -1.0)
	if o.RegretScore != 0.0 {
		t.Errorf("Expected regret clamped to 0.0, got %f", o.RegretScore)
	}
	if o.Confidence != 0.0 {
		t.Errorf("Expected confidence clamped to 0.0, got %f", o.Confidence)
	}

	// Test valid values
	o.WithScoring(0.7, 0.85)
	if o.RegretScore != 0.7 {
		t.Errorf("Expected regret 0.7, got %f", o.RegretScore)
	}
	if o.Confidence != 0.85 {
		t.Errorf("Expected confidence 0.85, got %f", o.Confidence)
	}
}

func TestCanonicalStringDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	dueBy := time.Date(2025, 1, 16, 18, 0, 0, 0, time.UTC)

	o1 := NewObligation("circle-work", "email-123", "email", ObligationReview, now).
		WithDueBy(dueBy, now).
		WithScoring(0.75, 0.85).
		WithEvidence("subject", "Important Email").
		WithEvidence("sender", "boss@company.com")

	o2 := NewObligation("circle-work", "email-123", "email", ObligationReview, now).
		WithDueBy(dueBy, now).
		WithScoring(0.75, 0.85).
		WithEvidence("sender", "boss@company.com"). // Different order
		WithEvidence("subject", "Important Email")

	cs1 := o1.CanonicalString()
	cs2 := o2.CanonicalString()

	if cs1 != cs2 {
		t.Errorf("Canonical strings differ:\n%s\nvs\n%s", cs1, cs2)
	}
}

func TestSortObligations(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create obligations in random order
	obligs := []*Obligation{
		NewObligation("c1", "e1", "email", ObligationReview, now).
			WithScoring(0.5, 0.8),
		NewObligation("c2", "e2", "email", ObligationReview, now).
			WithDueBy(now.Add(2*time.Hour), now).
			WithScoring(0.8, 0.9),
		NewObligation("c3", "e3", "calendar", ObligationAttend, now).
			WithDueBy(now.Add(30*24*time.Hour), now).
			WithScoring(0.3, 0.7),
		NewObligation("c4", "e4", "email", ObligationReply, now).
			WithDueBy(now.Add(1*time.Hour), now).
			WithScoring(0.9, 0.95),
	}

	SortObligations(obligs)

	// First should be highest priority (today + highest regret)
	if obligs[0].SourceEventID != "e4" {
		t.Errorf("Expected e4 first (today + regret 0.9), got %s with horizon %s regret %.2f",
			obligs[0].SourceEventID, obligs[0].Horizon, obligs[0].RegretScore)
	}

	// Second should be e2 (today + regret 0.8)
	if obligs[1].SourceEventID != "e2" {
		t.Errorf("Expected e2 second, got %s", obligs[1].SourceEventID)
	}
}

func TestComputeObligationsHash(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	obligs1 := []*Obligation{
		NewObligation("c1", "e1", "email", ObligationReview, now).WithScoring(0.5, 0.8),
		NewObligation("c2", "e2", "calendar", ObligationAttend, now).WithScoring(0.6, 0.9),
	}

	obligs2 := []*Obligation{
		NewObligation("c2", "e2", "calendar", ObligationAttend, now).WithScoring(0.6, 0.9),
		NewObligation("c1", "e1", "email", ObligationReview, now).WithScoring(0.5, 0.8),
	}

	hash1 := ComputeObligationsHash(obligs1)
	hash2 := ComputeObligationsHash(obligs2)

	// Order shouldn't matter - both should sort to same order
	if hash1 != hash2 {
		t.Errorf("Hash should be same regardless of input order: %s vs %s", hash1, hash2)
	}

	// Empty list should return "empty"
	emptyHash := ComputeObligationsHash([]*Obligation{})
	if emptyHash != "empty" {
		t.Errorf("Expected 'empty' for empty list, got %s", emptyHash)
	}
}

func TestFilterByHorizon(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Note: HorizonToday = overdue (until <= 0), Horizon24h = within 24 hours
	obligs := []*Obligation{
		NewObligation("c1", "e1", "email", ObligationReview, now).
			WithDueBy(now.Add(-1*time.Hour), now), // today (overdue)
		NewObligation("c2", "e2", "email", ObligationReview, now).
			WithDueBy(now.Add(12*time.Hour), now), // 24h (within 24 hours)
		NewObligation("c3", "e3", "email", ObligationReview, now).
			WithDueBy(now.Add(3*24*time.Hour), now), // 7d
		NewObligation("c4", "e4", "email", ObligationReview, now), // someday (no due)
	}

	// Filter for today only (overdue items)
	today := FilterByHorizon(obligs, HorizonToday)
	if len(today) != 1 {
		t.Errorf("Expected 1 today (overdue) obligation, got %d", len(today))
	}

	// Filter for today and 24h (urgent items)
	urgent := FilterByHorizon(obligs, HorizonToday, Horizon24h)
	if len(urgent) != 2 {
		t.Errorf("Expected 2 urgent obligations (today + 24h), got %d", len(urgent))
	}
}

func TestFilterByMinRegret(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	obligs := []*Obligation{
		NewObligation("c1", "e1", "email", ObligationReview, now).WithScoring(0.3, 0.8),
		NewObligation("c2", "e2", "email", ObligationReview, now).WithScoring(0.5, 0.8),
		NewObligation("c3", "e3", "email", ObligationReview, now).WithScoring(0.7, 0.8),
		NewObligation("c4", "e4", "email", ObligationReview, now).WithScoring(0.9, 0.8),
	}

	high := FilterByMinRegret(obligs, 0.5)
	if len(high) != 3 {
		t.Errorf("Expected 3 high-regret obligations (>=0.5), got %d", len(high))
	}

	veryHigh := FilterByMinRegret(obligs, 0.8)
	if len(veryHigh) != 1 {
		t.Errorf("Expected 1 very-high-regret obligation (>=0.8), got %d", len(veryHigh))
	}
}

func TestFilterByCircle(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	obligs := []*Obligation{
		NewObligation("circle-work", "e1", "email", ObligationReview, now),
		NewObligation("circle-work", "e2", "email", ObligationReview, now),
		NewObligation("circle-family", "e3", "email", ObligationReview, now),
		NewObligation("circle-finance", "e4", "finance", ObligationPay, now),
	}

	work := FilterByCircle(obligs, "circle-work")
	if len(work) != 2 {
		t.Errorf("Expected 2 work obligations, got %d", len(work))
	}

	family := FilterByCircle(obligs, "circle-family")
	if len(family) != 1 {
		t.Errorf("Expected 1 family obligation, got %d", len(family))
	}
}

func TestHorizonAndSeverityOrdering(t *testing.T) {
	tests := []struct {
		horizon AttentionHorizon
		order   int
	}{
		{HorizonToday, 0},
		{Horizon24h, 1},
		{Horizon7d, 2},
		{HorizonSomeday, 3},
	}

	for _, tt := range tests {
		got := HorizonOrder(tt.horizon)
		if got != tt.order {
			t.Errorf("HorizonOrder(%s) = %d, want %d", tt.horizon, got, tt.order)
		}
	}

	severityTests := []struct {
		severity Severity
		order    int
	}{
		{SeverityCritical, 0},
		{SeverityHigh, 1},
		{SeverityMedium, 2},
		{SeverityLow, 3},
	}

	for _, tt := range severityTests {
		got := SeverityOrder(tt.severity)
		if got != tt.order {
			t.Errorf("SeverityOrder(%s) = %d, want %d", tt.severity, got, tt.order)
		}
	}
}
