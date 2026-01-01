// Package suppress defines suppression rules for interruptions.
//
// Phase 14: Circle Policies + Preference Learning (Deterministic)
//
// Suppression rules allow the system to learn from user feedback and
// temporarily or permanently hide interruptions matching specific criteria.
//
// Scopes:
// - ScopeCircle: suppress all interruptions in a circle
// - ScopePerson: suppress interruptions from a specific person
// - ScopeVendor: suppress interruptions from a specific vendor
// - ScopeTrigger: suppress a specific trigger type
// - ScopeItemKey: suppress a specific dedup key
//
// Reference: docs/ADR/ADR-0030-phase14-policy-learning.md
package suppress

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Scope defines what a suppression rule applies to.
type Scope string

const (
	// ScopeCircle suppresses all interruptions in a circle.
	ScopeCircle Scope = "scope_circle"

	// ScopePerson suppresses interruptions from a specific person.
	ScopePerson Scope = "scope_person"

	// ScopeVendor suppresses interruptions from a specific vendor.
	ScopeVendor Scope = "scope_vendor"

	// ScopeTrigger suppresses a specific trigger type.
	ScopeTrigger Scope = "scope_trigger"

	// ScopeItemKey suppresses a specific dedup key.
	ScopeItemKey Scope = "scope_itemkey"
)

// Source indicates how a suppression rule was created.
type Source string

const (
	// SourceManual indicates the rule was created by user action.
	SourceManual Source = "manual"

	// SourceFeedback indicates the rule was created by feedback learning.
	SourceFeedback Source = "feedback"
)

// SuppressionRule defines a single suppression rule.
type SuppressionRule struct {
	// RuleID is a deterministic identifier.
	RuleID string

	// CircleID is the circle this rule applies to.
	CircleID string

	// Scope is the type of suppression.
	Scope Scope

	// Key is the matching value (person_id, vendor_id, trigger, dedup_key).
	Key string

	// CreatedAt is when the rule was created.
	CreatedAt time.Time

	// ExpiresAt is when the rule expires (nil = permanent).
	ExpiresAt *time.Time

	// Reason is a short description of why this rule exists.
	Reason string

	// Source indicates how this rule was created.
	Source Source
}

// NewSuppressionRule creates a new suppression rule with computed RuleID.
func NewSuppressionRule(
	circleID string,
	scope Scope,
	key string,
	createdAt time.Time,
	expiresAt *time.Time,
	reason string,
	source Source,
) SuppressionRule {
	rule := SuppressionRule{
		CircleID:  circleID,
		Scope:     scope,
		Key:       key,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
		Reason:    reason,
		Source:    source,
	}
	rule.RuleID = rule.computeRuleID()
	return rule
}

// computeRuleID generates a deterministic ID from rule properties.
func (r SuppressionRule) computeRuleID() string {
	var expiresStr string
	if r.ExpiresAt != nil {
		expiresStr = r.ExpiresAt.UTC().Format(time.RFC3339)
	}
	input := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		r.CircleID,
		r.Scope,
		r.Key,
		r.CreatedAt.UTC().Format(time.RFC3339),
		expiresStr,
		r.Source,
	)
	hash := sha256.Sum256([]byte(input))
	return "sr_" + hex.EncodeToString(hash[:8]) // 16 char hex
}

// CanonicalString returns a deterministic string representation.
func (r SuppressionRule) CanonicalString() string {
	var sb strings.Builder
	sb.WriteString("rule_id:")
	sb.WriteString(r.RuleID)
	sb.WriteString("|circle:")
	sb.WriteString(r.CircleID)
	sb.WriteString("|scope:")
	sb.WriteString(string(r.Scope))
	sb.WriteString("|key:")
	sb.WriteString(r.Key)
	sb.WriteString("|created:")
	sb.WriteString(r.CreatedAt.UTC().Format(time.RFC3339))
	sb.WriteString("|expires:")
	if r.ExpiresAt != nil {
		sb.WriteString(r.ExpiresAt.UTC().Format(time.RFC3339))
	} else {
		sb.WriteString("permanent")
	}
	sb.WriteString("|reason:")
	sb.WriteString(r.Reason)
	sb.WriteString("|source:")
	sb.WriteString(string(r.Source))
	return sb.String()
}

// Hash returns a SHA256 hash of the canonical string.
func (r SuppressionRule) Hash() string {
	hash := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(hash[:])
}

// IsActive checks if the rule is active at the given time.
func (r SuppressionRule) IsActive(at time.Time) bool {
	// Must be created before or at the given time
	if at.Before(r.CreatedAt) {
		return false
	}
	// If no expiration, always active
	if r.ExpiresAt == nil {
		return true
	}
	// Check if not expired
	return at.Before(*r.ExpiresAt)
}

// Matches checks if this rule matches the given criteria.
func (r SuppressionRule) Matches(circleID string, scope Scope, key string) bool {
	if r.CircleID != circleID && r.CircleID != "*" {
		return false
	}
	if r.Scope != scope {
		return false
	}
	if r.Key != key && r.Key != "*" {
		return false
	}
	return true
}

// SuppressionSet contains all suppression rules with versioning.
type SuppressionSet struct {
	// Version is incremented on each update.
	Version int

	// Rules contains all suppression rules (sorted deterministically).
	Rules []SuppressionRule

	// Hash is the computed SHA256 hash.
	Hash string
}

// NewSuppressionSet creates an empty suppression set.
func NewSuppressionSet() *SuppressionSet {
	ss := &SuppressionSet{
		Version: 1,
		Rules:   []SuppressionRule{},
	}
	ss.ComputeHash()
	return ss
}

// AddRule adds a rule and re-sorts the set.
func (s *SuppressionSet) AddRule(rule SuppressionRule) {
	s.Rules = append(s.Rules, rule)
	s.sort()
	s.Version++
	s.ComputeHash()
}

// RemoveRule removes a rule by ID.
func (s *SuppressionSet) RemoveRule(ruleID string) bool {
	for i, r := range s.Rules {
		if r.RuleID == ruleID {
			// Remove by shifting
			s.Rules = append(s.Rules[:i], s.Rules[i+1:]...)
			s.Version++
			s.ComputeHash()
			return true
		}
	}
	return false
}

// GetRule returns a rule by ID.
func (s *SuppressionSet) GetRule(ruleID string) *SuppressionRule {
	for _, r := range s.Rules {
		if r.RuleID == ruleID {
			return &r
		}
	}
	return nil
}

// ListActive returns all active rules at the given time.
func (s *SuppressionSet) ListActive(at time.Time) []SuppressionRule {
	active := []SuppressionRule{}
	for _, r := range s.Rules {
		if r.IsActive(at) {
			active = append(active, r)
		}
	}
	return active
}

// ListByCircle returns all rules for a circle.
func (s *SuppressionSet) ListByCircle(circleID string) []SuppressionRule {
	result := []SuppressionRule{}
	for _, r := range s.Rules {
		if r.CircleID == circleID {
			result = append(result, r)
		}
	}
	return result
}

// FindMatch returns the first active rule that matches the criteria.
func (s *SuppressionSet) FindMatch(at time.Time, circleID string, scope Scope, key string) *SuppressionRule {
	for _, r := range s.Rules {
		if r.IsActive(at) && r.Matches(circleID, scope, key) {
			return &r
		}
	}
	return nil
}

// PruneExpired removes all expired rules.
func (s *SuppressionSet) PruneExpired(at time.Time) int {
	pruned := 0
	newRules := []SuppressionRule{}
	for _, r := range s.Rules {
		if r.IsActive(at) {
			newRules = append(newRules, r)
		} else {
			pruned++
		}
	}
	if pruned > 0 {
		s.Rules = newRules
		s.Version++
		s.ComputeHash()
	}
	return pruned
}

// CanonicalString returns a deterministic string representation.
func (s *SuppressionSet) CanonicalString() string {
	var sb strings.Builder
	sb.WriteString("version:")
	sb.WriteString(fmt.Sprintf("%d", s.Version))
	sb.WriteString("|rules:[")
	for i, r := range s.Rules {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(r.CanonicalString())
	}
	sb.WriteString("]")
	return sb.String()
}

// ComputeHash computes and sets the Hash field.
func (s *SuppressionSet) ComputeHash() string {
	canonical := s.CanonicalString()
	hash := sha256.Sum256([]byte(canonical))
	s.Hash = hex.EncodeToString(hash[:])
	return s.Hash
}

// sort sorts rules deterministically by CircleID, Scope, Key, CreatedAt.
func (s *SuppressionSet) sort() {
	// Bubble sort for stdlib-only
	for i := 0; i < len(s.Rules); i++ {
		for j := i + 1; j < len(s.Rules); j++ {
			if s.shouldSwap(s.Rules[i], s.Rules[j]) {
				s.Rules[i], s.Rules[j] = s.Rules[j], s.Rules[i]
			}
		}
	}
}

// shouldSwap returns true if a should come after b.
func (s *SuppressionSet) shouldSwap(a, b SuppressionRule) bool {
	if a.CircleID != b.CircleID {
		return a.CircleID > b.CircleID
	}
	if a.Scope != b.Scope {
		return string(a.Scope) > string(b.Scope)
	}
	if a.Key != b.Key {
		return a.Key > b.Key
	}
	return a.CreatedAt.After(b.CreatedAt)
}

// Stats returns statistics about the suppression set.
type Stats struct {
	TotalRules   int
	ActiveRules  int
	ExpiredRules int
	ByScope      map[Scope]int
	ByCircle     map[string]int
}

// GetStats returns statistics about the suppression set.
func (s *SuppressionSet) GetStats(at time.Time) Stats {
	stats := Stats{
		TotalRules: len(s.Rules),
		ByScope:    make(map[Scope]int),
		ByCircle:   make(map[string]int),
	}
	for _, r := range s.Rules {
		if r.IsActive(at) {
			stats.ActiveRules++
		} else {
			stats.ExpiredRules++
		}
		stats.ByScope[r.Scope]++
		stats.ByCircle[r.CircleID]++
	}
	return stats
}
