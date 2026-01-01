// Package routing provides deterministic event-to-circle routing rules.
//
// CRITICAL: All routing decisions are deterministic. Given the same event
// and configuration, the same circle is always returned.
//
// GUARDRAIL: This package does NOT use LLM or ML. All routing is rule-based.
//
// Reference: docs/ADR/ADR-0026-phase11-multicircle-real-loop.md
package routing

import (
	"strings"

	"quantumlife/pkg/domain/config"
	"quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"
)

// Router routes events to circles based on deterministic rules.
type Router struct {
	config *config.MultiCircleConfig

	// Pre-computed lookup tables for O(1) routing
	workDomains     map[string]bool
	personalDomains map[string]bool
	familyEmails    map[string]bool
	vipSenders      map[string]bool

	// Email-to-circle mapping from integrations
	emailToCircle map[string]identity.EntityID

	// Calendar-to-circle mapping from integrations
	calendarToCircle map[string]identity.EntityID

	// Default circle for unrouted events
	defaultCircle identity.EntityID
}

// NewRouter creates a new router with the given configuration.
func NewRouter(cfg *config.MultiCircleConfig) *Router {
	r := &Router{
		config:           cfg,
		workDomains:      make(map[string]bool),
		personalDomains:  make(map[string]bool),
		familyEmails:     make(map[string]bool),
		vipSenders:       make(map[string]bool),
		emailToCircle:    make(map[string]identity.EntityID),
		calendarToCircle: make(map[string]identity.EntityID),
	}

	// Populate domain lookups
	for _, domain := range cfg.Routing.WorkDomains {
		r.workDomains[strings.ToLower(domain)] = true
	}
	for _, domain := range cfg.Routing.PersonalDomains {
		r.personalDomains[strings.ToLower(domain)] = true
	}
	for _, email := range cfg.Routing.FamilyMembers {
		r.familyEmails[strings.ToLower(email)] = true
	}
	for _, email := range cfg.Routing.VIPSenders {
		r.vipSenders[strings.ToLower(email)] = true
	}

	// Build email-to-circle and calendar-to-circle mappings from integrations
	for _, circleID := range cfg.CircleIDs() {
		circle := cfg.GetCircle(circleID)

		for _, email := range circle.EmailIntegrations {
			r.emailToCircle[strings.ToLower(email.Identifier)] = circleID
		}

		for _, cal := range circle.CalendarIntegrations {
			r.calendarToCircle[strings.ToLower(cal.CalendarID)] = circleID
		}
	}

	// Set default circle (prefer "personal" if it exists)
	if cfg.GetCircle("personal") != nil {
		r.defaultCircle = "personal"
	} else {
		// Use first circle in sorted order
		ids := cfg.CircleIDs()
		if len(ids) > 0 {
			r.defaultCircle = ids[0]
		}
	}

	return r
}

// RouteEmailToCircle determines which circle an email event belongs to.
//
// Routing priority:
// 1. If receiver email is a known integration email, use that circle
// 2. If sender domain is in work_domains, route to work
// 3. If sender is a known family member, route to family
// 4. If sender domain is in personal_domains, route to personal
// 5. Default to personal (or first configured circle)
func (r *Router) RouteEmailToCircle(event *events.EmailMessageEvent) identity.EntityID {
	// Priority 1: Check if receiver email is a known integration
	receiverEmail := strings.ToLower(event.AccountEmail)
	if circleID, ok := r.emailToCircle[receiverEmail]; ok {
		return circleID
	}

	// Priority 2: Check sender domain against work domains
	senderDomain := strings.ToLower(event.SenderDomain)
	if senderDomain != "" && r.workDomains[senderDomain] {
		if r.config.GetCircle("work") != nil {
			return "work"
		}
	}

	// Priority 3: Check if sender is a family member
	senderEmail := strings.ToLower(event.From.Address)
	if r.familyEmails[senderEmail] {
		if r.config.GetCircle("family") != nil {
			return "family"
		}
	}

	// Priority 4: Check sender domain against personal domains
	if senderDomain != "" && r.personalDomains[senderDomain] {
		if r.config.GetCircle("personal") != nil {
			return "personal"
		}
	}

	// Default
	return r.defaultCircle
}

// RouteCalendarToCircle determines which circle a calendar event belongs to.
//
// Routing priority:
// 1. If calendar ID is a known integration calendar, use that circle
// 2. If any attendee is a family member, consider routing to family
// 3. If organizer domain is in work_domains, route to work
// 4. Default to personal (or first configured circle)
func (r *Router) RouteCalendarToCircle(event *events.CalendarEventEvent) identity.EntityID {
	// Priority 1: Check if calendar ID is a known integration
	calendarID := strings.ToLower(event.CalendarID)
	if circleID, ok := r.calendarToCircle[calendarID]; ok {
		return circleID
	}

	// Also check account email as calendar ID
	accountEmail := strings.ToLower(event.AccountEmail)
	if circleID, ok := r.calendarToCircle[accountEmail]; ok {
		return circleID
	}

	// Priority 2: Check if any attendee is a family member
	for _, attendee := range event.Attendees {
		if r.familyEmails[strings.ToLower(attendee.Email)] {
			if r.config.GetCircle("family") != nil {
				return "family"
			}
		}
	}

	// Priority 3: Check organizer domain
	if event.Organizer != nil {
		organizerDomain := extractDomain(event.Organizer.Email)
		if organizerDomain != "" && r.workDomains[organizerDomain] {
			if r.config.GetCircle("work") != nil {
				return "work"
			}
		}
	}

	// Default
	return r.defaultCircle
}

// IsVIPSender checks if an email address is a VIP sender.
func (r *Router) IsVIPSender(email string) bool {
	return r.vipSenders[strings.ToLower(email)]
}

// IsFamilyMember checks if an email address is a known family member.
func (r *Router) IsFamilyMember(email string) bool {
	return r.familyEmails[strings.ToLower(email)]
}

// IsWorkDomain checks if a domain is a work domain.
func (r *Router) IsWorkDomain(domain string) bool {
	return r.workDomains[strings.ToLower(domain)]
}

// DefaultCircle returns the default circle for unrouted events.
func (r *Router) DefaultCircle() identity.EntityID {
	return r.defaultCircle
}

// extractDomain extracts the domain from an email address.
func extractDomain(email string) string {
	email = strings.ToLower(email)
	atIdx := strings.LastIndex(email, "@")
	if atIdx < 0 || atIdx >= len(email)-1 {
		return ""
	}
	return email[atIdx+1:]
}
