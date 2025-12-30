// Package demo_family provides the Family Intersection demo.
package demo_family

import (
	"sort"
	"time"
)

// FamilyCalendarEvent represents a calendar event for the family demo.
type FamilyCalendarEvent struct {
	ID        string
	Title     string
	StartTime time.Time
	EndTime   time.Time
	Category  string
	Owner     string // "you", "spouse", "shared"
	Recurring bool
}

// FamilyCalendar provides mock calendar data for two family members.
type FamilyCalendar struct {
	YouEvents    []FamilyCalendarEvent
	SpouseEvents []FamilyCalendarEvent
}

// NewFamilyCalendar creates a mock family calendar with sample data.
// The data is deterministic for reproducible demo output.
func NewFamilyCalendar() *FamilyCalendar {
	// Use a fixed reference date for deterministic output
	baseDate := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC) // Monday

	return &FamilyCalendar{
		YouEvents: []FamilyCalendarEvent{
			// Monday - You
			{ID: "you-1", Title: "Team standup", StartTime: baseDate.Add(9 * time.Hour), EndTime: baseDate.Add(9*time.Hour + 30*time.Minute), Category: "work", Owner: "you"},
			{ID: "you-2", Title: "Project work", StartTime: baseDate.Add(10 * time.Hour), EndTime: baseDate.Add(17 * time.Hour), Category: "work", Owner: "you"},

			// Tuesday - You
			{ID: "you-3", Title: "Team standup", StartTime: baseDate.Add(24*time.Hour + 9*time.Hour), EndTime: baseDate.Add(24*time.Hour + 9*time.Hour + 30*time.Minute), Category: "work", Owner: "you"},
			{ID: "you-4", Title: "Client meeting", StartTime: baseDate.Add(24*time.Hour + 14*time.Hour), EndTime: baseDate.Add(24*time.Hour + 16*time.Hour), Category: "work", Owner: "you"},

			// Wednesday - You
			{ID: "you-5", Title: "Work", StartTime: baseDate.Add(48*time.Hour + 9*time.Hour), EndTime: baseDate.Add(48*time.Hour + 17*time.Hour), Category: "work", Owner: "you"},
			{ID: "you-6", Title: "Gym", StartTime: baseDate.Add(48*time.Hour + 6*time.Hour), EndTime: baseDate.Add(48*time.Hour + 7*time.Hour), Category: "health", Owner: "you"},

			// Thursday - You
			{ID: "you-7", Title: "Work", StartTime: baseDate.Add(72*time.Hour + 9*time.Hour), EndTime: baseDate.Add(72*time.Hour + 17*time.Hour), Category: "work", Owner: "you"},

			// Friday - You
			{ID: "you-8", Title: "Work", StartTime: baseDate.Add(96*time.Hour + 9*time.Hour), EndTime: baseDate.Add(96*time.Hour + 15*time.Hour), Category: "work", Owner: "you"},
		},
		SpouseEvents: []FamilyCalendarEvent{
			// Monday - Spouse
			{ID: "spouse-1", Title: "Morning yoga", StartTime: baseDate.Add(7 * time.Hour), EndTime: baseDate.Add(8 * time.Hour), Category: "health", Owner: "spouse"},
			{ID: "spouse-2", Title: "Work", StartTime: baseDate.Add(9 * time.Hour), EndTime: baseDate.Add(17 * time.Hour), Category: "work", Owner: "spouse"},

			// Tuesday - Spouse
			{ID: "spouse-3", Title: "Work", StartTime: baseDate.Add(24*time.Hour + 9*time.Hour), EndTime: baseDate.Add(24*time.Hour + 17*time.Hour), Category: "work", Owner: "spouse"},
			{ID: "spouse-4", Title: "Book club", StartTime: baseDate.Add(24*time.Hour + 19*time.Hour), EndTime: baseDate.Add(24*time.Hour + 21*time.Hour), Category: "social", Owner: "spouse"},

			// Wednesday - Spouse
			{ID: "spouse-5", Title: "Work", StartTime: baseDate.Add(48*time.Hour + 9*time.Hour), EndTime: baseDate.Add(48*time.Hour + 17*time.Hour), Category: "work", Owner: "spouse"},

			// Thursday - Spouse
			{ID: "spouse-6", Title: "Work", StartTime: baseDate.Add(72*time.Hour + 9*time.Hour), EndTime: baseDate.Add(72*time.Hour + 17*time.Hour), Category: "work", Owner: "spouse"},
			{ID: "spouse-7", Title: "Yoga", StartTime: baseDate.Add(72*time.Hour + 18*time.Hour), EndTime: baseDate.Add(72*time.Hour + 19*time.Hour), Category: "health", Owner: "spouse"},

			// Friday - Spouse
			{ID: "spouse-8", Title: "Work", StartTime: baseDate.Add(96*time.Hour + 9*time.Hour), EndTime: baseDate.Add(96*time.Hour + 15*time.Hour), Category: "work", Owner: "spouse"},
		},
	}
}

// FreeSlot represents a time slot when both family members are free.
type FreeSlot struct {
	Start    time.Time
	End      time.Time
	Duration time.Duration
	DayName  string
}

// GetFamilyFreeSlots returns slots when BOTH family members are free.
func (c *FamilyCalendar) GetFamilyFreeSlots(minDuration time.Duration) []FreeSlot {
	baseDate := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)
	var slots []FreeSlot

	// Combine all events
	allEvents := append(c.YouEvents, c.SpouseEvents...)

	// Check each day for free slots
	for day := 0; day < 7; day++ {
		dayStart := baseDate.Add(time.Duration(day) * 24 * time.Hour)

		// Define family time windows
		var checkWindows []struct{ start, end int }

		if day < 5 { // Weekdays: evening only (17:00-21:00)
			checkWindows = []struct{ start, end int }{{17, 21}}
		} else { // Weekends: full day (10:00-21:00)
			checkWindows = []struct{ start, end int }{{10, 21}}
		}

		for _, window := range checkWindows {
			windowStart := dayStart.Add(time.Duration(window.start) * time.Hour)
			windowEnd := dayStart.Add(time.Duration(window.end) * time.Hour)

			// Get events for this day that overlap with the window
			var dayEvents []FamilyCalendarEvent
			for _, evt := range allEvents {
				if evt.StartTime.After(dayStart) && evt.StartTime.Before(dayStart.Add(24*time.Hour)) {
					// Check if event overlaps with our window
					if evt.EndTime.After(windowStart) && evt.StartTime.Before(windowEnd) {
						dayEvents = append(dayEvents, evt)
					}
				}
			}

			// Sort by start time
			sort.Slice(dayEvents, func(i, j int) bool {
				return dayEvents[i].StartTime.Before(dayEvents[j].StartTime)
			})

			// Find gaps within the window
			current := windowStart
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

			// Check gap from last event to end of window
			if current.Before(windowEnd) {
				gap := windowEnd.Sub(current)
				if gap >= minDuration {
					slots = append(slots, FreeSlot{
						Start:    current,
						End:      windowEnd,
						Duration: gap,
						DayName:  getDayName(day),
					})
				}
			}
		}
	}

	return slots
}

func getDayName(day int) string {
	days := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	return days[day]
}
