package neutrality

import (
	"testing"
	"time"

	"quantumlife/internal/finance/sharedview"
	"quantumlife/pkg/primitives/finance"
)

func TestNeutralityVerifier_Symmetric(t *testing.T) {
	verifier := NewNeutralityVerifier()

	// Create a reference view
	view := &sharedview.SharedFinancialView{
		IntersectionID: "int_123",
		ViewID:         "view_456",
		ContentHash:    "abc123hash",
		Provenance: sharedview.ViewProvenance{
			ContributingCircleIDs: []string{"alice", "bob"},
			ContributorCount:      2,
			SymmetryVerified:      true,
		},
	}

	// All parties get identical views
	req := sharedview.VerifyRequest{
		View: view,
		PartyViews: map[string]*sharedview.SharedFinancialView{
			"alice": {ContentHash: "abc123hash"},
			"bob":   {ContentHash: "abc123hash"},
		},
	}

	proof, err := verifier.Verify(req)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if !proof.Symmetric {
		t.Error("expected symmetric result")
	}
	if len(proof.Discrepancies) != 0 {
		t.Errorf("expected no discrepancies, got %d", len(proof.Discrepancies))
	}
	if proof.ProofID == "" {
		t.Error("proof ID should be set")
	}
	if proof.ProofHash == "" {
		t.Error("proof hash should be set")
	}
}

func TestNeutralityVerifier_Asymmetric(t *testing.T) {
	verifier := NewNeutralityVerifier()

	view := &sharedview.SharedFinancialView{
		IntersectionID: "int_123",
		ViewID:         "view_456",
		ContentHash:    "abc123hash",
	}

	// Bob gets a different view
	req := sharedview.VerifyRequest{
		View: view,
		PartyViews: map[string]*sharedview.SharedFinancialView{
			"alice": {ContentHash: "abc123hash"},
			"bob":   {ContentHash: "different_hash"}, // Different!
		},
	}

	proof, err := verifier.Verify(req)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if proof.Symmetric {
		t.Error("expected asymmetric result")
	}
	if len(proof.Discrepancies) == 0 {
		t.Error("expected discrepancies")
	}

	// Check discrepancy details
	found := false
	for _, d := range proof.Discrepancies {
		if d.PartyB == "bob" && d.PartyBHash == "different_hash" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected bob discrepancy")
	}
}

func TestNeutralityVerifier_QuickVerify(t *testing.T) {
	verifier := NewNeutralityVerifier()

	view := &sharedview.SharedFinancialView{
		ContentHash: "expected_hash",
	}

	if !verifier.QuickVerify(view, "expected_hash") {
		t.Error("quick verify should pass with matching hash")
	}

	if verifier.QuickVerify(view, "wrong_hash") {
		t.Error("quick verify should fail with different hash")
	}
}

func TestLanguageChecker_NeutralText(t *testing.T) {
	checker := NewLanguageChecker()

	neutralTexts := []string{
		"Groceries represents approximately 45% of shared spending.",
		"This view includes data from 3 contributors.",
		"Consider discussing shared financial priorities when convenient.",
		"Spending is spread across many categories.",
	}

	for _, text := range neutralTexts {
		violations := checker.Check(text)
		if len(violations) > 0 {
			t.Errorf("text should be neutral: %q, got violations: %v", text, violations)
		}
	}
}

func TestLanguageChecker_UrgencyViolation(t *testing.T) {
	checker := NewLanguageChecker()

	text := "You need to act immediately on this urgent matter."
	violations := checker.Check(text)

	if len(violations) == 0 {
		t.Error("expected urgency violations")
	}

	categories := make(map[string]bool)
	for _, v := range violations {
		categories[v.Category] = true
	}

	if !categories["urgency"] {
		t.Error("expected urgency category violation")
	}
}

func TestLanguageChecker_FearViolation(t *testing.T) {
	checker := NewLanguageChecker()

	text := "This concerning pattern poses a dangerous risk."
	violations := checker.Check(text)

	if len(violations) == 0 {
		t.Error("expected fear violations")
	}

	foundFear := false
	for _, v := range violations {
		if v.Category == "fear" {
			foundFear = true
			break
		}
	}
	if !foundFear {
		t.Error("expected fear category violation")
	}
}

func TestLanguageChecker_ShameViolation(t *testing.T) {
	checker := NewLanguageChecker()

	text := "Excessive spending and wasteful purchases."
	violations := checker.Check(text)

	if len(violations) == 0 {
		t.Error("expected shame violations")
	}

	foundShame := false
	for _, v := range violations {
		if v.Category == "shame" {
			foundShame = true
			break
		}
	}
	if !foundShame {
		t.Error("expected shame category violation")
	}
}

func TestLanguageChecker_AuthorityViolation(t *testing.T) {
	checker := NewLanguageChecker()

	text := "You must reduce spending and should cut back."
	violations := checker.Check(text)

	if len(violations) == 0 {
		t.Error("expected authority violations")
	}

	foundAuthority := false
	for _, v := range violations {
		if v.Category == "authority" {
			foundAuthority = true
			break
		}
	}
	if !foundAuthority {
		t.Error("expected authority category violation")
	}
}

func TestLanguageChecker_OptimizationViolation(t *testing.T) {
	checker := NewLanguageChecker()

	text := "You could optimize and improve your budget."
	violations := checker.Check(text)

	if len(violations) == 0 {
		t.Error("expected optimization violations")
	}

	foundOptimization := false
	for _, v := range violations {
		if v.Category == "optimization" {
			foundOptimization = true
			break
		}
	}
	if !foundOptimization {
		t.Error("expected optimization category violation")
	}
}

func TestLanguageChecker_ViolationContext(t *testing.T) {
	checker := NewLanguageChecker()

	text := "This is an urgent matter that requires attention."
	violations := checker.Check(text)

	if len(violations) == 0 {
		t.Fatal("expected violations")
	}

	// Check that context is captured
	found := false
	for _, v := range violations {
		if v.Word == "urgent" {
			found = true
			if v.Position < 0 {
				t.Error("position should be set")
			}
			if v.Context == "" {
				t.Error("context should be set")
			}
			break
		}
	}
	if !found {
		t.Error("expected urgent word violation")
	}
}

func TestViewBuilder_SymmetryHash(t *testing.T) {
	// Integration test: verify that the same inputs produce the same hash
	builder := sharedview.NewViewBuilder(func() string { return "test-id" })

	policy := finance.VisibilityPolicy{
		Enabled:           true,
		RequireSymmetry:   true,
		AmountGranularity: finance.GranularityExact,
	}

	contributions := []sharedview.CircleContribution{
		{
			CircleID: "alice",
			SpendByCategory: map[string]map[string]int64{
				"USD": {"groceries": 10000},
			},
			TotalsByCurrency: map[string]int64{"USD": 10000},
			LastSyncTime:     time.Now(),
		},
		{
			CircleID: "bob",
			SpendByCategory: map[string]map[string]int64{
				"USD": {"groceries": 15000},
			},
			TotalsByCurrency: map[string]int64{"USD": 15000},
			LastSyncTime:     time.Now(),
		},
	}

	windowStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	// Build view for Alice's perspective
	viewAlice, _ := builder.Build(sharedview.BuildRequest{
		IntersectionID: "int_123",
		Policy:         policy,
		Contributions:  contributions,
		WindowStart:    windowStart,
		WindowEnd:      windowEnd,
	})

	// Build view for Bob's perspective (same inputs, different order)
	viewBob, _ := builder.Build(sharedview.BuildRequest{
		IntersectionID: "int_123",
		Policy:         policy,
		Contributions:  []sharedview.CircleContribution{contributions[1], contributions[0]},
		WindowStart:    windowStart,
		WindowEnd:      windowEnd,
	})

	// Hashes should be identical
	if viewAlice.ContentHash != viewBob.ContentHash {
		t.Errorf("symmetry violation: alice hash %s != bob hash %s",
			viewAlice.ContentHash, viewBob.ContentHash)
	}

	// Verify with neutrality verifier
	verifier := NewNeutralityVerifier()
	proof, _ := verifier.Verify(sharedview.VerifyRequest{
		View: viewAlice,
		PartyViews: map[string]*sharedview.SharedFinancialView{
			"alice": viewAlice,
			"bob":   viewBob,
		},
	})

	if !proof.Symmetric {
		t.Error("views should be verified as symmetric")
	}
}
