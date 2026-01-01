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

// IdentityRouter provides identity graph lookup methods for routing.
// This is a subset of identity.InMemoryRepository for decoupling.
type IdentityRouter interface {
	// FindPersonByEmail finds a person by any of their email addresses.
	FindPersonByEmail(email string) (*identity.Person, error)

	// FindOrganizationByDomain finds an org by domain.
	FindOrganizationByDomain(domain string) (*identity.Organization, error)

	// IsHouseholdMember checks if a person belongs to any household.
	IsHouseholdMember(personID identity.EntityID) bool

	// GetPersonOrganizations returns all organizations a person works at.
	GetPersonOrganizations(personID identity.EntityID) []*identity.Organization
}

// Router routes events to circles based on deterministic rules.
// Phase 13.1: Supports identity graph for person/org-based routing.
type Router struct {
	config *config.MultiCircleConfig

	// Identity repository for person/org lookup (optional, Phase 13.1)
	identityRepo IdentityRouter

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

	// familyCircle is the configured circle for family/household events
	familyCircle identity.EntityID

	// workCircle is the configured circle for work-related events
	workCircle identity.EntityID
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

	// Set family and work circles if they exist
	if cfg.GetCircle("family") != nil {
		r.familyCircle = "family"
	}
	if cfg.GetCircle("work") != nil {
		r.workCircle = "work"
	}

	return r
}

// SetIdentityRepository sets the identity repository for identity-based routing.
// Phase 13.1: Enables person/org-based routing decisions.
func (r *Router) SetIdentityRepository(repo IdentityRouter) {
	r.identityRepo = repo
}

// RouteEmailToCircle determines which circle an email event belongs to.
//
// Phase 13.1 Routing Precedence (deterministic, stable order):
// P1: If receiver email is bound to a circle integration → that circle
// P2: If sender resolves to PersonID in Household → family circle
// P3: If sender resolves to works_at OrgID with domain in work_domains → work circle
// P4: If sender domain is in personal_domains → personal circle
// P5: Fallback → default circle (typically personal)
func (r *Router) RouteEmailToCircle(event *events.EmailMessageEvent) identity.EntityID {
	// P1: Check if receiver email is bound to a circle integration
	receiverEmail := strings.ToLower(event.AccountEmail)
	if circleID, ok := r.emailToCircle[receiverEmail]; ok {
		return circleID
	}

	senderEmail := strings.ToLower(event.From.Address)
	senderDomain := strings.ToLower(event.SenderDomain)

	// P2: Check if sender resolves to a person in a household (identity graph)
	if r.identityRepo != nil {
		person, err := r.identityRepo.FindPersonByEmail(senderEmail)
		if err == nil && person != nil {
			// Check if person is in a household
			if r.identityRepo.IsHouseholdMember(person.ID()) {
				if r.familyCircle != "" {
					return r.familyCircle
				}
			}
		}
	}

	// P2 fallback: Check config-based family members
	if r.familyEmails[senderEmail] {
		if r.familyCircle != "" {
			return r.familyCircle
		}
	}

	// P3: Check if sender works at an organization with domain in work_domains
	if r.identityRepo != nil && senderDomain != "" {
		person, err := r.identityRepo.FindPersonByEmail(senderEmail)
		if err == nil && person != nil {
			orgs := r.identityRepo.GetPersonOrganizations(person.ID())
			for _, org := range orgs {
				// Check if org domain is in work_domains
				if org.Domain != "" && r.workDomains[strings.ToLower(org.Domain)] {
					if r.workCircle != "" {
						return r.workCircle
					}
				}
			}
		}
	}

	// P3 fallback: Check sender domain directly against work_domains
	if senderDomain != "" && r.workDomains[senderDomain] {
		if r.workCircle != "" {
			return r.workCircle
		}
	}

	// P4: Check sender domain against personal_domains
	if senderDomain != "" && r.personalDomains[senderDomain] {
		if r.config.GetCircle("personal") != nil {
			return "personal"
		}
	}

	// P5: Default
	return r.defaultCircle
}

// RouteEmailToCircleWithReason returns the routed circle and the reason code.
// Useful for debugging and audit trails.
func (r *Router) RouteEmailToCircleWithReason(event *events.EmailMessageEvent) (identity.EntityID, string) {
	receiverEmail := strings.ToLower(event.AccountEmail)
	if circleID, ok := r.emailToCircle[receiverEmail]; ok {
		return circleID, "P1:integration_email"
	}

	senderEmail := strings.ToLower(event.From.Address)
	senderDomain := strings.ToLower(event.SenderDomain)

	if r.identityRepo != nil {
		person, err := r.identityRepo.FindPersonByEmail(senderEmail)
		if err == nil && person != nil {
			if r.identityRepo.IsHouseholdMember(person.ID()) {
				if r.familyCircle != "" {
					return r.familyCircle, "P2:household_member"
				}
			}
		}
	}

	if r.familyEmails[senderEmail] {
		if r.familyCircle != "" {
			return r.familyCircle, "P2:config_family"
		}
	}

	if r.identityRepo != nil && senderDomain != "" {
		person, err := r.identityRepo.FindPersonByEmail(senderEmail)
		if err == nil && person != nil {
			orgs := r.identityRepo.GetPersonOrganizations(person.ID())
			for _, org := range orgs {
				if org.Domain != "" && r.workDomains[strings.ToLower(org.Domain)] {
					if r.workCircle != "" {
						return r.workCircle, "P3:works_at_org"
					}
				}
			}
		}
	}

	if senderDomain != "" && r.workDomains[senderDomain] {
		if r.workCircle != "" {
			return r.workCircle, "P3:work_domain"
		}
	}

	if senderDomain != "" && r.personalDomains[senderDomain] {
		if r.config.GetCircle("personal") != nil {
			return "personal", "P4:personal_domain"
		}
	}

	return r.defaultCircle, "P5:default"
}

// RouteCalendarToCircle determines which circle a calendar event belongs to.
//
// Phase 13.1 Routing Precedence (deterministic, stable order):
// P1: If calendar ID is bound to a circle integration → that circle
// P2: If organizer resolves to PersonID in Household → family circle
// P3: If organizer resolves to works_at OrgID with domain in work_domains → work circle
// P4: Fallback → default circle (typically personal)
func (r *Router) RouteCalendarToCircle(event *events.CalendarEventEvent) identity.EntityID {
	// P1: Check if calendar ID is bound to a circle integration
	calendarID := strings.ToLower(event.CalendarID)
	if circleID, ok := r.calendarToCircle[calendarID]; ok {
		return circleID
	}

	// Also check account email as calendar ID
	accountEmail := strings.ToLower(event.AccountEmail)
	if circleID, ok := r.calendarToCircle[accountEmail]; ok {
		return circleID
	}

	// Get organizer email for identity lookup
	var organizerEmail string
	if event.Organizer != nil {
		organizerEmail = strings.ToLower(event.Organizer.Email)
	}

	// P2: Check if organizer resolves to a person in a household (identity graph)
	if r.identityRepo != nil && organizerEmail != "" {
		person, err := r.identityRepo.FindPersonByEmail(organizerEmail)
		if err == nil && person != nil {
			if r.identityRepo.IsHouseholdMember(person.ID()) {
				if r.familyCircle != "" {
					return r.familyCircle
				}
			}
		}
	}

	// P2 fallback: Check if any attendee is a config-based family member
	for _, attendee := range event.Attendees {
		if r.familyEmails[strings.ToLower(attendee.Email)] {
			if r.familyCircle != "" {
				return r.familyCircle
			}
		}
	}

	// P3: Check if organizer works at an organization with domain in work_domains
	if r.identityRepo != nil && organizerEmail != "" {
		person, err := r.identityRepo.FindPersonByEmail(organizerEmail)
		if err == nil && person != nil {
			orgs := r.identityRepo.GetPersonOrganizations(person.ID())
			for _, org := range orgs {
				if org.Domain != "" && r.workDomains[strings.ToLower(org.Domain)] {
					if r.workCircle != "" {
						return r.workCircle
					}
				}
			}
		}
	}

	// P3 fallback: Check organizer domain directly against work_domains
	if organizerEmail != "" {
		organizerDomain := extractDomain(organizerEmail)
		if organizerDomain != "" && r.workDomains[organizerDomain] {
			if r.workCircle != "" {
				return r.workCircle
			}
		}
	}

	// P4: Default
	return r.defaultCircle
}

// RouteCalendarToCircleWithReason returns the routed circle and the reason code.
func (r *Router) RouteCalendarToCircleWithReason(event *events.CalendarEventEvent) (identity.EntityID, string) {
	calendarID := strings.ToLower(event.CalendarID)
	if circleID, ok := r.calendarToCircle[calendarID]; ok {
		return circleID, "P1:integration_calendar"
	}

	accountEmail := strings.ToLower(event.AccountEmail)
	if circleID, ok := r.calendarToCircle[accountEmail]; ok {
		return circleID, "P1:integration_account"
	}

	var organizerEmail string
	if event.Organizer != nil {
		organizerEmail = strings.ToLower(event.Organizer.Email)
	}

	if r.identityRepo != nil && organizerEmail != "" {
		person, err := r.identityRepo.FindPersonByEmail(organizerEmail)
		if err == nil && person != nil {
			if r.identityRepo.IsHouseholdMember(person.ID()) {
				if r.familyCircle != "" {
					return r.familyCircle, "P2:household_member"
				}
			}
		}
	}

	for _, attendee := range event.Attendees {
		if r.familyEmails[strings.ToLower(attendee.Email)] {
			if r.familyCircle != "" {
				return r.familyCircle, "P2:config_family"
			}
		}
	}

	if r.identityRepo != nil && organizerEmail != "" {
		person, err := r.identityRepo.FindPersonByEmail(organizerEmail)
		if err == nil && person != nil {
			orgs := r.identityRepo.GetPersonOrganizations(person.ID())
			for _, org := range orgs {
				if org.Domain != "" && r.workDomains[strings.ToLower(org.Domain)] {
					if r.workCircle != "" {
						return r.workCircle, "P3:works_at_org"
					}
				}
			}
		}
	}

	if organizerEmail != "" {
		organizerDomain := extractDomain(organizerEmail)
		if organizerDomain != "" && r.workDomains[organizerDomain] {
			if r.workCircle != "" {
				return r.workCircle, "P3:work_domain"
			}
		}
	}

	return r.defaultCircle, "P4:default"
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
