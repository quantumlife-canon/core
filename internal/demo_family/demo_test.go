// Package demo_family provides tests for the Family Intersection demo (Vertical Slice v2).
package demo_family

import (
	"context"
	"strings"
	"testing"
	"time"

	"quantumlife/internal/audit"
	auditImpl "quantumlife/internal/audit/impl_inmem"
	"quantumlife/internal/circle"
	circleImpl "quantumlife/internal/circle/impl_inmem"
	intImpl "quantumlife/internal/intersection/impl_inmem"
	cryptoImpl "quantumlife/pkg/crypto/impl_inmem"
	"quantumlife/pkg/primitives"
)

// TestFamilyDemoFullFlow runs the complete family demo and validates outputs.
func TestFamilyDemoFullFlow(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("Demo was not successful: %v", result.Error)
	}

	// Verify circles were created
	if result.CircleAID == "" {
		t.Error("Expected CircleAID to be set")
	}
	if result.CircleBID == "" {
		t.Error("Expected CircleBID to be set")
	}

	// Verify intersection was created
	if result.IntersectionID == "" {
		t.Error("Expected IntersectionID to be set")
	}

	// Verify token was created
	if result.TokenID == "" {
		t.Error("Expected TokenID to be set")
	}
}

// TestTokenVerificationFailsIfExpired ensures expired tokens are rejected.
func TestTokenVerificationFailsIfExpired(t *testing.T) {
	ctx := context.Background()

	// Create components
	auditStore := auditImpl.NewStore()
	circleRuntime := circleImpl.NewRuntime()
	intRuntime := intImpl.NewRuntime()
	keyManager := cryptoImpl.NewKeyManager()

	inviteService := circleImpl.NewInviteService(circleImpl.InviteServiceConfig{
		CircleRuntime: circleRuntime,
		IntRuntime:    intRuntime,
		KeyManager:    keyManager,
		AuditLogger:   auditStore,
	})

	// Create circle and key
	circleA, err := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	if err != nil {
		t.Fatalf("Failed to create circle: %v", err)
	}
	_, err = keyManager.CreateKey(ctx, "key-"+circleA.ID, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create key: %v", err)
	}

	// Issue token with very short expiry
	template := primitives.IntersectionTemplate{
		Scopes: []primitives.IntersectionScope{
			{Name: "test:read", Permission: "read"},
		},
	}

	token, err := inviteService.IssueInviteToken(ctx, circle.IssueInviteRequest{
		IssuerCircleID: circleA.ID,
		ProposedName:   "Test Intersection",
		Template:       template,
		ValidFor:       1 * time.Nanosecond, // Expire immediately
	})
	if err != nil {
		t.Fatalf("Failed to issue token: %v", err)
	}

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	// Validate should fail
	err = inviteService.ValidateInviteToken(ctx, token)
	if err == nil {
		t.Error("Expected validation to fail for expired token")
	}
	if err != primitives.ErrTokenExpired {
		t.Errorf("Expected ErrTokenExpired, got: %v", err)
	}
}

// TestTokenVerificationFailsIfTampered ensures tampered tokens are rejected.
func TestTokenVerificationFailsIfTampered(t *testing.T) {
	ctx := context.Background()

	// Create components
	auditStore := auditImpl.NewStore()
	circleRuntime := circleImpl.NewRuntime()
	intRuntime := intImpl.NewRuntime()
	keyManager := cryptoImpl.NewKeyManager()

	inviteService := circleImpl.NewInviteService(circleImpl.InviteServiceConfig{
		CircleRuntime: circleRuntime,
		IntRuntime:    intRuntime,
		KeyManager:    keyManager,
		AuditLogger:   auditStore,
	})

	// Create circle and key
	circleA, err := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	if err != nil {
		t.Fatalf("Failed to create circle: %v", err)
	}
	_, err = keyManager.CreateKey(ctx, "key-"+circleA.ID, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create key: %v", err)
	}

	// Issue valid token
	template := primitives.IntersectionTemplate{
		Scopes: []primitives.IntersectionScope{
			{Name: "test:read", Permission: "read"},
		},
	}

	token, err := inviteService.IssueInviteToken(ctx, circle.IssueInviteRequest{
		IssuerCircleID: circleA.ID,
		ProposedName:   "Test Intersection",
		Template:       template,
		ValidFor:       1 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Failed to issue token: %v", err)
	}

	// Tamper with the token
	token.ProposedName = "TAMPERED NAME"

	// Validate should fail
	err = inviteService.ValidateInviteToken(ctx, token)
	if err == nil {
		t.Error("Expected validation to fail for tampered token")
	}
	if err != primitives.ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature, got: %v", err)
	}
}

// TestAcceptingTokenCreatesIntersectionV1 ensures accepting a token creates intersection v1.0.0.
func TestAcceptingTokenCreatesIntersectionV1(t *testing.T) {
	ctx := context.Background()

	// Create components
	auditStore := auditImpl.NewStore()
	circleRuntime := circleImpl.NewRuntime()
	intRuntime := intImpl.NewRuntime()
	keyManager := cryptoImpl.NewKeyManager()

	inviteService := circleImpl.NewInviteService(circleImpl.InviteServiceConfig{
		CircleRuntime: circleRuntime,
		IntRuntime:    intRuntime,
		KeyManager:    keyManager,
		AuditLogger:   auditStore,
	})

	// Create circles and keys
	circleA, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	circleB, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	keyManager.CreateKey(ctx, "key-"+circleA.ID, 24*time.Hour)
	keyManager.CreateKey(ctx, "key-"+circleB.ID, 24*time.Hour)

	// Issue token
	template := primitives.IntersectionTemplate{
		Scopes: []primitives.IntersectionScope{
			{Name: "calendar:read", Permission: "read"},
			{Name: "calendar:write", Permission: "write"},
		},
		Ceilings: []primitives.IntersectionCeiling{
			{Type: "time_window", Value: "17:00-21:00", Unit: "hours"},
		},
		Governance: primitives.IntersectionGovernance{
			AmendmentRequires: "all_parties",
			DissolutionPolicy: "any_party",
		},
	}

	token, err := inviteService.IssueInviteToken(ctx, circle.IssueInviteRequest{
		IssuerCircleID: circleA.ID,
		TargetCircleID: circleB.ID,
		ProposedName:   "Family Intersection",
		Template:       template,
		ValidFor:       1 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Failed to issue token: %v", err)
	}

	// Accept token
	intRef, err := inviteService.AcceptInviteToken(ctx, token, circleB.ID)
	if err != nil {
		t.Fatalf("Failed to accept token: %v", err)
	}

	// Verify intersection was created
	if intRef.IntersectionID == "" {
		t.Error("Expected IntersectionID to be set")
	}

	// Verify version is 1.0.0
	if intRef.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", intRef.Version)
	}

	// Verify contract parties
	contract, err := intRuntime.GetContract(ctx, intRef.IntersectionID)
	if err != nil {
		t.Fatalf("Failed to get contract: %v", err)
	}

	if len(contract.Parties) != 2 {
		t.Errorf("Expected 2 parties, got %d", len(contract.Parties))
	}

	partyIDs := make(map[string]bool)
	for _, party := range contract.Parties {
		partyIDs[party.CircleID] = true
	}

	if !partyIDs[circleA.ID] {
		t.Errorf("Circle A (%s) not in parties", circleA.ID)
	}
	if !partyIDs[circleB.ID] {
		t.Errorf("Circle B (%s) not in parties", circleB.ID)
	}

	// Verify scopes were transferred
	if len(contract.Scopes) != 2 {
		t.Errorf("Expected 2 scopes, got %d", len(contract.Scopes))
	}

	// Verify ceilings were transferred
	if len(contract.Ceilings) != 1 {
		t.Errorf("Expected 1 ceiling, got %d", len(contract.Ceilings))
	}
}

// TestSuggestionsIncludeIntersectionID ensures suggestions reference intersection.
func TestSuggestionsIncludeIntersectionID(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	if len(result.Suggestions) == 0 {
		t.Fatal("No suggestions produced")
	}

	for i, sug := range result.Suggestions {
		if sug.IntersectionID == "" {
			t.Errorf("Suggestion %d has no IntersectionID", i)
		}
		if sug.IntersectionID != result.IntersectionID {
			t.Errorf("Suggestion %d has wrong IntersectionID: got %s, want %s",
				i, sug.IntersectionID, result.IntersectionID)
		}
	}
}

// TestSuggestionsRespectCeilings ensures suggestions respect intersection ceilings.
func TestSuggestionsRespectCeilings(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	if len(result.Suggestions) == 0 {
		t.Fatal("No suggestions produced")
	}

	// The demo configures time_window ceiling of 17:00-21:00
	for i, sug := range result.Suggestions {
		// Check that ceiling info is included
		if len(sug.CeilingsApplied) == 0 {
			t.Errorf("Suggestion %d has no ceilings applied", i)
		}

		// Check that time window ceiling was checked
		foundTimeWindow := false
		for _, c := range sug.CeilingsApplied {
			if strings.Contains(c, "time_window") {
				foundTimeWindow = true
				break
			}
		}
		if !foundTimeWindow {
			t.Errorf("Suggestion %d does not reference time_window ceiling", i)
		}

		// Verify suggestion is within allowed hours (17:00-21:00)
		// TimeSlot format is like "Monday 17:00-21:00"
		if !strings.Contains(sug.TimeSlot, "17:") &&
			!strings.Contains(sug.TimeSlot, "18:") &&
			!strings.Contains(sug.TimeSlot, "19:") &&
			!strings.Contains(sug.TimeSlot, "20:") {
			t.Errorf("Suggestion %d time slot %s may be outside allowed window",
				i, sug.TimeSlot)
		}
	}
}

// TestSuggestionsIncludeScopesUsed ensures suggestions show which scopes were used.
func TestSuggestionsIncludeScopesUsed(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	for i, sug := range result.Suggestions {
		if len(sug.ScopesUsed) == 0 {
			t.Errorf("Suggestion %d has no scopes used", i)
		}
		// Should use calendar:read scope
		foundCalendarRead := false
		for _, scope := range sug.ScopesUsed {
			if scope == "calendar:read" {
				foundCalendarRead = true
				break
			}
		}
		if !foundCalendarRead {
			t.Errorf("Suggestion %d does not use calendar:read scope", i)
		}
	}
}

// TestNoExecutionLayerInvoked ensures no execution layer is invoked.
func TestNoExecutionLayerInvoked(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	// Check that no audit entries indicate external execution
	for _, entry := range result.AuditLog {
		if strings.Contains(entry.Action, "execute_external") {
			t.Errorf("Found external execution in audit: %s", entry.Action)
		}
		if strings.Contains(entry.Action, "write_external") {
			t.Errorf("Found external write in audit: %s", entry.Action)
		}
		if strings.Contains(entry.Action, "connector_call") {
			t.Errorf("Found connector call in audit: %s", entry.Action)
		}
		if strings.Contains(entry.Action, "calendar:write") && entry.Outcome == "executed" {
			t.Errorf("Found calendar write execution in audit: %s", entry.Action)
		}
	}

	// Verify suggestions don't claim to have executed anything
	for i, sug := range result.Suggestions {
		if strings.Contains(strings.ToLower(sug.Description), "executed") {
			t.Errorf("Suggestion %d appears to describe execution: %s", i, sug.Description)
		}
		if strings.Contains(strings.ToLower(sug.Description), "created event") {
			t.Errorf("Suggestion %d appears to claim event creation: %s", i, sug.Description)
		}
	}
}

// TestAuditContainsExpectedEvents ensures audit log contains expected events in order.
func TestAuditContainsExpectedEvents(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	// Expected events in order
	expectedEvents := []string{
		"invite.token.issued",
		"invite.token.accepted",
		"intersection.created",
	}

	// Track event order
	eventOrder := make(map[string]int)
	for i, entry := range result.AuditLog {
		if _, exists := eventOrder[entry.EventType]; !exists {
			eventOrder[entry.EventType] = i
		}
	}

	// Verify all expected events exist
	for _, event := range expectedEvents {
		if _, exists := eventOrder[event]; !exists {
			t.Errorf("Expected event %s not found in audit log", event)
		}
	}

	// Verify order
	if eventOrder["invite.token.issued"] >= eventOrder["invite.token.accepted"] {
		t.Error("invite.token.issued should come before invite.token.accepted")
	}
	if eventOrder["invite.token.accepted"] >= eventOrder["intersection.created"] {
		t.Error("invite.token.accepted should come before intersection.created")
	}

	// Verify scope usage events exist
	foundScopeUsed := false
	for _, entry := range result.AuditLog {
		if entry.EventType == "intersection.scope.used" {
			foundScopeUsed = true
			break
		}
	}
	if !foundScopeUsed {
		t.Error("Expected intersection.scope.used events in audit log")
	}
}

// TestUnauthorizedAcceptorRejected ensures non-target circles can't accept targeted tokens.
func TestUnauthorizedAcceptorRejected(t *testing.T) {
	ctx := context.Background()

	// Create components
	auditStore := auditImpl.NewStore()
	circleRuntime := circleImpl.NewRuntime()
	intRuntime := intImpl.NewRuntime()
	keyManager := cryptoImpl.NewKeyManager()

	inviteService := circleImpl.NewInviteService(circleImpl.InviteServiceConfig{
		CircleRuntime: circleRuntime,
		IntRuntime:    intRuntime,
		KeyManager:    keyManager,
		AuditLogger:   auditStore,
	})

	// Create three circles
	circleA, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	circleB, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	circleC, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	keyManager.CreateKey(ctx, "key-"+circleA.ID, 24*time.Hour)

	// Issue token specifically for Circle B
	template := primitives.IntersectionTemplate{
		Scopes: []primitives.IntersectionScope{
			{Name: "test:read", Permission: "read"},
		},
	}

	token, err := inviteService.IssueInviteToken(ctx, circle.IssueInviteRequest{
		IssuerCircleID: circleA.ID,
		TargetCircleID: circleB.ID, // Specifically for B
		ProposedName:   "Test Intersection",
		Template:       template,
		ValidFor:       1 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Failed to issue token: %v", err)
	}

	// Circle C tries to accept - should fail
	_, err = inviteService.AcceptInviteToken(ctx, token, circleC.ID)
	if err == nil {
		t.Error("Expected rejection when unauthorized circle tries to accept")
	}
	if err != primitives.ErrUnauthorizedAcceptor {
		t.Errorf("Expected ErrUnauthorizedAcceptor, got: %v", err)
	}
}

// TestOpenInviteCanBeAcceptedByAnyone ensures tokens without target can be accepted by anyone.
func TestOpenInviteCanBeAcceptedByAnyone(t *testing.T) {
	ctx := context.Background()

	// Create components
	auditStore := auditImpl.NewStore()
	circleRuntime := circleImpl.NewRuntime()
	intRuntime := intImpl.NewRuntime()
	keyManager := cryptoImpl.NewKeyManager()

	inviteService := circleImpl.NewInviteService(circleImpl.InviteServiceConfig{
		CircleRuntime: circleRuntime,
		IntRuntime:    intRuntime,
		KeyManager:    keyManager,
		AuditLogger:   auditStore,
	})

	// Create circles
	circleA, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	circleC, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	keyManager.CreateKey(ctx, "key-"+circleA.ID, 24*time.Hour)
	keyManager.CreateKey(ctx, "key-"+circleC.ID, 24*time.Hour)

	// Issue open invite (no target)
	template := primitives.IntersectionTemplate{
		Scopes: []primitives.IntersectionScope{
			{Name: "test:read", Permission: "read"},
		},
	}

	token, err := inviteService.IssueInviteToken(ctx, circle.IssueInviteRequest{
		IssuerCircleID: circleA.ID,
		TargetCircleID: "", // Open invite
		ProposedName:   "Open Intersection",
		Template:       template,
		ValidFor:       1 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Failed to issue token: %v", err)
	}

	// Any circle can accept
	intRef, err := inviteService.AcceptInviteToken(ctx, token, circleC.ID)
	if err != nil {
		t.Fatalf("Open invite should be accepted by any circle: %v", err)
	}
	if intRef.IntersectionID == "" {
		t.Error("Expected intersection to be created")
	}
}

// TestSuggestionsAreDeterministic ensures suggestions are produced deterministically.
func TestSuggestionsAreDeterministic(t *testing.T) {
	const runs = 3
	var allSuggestions [][]string

	for i := 0; i < runs; i++ {
		runner := NewRunner()
		result, err := runner.Run(context.Background())

		if err != nil {
			t.Fatalf("Demo run %d failed: %v", i, err)
		}

		var descriptions []string
		for _, sug := range result.Suggestions {
			descriptions = append(descriptions, sug.Description)
		}
		allSuggestions = append(allSuggestions, descriptions)
	}

	baseline := allSuggestions[0]
	for i := 1; i < len(allSuggestions); i++ {
		if len(allSuggestions[i]) != len(baseline) {
			t.Errorf("Run %d produced %d suggestions, baseline had %d",
				i, len(allSuggestions[i]), len(baseline))
			continue
		}

		for j := range baseline {
			if allSuggestions[i][j] != baseline[j] {
				t.Errorf("Run %d suggestion %d differs: got %q, want %q",
					i, j, allSuggestions[i][j], baseline[j])
			}
		}
	}
}

// TestMinimumThreeSuggestions ensures at least 3 suggestions are produced.
func TestMinimumThreeSuggestions(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	const minSuggestions = 3
	if len(result.Suggestions) < minSuggestions {
		t.Errorf("Expected at least %d suggestions, got %d", minSuggestions, len(result.Suggestions))
	}
}

// TestSuggestionsHaveExplanations ensures each suggestion has a "why" explanation.
func TestSuggestionsHaveExplanations(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	for i, sug := range result.Suggestions {
		if sug.Explanation == "" {
			t.Errorf("Suggestion %d has no explanation", i)
		}
		// Explanation should reference intersection
		if !strings.Contains(sug.Explanation, "INTERSECTION") {
			t.Errorf("Suggestion %d explanation does not reference intersection", i)
		}
		// Explanation should reference scope
		if !strings.Contains(sug.Explanation, "scope") && !strings.Contains(sug.Explanation, "Scope") {
			t.Errorf("Suggestion %d explanation does not reference scope usage", i)
		}
	}
}

// TestTokenValidationRequiresAllFields ensures token validation catches missing fields.
func TestTokenValidationRequiresAllFields(t *testing.T) {
	tests := []struct {
		name  string
		token primitives.InviteToken
		err   error
	}{
		{
			name:  "missing token ID",
			token: primitives.InviteToken{IssuerCircleID: "c1", IssuedAt: time.Now(), ExpiresAt: time.Now().Add(1 * time.Hour), Signature: []byte("sig"), SignatureKeyID: "k1", SignatureAlgorithm: "alg"},
			err:   primitives.ErrMissingTokenID,
		},
		{
			name:  "missing issuer",
			token: primitives.InviteToken{TokenID: "t1", IssuedAt: time.Now(), ExpiresAt: time.Now().Add(1 * time.Hour), Signature: []byte("sig"), SignatureKeyID: "k1", SignatureAlgorithm: "alg"},
			err:   primitives.ErrMissingIssuer,
		},
		{
			name:  "missing expiry",
			token: primitives.InviteToken{TokenID: "t1", IssuerCircleID: "c1", IssuedAt: time.Now(), Signature: []byte("sig"), SignatureKeyID: "k1", SignatureAlgorithm: "alg"},
			err:   primitives.ErrMissingExpiry,
		},
		{
			name:  "missing signature",
			token: primitives.InviteToken{TokenID: "t1", IssuerCircleID: "c1", IssuedAt: time.Now(), ExpiresAt: time.Now().Add(1 * time.Hour), SignatureKeyID: "k1", SignatureAlgorithm: "alg"},
			err:   primitives.ErrMissingSignature,
		},
		{
			name:  "missing key ID",
			token: primitives.InviteToken{TokenID: "t1", IssuerCircleID: "c1", IssuedAt: time.Now(), ExpiresAt: time.Now().Add(1 * time.Hour), Signature: []byte("sig"), SignatureAlgorithm: "alg"},
			err:   primitives.ErrMissingKeyID,
		},
		{
			name:  "missing algorithm",
			token: primitives.InviteToken{TokenID: "t1", IssuerCircleID: "c1", IssuedAt: time.Now(), ExpiresAt: time.Now().Add(1 * time.Hour), Signature: []byte("sig"), SignatureKeyID: "k1"},
			err:   primitives.ErrMissingAlgorithm,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.token.Validate()
			if err != tt.err {
				t.Errorf("Expected error %v, got %v", tt.err, err)
			}
		})
	}
}

// TestAuditEntriesHaveCircleOrIntersectionID ensures audit entries are properly scoped.
func TestAuditEntriesHaveCircleOrIntersectionID(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	for _, entry := range result.AuditLog {
		// Each entry should have either CircleID or IntersectionID (or both)
		if entry.CircleID == "" && entry.IntersectionID == "" {
			// Only intersection.scope.used can have empty CircleID with IntersectionID
			if entry.EventType != "intersection.scope.used" {
				t.Errorf("Audit entry %s (%s) has neither CircleID nor IntersectionID",
					entry.ID, entry.EventType)
			}
		}
	}
}

// TestCryptoSignerVerifierRoundtrip tests the crypto signer/verifier.
func TestCryptoSignerVerifierRoundtrip(t *testing.T) {
	ctx := context.Background()
	keyManager := cryptoImpl.NewKeyManager()

	// Create a key
	_, err := keyManager.CreateKey(ctx, "test-key", 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create key: %v", err)
	}

	// Get signer
	signer, err := keyManager.GetSigner(ctx, "test-key")
	if err != nil {
		t.Fatalf("Failed to get signer: %v", err)
	}

	// Sign data
	data := []byte("test payload")
	sig, err := signer.Sign(ctx, data)
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	// Verify algorithm is marked as placeholder
	if signer.Algorithm() != cryptoImpl.PlaceholderAlgorithm {
		t.Errorf("Expected placeholder algorithm, got %s", signer.Algorithm())
	}

	// Get verifier
	verifier, err := keyManager.GetVerifier(ctx, "test-key")
	if err != nil {
		t.Fatalf("Failed to get verifier: %v", err)
	}

	// Verify signature
	err = verifier.Verify(ctx, data, sig)
	if err != nil {
		t.Errorf("Valid signature should verify: %v", err)
	}

	// Tampered data should fail
	tamperedData := []byte("tampered payload")
	err = verifier.Verify(ctx, tamperedData, sig)
	if err == nil {
		t.Error("Tampered data should fail verification")
	}
}

// TestAuditStoreImplementsLogger verifies audit store implements the Logger interface.
func TestAuditStoreImplementsLogger(t *testing.T) {
	store := auditImpl.NewStore()

	// Verify it implements audit.Logger
	var _ audit.Logger = store

	ctx := context.Background()
	store.Log(ctx, audit.Entry{
		CircleID:  "test-circle",
		EventType: "test.event",
		Action:    "test_action",
		Outcome:   "success",
	})

	entries := store.GetAllEntries()
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
}
