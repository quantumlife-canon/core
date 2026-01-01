// Package demo_phase18_5_proof contains demonstration tests for Phase 18.5: Quiet Proof.
// These tests verify determinism, abstraction, and correctness of the proof system.
//
// Reference: docs/ADR/ADR-0037-phase18-5-quiet-proof.md
package demo_phase18_5_proof

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/proof"
)

// TestProofDeterminism verifies that the same inputs produce the same output.
func TestProofDeterminism(t *testing.T) {
	engine := proof.NewEngine()

	input := proof.ProofInput{
		SuppressedByCategory: map[proof.Category]int{
			proof.CategoryMoney: 2,
			proof.CategoryTime:  1,
		},
		PreferenceQuiet: true,
		Period:          "week",
	}

	// Generate twice
	proof1 := engine.BuildProof(input)
	proof2 := engine.BuildProof(input)

	// Must be identical
	if proof1.Hash != proof2.Hash {
		t.Errorf("Hash mismatch: %s != %s", proof1.Hash, proof2.Hash)
	}
	if proof1.Magnitude != proof2.Magnitude {
		t.Errorf("Magnitude mismatch: %s != %s", proof1.Magnitude, proof2.Magnitude)
	}
	if proof1.Statement != proof2.Statement {
		t.Errorf("Statement mismatch: %s != %s", proof1.Statement, proof2.Statement)
	}
}

// TestMagnitudeBuckets verifies magnitude bucketing rules.
func TestMagnitudeBuckets(t *testing.T) {
	engine := proof.NewEngine()

	tests := []struct {
		name     string
		counts   map[proof.Category]int
		expected proof.Magnitude
	}{
		{
			name:     "zero total -> nothing",
			counts:   map[proof.Category]int{},
			expected: proof.MagnitudeNothing,
		},
		{
			name:     "1 total -> a_few",
			counts:   map[proof.Category]int{proof.CategoryMoney: 1},
			expected: proof.MagnitudeAFew,
		},
		{
			name:     "3 total -> a_few",
			counts:   map[proof.Category]int{proof.CategoryMoney: 2, proof.CategoryTime: 1},
			expected: proof.MagnitudeAFew,
		},
		{
			name:     "4 total -> several",
			counts:   map[proof.Category]int{proof.CategoryMoney: 2, proof.CategoryWork: 2},
			expected: proof.MagnitudeSeveral,
		},
		{
			name:     "10 total -> several",
			counts:   map[proof.Category]int{proof.CategoryMoney: 5, proof.CategoryTime: 5},
			expected: proof.MagnitudeSeveral,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := proof.ProofInput{
				SuppressedByCategory: tc.counts,
				PreferenceQuiet:      true,
				Period:               "week",
			}
			result := engine.BuildProof(input)
			if result.Magnitude != tc.expected {
				t.Errorf("got %s, want %s", result.Magnitude, tc.expected)
			}
		})
	}
}

// TestCategoriesSorted verifies categories are always sorted deterministically.
func TestCategoriesSorted(t *testing.T) {
	engine := proof.NewEngine()

	// Add categories in different order
	input := proof.ProofInput{
		SuppressedByCategory: map[proof.Category]int{
			proof.CategoryWork:   1,
			proof.CategoryMoney:  1,
			proof.CategoryPeople: 1,
			proof.CategoryTime:   1,
			proof.CategoryHome:   1,
		},
		PreferenceQuiet: true,
		Period:          "week",
	}

	result := engine.BuildProof(input)

	// Should be sorted: home, money, people, time, work
	expected := []proof.Category{
		proof.CategoryHome,
		proof.CategoryMoney,
		proof.CategoryPeople,
		proof.CategoryTime,
		proof.CategoryWork,
	}

	if len(result.Categories) != len(expected) {
		t.Fatalf("got %d categories, want %d", len(result.Categories), len(expected))
	}

	for i, cat := range result.Categories {
		if cat != expected[i] {
			t.Errorf("categories[%d]: got %s, want %s", i, cat, expected[i])
		}
	}
}

// TestNoIdentifiersInStatement verifies statements contain no identifiers.
func TestNoIdentifiersInStatement(t *testing.T) {
	engine := proof.NewEngine()

	input := proof.ProofInput{
		SuppressedByCategory: map[proof.Category]int{
			proof.CategoryMoney: 5,
		},
		PreferenceQuiet: true,
		Period:          "week",
	}

	result := engine.BuildProof(input)

	// Statement must not contain specific identifiers
	forbidden := []string{"$", "£", "€", "@", "http", "amazon", "uber", "netflix"}
	for _, f := range forbidden {
		if strings.Contains(strings.ToLower(result.Statement), f) {
			t.Errorf("Statement contains forbidden identifier: %s", f)
		}
	}
}

// TestNonQuietPreferenceReturnsNothing verifies proof is empty when not in quiet mode.
func TestNonQuietPreferenceReturnsNothing(t *testing.T) {
	engine := proof.NewEngine()

	input := proof.ProofInput{
		SuppressedByCategory: map[proof.Category]int{
			proof.CategoryMoney: 10, // High count, but preference is not quiet
		},
		PreferenceQuiet: false,
		Period:          "week",
	}

	result := engine.BuildProof(input)

	if result.Magnitude != proof.MagnitudeNothing {
		t.Errorf("got magnitude %s, want %s", result.Magnitude, proof.MagnitudeNothing)
	}
	if len(result.Categories) != 0 {
		t.Errorf("got %d categories, want 0", len(result.Categories))
	}
	if result.Statement != "" {
		t.Errorf("got statement %q, want empty", result.Statement)
	}
}

// TestAckStoreHashOnly verifies store only stores hashes.
func TestAckStoreHashOnly(t *testing.T) {
	store := proof.NewAckStore(10)

	proofHash := "abc123hash"
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	err := store.Record(proof.AckViewed, proofHash, now)
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	// Check that proof hash is indexed
	if !store.HasRecent(proofHash) {
		t.Error("HasRecent returned false for recorded proof hash")
	}

	// Check unknown hash returns false
	if store.HasRecent("unknown_hash") {
		t.Error("HasRecent returned true for unknown hash")
	}
}

// TestAckStoreBoundedEviction verifies store respects maximum records.
func TestAckStoreBoundedEviction(t *testing.T) {
	maxRecords := 5
	store := proof.NewAckStore(maxRecords)

	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Add more than max records
	for i := 0; i < maxRecords+3; i++ {
		err := store.Record(proof.AckDismissed, "hash_"+string(rune('a'+i)), now.Add(time.Duration(i)*time.Hour))
		if err != nil {
			t.Fatalf("Record failed: %v", err)
		}
	}

	// Store should not exceed max records
	if store.Len() > maxRecords {
		t.Errorf("Store has %d records, max is %d", store.Len(), maxRecords)
	}
}

// TestCueAvailableWhenProofNotNothing verifies cue appears when appropriate.
func TestCueAvailableWhenProofNotNothing(t *testing.T) {
	engine := proof.NewEngine()

	tests := []struct {
		name         string
		magnitude    proof.Magnitude
		hasRecentAck bool
		wantCue      bool
	}{
		{"a_few, no ack", proof.MagnitudeAFew, false, true},
		{"several, no ack", proof.MagnitudeSeveral, false, true},
		{"nothing, no ack", proof.MagnitudeNothing, false, false},
		{"a_few, has ack", proof.MagnitudeAFew, true, false},
		{"several, has ack", proof.MagnitudeSeveral, true, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			summary := proof.ProofSummary{
				Magnitude: tc.magnitude,
				Hash:      "test_hash",
			}
			cue := engine.BuildCue(summary, tc.hasRecentAck)
			if cue.Available != tc.wantCue {
				t.Errorf("got Available=%v, want %v", cue.Available, tc.wantCue)
			}
		})
	}
}

// TestCueDisappearsAfterDismiss verifies cue disappears after dismissal.
func TestCueDisappearsAfterDismiss(t *testing.T) {
	engine := proof.NewEngine()
	store := proof.NewAckStore(10)

	input := proof.ProofInput{
		SuppressedByCategory: map[proof.Category]int{
			proof.CategoryMoney: 2,
		},
		PreferenceQuiet: true,
		Period:          "week",
	}

	summary := engine.BuildProof(input)

	// Before dismiss: cue should be available
	hasAck := store.HasRecent(summary.Hash)
	cue := engine.BuildCue(summary, hasAck)
	if !cue.Available {
		t.Error("Cue should be available before dismiss")
	}

	// Record dismiss
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	err := store.Record(proof.AckDismissed, summary.Hash, now)
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	// After dismiss: cue should not be available
	hasAck = store.HasRecent(summary.Hash)
	cue = engine.BuildCue(summary, hasAck)
	if cue.Available {
		t.Error("Cue should not be available after dismiss")
	}
}

// TestWhyLineOnlyForNonNothing verifies why line is empty for nothing magnitude.
func TestWhyLineOnlyForNonNothing(t *testing.T) {
	engine := proof.NewEngine()

	// Test with a_few
	input := proof.ProofInput{
		SuppressedByCategory: map[proof.Category]int{
			proof.CategoryMoney: 2,
		},
		PreferenceQuiet: true,
		Period:          "week",
	}
	result := engine.BuildProof(input)
	if result.WhyLine == "" {
		t.Error("WhyLine should not be empty for a_few")
	}

	// Test with nothing
	input.SuppressedByCategory = map[proof.Category]int{}
	result = engine.BuildProof(input)
	if result.WhyLine != "" {
		t.Errorf("WhyLine should be empty for nothing, got %q", result.WhyLine)
	}
}

// TestStatementVariants verifies correct statement selection.
func TestStatementVariants(t *testing.T) {
	engine := proof.NewEngine()

	tests := []struct {
		name     string
		counts   map[proof.Category]int
		contains string
	}{
		{
			name:     "a_few",
			counts:   map[proof.Category]int{proof.CategoryMoney: 2},
			contains: "a few times",
		},
		{
			name:     "several",
			counts:   map[proof.Category]int{proof.CategoryMoney: 5},
			contains: "often",
		},
		{
			name:     "nothing",
			counts:   map[proof.Category]int{},
			contains: "Nothing needed holding",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := proof.ProofInput{
				SuppressedByCategory: tc.counts,
				PreferenceQuiet:      true,
				Period:               "week",
			}
			result := engine.BuildProof(input)
			if !strings.Contains(result.Statement, tc.contains) {
				t.Errorf("Statement %q should contain %q", result.Statement, tc.contains)
			}
		})
	}
}

// TestCanonicalStringFormat verifies canonical string format.
func TestCanonicalStringFormat(t *testing.T) {
	summary := proof.ProofSummary{
		Magnitude:  proof.MagnitudeAFew,
		Categories: []proof.Category{proof.CategoryMoney, proof.CategoryTime},
		Statement:  "Test statement",
	}

	canonical := summary.CanonicalString()

	// Should follow format: PROOF|v1|<magnitude>|<cats>|<statement>
	if !strings.HasPrefix(canonical, "PROOF|v1|") {
		t.Errorf("Canonical string should start with PROOF|v1|, got %s", canonical)
	}
	if !strings.Contains(canonical, "a_few") {
		t.Error("Canonical string should contain magnitude")
	}
	if !strings.Contains(canonical, "money,time") {
		t.Error("Canonical string should contain categories")
	}
}

// TestNoRawCountsExposed verifies counts are never exposed in output.
func TestNoRawCountsExposed(t *testing.T) {
	engine := proof.NewEngine()

	input := proof.ProofInput{
		SuppressedByCategory: map[proof.Category]int{
			proof.CategoryMoney: 7,
			proof.CategoryTime:  13,
			proof.CategoryWork:  42,
		},
		PreferenceQuiet: true,
		Period:          "week",
	}

	result := engine.BuildProof(input)

	// The specific counts should never appear in any output
	forbiddenNumbers := []string{"7", "13", "42", "62"} // 62 = total
	for _, num := range forbiddenNumbers {
		if strings.Contains(result.Statement, num) {
			t.Errorf("Statement contains raw count: %s", num)
		}
		if strings.Contains(result.WhyLine, num) {
			t.Errorf("WhyLine contains raw count: %s", num)
		}
		if strings.Contains(result.CanonicalString(), num) {
			// Numbers might appear in hash, but not in the structured parts
			parts := strings.Split(result.CanonicalString(), "|")
			for i, part := range parts {
				if i < 4 && strings.Contains(part, num) { // Skip the statement part
					t.Errorf("Canonical string part %d contains raw count: %s", i, num)
				}
			}
		}
	}
}
