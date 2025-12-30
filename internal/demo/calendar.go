// Package demo provides demo-specific components for the suggest-only vertical slice.
//
// This package contains:
// - Mock calendar data
// - Deterministic suggestion engine
// - Demo runner
//
// CRITICAL: This is for demonstration only. No real-world actions are executed.
package demo

import (
	"context"
	"fmt"
	"sort"
	"time"

	impl "quantumlife/internal/orchestrator/impl_inmem"
	"quantumlife/pkg/primitives"
)

// CalendarEvent represents a calendar event (mock data).
type CalendarEvent struct {
	ID        string
	Title     string
	StartTime time.Time
	EndTime   time.Time
	Category  string
	Recurring bool
}

// MockCalendar provides mock calendar data for the demo.
type MockCalendar struct {
	Events []CalendarEvent
}

// NewMockCalendar creates a mock calendar with sample data.
// The data is deterministic for reproducible demo output.
func NewMockCalendar() *MockCalendar {
	// Use a fixed reference date for deterministic output
	baseDate := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC) // Monday

	return &MockCalendar{
		Events: []CalendarEvent{
			// Monday
			{ID: "evt-1", Title: "Team standup", StartTime: baseDate.Add(9 * time.Hour), EndTime: baseDate.Add(9*time.Hour + 30*time.Minute), Category: "work", Recurring: true},
			{ID: "evt-2", Title: "Project planning", StartTime: baseDate.Add(10 * time.Hour), EndTime: baseDate.Add(12 * time.Hour), Category: "work", Recurring: false},
			{ID: "evt-3", Title: "Lunch", StartTime: baseDate.Add(12 * time.Hour), EndTime: baseDate.Add(13 * time.Hour), Category: "personal", Recurring: true},
			{ID: "evt-4", Title: "Client call", StartTime: baseDate.Add(14 * time.Hour), EndTime: baseDate.Add(15 * time.Hour), Category: "work", Recurring: false},

			// Tuesday
			{ID: "evt-5", Title: "Team standup", StartTime: baseDate.Add(24*time.Hour + 9*time.Hour), EndTime: baseDate.Add(24*time.Hour + 9*time.Hour + 30*time.Minute), Category: "work", Recurring: true},
			{ID: "evt-6", Title: "Code review", StartTime: baseDate.Add(24*time.Hour + 10*time.Hour), EndTime: baseDate.Add(24*time.Hour + 11*time.Hour), Category: "work", Recurring: false},
			{ID: "evt-7", Title: "Lunch", StartTime: baseDate.Add(24*time.Hour + 12*time.Hour), EndTime: baseDate.Add(24*time.Hour + 13*time.Hour), Category: "personal", Recurring: true},
			// NOTE: Tuesday 6-8pm is FREE - this is the key slot for suggestions

			// Wednesday
			{ID: "evt-8", Title: "Team standup", StartTime: baseDate.Add(48*time.Hour + 9*time.Hour), EndTime: baseDate.Add(48*time.Hour + 9*time.Hour + 30*time.Minute), Category: "work", Recurring: true},
			{ID: "evt-9", Title: "All-hands meeting", StartTime: baseDate.Add(48*time.Hour + 14*time.Hour), EndTime: baseDate.Add(48*time.Hour + 16*time.Hour), Category: "work", Recurring: false},
			{ID: "evt-10", Title: "Gym", StartTime: baseDate.Add(48*time.Hour + 18*time.Hour), EndTime: baseDate.Add(48*time.Hour + 19*time.Hour), Category: "health", Recurring: true},

			// Thursday
			{ID: "evt-11", Title: "Team standup", StartTime: baseDate.Add(72*time.Hour + 9*time.Hour), EndTime: baseDate.Add(72*time.Hour + 9*time.Hour + 30*time.Minute), Category: "work", Recurring: true},
			{ID: "evt-12", Title: "1:1 with manager", StartTime: baseDate.Add(72*time.Hour + 11*time.Hour), EndTime: baseDate.Add(72*time.Hour + 12*time.Hour), Category: "work", Recurring: true},
			// NOTE: Thursday evening is FREE after 5pm

			// Friday
			{ID: "evt-13", Title: "Team standup", StartTime: baseDate.Add(96*time.Hour + 9*time.Hour), EndTime: baseDate.Add(96*time.Hour + 9*time.Hour + 30*time.Minute), Category: "work", Recurring: true},
			{ID: "evt-14", Title: "Sprint review", StartTime: baseDate.Add(96*time.Hour + 15*time.Hour), EndTime: baseDate.Add(96*time.Hour + 16*time.Hour), Category: "work", Recurring: false},

			// Weekend - mostly free
			{ID: "evt-15", Title: "Grocery shopping", StartTime: baseDate.Add(120*time.Hour + 10*time.Hour), EndTime: baseDate.Add(120*time.Hour + 11*time.Hour), Category: "personal", Recurring: false},
		},
	}
}

// GetFreeSlots returns free time slots during typical waking hours (8am-10pm).
func (c *MockCalendar) GetFreeSlots(minDuration time.Duration) []FreeSlot {
	baseDate := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)
	var slots []FreeSlot

	// Check each day for free slots
	for day := 0; day < 7; day++ {
		dayStart := baseDate.Add(time.Duration(day) * 24 * time.Hour)
		wakingStart := dayStart.Add(8 * time.Hour) // 8am
		wakingEnd := dayStart.Add(22 * time.Hour)  // 10pm

		// Get events for this day
		var dayEvents []CalendarEvent
		for _, evt := range c.Events {
			if evt.StartTime.After(dayStart) && evt.StartTime.Before(dayStart.Add(24*time.Hour)) {
				dayEvents = append(dayEvents, evt)
			}
		}

		// Sort by start time
		sort.Slice(dayEvents, func(i, j int) bool {
			return dayEvents[i].StartTime.Before(dayEvents[j].StartTime)
		})

		// Find gaps
		current := wakingStart
		for _, evt := range dayEvents {
			if evt.StartTime.After(current) {
				gap := evt.StartTime.Sub(current)
				if gap >= minDuration {
					slots = append(slots, FreeSlot{
						Start:    current,
						End:      evt.StartTime,
						Duration: gap,
						DayName:  getDayName(day),
					})
				}
			}
			if evt.EndTime.After(current) {
				current = evt.EndTime
			}
		}

		// Check gap from last event to end of waking hours
		if current.Before(wakingEnd) {
			gap := wakingEnd.Sub(current)
			if gap >= minDuration {
				slots = append(slots, FreeSlot{
					Start:    current,
					End:      wakingEnd,
					Duration: gap,
					DayName:  getDayName(day),
				})
			}
		}
	}

	return slots
}

// FreeSlot represents a free time slot.
type FreeSlot struct {
	Start    time.Time
	End      time.Time
	Duration time.Duration
	DayName  string
}

func getDayName(day int) string {
	days := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	return days[day]
}

// DeterministicSuggestionEngine produces suggestions deterministically from calendar data.
// This is a mock implementation that does NOT use any LLM/SLM.
type DeterministicSuggestionEngine struct {
	Calendar *MockCalendar
}

// NewDeterministicSuggestionEngine creates a new deterministic suggestion engine.
func NewDeterministicSuggestionEngine(calendar *MockCalendar) *DeterministicSuggestionEngine {
	return &DeterministicSuggestionEngine{
		Calendar: calendar,
	}
}

// GenerateSuggestions produces suggestions from the calendar data.
// This is completely deterministic - same input always produces same output.
func (e *DeterministicSuggestionEngine) GenerateSuggestions(ctx context.Context, loopCtx primitives.LoopContext, data interface{}) ([]impl.Suggestion, error) {
	// Get free slots of at least 1 hour
	freeSlots := e.Calendar.GetFreeSlots(1 * time.Hour)

	if len(freeSlots) == 0 {
		return []impl.Suggestion{
			{
				ID:          "sug-0",
				Description: "Your calendar is fully booked. Consider declining some meetings.",
				Explanation: "No free slots found during waking hours (8am-10pm)",
				Category:    "scheduling",
				Priority:    1,
				TimeSlot:    "N/A",
			},
		}, nil
	}

	// Generate up to 3 suggestions based on best free slots
	suggestions := make([]impl.Suggestion, 0, 3)

	// Prioritize evening slots for family/personal activities
	for _, slot := range freeSlots {
		if len(suggestions) >= 3 {
			break
		}

		hour := slot.Start.Hour()
		timeStr := fmt.Sprintf("%s %d:00-%d:00",
			slot.DayName,
			slot.Start.Hour(),
			slot.End.Hour())

		if hour >= 17 && hour <= 20 { // Evening slot (5pm-8pm start)
			suggestions = append(suggestions, impl.Suggestion{
				ID:          fmt.Sprintf("sug-%d", len(suggestions)),
				Description: fmt.Sprintf("Free slot %s; propose family activity", timeStr),
				Explanation: fmt.Sprintf("Evening free time detected on %s. "+
					"Based on pattern analysis: no work events, "+
					"ideal for family engagement. "+
					"Duration: %.0f hours.", slot.DayName, slot.Duration.Hours()),
				Category: "family",
				Priority: 1,
				TimeSlot: timeStr,
			})
		}
	}

	// Add weekend suggestions if we have room
	for _, slot := range freeSlots {
		if len(suggestions) >= 3 {
			break
		}

		if slot.DayName == "Saturday" || slot.DayName == "Sunday" {
			hour := slot.Start.Hour()
			if hour >= 10 && hour <= 16 { // Daytime weekend slot
				timeStr := fmt.Sprintf("%s %d:00-%d:00",
					slot.DayName,
					slot.Start.Hour(),
					slot.End.Hour())

				suggestions = append(suggestions, impl.Suggestion{
					ID:          fmt.Sprintf("sug-%d", len(suggestions)),
					Description: fmt.Sprintf("Weekend availability %s; propose outdoor activity", timeStr),
					Explanation: fmt.Sprintf("Weekend daytime slot on %s. "+
						"Pattern: minimal scheduled activities. "+
						"Recommendation: outdoor/leisure activity. "+
						"Duration: %.0f hours.", slot.DayName, slot.Duration.Hours()),
					Category: "leisure",
					Priority: 2,
					TimeSlot: timeStr,
				})
			}
		}
	}

	// Fill remaining slots with any available time
	for _, slot := range freeSlots {
		if len(suggestions) >= 3 {
			break
		}

		timeStr := fmt.Sprintf("%s %d:00-%d:00",
			slot.DayName,
			slot.Start.Hour(),
			slot.End.Hour())

		// Check if we already have this slot
		alreadyAdded := false
		for _, s := range suggestions {
			if s.TimeSlot == timeStr {
				alreadyAdded = true
				break
			}
		}

		if !alreadyAdded {
			suggestions = append(suggestions, impl.Suggestion{
				ID:          fmt.Sprintf("sug-%d", len(suggestions)),
				Description: fmt.Sprintf("Available time %s; schedule personal task", timeStr),
				Explanation: fmt.Sprintf("Unscheduled slot on %s. "+
					"Pattern: gap between meetings/events. "+
					"Suggestion: personal tasks or rest. "+
					"Duration: %.0f hours.", slot.DayName, slot.Duration.Hours()),
				Category: "personal",
				Priority: 3,
				TimeSlot: timeStr,
			})
		}
	}

	return suggestions, nil
}

// Verify interface compliance at compile time.
var _ impl.SuggestionEngine = (*DeterministicSuggestionEngine)(nil)
