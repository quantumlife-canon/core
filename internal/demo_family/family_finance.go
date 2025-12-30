// v8.6 Family Financial Intersections Demo
//
// This demo shows how multiple family circles share a neutral financial view
// through an finance. All parties see identical data (symmetric).
//
// CRITICAL:
// - READ + PROPOSE ONLY. No execution.
// - No urgency, fear, shame, authority, or optimization language.
// - Silence is a valid response.
// - Dismissals are permanent.

package demo_family

import (
	"fmt"
	"strings"
	"time"

	"quantumlife/internal/finance/neutrality"
	"quantumlife/internal/finance/sharedview"
	"quantumlife/pkg/primitives/finance"
)

// FamilyFinanceDemo demonstrates v8.6 shared financial views.
type FamilyFinanceDemo struct {
	viewBuilder        *sharedview.ViewBuilder
	proposalGenerator  *sharedview.ProposalGenerator
	neutralityVerifier *neutrality.NeutralityVerifier
	languageChecker    *neutrality.LanguageNeutralityCheck
	printer            *Printer
}

// NewFamilyFinanceDemo creates a new demo instance.
func NewFamilyFinanceDemo(printer *Printer) *FamilyFinanceDemo {
	idCounter := 0
	idGen := func() string {
		idCounter++
		return fmt.Sprintf("demo_%d", idCounter)
	}

	return &FamilyFinanceDemo{
		viewBuilder:        sharedview.NewViewBuilder(idGen),
		proposalGenerator:  sharedview.NewProposalGenerator(idGen),
		neutralityVerifier: neutrality.NewNeutralityVerifier(),
		languageChecker:    neutrality.NewLanguageChecker(),
		printer:            printer,
	}
}

// Run executes the family finance demo.
func (d *FamilyFinanceDemo) Run() error {
	d.printer.Header("v8.6 Family Financial Intersections Demo")
	d.printer.Info("This demo shows how family circles share neutral financial views")
	d.printer.Info("")

	// 1. Create the family intersection
	d.printer.SubHeader("1. Family Intersection Setup")
	policy := d.createFamilyPolicy()
	d.printPolicy(policy)

	// 2. Simulate family members' financial data
	d.printer.SubHeader("2. Family Members' Financial Data (Private)")
	contributions := d.createFamilyContributions()
	d.printContributions(contributions)

	// 3. Build the shared view
	d.printer.SubHeader("3. Building Shared Financial View")
	view, err := d.buildSharedView(policy, contributions)
	if err != nil {
		return fmt.Errorf("failed to build view: %w", err)
	}
	d.printSharedView(view)

	// 4. Verify symmetry
	d.printer.SubHeader("4. Symmetry Verification")
	proof := d.verifySymmetry(view)
	d.printSymmetryProof(proof)

	// 5. Generate proposals (neutral language only)
	d.printer.SubHeader("5. Generating Neutral Proposals")
	proposals := d.generateProposals(view)
	d.printProposals(proposals)

	// 6. Demonstrate language checks
	d.printer.SubHeader("6. Language Neutrality Verification")
	d.demonstrateLanguageChecks()

	// 7. Demonstrate dismissal (permanent)
	d.printer.SubHeader("7. Proposal Dismissal (Permanent)")
	d.demonstrateDismissal(proposals)

	d.printer.Success("v8.6 Demo Complete - All parties receive identical views")
	return nil
}

func (d *FamilyFinanceDemo) createFamilyPolicy() finance.VisibilityPolicy {
	return finance.VisibilityPolicy{
		Enabled:             true,
		VisibilityLevel:     finance.VisibilityCategoryOnly, // Category summaries only
		AmountGranularity:   finance.GranularityBucketed,    // Bucketed amounts
		CategoriesAllowed:   []string{},                     // All categories
		AccountsIncluded:    []string{},                     // All accounts
		RequireSymmetry:     true,                           // CRITICAL: All parties see identical views
		ProposalAllowed:     true,
		ContributingCircles: []string{"alice", "bob", "charlie"},
		CurrencyDisplay:     "all",
	}
}

func (d *FamilyFinanceDemo) printPolicy(policy finance.VisibilityPolicy) {
	d.printer.Info("Policy Configuration:")
	d.printer.Info("  Visibility Level: %s", policy.VisibilityLevel)
	d.printer.Info("  Amount Granularity: %s", policy.AmountGranularity)
	d.printer.Info("  Require Symmetry: %v (CRITICAL)", policy.RequireSymmetry)
	d.printer.Info("  Proposals Allowed: %v", policy.ProposalAllowed)
	d.printer.Info("  Contributing Circles: %v", policy.ContributingCircles)
}

func (d *FamilyFinanceDemo) createFamilyContributions() []sharedview.CircleContribution {
	now := time.Now()

	return []sharedview.CircleContribution{
		{
			CircleID: "alice",
			SpendByCategory: map[string]map[string]int64{
				"USD": {
					"groceries":     35000, // $350
					"dining":        12000, // $120
					"entertainment": 8000,  // $80
					"utilities":     15000, // $150
				},
			},
			TotalsByCurrency: map[string]int64{"USD": 70000},
			TransactionCounts: map[string]map[string]int{
				"USD": {"groceries": 8, "dining": 4, "entertainment": 3, "utilities": 2},
			},
			LastSyncTime: now.Add(-30 * time.Minute),
		},
		{
			CircleID: "bob",
			SpendByCategory: map[string]map[string]int64{
				"USD": {
					"groceries":      28000, // $280
					"transportation": 45000, // $450
					"entertainment":  15000, // $150
				},
			},
			TotalsByCurrency: map[string]int64{"USD": 88000},
			TransactionCounts: map[string]map[string]int{
				"USD": {"groceries": 6, "transportation": 5, "entertainment": 4},
			},
			LastSyncTime: now.Add(-45 * time.Minute),
		},
		{
			CircleID: "charlie",
			SpendByCategory: map[string]map[string]int64{
				"USD": {
					"education":     120000, // $1200
					"entertainment": 5000,   // $50
				},
			},
			TotalsByCurrency: map[string]int64{"USD": 125000},
			TransactionCounts: map[string]map[string]int{
				"USD": {"education": 2, "entertainment": 2},
			},
			LastSyncTime: now.Add(-1 * time.Hour),
		},
	}
}

func (d *FamilyFinanceDemo) printContributions(contributions []sharedview.CircleContribution) {
	d.printer.Info("Individual data (NOT shared in view):")
	for _, c := range contributions {
		d.printer.Info("  %s:", c.CircleID)
		for currency, categories := range c.SpendByCategory {
			for cat, amt := range categories {
				d.printer.Info("    %s/%s: $%.2f", currency, cat, float64(amt)/100)
			}
		}
	}
	d.printer.Info("")
	d.printer.Info("NOTE: Individual amounts are NEVER exposed in the shared view")
}

func (d *FamilyFinanceDemo) buildSharedView(
	policy finance.VisibilityPolicy,
	contributions []sharedview.CircleContribution,
) (*sharedview.SharedFinancialView, error) {
	req := sharedview.BuildRequest{
		IntersectionID: "family_intersection_001",
		Policy:         policy,
		Contributions:  contributions,
		WindowStart:    time.Now().AddDate(0, 0, -30),
		WindowEnd:      time.Now(),
	}

	return d.viewBuilder.Build(req)
}

func (d *FamilyFinanceDemo) printSharedView(view *sharedview.SharedFinancialView) {
	d.printer.Info("Shared View (identical for all parties):")
	d.printer.Info("  View ID: %s", view.ViewID)
	d.printer.Info("  Content Hash: %s...", view.ContentHash[:16])
	d.printer.Info("")

	d.printer.Info("  Aggregated Spend by Category (USD):")
	if usdCategories, ok := view.SpendByCategory["USD"]; ok {
		for cat, cs := range usdCategories {
			if cs.Bucket != "" {
				d.printer.Info("    %s: %s (%.1f%%, %d transactions)",
					cat, bucketToDisplay(cs.Bucket), cs.PercentOfTotal, cs.TransactionCount)
			} else {
				d.printer.Info("    %s: $%.2f (%.1f%%, %d transactions)",
					cat, float64(cs.TotalCents)/100, cs.PercentOfTotal, cs.TransactionCount)
			}
		}
	}

	d.printer.Info("")
	d.printer.Info("  Provenance:")
	d.printer.Info("    Contributors: %v", view.Provenance.ContributingCircleIDs)
	d.printer.Info("    Contributor Count: %d", view.Provenance.ContributorCount)
	d.printer.Info("    Data Freshness: %s", view.Provenance.DataFreshness)
	d.printer.Info("    Symmetry Verified: %v", view.Provenance.SymmetryVerified)
}

func bucketToDisplay(bucket sharedview.AmountBucket) string {
	switch bucket {
	case sharedview.BucketLow:
		return "<$100"
	case sharedview.BucketMedium:
		return "$100-$500"
	case sharedview.BucketHigh:
		return "$500-$2000"
	case sharedview.BucketVeryHigh:
		return ">$2000"
	case sharedview.BucketHidden:
		return "[hidden]"
	default:
		return string(bucket)
	}
}

func (d *FamilyFinanceDemo) verifySymmetry(view *sharedview.SharedFinancialView) *neutrality.SymmetryProof {
	// Simulate all parties receiving the same view
	aliceView := *view
	bobView := *view
	charlieView := *view

	req := sharedview.VerifyRequest{
		View: view,
		PartyViews: map[string]*sharedview.SharedFinancialView{
			"alice":   &aliceView,
			"bob":     &bobView,
			"charlie": &charlieView,
		},
	}

	proof, _ := d.neutralityVerifier.Verify(req)
	return proof
}

func (d *FamilyFinanceDemo) printSymmetryProof(proof *neutrality.SymmetryProof) {
	d.printer.Info("Symmetry Proof:")
	d.printer.Info("  Proof ID: %s", proof.ProofID[:20]+"...")
	d.printer.Info("  Symmetric: %v", proof.Symmetric)
	d.printer.Info("  Party Hashes:")
	for party, hash := range proof.PartyHashes {
		d.printer.Info("    %s: %s...", party, hash[:16])
	}
	if proof.Symmetric {
		d.printer.Success("  All parties receive IDENTICAL views (verified)")
	} else {
		d.printer.Error("  ASYMMETRY DETECTED - this would be a critical error")
	}
}

func (d *FamilyFinanceDemo) generateProposals(view *sharedview.SharedFinancialView) []sharedview.SharedProposal {
	req := sharedview.GenerateRequest{
		View:           view,
		MaxProposals:   3,
		ExpiryDuration: 7 * 24 * time.Hour,
	}

	result := d.proposalGenerator.Generate(req)
	return result.Proposals
}

func (d *FamilyFinanceDemo) printProposals(proposals []sharedview.SharedProposal) {
	if len(proposals) == 0 {
		d.printer.Info("No proposals generated (silence is valid)")
		return
	}

	d.printer.Info("Generated Proposals (neutral language only):")
	for i, p := range proposals {
		d.printer.Info("  %d. [%s] %s", i+1, p.Type, p.Summary)
		if p.Details != "" {
			d.printer.Info("     Details: %s", p.Details)
		}
		d.printer.Info("     Action: %s | Priority: %d", p.ActionType, p.Priority)

		// Verify language neutrality
		violations := d.languageChecker.Check(p.Summary)
		if len(violations) > 0 {
			d.printer.Error("     LANGUAGE VIOLATION DETECTED")
		} else {
			d.printer.Success("     Language verified: neutral")
		}
	}
}

func (d *FamilyFinanceDemo) demonstrateLanguageChecks() {
	// Good examples (neutral)
	neutralExamples := []string{
		"Groceries represents approximately 25% of shared spending.",
		"This view includes data from 3 contributors.",
		"Consider discussing shared financial priorities when convenient.",
	}

	// Bad examples (violations)
	violatingExamples := []string{
		"You must reduce your excessive spending immediately!",
		"This concerning pattern is dangerous and needs urgent attention.",
		"You should optimize your budget to improve efficiency.",
	}

	d.printer.Info("Neutral Language Examples (PASS):")
	for _, text := range neutralExamples {
		violations := d.languageChecker.Check(text)
		if len(violations) == 0 {
			d.printer.Success("  OK: %s", text)
		}
	}

	d.printer.Info("")
	d.printer.Info("Violating Language Examples (BLOCKED):")
	for _, text := range violatingExamples {
		violations := d.languageChecker.Check(text)
		if len(violations) > 0 {
			categories := make(map[string]bool)
			for _, v := range violations {
				categories[v.Category] = true
			}
			cats := make([]string, 0, len(categories))
			for c := range categories {
				cats = append(cats, c)
			}
			d.printer.Error("  BLOCKED [%s]: %s", strings.Join(cats, ", "), text)
		}
	}
}

func (d *FamilyFinanceDemo) demonstrateDismissal(proposals []sharedview.SharedProposal) {
	if len(proposals) == 0 {
		d.printer.Info("No proposals to dismiss")
		return
	}

	// Simulate Alice dismissing the first proposal
	proposal := &proposals[0]
	d.printer.Info("Before dismissal:")
	d.printer.Info("  Proposal: %s", proposal.Summary)
	d.printer.Info("  Dismissed by: %v", proposal.DismissedBy)

	d.proposalGenerator.DismissProposal(proposal, "alice")

	d.printer.Info("")
	d.printer.Info("After Alice dismisses:")
	d.printer.Info("  Dismissed by: %v", proposal.DismissedBy)
	d.printer.Info("  Visible to Alice: %v", !d.proposalGenerator.IsDismissedBy(proposal, "alice"))
	d.printer.Info("  Visible to Bob: %v", !d.proposalGenerator.IsDismissedBy(proposal, "bob"))

	d.printer.Info("")
	d.printer.Warning("Dismissal is PERMANENT. Alice will never see this proposal again.")
}
