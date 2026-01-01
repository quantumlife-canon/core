package routing

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/config"
	"quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"
)

func TestRouter_RouteEmailToCircle_WorkDomain(t *testing.T) {
	cfg := &config.MultiCircleConfig{
		Circles: map[identity.EntityID]*config.CircleConfig{
			"work": {
				ID:   "work",
				Name: "Work",
				EmailIntegrations: []config.EmailIntegration{
					{Provider: "google", Identifier: "work@company.com"},
				},
			},
			"personal": {
				ID:   "personal",
				Name: "Personal",
				EmailIntegrations: []config.EmailIntegration{
					{Provider: "google", Identifier: "me@gmail.com"},
				},
			},
		},
		Routing: config.RoutingConfig{
			WorkDomains:     []string{"company.com", "corp.company.com"},
			PersonalDomains: []string{"gmail.com", "yahoo.com"},
		},
	}

	router := NewRouter(cfg)

	tests := []struct {
		name         string
		senderDomain string
		accountEmail string
		wantCircle   identity.EntityID
	}{
		{
			name:         "work domain sender to work email",
			senderDomain: "company.com",
			accountEmail: "work@company.com",
			wantCircle:   "work",
		},
		{
			name:         "work domain sender to personal email - receiver priority",
			senderDomain: "company.com",
			accountEmail: "me@gmail.com",
			wantCircle:   "personal", // receiver email determines circle first
		},
		{
			name:         "personal domain sender",
			senderDomain: "gmail.com",
			accountEmail: "me@gmail.com",
			wantCircle:   "personal",
		},
		{
			name:         "receiver is work email",
			senderDomain: "random.com",
			accountEmail: "work@company.com",
			wantCircle:   "work",
		},
		{
			name:         "receiver is personal email",
			senderDomain: "random.com",
			accountEmail: "me@gmail.com",
			wantCircle:   "personal",
		},
		{
			name:         "unknown receiver with work domain sender routes to work",
			senderDomain: "company.com",
			accountEmail: "unknown@other.com",
			wantCircle:   "work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
			event := events.NewEmailMessageEvent("google", "msg-123", tt.accountEmail, now, now)
			event.SenderDomain = tt.senderDomain
			event.From = events.EmailAddress{Address: "sender@" + tt.senderDomain}

			got := router.RouteEmailToCircle(event)
			if got != tt.wantCircle {
				t.Errorf("RouteEmailToCircle() = %q, want %q", got, tt.wantCircle)
			}
		})
	}
}

func TestRouter_RouteEmailToCircle_FamilyMember(t *testing.T) {
	cfg := &config.MultiCircleConfig{
		Circles: map[identity.EntityID]*config.CircleConfig{
			"work":     {ID: "work", Name: "Work"},
			"personal": {ID: "personal", Name: "Personal"},
			"family":   {ID: "family", Name: "Family"},
		},
		Routing: config.RoutingConfig{
			FamilyMembers: []string{"spouse@gmail.com", "kid@gmail.com"},
		},
	}

	router := NewRouter(cfg)

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	event := events.NewEmailMessageEvent("google", "msg-123", "me@gmail.com", now, now)
	event.From = events.EmailAddress{Address: "spouse@gmail.com"}
	event.SenderDomain = "gmail.com"

	got := router.RouteEmailToCircle(event)
	if got != "family" {
		t.Errorf("RouteEmailToCircle() = %q, want %q", got, "family")
	}
}

func TestRouter_RouteCalendarToCircle_CalendarID(t *testing.T) {
	cfg := &config.MultiCircleConfig{
		Circles: map[identity.EntityID]*config.CircleConfig{
			"work": {
				ID:   "work",
				Name: "Work",
				CalendarIntegrations: []config.CalendarIntegration{
					{Provider: "google", CalendarID: "work-calendar"},
				},
			},
			"personal": {
				ID:   "personal",
				Name: "Personal",
				CalendarIntegrations: []config.CalendarIntegration{
					{Provider: "google", CalendarID: "primary"},
				},
			},
		},
	}

	router := NewRouter(cfg)

	tests := []struct {
		name       string
		calendarID string
		wantCircle identity.EntityID
	}{
		{
			name:       "work calendar",
			calendarID: "work-calendar",
			wantCircle: "work",
		},
		{
			name:       "primary calendar",
			calendarID: "primary",
			wantCircle: "personal",
		},
		{
			name:       "unknown calendar defaults to personal",
			calendarID: "unknown-calendar",
			wantCircle: "personal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
			event := events.NewCalendarEventEvent("google", tt.calendarID, "event-123", "me@gmail.com", now, now)

			got := router.RouteCalendarToCircle(event)
			if got != tt.wantCircle {
				t.Errorf("RouteCalendarToCircle() = %q, want %q", got, tt.wantCircle)
			}
		})
	}
}

func TestRouter_RouteCalendarToCircle_FamilyAttendee(t *testing.T) {
	cfg := &config.MultiCircleConfig{
		Circles: map[identity.EntityID]*config.CircleConfig{
			"personal": {ID: "personal", Name: "Personal"},
			"family":   {ID: "family", Name: "Family"},
		},
		Routing: config.RoutingConfig{
			FamilyMembers: []string{"spouse@gmail.com"},
		},
	}

	router := NewRouter(cfg)

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	event := events.NewCalendarEventEvent("google", "unknown-cal", "event-123", "me@gmail.com", now, now)
	event.Attendees = []events.CalendarAttendee{
		{Email: "spouse@gmail.com", Name: "Spouse"},
		{Email: "me@gmail.com", Name: "Me"},
	}

	got := router.RouteCalendarToCircle(event)
	if got != "family" {
		t.Errorf("RouteCalendarToCircle() = %q, want %q", got, "family")
	}
}

func TestRouter_RouteCalendarToCircle_WorkOrganizer(t *testing.T) {
	cfg := &config.MultiCircleConfig{
		Circles: map[identity.EntityID]*config.CircleConfig{
			"work":     {ID: "work", Name: "Work"},
			"personal": {ID: "personal", Name: "Personal"},
		},
		Routing: config.RoutingConfig{
			WorkDomains: []string{"company.com"},
		},
	}

	router := NewRouter(cfg)

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	event := events.NewCalendarEventEvent("google", "unknown-cal", "event-123", "me@gmail.com", now, now)
	event.Organizer = &events.CalendarAttendee{
		Email: "boss@company.com",
		Name:  "Boss",
	}

	got := router.RouteCalendarToCircle(event)
	if got != "work" {
		t.Errorf("RouteCalendarToCircle() = %q, want %q", got, "work")
	}
}

func TestRouter_Determinism(t *testing.T) {
	cfg := &config.MultiCircleConfig{
		Circles: map[identity.EntityID]*config.CircleConfig{
			"work":     {ID: "work", Name: "Work"},
			"personal": {ID: "personal", Name: "Personal"},
			"family":   {ID: "family", Name: "Family"},
		},
		Routing: config.RoutingConfig{
			WorkDomains:     []string{"company.com"},
			PersonalDomains: []string{"gmail.com"},
			FamilyMembers:   []string{"spouse@gmail.com"},
		},
	}

	router1 := NewRouter(cfg)
	router2 := NewRouter(cfg)

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Test with same event multiple times
	for i := 0; i < 10; i++ {
		event := events.NewEmailMessageEvent("google", "msg-123", "me@gmail.com", now, now)
		event.SenderDomain = "company.com"
		event.From = events.EmailAddress{Address: "sender@company.com"}

		result1 := router1.RouteEmailToCircle(event)
		result2 := router2.RouteEmailToCircle(event)

		if result1 != result2 {
			t.Errorf("routing not deterministic: %q != %q", result1, result2)
		}
		if result1 != "work" {
			t.Errorf("expected 'work', got %q", result1)
		}
	}
}

func TestRouter_IsVIPSender(t *testing.T) {
	cfg := &config.MultiCircleConfig{
		Circles: map[identity.EntityID]*config.CircleConfig{
			"personal": {ID: "personal", Name: "Personal"},
		},
		Routing: config.RoutingConfig{
			VIPSenders: []string{"ceo@company.com", "manager@company.com"},
		},
	}

	router := NewRouter(cfg)

	if !router.IsVIPSender("ceo@company.com") {
		t.Error("expected ceo@company.com to be VIP")
	}
	if !router.IsVIPSender("CEO@COMPANY.COM") {
		t.Error("expected CEO@COMPANY.COM to be VIP (case insensitive)")
	}
	if router.IsVIPSender("random@company.com") {
		t.Error("expected random@company.com NOT to be VIP")
	}
}

func TestRouter_DefaultCircle(t *testing.T) {
	tests := []struct {
		name     string
		circles  map[identity.EntityID]*config.CircleConfig
		expected identity.EntityID
	}{
		{
			name: "personal exists",
			circles: map[identity.EntityID]*config.CircleConfig{
				"work":     {ID: "work", Name: "Work"},
				"personal": {ID: "personal", Name: "Personal"},
			},
			expected: "personal",
		},
		{
			name: "no personal - use first alphabetically",
			circles: map[identity.EntityID]*config.CircleConfig{
				"work":   {ID: "work", Name: "Work"},
				"family": {ID: "family", Name: "Family"},
			},
			expected: "family", // 'family' < 'work' alphabetically
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.MultiCircleConfig{Circles: tt.circles}
			router := NewRouter(cfg)

			if router.DefaultCircle() != tt.expected {
				t.Errorf("DefaultCircle() = %q, want %q", router.DefaultCircle(), tt.expected)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		email  string
		domain string
	}{
		{"user@example.com", "example.com"},
		{"user@sub.example.com", "sub.example.com"},
		{"USER@EXAMPLE.COM", "example.com"},
		{"invalid", ""},
		{"invalid@", ""},
		{"@example.com", "example.com"},
	}

	for _, tt := range tests {
		got := extractDomain(tt.email)
		if got != tt.domain {
			t.Errorf("extractDomain(%q) = %q, want %q", tt.email, got, tt.domain)
		}
	}
}
