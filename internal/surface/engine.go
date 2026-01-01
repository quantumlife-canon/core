package surface

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Engine provides deterministic surface cue and page generation.
// Same inputs + same clock = same output, always.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new surface engine with injected clock.
func NewEngine(clock func() time.Time) *Engine {
	if clock == nil {
		clock = time.Now
	}
	return &Engine{clock: clock}
}

// cueTexts are deterministic texts for the subtle cue.
var cueTexts = map[bool]string{
	true:  "If you wanted to, there's one thing you could look at.",
	false: "",
}

// linkTexts are deterministic link texts.
var linkTexts = map[bool]string{
	true:  "View, if you like",
	false: "",
}

// reasonSummaries are abstract reason summaries by category.
// These must NOT contain identifiers.
var reasonSummaries = map[Category]string{
	CategoryMoney:  "We noticed a pattern that tends to become urgent if ignored.",
	CategoryTime:   "Something time-related is being held that you might want to know about.",
	CategoryWork:   "There's a work-related item we're watching quietly.",
	CategoryPeople: "We're holding something related to people in your life.",
	CategoryHome:   "A household matter is being held for you.",
}

// explainLines are abstract explainability bullets by category.
var explainLines = map[Category][]ExplainLine{
	CategoryMoney: {
		{Text: "This category has shown patterns before."},
		{Text: "We're watching it so you don't have to."},
		{Text: "No action is required from you."},
	},
	CategoryTime: {
		{Text: "Time-sensitive items are being monitored."},
		{Text: "Nothing needs your attention right now."},
		{Text: "We'll surface it if it becomes urgent."},
	},
	CategoryWork: {
		{Text: "Work items are being held quietly."},
		{Text: "We noticed activity that may be relevant later."},
		{Text: "You can ignore this completely if you prefer."},
	},
	CategoryPeople: {
		{Text: "People-related items are being watched."},
		{Text: "Nothing requires your response."},
		{Text: "We're here if you want to look."},
	},
	CategoryHome: {
		{Text: "Household matters are being tracked."},
		{Text: "Everything is under control."},
		{Text: "View only if you're curious."},
	},
}

// BuildCue generates the subtle availability cue for /today.
// Returns a cue with Available=false if nothing should be shown.
func (e *Engine) BuildCue(input SurfaceInput) SurfaceCue {
	now := e.clock()

	// Rule: Cue is available when:
	// 1. There exists ≥1 held category with magnitude a_few or several
	// 2. AND user preference is "quiet" (if show_all, they'll see it elsewhere)
	available := false

	if input.UserPreference == "quiet" {
		for _, mag := range input.HeldCategories {
			if mag == MagnitudeAFew || mag == MagnitudeSeveral {
				available = true
				break
			}
		}
	}

	cue := SurfaceCue{
		Available:   available,
		CueText:     cueTexts[available],
		LinkText:    linkTexts[available],
		GeneratedAt: now,
	}

	// Compute deterministic hash
	canonical := fmt.Sprintf(
		"cue|avail:%v|text:%s|link:%s|ts:%d",
		cue.Available,
		cue.CueText,
		cue.LinkText,
		now.Unix(),
	)
	h := sha256.Sum256([]byte(canonical))
	cue.Hash = hex.EncodeToString(h[:])

	return cue
}

// selectCategory deterministically selects the highest priority category to surface.
func (e *Engine) selectCategory(input SurfaceInput) (Category, bool) {
	// Priority order: money > time > work > people > home
	for _, cat := range CategoryPriority {
		if mag, ok := input.HeldCategories[cat]; ok {
			if mag == MagnitudeAFew || mag == MagnitudeSeveral {
				return cat, true
			}
		}
	}
	return "", false
}

// determineHorizon assigns a horizon bucket based on suppression signals.
func (e *Engine) determineHorizon(cat Category, input SurfaceInput) HorizonBucket {
	// Deterministic horizon assignment:
	// - money with suppressed finance => soon
	// - work with suppressed work => this_week
	// - else => later
	switch cat {
	case CategoryMoney:
		if input.SuppressedFinance {
			return HorizonSoon
		}
		return HorizonThisWeek
	case CategoryWork:
		if input.SuppressedWork {
			return HorizonThisWeek
		}
		return HorizonLater
	case CategoryTime:
		return HorizonThisWeek
	default:
		return HorizonLater
	}
}

// BuildSurfacePage generates the full surface page data.
func (e *Engine) BuildSurfacePage(input SurfaceInput, showExplain bool) SurfacePage {
	now := e.clock()

	// Select category to surface
	cat, found := e.selectCategory(input)
	if !found {
		// Nothing to surface - return empty page
		return SurfacePage{
			Title:       "Nothing to show",
			Subtitle:    "Everything is being handled quietly.",
			GeneratedAt: now,
			Hash:        computeHash(fmt.Sprintf("empty|%d", now.Unix())),
		}
	}

	// Build the surfaced item
	magnitude := input.HeldCategories[cat]
	horizon := e.determineHorizon(cat, input)
	reason := reasonSummaries[cat]
	explain := explainLines[cat]

	// Compute item key hash (deterministic, based on category + input hash)
	itemKeyCanonical := fmt.Sprintf("item|cat:%s|input:%s", cat, input.Hash())
	itemKeyHash := computeHash(itemKeyCanonical)

	item := SurfaceItem{
		Category:      cat,
		Magnitude:     magnitude,
		Horizon:       horizon,
		ReasonSummary: reason,
		Explain:       explain,
		ItemKeyHash:   itemKeyHash,
	}

	page := SurfacePage{
		Title:       "Something you could look at",
		Subtitle:    "No pressure. Just available if you want it.",
		Item:        item,
		ShowExplain: showExplain,
		GeneratedAt: now,
	}

	// Compute page hash
	pageCanonical := fmt.Sprintf(
		"page|title:%s|sub:%s|cat:%s|mag:%s|hor:%s|reason:%s|explain:%v|ts:%d",
		page.Title,
		page.Subtitle,
		item.Category,
		item.Magnitude,
		item.Horizon,
		item.ReasonSummary,
		showExplain,
		now.Unix(),
	)
	page.Hash = computeHash(pageCanonical)

	return page
}

// HasSurfaceableContent checks if there's anything that could be surfaced.
func (e *Engine) HasSurfaceableContent(input SurfaceInput) bool {
	_, found := e.selectCategory(input)
	return found
}
