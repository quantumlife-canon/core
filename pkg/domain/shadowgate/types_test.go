package shadowgate

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowllm"
)

func TestCandidateCanonicalStringStability(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	c := Candidate{
		ID:                   "test-id",
		Hash:                 "test-hash",
		PeriodKey:            "2024-01-15",
		CircleID:             identity.EntityID("circle-1"),
		Origin:               OriginShadowOnly,
		Category:             shadowllm.CategoryMoney,
		HorizonBucket:        shadowllm.HorizonSoon,
		MagnitudeBucket:      shadowllm.MagnitudeAFew,
		WhyGeneric:           "A pattern we have seen before.",
		UsefulnessPct:        75,
		UsefulnessBucket:     UsefulnessMedium,
		VoteConfidenceBucket: VoteConfidenceMedium,
		VotesUseful:          3,
		VotesUnnecessary:     1,
		FirstSeenBucket:      "2024-01-14",
		LastSeenBucket:       "2024-01-15",
		CreatedAt:            fixedTime,
	}

	// Get canonical string twice
	s1 := c.CanonicalString()
	s2 := c.CanonicalString()

	if s1 != s2 {
		t.Errorf("Canonical string not stable: %s vs %s", s1, s2)
	}

	// Verify expected format
	expected := "SHADOW_CANDIDATE|v1|2024-01-15|circle-1|shadow_only|money|soon|a_few|A pattern we have seen before.|75|medium|medium|3|1|2024-01-14|2024-01-15|2024-01-15T10:30:00Z"
	if s1 != expected {
		t.Errorf("Canonical string mismatch:\ngot:  %s\nwant: %s", s1, expected)
	}
}

func TestCandidateHashDeterminism(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	c := Candidate{
		PeriodKey:            "2024-01-15",
		CircleID:             identity.EntityID("circle-1"),
		Origin:               OriginShadowOnly,
		Category:             shadowllm.CategoryMoney,
		HorizonBucket:        shadowllm.HorizonSoon,
		MagnitudeBucket:      shadowllm.MagnitudeAFew,
		WhyGeneric:           "A pattern we have seen before.",
		UsefulnessPct:        75,
		UsefulnessBucket:     UsefulnessMedium,
		VoteConfidenceBucket: VoteConfidenceMedium,
		VotesUseful:          3,
		VotesUnnecessary:     1,
		FirstSeenBucket:      "2024-01-14",
		LastSeenBucket:       "2024-01-15",
		CreatedAt:            fixedTime,
	}

	h1 := c.ComputeHash()
	h2 := c.ComputeHash()

	if h1 != h2 {
		t.Errorf("Hash not deterministic: %s vs %s", h1, h2)
	}

	// Hash should be 64 hex chars
	if len(h1) != 64 {
		t.Errorf("Hash length should be 64, got %d", len(h1))
	}
}

func TestCandidateIDDeterminism(t *testing.T) {
	c := Candidate{
		CircleID:        identity.EntityID("circle-1"),
		Origin:          OriginShadowOnly,
		Category:        shadowllm.CategoryMoney,
		HorizonBucket:   shadowllm.HorizonSoon,
		MagnitudeBucket: shadowllm.MagnitudeAFew,
		WhyGeneric:      "A pattern we have seen before.",
	}

	id1 := c.ComputeID()
	id2 := c.ComputeID()

	if id1 != id2 {
		t.Errorf("ID not deterministic: %s vs %s", id1, id2)
	}

	// ID should be 32 hex chars
	if len(id1) != 32 {
		t.Errorf("ID length should be 32, got %d", len(id1))
	}
}

func TestCandidateSortingOrder(t *testing.T) {
	now := time.Now()

	candidates := []Candidate{
		{
			UsefulnessBucket: UsefulnessLow,
			Origin:           OriginShadowOnly,
			Hash:             "aaa",
			CreatedAt:        now,
		},
		{
			UsefulnessBucket: UsefulnessHigh,
			Origin:           OriginCanonOnly,
			Hash:             "bbb",
			CreatedAt:        now,
		},
		{
			UsefulnessBucket: UsefulnessHigh,
			Origin:           OriginShadowOnly,
			Hash:             "ccc",
			CreatedAt:        now,
		},
		{
			UsefulnessBucket: UsefulnessMedium,
			Origin:           OriginConflict,
			Hash:             "ddd",
			CreatedAt:        now,
		},
	}

	SortCandidates(candidates)

	// Expected order:
	// 1. High + ShadowOnly (ccc)
	// 2. High + CanonOnly (bbb)
	// 3. Medium + Conflict (ddd)
	// 4. Low + ShadowOnly (aaa)

	expected := []string{"ccc", "bbb", "ddd", "aaa"}
	for i, c := range candidates {
		if c.Hash != expected[i] {
			t.Errorf("Position %d: expected hash %s, got %s", i, expected[i], c.Hash)
		}
	}
}

func TestCandidateSortingStability(t *testing.T) {
	now := time.Now()

	// Create candidates with same usefulness and origin
	candidates := []Candidate{
		{UsefulnessBucket: UsefulnessHigh, Origin: OriginShadowOnly, Hash: "zzz", CreatedAt: now},
		{UsefulnessBucket: UsefulnessHigh, Origin: OriginShadowOnly, Hash: "aaa", CreatedAt: now},
		{UsefulnessBucket: UsefulnessHigh, Origin: OriginShadowOnly, Hash: "mmm", CreatedAt: now},
	}

	SortCandidates(candidates)

	// Should be sorted by hash ascending
	expected := []string{"aaa", "mmm", "zzz"}
	for i, c := range candidates {
		if c.Hash != expected[i] {
			t.Errorf("Position %d: expected hash %s, got %s", i, expected[i], c.Hash)
		}
	}

	// Sort again to verify stability
	SortCandidates(candidates)
	for i, c := range candidates {
		if c.Hash != expected[i] {
			t.Errorf("After re-sort, position %d: expected hash %s, got %s", i, expected[i], c.Hash)
		}
	}
}

func TestPromotionIntentCanonicalString(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 35, 0, 0, time.UTC)

	p := PromotionIntent{
		IntentID:      "intent-1",
		IntentHash:    "intent-hash",
		CandidateID:   "candidate-1",
		CandidateHash: "candidate-hash",
		PeriodKey:     "2024-01-15",
		NoteCode:      NotePromoteRule,
		CreatedBucket: "2024-01-15T10:35",
		CreatedAt:     fixedTime,
	}

	s1 := p.CanonicalString()
	s2 := p.CanonicalString()

	if s1 != s2 {
		t.Errorf("Canonical string not stable: %s vs %s", s1, s2)
	}

	expected := "PROMOTION_INTENT|v1|candidate-1|candidate-hash|2024-01-15|promote_rule|2024-01-15T10:35|2024-01-15T10:35:00Z"
	if s1 != expected {
		t.Errorf("Canonical string mismatch:\ngot:  %s\nwant: %s", s1, expected)
	}
}

func TestPromotionIntentHashDeterminism(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 35, 0, 0, time.UTC)

	p := PromotionIntent{
		CandidateID:   "candidate-1",
		CandidateHash: "candidate-hash",
		PeriodKey:     "2024-01-15",
		NoteCode:      NotePromoteRule,
		CreatedBucket: "2024-01-15T10:35",
		CreatedAt:     fixedTime,
	}

	h1 := p.ComputeHash()
	h2 := p.ComputeHash()

	if h1 != h2 {
		t.Errorf("Hash not deterministic: %s vs %s", h1, h2)
	}
}

func TestUsefulnessBucketFromPct(t *testing.T) {
	tests := []struct {
		pct    int
		bucket UsefulnessBucket
	}{
		{-1, UsefulnessUnknown},
		{0, UsefulnessLow},
		{24, UsefulnessLow},
		{25, UsefulnessMedium},
		{50, UsefulnessMedium},
		{75, UsefulnessMedium},
		{76, UsefulnessHigh},
		{100, UsefulnessHigh},
	}

	for _, tt := range tests {
		got := UsefulnessBucketFromPct(tt.pct)
		if got != tt.bucket {
			t.Errorf("UsefulnessBucketFromPct(%d) = %s, want %s", tt.pct, got, tt.bucket)
		}
	}
}

func TestVoteConfidenceBucketFromCount(t *testing.T) {
	tests := []struct {
		count  int
		bucket VoteConfidenceBucket
	}{
		{0, VoteConfidenceUnknown},
		{1, VoteConfidenceLow},
		{2, VoteConfidenceLow},
		{3, VoteConfidenceMedium},
		{5, VoteConfidenceMedium},
		{6, VoteConfidenceHigh},
		{10, VoteConfidenceHigh},
	}

	for _, tt := range tests {
		got := VoteConfidenceBucketFromCount(tt.count)
		if got != tt.bucket {
			t.Errorf("VoteConfidenceBucketFromCount(%d) = %s, want %s", tt.count, got, tt.bucket)
		}
	}
}

func TestPeriodKeyFromTime(t *testing.T) {
	tt := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	got := PeriodKeyFromTime(tt)
	expected := "2024-01-15"
	if got != expected {
		t.Errorf("PeriodKeyFromTime() = %s, want %s", got, expected)
	}
}

func TestFiveMinuteBucket(t *testing.T) {
	tests := []struct {
		time     time.Time
		expected string
	}{
		{time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), "2024-01-15T10:0"},
		{time.Date(2024, 1, 15, 10, 4, 59, 0, time.UTC), "2024-01-15T10:0"},
		{time.Date(2024, 1, 15, 10, 5, 0, 0, time.UTC), "2024-01-15T10:5"},
		{time.Date(2024, 1, 15, 10, 59, 0, 0, time.UTC), "2024-01-15T10:55"},
	}

	for _, tt := range tests {
		got := FiveMinuteBucket(tt.time)
		if got != tt.expected {
			t.Errorf("FiveMinuteBucket(%v) = %s, want %s", tt.time, got, tt.expected)
		}
	}
}

func TestCandidateValidation(t *testing.T) {
	now := time.Now()
	valid := Candidate{
		PeriodKey:            "2024-01-15",
		CircleID:             identity.EntityID("circle-1"),
		Origin:               OriginShadowOnly,
		Category:             shadowllm.CategoryMoney,
		HorizonBucket:        shadowllm.HorizonSoon,
		MagnitudeBucket:      shadowllm.MagnitudeAFew,
		WhyGeneric:           "A pattern.",
		UsefulnessPct:        50,
		UsefulnessBucket:     UsefulnessMedium,
		VoteConfidenceBucket: VoteConfidenceLow,
		VotesUseful:          1,
		VotesUnnecessary:     1,
		CreatedAt:            now,
	}

	if err := valid.Validate(); err != nil {
		t.Errorf("Valid candidate should pass: %v", err)
	}

	// Test missing period key
	c := valid
	c.PeriodKey = ""
	if err := c.Validate(); err != ErrMissingPeriodKey {
		t.Errorf("Expected ErrMissingPeriodKey, got %v", err)
	}

	// Test invalid origin
	c = valid
	c.Origin = "invalid"
	if err := c.Validate(); err != ErrInvalidOrigin {
		t.Errorf("Expected ErrInvalidOrigin, got %v", err)
	}

	// Test invalid usefulness pct
	c = valid
	c.UsefulnessPct = 101
	if err := c.Validate(); err != ErrInvalidUsefulnessPct {
		t.Errorf("Expected ErrInvalidUsefulnessPct, got %v", err)
	}
}

func TestPromotionIntentValidation(t *testing.T) {
	now := time.Now()
	valid := PromotionIntent{
		CandidateID:   "candidate-1",
		CandidateHash: "hash-1",
		PeriodKey:     "2024-01-15",
		NoteCode:      NotePromoteRule,
		CreatedBucket: "2024-01-15T10:30",
		CreatedAt:     now,
	}

	if err := valid.Validate(); err != nil {
		t.Errorf("Valid intent should pass: %v", err)
	}

	// Test missing candidate ID
	p := valid
	p.CandidateID = ""
	if err := p.Validate(); err != ErrMissingCandidateID {
		t.Errorf("Expected ErrMissingCandidateID, got %v", err)
	}

	// Test invalid note code
	p = valid
	p.NoteCode = "invalid"
	if err := p.Validate(); err != ErrInvalidNoteCode {
		t.Errorf("Expected ErrInvalidNoteCode, got %v", err)
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{123, "123"},
		{-1, "-1"},
		{-123, "-123"},
	}

	for _, tt := range tests {
		got := itoa(tt.n)
		if got != tt.expected {
			t.Errorf("itoa(%d) = %s, want %s", tt.n, got, tt.expected)
		}
	}
}
