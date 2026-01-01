// Package identityresolve provides rule-based identity resolution from canonical events.
//
// The resolver consumes events (email, calendar, commerce) and emits identity updates:
// - Entity upserts: Person, Organization, EmailAccount, PhoneNumber, Household
// - Edge upserts: owns_email, spouse_of, parent_of, works_at, vendor_of, etc.
//
// All matching is deterministic and rule-based (no ML):
// - Exact email match → same person
// - Domain-based org matching → organization from email domain
// - Config-based family members → explicit household configuration
//
// Reference: docs/ADR/ADR-0028-phase13-identity-graph.md
package identityresolve

import (
	"fmt"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
)

// EventType identifies the kind of event being processed.
type EventType string

const (
	EventTypeEmail    EventType = "email"
	EventTypeCalendar EventType = "calendar"
	EventTypeCommerce EventType = "commerce"
)

// CanonicalEvent is a normalized event from any source.
// Uses pipe-delimited format for determinism (NOT JSON).
type CanonicalEvent struct {
	EventType     EventType
	EventID       string
	Timestamp     time.Time
	CircleID      string
	OwnerEmail    string // The account owner's email
	Participants  []Participant
	MerchantName  string // For commerce events
	MerchantEmail string // For commerce receipts
}

// Participant represents a person in an event.
type Participant struct {
	Email       string
	DisplayName string
	Role        string // from, to, cc, attendee, organizer
}

// CanonicalString returns the pipe-delimited canonical representation.
func (e CanonicalEvent) CanonicalString() string {
	parts := []string{
		string(e.EventType),
		e.EventID,
		e.Timestamp.UTC().Format(time.RFC3339),
		e.CircleID,
		e.OwnerEmail,
	}
	for _, p := range e.Participants {
		parts = append(parts, fmt.Sprintf("%s:%s:%s", p.Role, p.Email, p.DisplayName))
	}
	if e.MerchantName != "" {
		parts = append(parts, "merchant:"+e.MerchantName)
	}
	return strings.Join(parts, "|")
}

// IdentityUpdate represents an update to the identity graph.
type IdentityUpdate struct {
	UpdateType UpdateType
	EntityID   identity.EntityID
	EdgeID     identity.EntityID
	Entity     identity.Entity
	Edge       *identity.Edge
	Provenance string
}

// UpdateType identifies what kind of update this is.
type UpdateType string

const (
	UpdateTypeEntityUpsert UpdateType = "IDENTITY_ENTITY_UPSERT"
	UpdateTypeEdgeUpsert   UpdateType = "IDENTITY_EDGE_UPSERT"
)

// Resolver processes canonical events and emits identity updates.
// It uses rule-based matching only (no ML).
type Resolver struct {
	generator *identity.Generator
	config    *Config
}

// Config provides resolver configuration.
type Config struct {
	// FamilyMembers maps email to household name and relationship.
	// Key: normalized email, Value: FamilyMemberConfig
	FamilyMembers map[string]FamilyMemberConfig

	// KnownAliases maps one email to another for the same person.
	// Key: alias email, Value: primary email
	KnownAliases map[string]string

	// OwnerEmails lists all emails belonging to the owner (Satish).
	OwnerEmails []string
}

// FamilyMemberConfig describes a family member's relationship.
type FamilyMemberConfig struct {
	HouseholdName string
	Relationship  identity.EdgeType // spouse_of, parent_of, child_of
	DisplayName   string
}

// NewResolver creates a new identity resolver.
func NewResolver(config *Config) *Resolver {
	if config == nil {
		config = &Config{
			FamilyMembers: make(map[string]FamilyMemberConfig),
			KnownAliases:  make(map[string]string),
			OwnerEmails:   []string{},
		}
	}
	return &Resolver{
		generator: identity.NewGenerator(),
		config:    config,
	}
}

// ProcessEvent processes a canonical event and returns identity updates.
// No goroutines, no time.Now(), deterministic output.
func (r *Resolver) ProcessEvent(event CanonicalEvent) []IdentityUpdate {
	var updates []IdentityUpdate

	// Process owner email
	if event.OwnerEmail != "" {
		ownerUpdates := r.processEmail(event.OwnerEmail, "", event.Timestamp, "owner:"+event.CircleID)
		updates = append(updates, ownerUpdates...)
	}

	// Process participants
	for _, p := range event.Participants {
		if p.Email != "" {
			participantUpdates := r.processEmail(p.Email, p.DisplayName, event.Timestamp, event.EventType.Provenance())
			updates = append(updates, participantUpdates...)
		}
	}

	// Process merchant (commerce events)
	if event.MerchantName != "" {
		merchantUpdates := r.processMerchant(event.MerchantName, event.MerchantEmail, event.Timestamp)
		updates = append(updates, merchantUpdates...)
	}

	return updates
}

// Provenance returns the provenance string for an event type.
func (t EventType) Provenance() string {
	return "event:" + string(t)
}

// processEmail processes an email address and returns identity updates.
func (r *Resolver) processEmail(email, displayName string, timestamp time.Time, provenance string) []IdentityUpdate {
	var updates []IdentityUpdate

	// Normalize email
	normalizedEmail := normalizeEmail(email)
	if normalizedEmail == "" {
		return updates
	}

	// Check for known alias
	primaryEmail := normalizedEmail
	if alias, ok := r.config.KnownAliases[normalizedEmail]; ok {
		primaryEmail = alias
	}

	// Create or reference person
	person := r.generator.PersonFromEmail(primaryEmail, timestamp)
	if displayName != "" {
		person.DisplayName = displayName
	}
	updates = append(updates, IdentityUpdate{
		UpdateType: UpdateTypeEntityUpsert,
		EntityID:   person.ID(),
		Entity:     person,
		Provenance: provenance,
	})

	// Create email account
	emailAccount := r.generator.EmailAccountFromAddress(email, extractProvider(email), timestamp)
	updates = append(updates, IdentityUpdate{
		UpdateType: UpdateTypeEntityUpsert,
		EntityID:   emailAccount.ID(),
		Entity:     emailAccount,
		Provenance: provenance,
	})

	// Create owns_email edge
	edge := identity.NewEdge(
		identity.EdgeTypeOwnsEmail,
		person.ID(),
		emailAccount.ID(),
		identity.ConfidenceHigh,
		provenance,
		timestamp,
	)
	updates = append(updates, IdentityUpdate{
		UpdateType: UpdateTypeEdgeUpsert,
		EdgeID:     edge.ID(),
		Edge:       edge,
		Provenance: provenance,
	})

	// Create organization from domain (if work email)
	if isWorkEmail(email) {
		domain := extractDomain(email)
		org := r.generator.OrganizationFromDomain(domain, timestamp)
		updates = append(updates, IdentityUpdate{
			UpdateType: UpdateTypeEntityUpsert,
			EntityID:   org.ID(),
			Entity:     org,
			Provenance: provenance,
		})

		// Create works_at edge
		worksAtEdge := identity.NewEdge(
			identity.EdgeTypeWorksAt,
			person.ID(),
			org.ID(),
			identity.ConfidenceMedium, // Inferred from email domain
			provenance,
			timestamp,
		)
		updates = append(updates, IdentityUpdate{
			UpdateType: UpdateTypeEdgeUpsert,
			EdgeID:     worksAtEdge.ID(),
			Edge:       worksAtEdge,
			Provenance: provenance,
		})
	}

	// Check for family member relationship
	if familyConfig, ok := r.config.FamilyMembers[normalizedEmail]; ok {
		// Create household
		household := r.generator.HouseholdFromName(familyConfig.HouseholdName, timestamp)
		updates = append(updates, IdentityUpdate{
			UpdateType: UpdateTypeEntityUpsert,
			EntityID:   household.ID(),
			Entity:     household,
			Provenance: "config:family",
		})

		// Create member_of_hh edge
		memberEdge := identity.NewEdge(
			identity.EdgeTypeMemberOfHH,
			person.ID(),
			household.ID(),
			identity.ConfidenceHigh, // From config
			"config:family",
			timestamp,
		)
		updates = append(updates, IdentityUpdate{
			UpdateType: UpdateTypeEdgeUpsert,
			EdgeID:     memberEdge.ID(),
			Edge:       memberEdge,
			Provenance: "config:family",
		})
	}

	return updates
}

// processMerchant processes a merchant and returns identity updates.
func (r *Resolver) processMerchant(merchantName, merchantEmail string, timestamp time.Time) []IdentityUpdate {
	var updates []IdentityUpdate

	// Create organization from merchant name
	org := r.generator.OrganizationFromMerchant(merchantName, timestamp)
	updates = append(updates, IdentityUpdate{
		UpdateType: UpdateTypeEntityUpsert,
		EntityID:   org.ID(),
		Entity:     org,
		Provenance: "event:commerce",
	})

	// If we have a merchant email, also create from domain
	if merchantEmail != "" && isWorkEmail(merchantEmail) {
		domain := extractDomain(merchantEmail)
		domainOrg := r.generator.OrganizationFromDomain(domain, timestamp)

		// Only add if different from merchant-based org
		if domainOrg.ID() != org.ID() {
			updates = append(updates, IdentityUpdate{
				UpdateType: UpdateTypeEntityUpsert,
				EntityID:   domainOrg.ID(),
				Entity:     domainOrg,
				Provenance: "event:commerce",
			})

			// Create alias edge between merchant and domain orgs
			aliasEdge := identity.NewEdge(
				identity.EdgeTypeAliasOf,
				org.ID(),
				domainOrg.ID(),
				identity.ConfidenceMedium,
				"event:commerce",
				timestamp,
			)
			updates = append(updates, IdentityUpdate{
				UpdateType: UpdateTypeEdgeUpsert,
				EdgeID:     aliasEdge.ID(),
				Edge:       aliasEdge,
				Provenance: "event:commerce",
			})
		}
	}

	return updates
}

// Helper functions

func normalizeEmail(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") {
		return ""
	}

	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return ""
	}

	local, domain := parts[0], parts[1]

	// Gmail specific: remove dots and plus-addressing
	if domain == "gmail.com" || domain == "googlemail.com" {
		local = strings.ReplaceAll(local, ".", "")
		if idx := strings.Index(local, "+"); idx != -1 {
			local = local[:idx]
		}
		domain = "gmail.com"
	}

	return local + "@" + domain
}

func extractDomain(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.ToLower(parts[1])
}

func extractProvider(email string) string {
	domain := extractDomain(email)
	switch {
	case strings.Contains(domain, "gmail") || strings.Contains(domain, "google"):
		return "google"
	case strings.Contains(domain, "outlook") || strings.Contains(domain, "hotmail") || strings.Contains(domain, "live"):
		return "microsoft"
	case strings.Contains(domain, "yahoo"):
		return "yahoo"
	case strings.Contains(domain, "icloud") || strings.Contains(domain, "me.com"):
		return "apple"
	default:
		return "other"
	}
}

func isWorkEmail(email string) bool {
	personalDomains := []string{
		"gmail.com", "googlemail.com", "yahoo.com", "yahoo.co.uk",
		"hotmail.com", "hotmail.co.uk", "outlook.com", "live.com",
		"icloud.com", "me.com", "aol.com", "protonmail.com",
	}

	domain := extractDomain(email)
	for _, pd := range personalDomains {
		if domain == pd {
			return false
		}
	}
	return true
}
