package obligation

import (
	"testing"
	"time"
)

func TestParseDueDateISO(t *testing.T) {
	ref := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		text     string
		expected time.Time
		found    bool
	}{
		{"by 2025-01-20", time.Date(2025, 1, 20, 23, 59, 59, 0, time.UTC), true},
		{"due: 2025-02-14", time.Date(2025, 2, 14, 23, 59, 59, 0, time.UTC), true},
		{"deadline: 2025-12-31", time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC), true},
		{"by 2024-01-01", time.Date(2024, 1, 1, 23, 59, 59, 0, time.UTC), true},
		{"no date here", time.Time{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := ParseDueDate(tt.text, ref)
			if result.Found != tt.found {
				t.Errorf("Found = %v, want %v", result.Found, tt.found)
				return
			}
			if tt.found && !result.DueDate.Equal(tt.expected) {
				t.Errorf("DueDate = %v, want %v", result.DueDate, tt.expected)
			}
			if tt.found && result.Pattern != "iso_date" {
				t.Errorf("Pattern = %s, want iso_date", result.Pattern)
			}
		})
	}
}

func TestParseDueDateUSFormat(t *testing.T) {
	ref := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		text     string
		expected time.Time
	}{
		{"by 01/20/2025", time.Date(2025, 1, 20, 23, 59, 59, 0, time.UTC)},
		{"due: 12/25/2025", time.Date(2025, 12, 25, 23, 59, 59, 0, time.UTC)},
		{"deadline: 1/5/2025", time.Date(2025, 1, 5, 23, 59, 59, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := ParseDueDate(tt.text, ref)
			if !result.Found {
				t.Error("Expected to find date")
				return
			}
			if !result.DueDate.Equal(tt.expected) {
				t.Errorf("DueDate = %v, want %v", result.DueDate, tt.expected)
			}
		})
	}
}

func TestParseDueDateMonthDay(t *testing.T) {
	ref := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		text     string
		expected time.Time
	}{
		{"by January 20", time.Date(2025, 1, 20, 23, 59, 59, 0, time.UTC)},
		{"by Jan 25", time.Date(2025, 1, 25, 23, 59, 59, 0, time.UTC)},
		{"due: February 14, 2025", time.Date(2025, 2, 14, 23, 59, 59, 0, time.UTC)},
		{"by March 1st", time.Date(2025, 3, 1, 23, 59, 59, 0, time.UTC)},
		{"by December 25th, 2026", time.Date(2026, 12, 25, 23, 59, 59, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := ParseDueDate(tt.text, ref)
			if !result.Found {
				t.Error("Expected to find date")
				return
			}
			if !result.DueDate.Equal(tt.expected) {
				t.Errorf("DueDate = %v, want %v", result.DueDate, tt.expected)
			}
		})
	}
}

func TestParseDueDateDayMonth(t *testing.T) {
	ref := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		text     string
		expected time.Time
	}{
		{"by 20 January", time.Date(2025, 1, 20, 23, 59, 59, 0, time.UTC)},
		{"due: 14 Feb 2025", time.Date(2025, 2, 14, 23, 59, 59, 0, time.UTC)},
		{"deadline: 1st March", time.Date(2025, 3, 1, 23, 59, 59, 0, time.UTC)},
		{"by 25th December 2026", time.Date(2026, 12, 25, 23, 59, 59, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := ParseDueDate(tt.text, ref)
			if !result.Found {
				t.Error("Expected to find date")
				return
			}
			if !result.DueDate.Equal(tt.expected) {
				t.Errorf("DueDate = %v, want %v", result.DueDate, tt.expected)
			}
		})
	}
}

func TestParseDueDateWeekday(t *testing.T) {
	// Reference: Wednesday, January 15, 2025
	ref := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		text     string
		expected time.Time
	}{
		// Thursday Jan 16
		{"by Thursday", time.Date(2025, 1, 16, 23, 59, 59, 0, time.UTC)},
		// Friday Jan 17
		{"by Friday", time.Date(2025, 1, 17, 23, 59, 59, 0, time.UTC)},
		// Monday Jan 20
		{"by Monday", time.Date(2025, 1, 20, 23, 59, 59, 0, time.UTC)},
		// Next Wednesday Jan 22 (not same day)
		{"by Wednesday", time.Date(2025, 1, 22, 23, 59, 59, 0, time.UTC)},
		{"by next Monday", time.Date(2025, 1, 20, 23, 59, 59, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := ParseDueDate(tt.text, ref)
			if !result.Found {
				t.Error("Expected to find date")
				return
			}
			if !result.DueDate.Equal(tt.expected) {
				t.Errorf("DueDate = %v, want %v", result.DueDate, tt.expected)
			}
			if result.Pattern != "weekday" {
				t.Errorf("Pattern = %s, want weekday", result.Pattern)
			}
		})
	}
}

func TestParseDueDateRelativePeriod(t *testing.T) {
	// Reference: Wednesday, January 15, 2025, 10:00
	ref := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		text     string
		expected time.Time
	}{
		// EOD
		{"by EOD", time.Date(2025, 1, 15, 23, 59, 59, 0, time.UTC)},
		{"due: end of day", time.Date(2025, 1, 15, 23, 59, 59, 0, time.UTC)},
		// EOW (next Friday)
		{"by EOW", time.Date(2025, 1, 17, 23, 59, 59, 0, time.UTC)},
		{"deadline: end of week", time.Date(2025, 1, 17, 23, 59, 59, 0, time.UTC)},
		// EOM (Jan 31)
		{"by EOM", time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC)},
		{"due: end of month", time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := ParseDueDate(tt.text, ref)
			if !result.Found {
				t.Error("Expected to find date")
				return
			}
			if !result.DueDate.Equal(tt.expected) {
				t.Errorf("DueDate = %v, want %v", result.DueDate, tt.expected)
			}
		})
	}
}

func TestParseDueDateWithinDays(t *testing.T) {
	ref := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		text     string
		expected time.Time
	}{
		{"within 3 days", time.Date(2025, 1, 18, 23, 59, 59, 0, time.UTC)},
		{"within 1 day", time.Date(2025, 1, 16, 23, 59, 59, 0, time.UTC)},
		{"within 7 days", time.Date(2025, 1, 22, 23, 59, 59, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := ParseDueDate(tt.text, ref)
			if !result.Found {
				t.Error("Expected to find date")
				return
			}
			if !result.DueDate.Equal(tt.expected) {
				t.Errorf("DueDate = %v, want %v", result.DueDate, tt.expected)
			}
		})
	}
}

func TestParseDueDateActionRequired(t *testing.T) {
	ref := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		text     string
		expected time.Time
	}{
		{"Action required by Friday", time.Date(2025, 1, 17, 23, 59, 59, 0, time.UTC)},
		{"action required by 2025-01-25", time.Date(2025, 1, 25, 23, 59, 59, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := ParseDueDate(tt.text, ref)
			if !result.Found {
				t.Error("Expected to find date")
				return
			}
			if !result.DueDate.Equal(tt.expected) {
				t.Errorf("DueDate = %v, want %v", result.DueDate, tt.expected)
			}
		})
	}
}

func TestParseDueDateNoMatch(t *testing.T) {
	ref := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []string{
		"",
		"Hello, how are you?",
		"Meeting at 3pm",
		"Project deadline discussion",
		"2025 fiscal year",
	}

	for _, text := range tests {
		t.Run(text, func(t *testing.T) {
			result := ParseDueDate(text, ref)
			if result.Found {
				t.Errorf("Did not expect to find date in %q, got %v", text, result.DueDate)
			}
		})
	}
}

func TestParseDueDatePastDateNextYear(t *testing.T) {
	// If date is in the past (without year), assume next year
	ref := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC) // March 15

	result := ParseDueDate("by January 10", ref) // Already passed
	if !result.Found {
		t.Error("Expected to find date")
		return
	}

	// Should be January 10, 2026 (next year)
	expected := time.Date(2026, 1, 10, 23, 59, 59, 0, time.UTC)
	if !result.DueDate.Equal(expected) {
		t.Errorf("DueDate = %v, want %v", result.DueDate, expected)
	}
}

func TestParseDueDateConfidence(t *testing.T) {
	ref := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// ISO date should have highest confidence
	isoResult := ParseDueDate("by 2025-01-20", ref)
	if isoResult.Confidence < 0.9 {
		t.Errorf("ISO date confidence %f < 0.9", isoResult.Confidence)
	}

	// Weekday should have lower confidence
	weekdayResult := ParseDueDate("by Friday", ref)
	if weekdayResult.Confidence > 0.85 {
		t.Errorf("Weekday confidence %f > 0.85", weekdayResult.Confidence)
	}
}

func TestHasDueCues(t *testing.T) {
	tests := []struct {
		text     string
		expected bool
	}{
		{"by Friday", true},
		{"due: tomorrow", true},
		{"deadline approaching", true},
		{"action required", true},
		{"within 3 days", true},
		{"EOD", true},
		{"end of week", true},
		{"Hello World", false},
		{"Meeting notes", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := HasDueCues(tt.text)
			if result != tt.expected {
				t.Errorf("HasDueCues(%q) = %v, want %v", tt.text, result, tt.expected)
			}
		})
	}
}

func TestParseDueDateCaseInsensitive(t *testing.T) {
	ref := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []string{
		"BY FRIDAY",
		"By Friday",
		"by friday",
		"DUE: 2025-01-20",
		"Due: 2025-01-20",
		"DEADLINE: January 20",
	}

	for _, text := range tests {
		t.Run(text, func(t *testing.T) {
			result := ParseDueDate(text, ref)
			if !result.Found {
				t.Errorf("Expected to find date in %q (case insensitive)", text)
			}
		})
	}
}

func TestParseDueDateDeterminism(t *testing.T) {
	ref := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	text := "Please respond by Friday or by 2025-01-25 at the latest"

	// Run multiple times
	var results []DueParseResult
	for i := 0; i < 10; i++ {
		results = append(results, ParseDueDate(text, ref))
	}

	// All should be identical
	for i := 1; i < len(results); i++ {
		if results[i].DueDate != results[0].DueDate {
			t.Errorf("Result %d differs: %v vs %v", i, results[i].DueDate, results[0].DueDate)
		}
		if results[i].Pattern != results[0].Pattern {
			t.Errorf("Pattern %d differs: %s vs %s", i, results[i].Pattern, results[0].Pattern)
		}
	}
}
