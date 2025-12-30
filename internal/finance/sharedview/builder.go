package sharedview

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"quantumlife/pkg/primitives/finance"
)

// ViewBuilder constructs shared financial views from circle contributions.
//
// CRITICAL: All aggregation is deterministic and order-independent.
// No individual attribution in output.
type ViewBuilder struct {
	idGenerator func() string
}

// NewViewBuilder creates a new view builder.
func NewViewBuilder(idGen func() string) *ViewBuilder {
	return &ViewBuilder{
		idGenerator: idGen,
	}
}

// Build creates a SharedFinancialView from the contributions.
// The output is deterministic given the same inputs.
func (b *ViewBuilder) Build(req BuildRequest) (*SharedFinancialView, error) {
	if err := b.validateRequest(req); err != nil {
		return nil, err
	}

	view := &SharedFinancialView{
		IntersectionID:   req.IntersectionID,
		ViewID:           b.idGenerator(),
		GeneratedAt:      time.Now().UTC(),
		Policy:           req.Policy,
		WindowStart:      req.WindowStart,
		WindowEnd:        req.WindowEnd,
		SpendByCategory:  make(map[string]map[string]CategorySpend),
		TotalsByCurrency: make(map[string]CurrencyTotal),
		Observations:     []SharedObservation{},
	}

	// Aggregate contributions
	aggregatedSpend, aggregatedTotals, txCounts := b.aggregate(req.Contributions, req.Policy)

	// Apply visibility policy
	view.SpendByCategory = b.applyVisibility(aggregatedSpend, txCounts, aggregatedTotals, req.Policy)
	view.TotalsByCurrency = b.computeTotals(aggregatedTotals, txCounts, req.Policy)

	// Build provenance (no individual attribution)
	view.Provenance = b.buildProvenance(req.Contributions, req.Policy)

	// Compute content hash for symmetry verification
	view.ContentHash = b.computeContentHash(view)

	return view, nil
}

// validateRequest checks the build request is valid.
func (b *ViewBuilder) validateRequest(req BuildRequest) error {
	if req.IntersectionID == "" {
		return fmt.Errorf("intersection ID required")
	}
	if !req.Policy.Enabled {
		return fmt.Errorf("financial view policy not enabled")
	}
	if len(req.Contributions) == 0 {
		return fmt.Errorf("at least one contribution required")
	}
	if err := req.Policy.Validate(); err != nil {
		return fmt.Errorf("invalid policy: %w", err)
	}
	return nil
}

// aggregate combines contributions into totals.
// This is deterministic and order-independent.
func (b *ViewBuilder) aggregate(contributions []CircleContribution, policy finance.VisibilityPolicy) (
	spend map[string]map[string]int64, // currency -> category -> cents
	totals map[string]int64, // currency -> cents
	txCounts map[string]map[string]int, // currency -> category -> count
) {
	spend = make(map[string]map[string]int64)
	totals = make(map[string]int64)
	txCounts = make(map[string]map[string]int)

	for _, contrib := range contributions {
		// Check if this circle is in the contributing list (if specified)
		if len(policy.ContributingCircles) > 0 {
			found := false
			for _, cid := range policy.ContributingCircles {
				if cid == contrib.CircleID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Aggregate spend by category
		for currency, categories := range contrib.SpendByCategory {
			// Filter currency if policy specifies primary only
			if policy.CurrencyDisplay == "primary" && currency != "USD" {
				continue
			}

			if spend[currency] == nil {
				spend[currency] = make(map[string]int64)
			}
			if txCounts[currency] == nil {
				txCounts[currency] = make(map[string]int)
			}

			for category, amount := range categories {
				// Filter category if policy specifies allowed categories
				if len(policy.CategoriesAllowed) > 0 {
					allowed := false
					for _, ac := range policy.CategoriesAllowed {
						if ac == category {
							allowed = true
							break
						}
					}
					if !allowed {
						continue
					}
				}

				spend[currency][category] += amount

				// Add transaction counts if available
				if contrib.TransactionCounts != nil {
					if currCounts, ok := contrib.TransactionCounts[currency]; ok {
						if count, ok := currCounts[category]; ok {
							txCounts[currency][category] += count
						}
					}
				}
			}
		}

		// Aggregate totals
		for currency, total := range contrib.TotalsByCurrency {
			if policy.CurrencyDisplay == "primary" && currency != "USD" {
				continue
			}
			totals[currency] += total
		}
	}

	return spend, totals, txCounts
}

// applyVisibility applies the visibility policy to aggregated data.
func (b *ViewBuilder) applyVisibility(
	spend map[string]map[string]int64,
	txCounts map[string]map[string]int,
	totals map[string]int64,
	policy finance.VisibilityPolicy,
) map[string]map[string]CategorySpend {
	result := make(map[string]map[string]CategorySpend)

	for currency, categories := range spend {
		result[currency] = make(map[string]CategorySpend)
		currTotal := totals[currency]
		if currTotal == 0 {
			// Compute from categories
			for _, amt := range categories {
				currTotal += amt
			}
		}

		for category, amount := range categories {
			cs := CategorySpend{
				Category: category,
				Currency: currency,
			}

			// Apply amount granularity
			switch policy.AmountGranularity {
			case finance.GranularityExact:
				cs.TotalCents = amount
				cs.Bucket = "" // No bucket when exact
			case finance.GranularityBucketed:
				cs.TotalCents = 0 // Hide exact amount
				cs.Bucket = computeBucket(amount)
			case finance.GranularityHidden:
				cs.TotalCents = 0
				cs.Bucket = BucketHidden
			default:
				// Default to bucketed
				cs.TotalCents = 0
				cs.Bucket = computeBucket(amount)
			}

			// Add transaction count
			if currCounts, ok := txCounts[currency]; ok {
				cs.TransactionCount = currCounts[category]
			}

			// Compute percentage
			if currTotal > 0 {
				cs.PercentOfTotal = float64(amount) * 100 / float64(currTotal)
			}

			result[currency][category] = cs
		}
	}

	return result
}

// computeTotals computes currency totals with visibility applied.
func (b *ViewBuilder) computeTotals(
	totals map[string]int64,
	txCounts map[string]map[string]int,
	policy finance.VisibilityPolicy,
) map[string]CurrencyTotal {
	result := make(map[string]CurrencyTotal)

	for currency, total := range totals {
		ct := CurrencyTotal{
			Currency: currency,
		}

		// Apply amount granularity
		switch policy.AmountGranularity {
		case finance.GranularityExact:
			ct.TotalCents = total
			ct.Bucket = ""
		case finance.GranularityBucketed:
			ct.TotalCents = 0
			ct.Bucket = computeBucket(total)
		case finance.GranularityHidden:
			ct.TotalCents = 0
			ct.Bucket = BucketHidden
		default:
			ct.TotalCents = 0
			ct.Bucket = computeBucket(total)
		}

		// Sum transaction counts
		if currCounts, ok := txCounts[currency]; ok {
			for _, count := range currCounts {
				ct.TransactionCount += count
			}
		}

		result[currency] = ct
	}

	return result
}

// buildProvenance creates provenance without individual attribution.
func (b *ViewBuilder) buildProvenance(contributions []CircleContribution, policy finance.VisibilityPolicy) ViewProvenance {
	prov := ViewProvenance{
		ContributingCircleIDs: make([]string, 0, len(contributions)),
		ContributorCount:      0,
		DataFreshness:         FreshnessCurrent,
		SymmetryVerified:      policy.RequireSymmetry,
	}

	var oldestSync time.Time
	now := time.Now().UTC()

	for _, contrib := range contributions {
		// Check if this circle should be included
		if len(policy.ContributingCircles) > 0 {
			found := false
			for _, cid := range policy.ContributingCircles {
				if cid == contrib.CircleID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		prov.ContributingCircleIDs = append(prov.ContributingCircleIDs, contrib.CircleID)
		prov.ContributorCount++

		if oldestSync.IsZero() || contrib.LastSyncTime.Before(oldestSync) {
			oldestSync = contrib.LastSyncTime
		}
	}

	// Sort circle IDs for deterministic output
	sort.Strings(prov.ContributingCircleIDs)

	// Determine freshness
	if !oldestSync.IsZero() {
		prov.LastSyncTime = oldestSync
		age := now.Sub(oldestSync)
		switch {
		case age < time.Hour:
			prov.DataFreshness = FreshnessCurrent
		case age < 24*time.Hour:
			prov.DataFreshness = FreshnessRecent
		default:
			prov.DataFreshness = FreshnessStale
		}
	} else {
		prov.DataFreshness = FreshnessUnknown
	}

	return prov
}

// computeContentHash creates a deterministic hash of the view content.
// This is used to prove symmetry - all parties should get the same hash.
func (b *ViewBuilder) computeContentHash(view *SharedFinancialView) string {
	h := sha256.New()

	// Hash intersection ID
	h.Write([]byte(view.IntersectionID))

	// Hash window
	h.Write([]byte(view.WindowStart.Format(time.RFC3339)))
	h.Write([]byte(view.WindowEnd.Format(time.RFC3339)))

	// Hash spend by category (sorted for determinism)
	currencies := make([]string, 0, len(view.SpendByCategory))
	for c := range view.SpendByCategory {
		currencies = append(currencies, c)
	}
	sort.Strings(currencies)

	for _, currency := range currencies {
		h.Write([]byte(currency))
		categories := view.SpendByCategory[currency]

		catNames := make([]string, 0, len(categories))
		for c := range categories {
			catNames = append(catNames, c)
		}
		sort.Strings(catNames)

		for _, cat := range catNames {
			cs := categories[cat]
			h.Write([]byte(cat))
			h.Write([]byte(fmt.Sprintf("%d", cs.TotalCents)))
			h.Write([]byte(cs.Bucket))
			h.Write([]byte(fmt.Sprintf("%d", cs.TransactionCount)))
			h.Write([]byte(fmt.Sprintf("%.2f", cs.PercentOfTotal)))
		}
	}

	// Hash totals (sorted)
	for _, currency := range currencies {
		if ct, ok := view.TotalsByCurrency[currency]; ok {
			h.Write([]byte(fmt.Sprintf("%d", ct.TotalCents)))
			h.Write([]byte(ct.Bucket))
			h.Write([]byte(fmt.Sprintf("%d", ct.TransactionCount)))
		}
	}

	// Hash provenance
	for _, cid := range view.Provenance.ContributingCircleIDs {
		h.Write([]byte(cid))
	}

	return hex.EncodeToString(h.Sum(nil))
}

// computeBucket determines the amount bucket.
// Buckets (in cents):
// - low: < 10000 ($100)
// - medium: 10000-50000 ($100-$500)
// - high: 50000-200000 ($500-$2000)
// - very_high: > 200000 ($2000+)
func computeBucket(amountCents int64) AmountBucket {
	// Use absolute value for bucketing
	if amountCents < 0 {
		amountCents = -amountCents
	}

	switch {
	case amountCents < 10000:
		return BucketLow
	case amountCents < 50000:
		return BucketMedium
	case amountCents < 200000:
		return BucketHigh
	default:
		return BucketVeryHigh
	}
}

// ComputeBucket is exported for testing and external use.
func ComputeBucket(amountCents int64) AmountBucket {
	return computeBucket(amountCents)
}
