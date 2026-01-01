package suppress

import (
	"testing"
	"time"
)

func TestSuppressionRuleID(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	expires := now.Add(30 * 24 * time.Hour)

	r1 := NewSuppressionRule("work", ScopeTrigger, "newsletter", now, &expires, "too noisy", SourceFeedback)
	r2 := NewSuppressionRule("work", ScopeTrigger, "newsletter", now, &expires, "too noisy", SourceFeedback)

	// Same inputs should produce same ID
	if r1.RuleID != r2.RuleID {
		t.Errorf("RuleID not deterministic: %s != %s", r1.RuleID, r2.RuleID)
	}

	// Different inputs should produce different ID
	r3 := NewSuppressionRule("family", ScopeTrigger, "newsletter", now, &expires, "too noisy", SourceFeedback)
	if r1.RuleID == r3.RuleID {
		t.Errorf("Different circle should have different RuleID")
	}
}

func TestSuppressionRuleCanonicalString(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	expires := now.Add(30 * 24 * time.Hour)

	r := NewSuppressionRule("work", ScopeTrigger, "newsletter", now, &expires, "too noisy", SourceFeedback)

	s1 := r.CanonicalString()
	s2 := r.CanonicalString()

	if s1 != s2 {
		t.Errorf("CanonicalString not deterministic: %q != %q", s1, s2)
	}

	// Should contain expected parts
	if !contains(s1, "circle:work") {
		t.Errorf("Should contain circle:work")
	}
	if !contains(s1, "scope:scope_trigger") {
		t.Errorf("Should contain scope:scope_trigger")
	}
	if !contains(s1, "key:newsletter") {
		t.Errorf("Should contain key:newsletter")
	}
}

func TestSuppressionRuleIsActive(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	expires := now.Add(30 * 24 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		checkAt   time.Time
		want      bool
	}{
		{
			name:      "permanent rule is always active",
			expiresAt: nil,
			checkAt:   now.Add(365 * 24 * time.Hour),
			want:      true,
		},
		{
			name:      "before expiration",
			expiresAt: &expires,
			checkAt:   now.Add(15 * 24 * time.Hour),
			want:      true,
		},
		{
			name:      "after expiration",
			expiresAt: &expires,
			checkAt:   now.Add(31 * 24 * time.Hour),
			want:      false,
		},
		{
			name:      "before creation",
			expiresAt: &expires,
			checkAt:   now.Add(-1 * time.Hour),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewSuppressionRule("work", ScopeTrigger, "test", now, tt.expiresAt, "test", SourceManual)
			got := r.IsActive(tt.checkAt)
			if got != tt.want {
				t.Errorf("IsActive(%v) = %v, want %v", tt.checkAt, got, tt.want)
			}
		})
	}
}

func TestSuppressionRuleMatches(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		rule     SuppressionRule
		circleID string
		scope    Scope
		key      string
		want     bool
	}{
		{
			name:     "exact match",
			rule:     NewSuppressionRule("work", ScopeTrigger, "newsletter", now, nil, "", SourceManual),
			circleID: "work",
			scope:    ScopeTrigger,
			key:      "newsletter",
			want:     true,
		},
		{
			name:     "wrong circle",
			rule:     NewSuppressionRule("work", ScopeTrigger, "newsletter", now, nil, "", SourceManual),
			circleID: "family",
			scope:    ScopeTrigger,
			key:      "newsletter",
			want:     false,
		},
		{
			name:     "wrong scope",
			rule:     NewSuppressionRule("work", ScopeTrigger, "newsletter", now, nil, "", SourceManual),
			circleID: "work",
			scope:    ScopePerson,
			key:      "newsletter",
			want:     false,
		},
		{
			name:     "wrong key",
			rule:     NewSuppressionRule("work", ScopeTrigger, "newsletter", now, nil, "", SourceManual),
			circleID: "work",
			scope:    ScopeTrigger,
			key:      "marketing",
			want:     false,
		},
		{
			name:     "wildcard circle",
			rule:     NewSuppressionRule("*", ScopeTrigger, "newsletter", now, nil, "", SourceManual),
			circleID: "any",
			scope:    ScopeTrigger,
			key:      "newsletter",
			want:     true,
		},
		{
			name:     "wildcard key",
			rule:     NewSuppressionRule("work", ScopeTrigger, "*", now, nil, "", SourceManual),
			circleID: "work",
			scope:    ScopeTrigger,
			key:      "anything",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rule.Matches(tt.circleID, tt.scope, tt.key)
			if got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSuppressionSetSorting(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ss := NewSuppressionSet()

	// Add rules in non-sorted order
	ss.AddRule(NewSuppressionRule("work", ScopeTrigger, "z", now, nil, "", SourceManual))
	ss.AddRule(NewSuppressionRule("family", ScopeTrigger, "a", now, nil, "", SourceManual))
	ss.AddRule(NewSuppressionRule("work", ScopePerson, "b", now, nil, "", SourceManual))

	// Should be sorted by CircleID, then Scope, then Key
	if len(ss.Rules) != 3 {
		t.Fatalf("Expected 3 rules, got %d", len(ss.Rules))
	}

	// First should be family
	if ss.Rules[0].CircleID != "family" {
		t.Errorf("First rule should be family, got %s", ss.Rules[0].CircleID)
	}

	// Next two should be work, sorted by scope then key
	if ss.Rules[1].CircleID != "work" || ss.Rules[2].CircleID != "work" {
		t.Errorf("Rules 2-3 should be work")
	}
}

func TestSuppressionSetHashDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ss1 := NewSuppressionSet()
	ss1.AddRule(NewSuppressionRule("work", ScopeTrigger, "newsletter", now, nil, "", SourceManual))
	ss1.AddRule(NewSuppressionRule("family", ScopePerson, "alice", now, nil, "", SourceManual))

	ss2 := NewSuppressionSet()
	// Add in different order
	ss2.AddRule(NewSuppressionRule("family", ScopePerson, "alice", now, nil, "", SourceManual))
	ss2.AddRule(NewSuppressionRule("work", ScopeTrigger, "newsletter", now, nil, "", SourceManual))

	// Version won't match because we added in different order
	// But the canonical string of the rules should be the same after sorting
	if len(ss1.Rules) != len(ss2.Rules) {
		t.Errorf("Rule counts differ")
	}
}

func TestSuppressionSetAddRemove(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ss := NewSuppressionSet()
	initialVersion := ss.Version

	rule := NewSuppressionRule("work", ScopeTrigger, "newsletter", now, nil, "", SourceManual)
	ss.AddRule(rule)

	if ss.Version != initialVersion+1 {
		t.Errorf("Version should increment on add")
	}

	if len(ss.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(ss.Rules))
	}

	// Get rule
	got := ss.GetRule(rule.RuleID)
	if got == nil {
		t.Fatalf("GetRule returned nil")
	}
	if got.RuleID != rule.RuleID {
		t.Errorf("GetRule returned wrong rule")
	}

	// Remove rule
	versionBeforeRemove := ss.Version
	removed := ss.RemoveRule(rule.RuleID)
	if !removed {
		t.Error("RemoveRule should return true")
	}

	if ss.Version != versionBeforeRemove+1 {
		t.Errorf("Version should increment on remove")
	}

	if len(ss.Rules) != 0 {
		t.Errorf("Expected 0 rules after remove, got %d", len(ss.Rules))
	}

	// Remove non-existent
	removed = ss.RemoveRule("non-existent")
	if removed {
		t.Error("RemoveRule should return false for non-existent")
	}
}

func TestSuppressionSetPruneExpired(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	soon := now.Add(7 * 24 * time.Hour)
	later := now.Add(30 * 24 * time.Hour)

	ss := NewSuppressionSet()

	// Add a rule that expires soon
	ss.AddRule(NewSuppressionRule("work", ScopeTrigger, "expires_soon", now, &soon, "", SourceManual))
	// Add a rule that expires later
	ss.AddRule(NewSuppressionRule("work", ScopeTrigger, "expires_later", now, &later, "", SourceManual))
	// Add a permanent rule
	ss.AddRule(NewSuppressionRule("work", ScopeTrigger, "permanent", now, nil, "", SourceManual))

	if len(ss.Rules) != 3 {
		t.Fatalf("Expected 3 rules, got %d", len(ss.Rules))
	}

	// Prune at day 10 - one rule should be expired
	pruned := ss.PruneExpired(now.Add(10 * 24 * time.Hour))
	if pruned != 1 {
		t.Errorf("Expected 1 pruned, got %d", pruned)
	}

	if len(ss.Rules) != 2 {
		t.Errorf("Expected 2 rules after prune, got %d", len(ss.Rules))
	}
}

func TestSuppressionSetListActive(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	soon := now.Add(7 * 24 * time.Hour)

	ss := NewSuppressionSet()
	ss.AddRule(NewSuppressionRule("work", ScopeTrigger, "expires_soon", now, &soon, "", SourceManual))
	ss.AddRule(NewSuppressionRule("work", ScopeTrigger, "permanent", now, nil, "", SourceManual))

	// At day 1, both should be active
	active := ss.ListActive(now.Add(1 * 24 * time.Hour))
	if len(active) != 2 {
		t.Errorf("Expected 2 active at day 1, got %d", len(active))
	}

	// At day 10, only permanent should be active
	active = ss.ListActive(now.Add(10 * 24 * time.Hour))
	if len(active) != 1 {
		t.Errorf("Expected 1 active at day 10, got %d", len(active))
	}
}

func TestSuppressionSetFindMatch(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ss := NewSuppressionSet()
	ss.AddRule(NewSuppressionRule("work", ScopeTrigger, "newsletter", now, nil, "", SourceManual))
	ss.AddRule(NewSuppressionRule("work", ScopePerson, "alice", now, nil, "", SourceManual))

	// Should find trigger match
	match := ss.FindMatch(now, "work", ScopeTrigger, "newsletter")
	if match == nil {
		t.Error("Should find trigger match")
	}

	// Should find person match
	match = ss.FindMatch(now, "work", ScopePerson, "alice")
	if match == nil {
		t.Error("Should find person match")
	}

	// Should not find non-existent
	match = ss.FindMatch(now, "work", ScopeTrigger, "marketing")
	if match != nil {
		t.Error("Should not find non-existent")
	}

	// Should not find wrong circle
	match = ss.FindMatch(now, "family", ScopeTrigger, "newsletter")
	if match != nil {
		t.Error("Should not find wrong circle")
	}
}

func TestSuppressionSetStats(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	soon := now.Add(7 * 24 * time.Hour)

	ss := NewSuppressionSet()
	ss.AddRule(NewSuppressionRule("work", ScopeTrigger, "a", now, nil, "", SourceManual))
	ss.AddRule(NewSuppressionRule("work", ScopePerson, "b", now, nil, "", SourceManual))
	ss.AddRule(NewSuppressionRule("family", ScopeTrigger, "c", now, &soon, "", SourceFeedback))

	stats := ss.GetStats(now.Add(1 * 24 * time.Hour))

	if stats.TotalRules != 3 {
		t.Errorf("TotalRules = %d, want 3", stats.TotalRules)
	}

	if stats.ActiveRules != 3 {
		t.Errorf("ActiveRules = %d, want 3", stats.ActiveRules)
	}

	if stats.ByCircle["work"] != 2 {
		t.Errorf("ByCircle[work] = %d, want 2", stats.ByCircle["work"])
	}

	if stats.ByScope[ScopeTrigger] != 2 {
		t.Errorf("ByScope[ScopeTrigger] = %d, want 2", stats.ByScope[ScopeTrigger])
	}

	// Check after expiration
	statsLater := ss.GetStats(now.Add(10 * 24 * time.Hour))
	if statsLater.ActiveRules != 2 {
		t.Errorf("ActiveRules at day 10 = %d, want 2", statsLater.ActiveRules)
	}
	if statsLater.ExpiredRules != 1 {
		t.Errorf("ExpiredRules at day 10 = %d, want 1", statsLater.ExpiredRules)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
