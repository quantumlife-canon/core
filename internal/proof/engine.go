package proof

// Engine computes proof summaries deterministically.
// No goroutines, no time.Now(), stdlib only.
type Engine struct{}

// NewEngine creates a new proof engine.
func NewEngine() *Engine {
	return &Engine{}
}

// BuildProof computes a ProofSummary from input.
// Deterministic: same input => same output.
func (e *Engine) BuildProof(in ProofInput) ProofSummary {
	// If not in quiet mode, proof is unused
	if !in.PreferenceQuiet {
		return ProofSummary{
			Magnitude:  MagnitudeNothing,
			Categories: []Category{},
			Statement:  "",
			WhyLine:    "",
			Hash:       computeEmptyHash(),
		}
	}

	// Compute total suppressed and collect active categories
	total := 0
	var activeCategories []Category
	for cat, count := range in.SuppressedByCategory {
		if count > 0 {
			total += count
			activeCategories = append(activeCategories, cat)
		}
	}

	// Sort categories deterministically (lexicographic order)
	activeCategories = SortCategories(activeCategories)

	// Map total to magnitude bucket
	magnitude := bucketMagnitude(total)

	// Select statement based on magnitude
	statement := selectStatement(magnitude)

	// Select why line based on magnitude
	whyLine := selectWhyLine(magnitude)

	proof := ProofSummary{
		Magnitude:  magnitude,
		Categories: activeCategories,
		Statement:  statement,
		WhyLine:    whyLine,
	}
	proof.Hash = proof.ComputeHash()

	return proof
}

// bucketMagnitude converts raw count to magnitude bucket.
// Counts are used internally only - never exposed.
func bucketMagnitude(count int) Magnitude {
	switch {
	case count == 0:
		return MagnitudeNothing
	case count >= 1 && count <= 3:
		return MagnitudeAFew
	default:
		return MagnitudeSeveral
	}
}

// selectStatement returns the calm, abstract statement for magnitude.
func selectStatement(mag Magnitude) string {
	switch mag {
	case MagnitudeAFew:
		return "We chose not to interrupt you a few times."
	case MagnitudeSeveral:
		return "We chose not to interrupt you often."
	case MagnitudeNothing:
		return "Nothing needed holding."
	default:
		return ""
	}
}

// selectWhyLine returns the reassurance line for magnitude.
func selectWhyLine(mag Magnitude) string {
	switch mag {
	case MagnitudeAFew, MagnitudeSeveral:
		return "Quiet is a feature. Not a gap."
	default:
		return ""
	}
}

// computeEmptyHash returns hash for empty/unused proof.
func computeEmptyHash() string {
	empty := ProofSummary{
		Magnitude:  MagnitudeNothing,
		Categories: []Category{},
		Statement:  "",
		WhyLine:    "",
	}
	return empty.ComputeHash()
}

// ProofCue represents the whisper cue shown on /today.
type ProofCue struct {
	Available bool
	CueText   string
	LinkText  string
	ProofHash string
}

// BuildCue determines if and what cue to show on /today.
// Cue shows when:
// - preference is quiet
// - proof magnitude is not nothing
// - user hasn't dismissed/acknowledged proof recently
func (e *Engine) BuildCue(proof ProofSummary, hasRecentAck bool) ProofCue {
	// No cue if proof is nothing
	if proof.Magnitude == MagnitudeNothing {
		return ProofCue{Available: false}
	}

	// No cue if already acknowledged
	if hasRecentAck {
		return ProofCue{Available: false}
	}

	return ProofCue{
		Available: true,
		CueText:   "If you ever wondered—silence is intentional.",
		LinkText:  "Proof, if you want it.",
		ProofHash: proof.Hash,
	}
}
