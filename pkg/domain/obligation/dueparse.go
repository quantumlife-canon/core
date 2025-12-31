// Package obligation - due date parsing module.
//
// CRITICAL: Deterministic parsing only. No fuzzy matching.
// Parses explicit date patterns from text using stdlib regexp.
//
// Supported patterns:
// - "by Friday", "by Monday" etc. (relative to reference date)
// - "by January 15", "by Jan 15" (absolute month-day)
// - "by 2024-01-15", "by 01/15/2024" (ISO and US formats)
// - "due: 15 Jan", "due: 15 January 2024"
// - "deadline: ..."
// - "action required by ..."
package obligation

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DueParseResult contains the result of due date extraction.
type DueParseResult struct {
	Found      bool
	DueDate    time.Time
	Confidence float64
	Pattern    string // Which pattern matched
}

// dueDatePatterns defines regex patterns for due date extraction.
// Order matters: more specific patterns first.
var dueDatePatterns = []struct {
	pattern    *regexp.Regexp
	name       string
	confidence float64
}{
	// ISO format: 2024-01-15
	{
		pattern:    regexp.MustCompile(`(?i)(?:by|due|deadline)[:\s]+(\d{4})-(\d{2})-(\d{2})`),
		name:       "iso_date",
		confidence: 0.95,
	},
	// US format: 01/15/2024 or 1/15/2024
	{
		pattern:    regexp.MustCompile(`(?i)(?:by|due|deadline)[:\s]+(\d{1,2})/(\d{1,2})/(\d{4})`),
		name:       "us_date",
		confidence: 0.90,
	},
	// UK format: 15/01/2024
	{
		pattern:    regexp.MustCompile(`(?i)(?:by|due|deadline)[:\s]+(\d{1,2})/(\d{1,2})/(\d{4})`),
		name:       "uk_date",
		confidence: 0.85, // Lower because ambiguous with US
	},
	// "by January 15, 2024" or "by Jan 15, 2024"
	{
		pattern:    regexp.MustCompile(`(?i)(?:by|due|deadline)[:\s]+(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\s+(\d{1,2})(?:st|nd|rd|th)?(?:,?\s*(\d{4}))?`),
		name:       "month_day_year",
		confidence: 0.90,
	},
	// "by 15 January 2024" or "by 15 Jan"
	{
		pattern:    regexp.MustCompile(`(?i)(?:by|due|deadline)[:\s]+(\d{1,2})(?:st|nd|rd|th)?\s+(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)(?:\s*(\d{4}))?`),
		name:       "day_month_year",
		confidence: 0.90,
	},
	// Relative weekday: "by Friday", "by next Monday"
	{
		pattern:    regexp.MustCompile(`(?i)(?:by|due|deadline)[:\s]+(?:next\s+)?(monday|tuesday|wednesday|thursday|friday|saturday|sunday)`),
		name:       "weekday",
		confidence: 0.80,
	},
	// "action required by ..."
	{
		pattern:    regexp.MustCompile(`(?i)action\s+required\s+by[:\s]+(.*?)(?:\.|$)`),
		name:       "action_required",
		confidence: 0.85,
	},
	// EOD/EOW/EOM patterns
	{
		pattern:    regexp.MustCompile(`(?i)(?:by|due|deadline)[:\s]+(eod|end\s+of\s+day|eow|end\s+of\s+week|eom|end\s+of\s+month)`),
		name:       "relative_period",
		confidence: 0.85,
	},
	// "within X days"
	{
		pattern:    regexp.MustCompile(`(?i)within\s+(\d+)\s+days?`),
		name:       "within_days",
		confidence: 0.80,
	},
}

// monthMap maps month names to numbers.
var monthMap = map[string]time.Month{
	"jan": time.January, "january": time.January,
	"feb": time.February, "february": time.February,
	"mar": time.March, "march": time.March,
	"apr": time.April, "april": time.April,
	"may": time.May,
	"jun": time.June, "june": time.June,
	"jul": time.July, "july": time.July,
	"aug": time.August, "august": time.August,
	"sep": time.September, "september": time.September,
	"oct": time.October, "october": time.October,
	"nov": time.November, "november": time.November,
	"dec": time.December, "december": time.December,
}

// weekdayMap maps weekday names to time.Weekday.
var weekdayMap = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
}

// ParseDueDate extracts a due date from text.
// Reference time is used for relative dates (e.g., "by Friday").
// Returns DueParseResult with Found=false if no date found.
func ParseDueDate(text string, reference time.Time) DueParseResult {
	text = strings.TrimSpace(text)
	if text == "" {
		return DueParseResult{Found: false}
	}

	for _, p := range dueDatePatterns {
		matches := p.pattern.FindStringSubmatch(text)
		if matches == nil {
			continue
		}

		result := parseMatchedDate(p.name, matches, reference)
		if result.Found {
			result.Confidence = p.confidence
			result.Pattern = p.name
			return result
		}
	}

	return DueParseResult{Found: false}
}

// parseMatchedDate interprets regex matches based on pattern type.
func parseMatchedDate(patternName string, matches []string, reference time.Time) DueParseResult {
	switch patternName {
	case "iso_date":
		// matches: [full, year, month, day]
		if len(matches) < 4 {
			return DueParseResult{Found: false}
		}
		year, _ := strconv.Atoi(matches[1])
		month, _ := strconv.Atoi(matches[2])
		day, _ := strconv.Atoi(matches[3])
		if year > 0 && month >= 1 && month <= 12 && day >= 1 && day <= 31 {
			return DueParseResult{
				Found:   true,
				DueDate: time.Date(year, time.Month(month), day, 23, 59, 59, 0, reference.Location()),
			}
		}

	case "us_date":
		// matches: [full, month, day, year]
		if len(matches) < 4 {
			return DueParseResult{Found: false}
		}
		month, _ := strconv.Atoi(matches[1])
		day, _ := strconv.Atoi(matches[2])
		year, _ := strconv.Atoi(matches[3])
		if year > 0 && month >= 1 && month <= 12 && day >= 1 && day <= 31 {
			return DueParseResult{
				Found:   true,
				DueDate: time.Date(year, time.Month(month), day, 23, 59, 59, 0, reference.Location()),
			}
		}

	case "month_day_year":
		// matches: [full, month_name, day, year?]
		if len(matches) < 3 {
			return DueParseResult{Found: false}
		}
		monthName := strings.ToLower(matches[1])
		month, ok := monthMap[monthName]
		if !ok {
			return DueParseResult{Found: false}
		}
		day, _ := strconv.Atoi(matches[2])
		year := reference.Year()
		if len(matches) >= 4 && matches[3] != "" {
			year, _ = strconv.Atoi(matches[3])
		}
		// If date is in the past and no explicit year provided, assume next year
		candidate := time.Date(year, month, day, 23, 59, 59, 0, reference.Location())
		yearNotProvided := len(matches) < 4 || matches[3] == ""
		if candidate.Before(reference) && yearNotProvided {
			candidate = candidate.AddDate(1, 0, 0)
		}
		return DueParseResult{
			Found:   true,
			DueDate: candidate,
		}

	case "day_month_year":
		// matches: [full, day, month_name, year?]
		if len(matches) < 3 {
			return DueParseResult{Found: false}
		}
		day, _ := strconv.Atoi(matches[1])
		monthName := strings.ToLower(matches[2])
		month, ok := monthMap[monthName]
		if !ok {
			return DueParseResult{Found: false}
		}
		year := reference.Year()
		if len(matches) >= 4 && matches[3] != "" {
			year, _ = strconv.Atoi(matches[3])
		}
		// If date is in the past and no explicit year provided, assume next year
		candidate := time.Date(year, month, day, 23, 59, 59, 0, reference.Location())
		yearNotProvided := len(matches) < 4 || matches[3] == ""
		if candidate.Before(reference) && yearNotProvided {
			candidate = candidate.AddDate(1, 0, 0)
		}
		return DueParseResult{
			Found:   true,
			DueDate: candidate,
		}

	case "weekday":
		// matches: [full, weekday_name]
		if len(matches) < 2 {
			return DueParseResult{Found: false}
		}
		weekdayName := strings.ToLower(matches[1])
		targetWeekday, ok := weekdayMap[weekdayName]
		if !ok {
			return DueParseResult{Found: false}
		}
		dueDate := nextWeekday(reference, targetWeekday)
		return DueParseResult{
			Found:   true,
			DueDate: dueDate,
		}

	case "relative_period":
		// matches: [full, period]
		if len(matches) < 2 {
			return DueParseResult{Found: false}
		}
		period := strings.ToLower(strings.ReplaceAll(matches[1], " ", ""))
		var dueDate time.Time
		switch period {
		case "eod", "endofday":
			dueDate = time.Date(reference.Year(), reference.Month(), reference.Day(),
				23, 59, 59, 0, reference.Location())
		case "eow", "endofweek":
			dueDate = nextWeekday(reference, time.Friday)
		case "eom", "endofmonth":
			// Last day of current month
			nextMonth := time.Date(reference.Year(), reference.Month()+1, 1,
				0, 0, 0, 0, reference.Location())
			dueDate = nextMonth.Add(-time.Second)
		default:
			return DueParseResult{Found: false}
		}
		return DueParseResult{
			Found:   true,
			DueDate: dueDate,
		}

	case "within_days":
		// matches: [full, days]
		if len(matches) < 2 {
			return DueParseResult{Found: false}
		}
		days, err := strconv.Atoi(matches[1])
		if err != nil || days <= 0 {
			return DueParseResult{Found: false}
		}
		dueDate := reference.AddDate(0, 0, days)
		dueDate = time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(),
			23, 59, 59, 0, reference.Location())
		return DueParseResult{
			Found:   true,
			DueDate: dueDate,
		}

	case "action_required":
		// Recursive parse on the captured group
		if len(matches) < 2 {
			return DueParseResult{Found: false}
		}
		// Try to parse the captured text
		subText := "by " + strings.TrimSpace(matches[1])
		return ParseDueDate(subText, reference)
	}

	return DueParseResult{Found: false}
}

// nextWeekday returns the next occurrence of the given weekday.
// If today is the target weekday, returns next week.
func nextWeekday(from time.Time, target time.Weekday) time.Time {
	current := from.Weekday()
	daysUntil := int(target - current)
	if daysUntil <= 0 {
		daysUntil += 7
	}
	result := from.AddDate(0, 0, daysUntil)
	return time.Date(result.Year(), result.Month(), result.Day(),
		23, 59, 59, 0, from.Location())
}

// HasDueCues checks if text contains due-date indicator keywords.
// Quick check before expensive regex parsing.
func HasDueCues(text string) bool {
	lower := strings.ToLower(text)
	cues := []string{
		"by ", "due", "deadline", "action required",
		"within", "eod", "eow", "eom", "end of",
	}
	for _, cue := range cues {
		if strings.Contains(lower, cue) {
			return true
		}
	}
	return false
}
