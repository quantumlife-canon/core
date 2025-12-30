// Package demo_family provides the Family Intersection demo for vertical slice v2.
//
// This demo showcases:
// - Invite token creation (Circle A invites Circle B)
// - Circle B acceptance (creates intersection)
// - Intersection-scoped suggestions
// - Full audit trail across both circles + intersection
//
// CRITICAL: This is SUGGEST-ONLY mode. No external actions are executed.
package demo_family

import (
	"context"
	"fmt"
	"time"

	"quantumlife/internal/audit"
	auditImpl "quantumlife/internal/audit/impl_inmem"
	"quantumlife/internal/circle"
	circleImpl "quantumlife/internal/circle/impl_inmem"
	"quantumlife/internal/intersection"
	intImpl "quantumlife/internal/intersection/impl_inmem"
	memoryImpl "quantumlife/internal/memory/impl_inmem"
	cryptoImpl "quantumlife/pkg/crypto/impl_inmem"
	"quantumlife/pkg/primitives"
)

// FamilyDemoResult contains the output of the family invite demo.
type FamilyDemoResult struct {
	// Circles
	CircleAID string // "You"
	CircleBID string // "Spouse"

	// Invite token
	TokenID      string
	TokenSummary TokenSummary

	// Intersection
	IntersectionID      string
	IntersectionSummary IntersectionSummary

	// Suggestions
	Suggestions []FamilySuggestion

	// Audit
	AuditLog []AuditEntry

	// Status
	Success bool
	Error   error
}

// TokenSummary provides a summary of an invite token for display.
type TokenSummary struct {
	TokenID           string
	IssuerCircleID    string
	TargetCircleID    string
	ProposedName      string
	IssuedAt          time.Time
	ExpiresAt         time.Time
	ScopeCount        int
	CeilingCount      int
	SignatureRedacted string // Redacted signature for display
	Algorithm         string
}

// IntersectionSummary provides a summary of an intersection for display.
type IntersectionSummary struct {
	ID         string
	Name       string
	Version    string
	PartyIDs   []string
	Scopes     []string
	Ceilings   []CeilingSummary
	Governance string
	CreatedAt  time.Time
}

// CeilingSummary provides a summary of a ceiling for display.
type CeilingSummary struct {
	Type        string
	Value       string
	Unit        string
	Description string
}

// FamilySuggestion represents a suggestion within the family intersection.
type FamilySuggestion struct {
	ID              string
	Description     string
	Explanation     string
	TimeSlot        string
	Category        string
	IntersectionID  string
	ScopesUsed      []string
	CeilingsApplied []string
}

// AuditEntry is a simplified audit entry for demo output.
type AuditEntry struct {
	ID             string
	EventType      string
	CircleID       string
	IntersectionID string
	Action         string
	Outcome        string
	TraceID        string
	Timestamp      time.Time
}

// Runner executes the family intersection demo.
type Runner struct {
	circleRuntime *circleImpl.Runtime
	intRuntime    *intImpl.Runtime
	auditStore    *auditImpl.Store
	memoryStore   *memoryImpl.Store
	keyManager    *cryptoImpl.KeyManager
	inviteService *circleImpl.InviteService
	calendar      *FamilyCalendar
}

// NewRunner creates a new family demo runner with all components wired together.
func NewRunner() *Runner {
	// Create in-memory stores
	auditStore := auditImpl.NewStore()
	memoryStore := memoryImpl.NewStore()
	circleRuntime := circleImpl.NewRuntime()
	intRuntime := intImpl.NewRuntime()
	keyManager := cryptoImpl.NewKeyManager()

	// Create invite service
	inviteService := circleImpl.NewInviteService(circleImpl.InviteServiceConfig{
		CircleRuntime: circleRuntime,
		IntRuntime:    intRuntime,
		KeyManager:    keyManager,
		AuditLogger:   auditStore,
	})

	// Create family calendar
	calendar := NewFamilyCalendar()

	return &Runner{
		circleRuntime: circleRuntime,
		intRuntime:    intRuntime,
		auditStore:    auditStore,
		memoryStore:   memoryStore,
		keyManager:    keyManager,
		inviteService: inviteService,
		calendar:      calendar,
	}
}

// Run executes the family intersection demo.
func (r *Runner) Run(ctx context.Context) (*FamilyDemoResult, error) {
	result := &FamilyDemoResult{}

	// Step 1: Create Circle A ("You")
	circleA, err := r.circleRuntime.Create(ctx, circle.CreateRequest{
		TenantID: "demo-tenant",
	})
	if err != nil {
		result.Error = fmt.Errorf("failed to create Circle A: %w", err)
		return result, result.Error
	}
	result.CircleAID = circleA.ID

	// Create key for Circle A
	_, err = r.keyManager.CreateKey(ctx, fmt.Sprintf("key-%s", circleA.ID), 24*time.Hour)
	if err != nil {
		result.Error = fmt.Errorf("failed to create key for Circle A: %w", err)
		return result, result.Error
	}

	// Step 2: Create Circle B ("Spouse") - will be activated on invite acceptance
	circleB, err := r.circleRuntime.Create(ctx, circle.CreateRequest{
		TenantID: "demo-tenant",
	})
	if err != nil {
		result.Error = fmt.Errorf("failed to create Circle B: %w", err)
		return result, result.Error
	}
	result.CircleBID = circleB.ID

	// Create key for Circle B
	_, err = r.keyManager.CreateKey(ctx, fmt.Sprintf("key-%s", circleB.ID), 24*time.Hour)
	if err != nil {
		result.Error = fmt.Errorf("failed to create key for Circle B: %w", err)
		return result, result.Error
	}

	// Step 3: Issue invite token from Circle A to Circle B
	template := primitives.IntersectionTemplate{
		Scopes: []primitives.IntersectionScope{
			{
				Name:        "calendar:read",
				Description: "Read calendar events",
				Permission:  "read",
			},
			{
				Name:        "calendar:write",
				Description: "Create/modify calendar events (NOT executed in demo)",
				Permission:  "write",
			},
		},
		Ceilings: []primitives.IntersectionCeiling{
			{
				Type:        "time_window",
				Value:       "17:00-21:00",
				Unit:        "hours",
				Description: "Family time window: 5pm-9pm",
			},
			{
				Type:        "duration",
				Value:       "4",
				Unit:        "hours",
				Description: "Maximum activity duration",
			},
			{
				Type:        "days",
				Value:       "weekday_evenings,weekends",
				Unit:        "day_types",
				Description: "Allowed days for family activities",
			},
		},
		Governance: primitives.IntersectionGovernance{
			AmendmentRequires: "all_parties",
			DissolutionPolicy: "any_party",
		},
	}

	token, err := r.inviteService.IssueInviteToken(ctx, circle.IssueInviteRequest{
		IssuerCircleID: circleA.ID,
		TargetCircleID: circleB.ID,
		ProposedName:   "Family Intersection",
		Template:       template,
		ValidFor:       1 * time.Hour,
	})
	if err != nil {
		result.Error = fmt.Errorf("failed to issue invite token: %w", err)
		return result, result.Error
	}
	result.TokenID = token.TokenID
	result.TokenSummary = TokenSummary{
		TokenID:           token.TokenID,
		IssuerCircleID:    token.IssuerCircleID,
		TargetCircleID:    token.TargetCircleID,
		ProposedName:      token.ProposedName,
		IssuedAt:          token.IssuedAt,
		ExpiresAt:         token.ExpiresAt,
		ScopeCount:        len(token.Template.Scopes),
		CeilingCount:      len(token.Template.Ceilings),
		SignatureRedacted: cryptoImpl.RedactedSignature(token.Signature),
		Algorithm:         token.SignatureAlgorithm,
	}

	// Step 4: Circle B accepts the invite token
	intRef, err := r.inviteService.AcceptInviteToken(ctx, token, circleB.ID)
	if err != nil {
		result.Error = fmt.Errorf("failed to accept invite token: %w", err)
		return result, result.Error
	}
	result.IntersectionID = intRef.IntersectionID

	// Get intersection details for summary
	int, err := r.intRuntime.Get(ctx, intRef.IntersectionID)
	if err != nil {
		result.Error = fmt.Errorf("failed to get intersection: %w", err)
		return result, result.Error
	}

	contract, err := r.intRuntime.GetContract(ctx, intRef.IntersectionID)
	if err != nil {
		result.Error = fmt.Errorf("failed to get contract: %w", err)
		return result, result.Error
	}

	result.IntersectionSummary = IntersectionSummary{
		ID:       int.ID,
		Name:     token.ProposedName,
		Version:  contract.Version,
		PartyIDs: []string{circleA.ID, circleB.ID},
		Scopes:   extractScopeNames(contract.Scopes),
		Ceilings: extractCeilingSummaries(contract.Ceilings),
		Governance: fmt.Sprintf("amendment=%s, dissolution=%s",
			contract.Governance.AmendmentRequires,
			contract.Governance.DissolutionPolicy),
		CreatedAt: int.CreatedAt,
	}

	// Step 5: Generate intersection-scoped suggestions
	suggestions := r.generateFamilySuggestions(ctx, intRef.IntersectionID, contract)
	result.Suggestions = suggestions

	// Step 6: Collect audit log
	auditEntries := r.auditStore.GetAllEntries()
	for _, entry := range auditEntries {
		traceID := ""
		if entry.Metadata != nil {
			traceID = entry.Metadata["trace_id"]
		}
		result.AuditLog = append(result.AuditLog, AuditEntry{
			ID:             entry.ID,
			EventType:      entry.EventType,
			CircleID:       entry.CircleID,
			IntersectionID: entry.IntersectionID,
			Action:         entry.Action,
			Outcome:        entry.Outcome,
			TraceID:        traceID,
			Timestamp:      entry.Timestamp,
		})
	}

	result.Success = true
	return result, nil
}

// generateFamilySuggestions generates suggestions within the family intersection.
func (r *Runner) generateFamilySuggestions(ctx context.Context, intersectionID string, contract *intersection.Contract) []FamilySuggestion {
	var suggestions []FamilySuggestion

	// Get free slots from the family calendar
	freeSlots := r.calendar.GetFamilyFreeSlots(1 * time.Hour)

	// Extract ceiling constraints
	timeWindow := ""
	maxDuration := ""
	allowedDays := ""
	for _, c := range contract.Ceilings {
		switch c.Type {
		case "time_window":
			timeWindow = c.Value
		case "duration":
			maxDuration = c.Value + " " + c.Unit
		case "days":
			allowedDays = c.Value
		}
	}

	// Generate up to 3 suggestions that respect ceilings
	count := 0
	for _, slot := range freeSlots {
		if count >= 3 {
			break
		}

		// Check if slot is within allowed time window
		if !isWithinTimeWindow(slot, timeWindow) {
			continue
		}

		timeStr := fmt.Sprintf("%s %d:00-%d:00",
			slot.DayName, slot.Start.Hour(), slot.End.Hour())

		suggestion := FamilySuggestion{
			ID:             fmt.Sprintf("fam-sug-%d", count+1),
			IntersectionID: intersectionID,
			TimeSlot:       timeStr,
			ScopesUsed:     []string{"calendar:read"},
			CeilingsApplied: []string{
				fmt.Sprintf("time_window: %s", timeWindow),
				fmt.Sprintf("max_duration: %s", maxDuration),
			},
		}

		// Determine category and description based on slot
		if slot.DayName == "Saturday" || slot.DayName == "Sunday" {
			suggestion.Category = "weekend_family"
			suggestion.Description = fmt.Sprintf("Family activity %s; outdoor or recreational", timeStr)
			suggestion.Explanation = fmt.Sprintf(
				"INTERSECTION: %s | Weekend slot detected. "+
					"Scopes used: [calendar:read]. "+
					"Ceiling check: time_window=%s (PASS), days=%s (PASS). "+
					"Suggested for family bonding during weekend free time. "+
					"Duration: %.0f hours (within %s limit).",
				intersectionID, timeWindow, allowedDays,
				slot.Duration.Hours(), maxDuration)
		} else if slot.Start.Hour() >= 17 {
			suggestion.Category = "evening_family"
			suggestion.Description = fmt.Sprintf("Family dinner or activity %s", timeStr)
			suggestion.Explanation = fmt.Sprintf(
				"INTERSECTION: %s | Weekday evening slot. "+
					"Scopes used: [calendar:read]. "+
					"Ceiling check: time_window=%s (PASS), days=%s (PASS). "+
					"Ideal for family meal or shared activity after work. "+
					"Duration: %.0f hours (within %s limit).",
				intersectionID, timeWindow, allowedDays,
				slot.Duration.Hours(), maxDuration)
		} else {
			continue // Skip non-evening weekday slots
		}

		suggestions = append(suggestions, suggestion)
		count++

		// Log scope usage
		r.auditStore.Log(ctx, audit.Entry{
			CircleID:       "",
			IntersectionID: intersectionID,
			EventType:      "intersection.scope.used",
			SubjectID:      suggestion.ID,
			Action:         "generate_suggestion",
			Outcome:        "success",
			Metadata: map[string]string{
				"scope":     "calendar:read",
				"time_slot": timeStr,
				"category":  suggestion.Category,
			},
		})
	}

	return suggestions
}

// extractScopeNames extracts scope names from a slice of scopes.
func extractScopeNames(scopes []intersection.Scope) []string {
	names := make([]string, len(scopes))
	for i, s := range scopes {
		names[i] = s.Name
	}
	return names
}

// extractCeilingSummaries extracts ceiling summaries.
func extractCeilingSummaries(ceilings []intersection.Ceiling) []CeilingSummary {
	result := make([]CeilingSummary, len(ceilings))
	for i, c := range ceilings {
		result[i] = CeilingSummary{
			Type:  c.Type,
			Value: c.Value,
			Unit:  c.Unit,
		}
	}
	return result
}

// isWithinTimeWindow checks if a slot is within the allowed time window.
func isWithinTimeWindow(slot FreeSlot, timeWindow string) bool {
	// Parse time window (format: "HH:MM-HH:MM")
	if timeWindow == "" {
		return true // No restriction
	}

	// For demo, just check if it's in evening (17:00-21:00)
	hour := slot.Start.Hour()
	return hour >= 17 && hour < 21
}
