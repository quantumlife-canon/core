package demo_phase18_6_first_connect

import (
	"testing"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/pkg/domain/connection"
)

// ═══════════════════════════════════════════════════════════════════════════
// Phase 18.6: First Connect - Demo Tests
// Reference: docs/ADR/ADR-0038-phase18-6-first-connect.md
//
// CRITICAL: All tests must be deterministic.
// CRITICAL: No time.Now() - use fixed timestamps.
// ═══════════════════════════════════════════════════════════════════════════

// fixedTime provides deterministic timestamps for testing.
var fixedTime = time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

// TestFreshStartShowsAllNotConnected verifies that a fresh start
// shows all connections as NotConnected.
func TestFreshStartShowsAllNotConnected(t *testing.T) {
	store := persist.NewInMemoryConnectionStore()

	state := store.State()

	for _, kind := range connection.AllKinds() {
		kindState := state.Get(kind)
		if kindState == nil {
			t.Errorf("Missing state for kind: %s", kind)
			continue
		}
		if kindState.Status != connection.StatusNotConnected {
			t.Errorf("Expected %s to be NotConnected, got %s", kind, kindState.Status)
		}
	}
}

// TestConnectEmailMockShowsConnectedMock verifies that connecting
// email in mock mode shows ConnectedMock status.
func TestConnectEmailMockShowsConnectedMock(t *testing.T) {
	store := persist.NewInMemoryConnectionStore()

	// Connect email in mock mode
	intent := connection.NewConnectIntent(
		connection.KindEmail,
		connection.ModeMock,
		fixedTime,
		connection.NoteUserInitiated,
	)
	if err := store.AppendIntent(intent); err != nil {
		t.Fatalf("AppendIntent failed: %v", err)
	}

	state := store.State()
	emailState := state.Get(connection.KindEmail)

	if emailState.Status != connection.StatusConnectedMock {
		t.Errorf("Expected Email ConnectedMock, got %s", emailState.Status)
	}
}

// TestDisconnectEmailShowsNotConnected verifies that disconnecting
// a connected source returns to NotConnected.
func TestDisconnectEmailShowsNotConnected(t *testing.T) {
	store := persist.NewInMemoryConnectionStore()

	// First connect
	connectIntent := connection.NewConnectIntent(
		connection.KindEmail,
		connection.ModeMock,
		fixedTime,
		connection.NoteUserInitiated,
	)
	if err := store.AppendIntent(connectIntent); err != nil {
		t.Fatalf("AppendIntent (connect) failed: %v", err)
	}

	// Then disconnect
	disconnectIntent := connection.NewDisconnectIntent(
		connection.KindEmail,
		connection.ModeMock,
		fixedTime.Add(time.Hour),
		connection.NoteUserInitiated,
	)
	if err := store.AppendIntent(disconnectIntent); err != nil {
		t.Fatalf("AppendIntent (disconnect) failed: %v", err)
	}

	state := store.State()
	emailState := state.Get(connection.KindEmail)

	if emailState.Status != connection.StatusNotConnected {
		t.Errorf("Expected Email NotConnected after disconnect, got %s", emailState.Status)
	}
}

// TestConnectCalendarRealShowsNeedsConfig verifies that connecting
// calendar in real mode (without config) shows NeedsConfig.
func TestConnectCalendarRealShowsNeedsConfig(t *testing.T) {
	store := persist.NewInMemoryConnectionStore()

	// Connect calendar in real mode
	intent := connection.NewConnectIntent(
		connection.KindCalendar,
		connection.ModeReal,
		fixedTime,
		connection.NoteUserInitiated,
	)
	if err := store.AppendIntent(intent); err != nil {
		t.Fatalf("AppendIntent failed: %v", err)
	}

	state := store.State()
	calState := state.Get(connection.KindCalendar)

	// Without config present, real connections show NeedsConfig
	if calState.Status != connection.StatusNeedsConfig {
		t.Errorf("Expected Calendar NeedsConfig, got %s", calState.Status)
	}
}

// TestReplayProducesIdenticalStateHash verifies that replaying
// the same intents produces the same state hash.
func TestReplayProducesIdenticalStateHash(t *testing.T) {
	// Create first store and add intents
	store1 := persist.NewInMemoryConnectionStore()

	intents := []*connection.ConnectionIntent{
		connection.NewConnectIntent(connection.KindEmail, connection.ModeMock, fixedTime, connection.NoteUserInitiated),
		connection.NewConnectIntent(connection.KindCalendar, connection.ModeMock, fixedTime.Add(time.Minute), connection.NoteUserInitiated),
		connection.NewDisconnectIntent(connection.KindEmail, connection.ModeMock, fixedTime.Add(2*time.Minute), connection.NoteUserInitiated),
		connection.NewConnectIntent(connection.KindFinance, connection.ModeReal, fixedTime.Add(3*time.Minute), connection.NoteUserInitiated),
	}

	for _, intent := range intents {
		if err := store1.AppendIntent(intent); err != nil {
			t.Fatalf("AppendIntent failed: %v", err)
		}
	}

	hash1 := store1.StateHash()

	// Create second store and add same intents
	store2 := persist.NewInMemoryConnectionStore()
	for _, intent := range intents {
		if err := store2.AppendIntent(intent); err != nil {
			t.Fatalf("AppendIntent (store2) failed: %v", err)
		}
	}

	hash2 := store2.StateHash()

	if hash1 != hash2 {
		t.Errorf("State hashes don't match:\n  store1: %s\n  store2: %s", hash1, hash2)
	}
}

// TestDeterministicOrdering verifies that state list is always
// in deterministic order: email, calendar, finance.
func TestDeterministicOrdering(t *testing.T) {
	store := persist.NewInMemoryConnectionStore()

	// Add intents in reverse order
	intents := []*connection.ConnectionIntent{
		connection.NewConnectIntent(connection.KindFinance, connection.ModeMock, fixedTime, connection.NoteUserInitiated),
		connection.NewConnectIntent(connection.KindCalendar, connection.ModeMock, fixedTime.Add(time.Minute), connection.NoteUserInitiated),
		connection.NewConnectIntent(connection.KindEmail, connection.ModeMock, fixedTime.Add(2*time.Minute), connection.NoteUserInitiated),
	}

	for _, intent := range intents {
		if err := store.AppendIntent(intent); err != nil {
			t.Fatalf("AppendIntent failed: %v", err)
		}
	}

	state := store.State()
	list := state.List()

	expectedOrder := []connection.ConnectionKind{
		connection.KindEmail,
		connection.KindCalendar,
		connection.KindFinance,
	}

	if len(list) != len(expectedOrder) {
		t.Fatalf("Expected %d items, got %d", len(expectedOrder), len(list))
	}

	for i, expected := range expectedOrder {
		if list[i].Kind != expected {
			t.Errorf("Position %d: expected %s, got %s", i, expected, list[i].Kind)
		}
	}
}

// TestTieBreakByHash verifies that when timestamps are equal,
// intent ordering is determined by hash (lexical).
func TestTieBreakByHash(t *testing.T) {
	// Create two intents with the same timestamp
	intent1 := connection.NewConnectIntent(connection.KindEmail, connection.ModeMock, fixedTime, connection.NoteUserInitiated)
	intent2 := connection.NewConnectIntent(connection.KindEmail, connection.ModeReal, fixedTime, connection.NoteUserInitiated)

	// Create intent list
	list := connection.IntentList{intent1, intent2}
	list.Sort()

	// Verify deterministic ordering by hash
	if list[0].ID > list[1].ID {
		t.Errorf("Intents not sorted by hash: %s > %s", list[0].ID[:8], list[1].ID[:8])
	}
}

// TestCanonicalStringFormat verifies that canonical strings
// are pipe-delimited (not JSON).
func TestCanonicalStringFormat(t *testing.T) {
	intent := connection.NewConnectIntent(
		connection.KindEmail,
		connection.ModeMock,
		fixedTime,
		connection.NoteUserInitiated,
	)

	canonical := intent.CanonicalString()

	// Must start with CONN_INTENT|v1
	if canonical[:14] != "CONN_INTENT|v1" {
		t.Errorf("Canonical string doesn't start with 'CONN_INTENT|v1': %s", canonical[:20])
	}

	// Must not contain JSON markers
	if containsJSONMarker(canonical) {
		t.Error("Canonical string contains JSON markers")
	}
}

func containsJSONMarker(s string) bool {
	for _, marker := range []string{"{", "}", "\":", "null"} {
		for i := 0; i < len(s)-len(marker)+1; i++ {
			if s[i:i+len(marker)] == marker {
				return true
			}
		}
	}
	return false
}

// TestIntentHashDeterminism verifies that the same intent data
// always produces the same hash.
func TestIntentHashDeterminism(t *testing.T) {
	// Create same intent twice
	intent1 := &connection.ConnectionIntent{
		Kind:   connection.KindEmail,
		Action: connection.ActionConnect,
		Mode:   connection.ModeMock,
		At:     fixedTime,
		Note:   connection.NoteUserInitiated,
	}
	intent1.ComputeID()

	intent2 := &connection.ConnectionIntent{
		Kind:   connection.KindEmail,
		Action: connection.ActionConnect,
		Mode:   connection.ModeMock,
		At:     fixedTime,
		Note:   connection.NoteUserInitiated,
	}
	intent2.ComputeID()

	if intent1.ID != intent2.ID {
		t.Errorf("Same data produced different hashes:\n  %s\n  %s", intent1.ID, intent2.ID)
	}
}

// TestParseCanonicalRoundtrip verifies that parsing a canonical
// string produces the original intent.
func TestParseCanonicalRoundtrip(t *testing.T) {
	original := connection.NewConnectIntent(
		connection.KindCalendar,
		connection.ModeReal,
		fixedTime,
		connection.NoteUserInitiated,
	)

	canonical := original.CanonicalString()
	parsed, err := connection.ParseCanonicalIntent(canonical)

	if err != nil {
		t.Fatalf("ParseCanonicalIntent failed: %v", err)
	}

	if parsed.Kind != original.Kind {
		t.Errorf("Kind mismatch: %s vs %s", parsed.Kind, original.Kind)
	}
	if parsed.Action != original.Action {
		t.Errorf("Action mismatch: %s vs %s", parsed.Action, original.Action)
	}
	if parsed.Mode != original.Mode {
		t.Errorf("Mode mismatch: %s vs %s", parsed.Mode, original.Mode)
	}
	if parsed.ID != original.ID {
		t.Errorf("ID mismatch: %s vs %s", parsed.ID, original.ID)
	}
}

// TestAllKindsValid verifies that all defined kinds pass validation.
func TestAllKindsValid(t *testing.T) {
	for _, kind := range connection.AllKinds() {
		if !kind.Valid() {
			t.Errorf("Kind %s should be valid", kind)
		}
	}
}

// TestInvalidKindRejected verifies that invalid kinds are rejected.
func TestInvalidKindRejected(t *testing.T) {
	invalid := connection.ConnectionKind("invalid")
	if invalid.Valid() {
		t.Error("Invalid kind should not be valid")
	}
}

// TestStateDisplayText verifies that all statuses have display text.
func TestStateDisplayText(t *testing.T) {
	statuses := []connection.ConnectionStatus{
		connection.StatusNotConnected,
		connection.StatusConnectedMock,
		connection.StatusConnectedReal,
		connection.StatusNeedsConfig,
	}

	for _, status := range statuses {
		text := status.DisplayText()
		if text == "" || text == "Unknown" {
			t.Errorf("Status %s has invalid display text: %q", status, text)
		}
	}
}

// TestIntentCountTracking verifies that intent count is tracked correctly.
func TestIntentCountTracking(t *testing.T) {
	store := persist.NewInMemoryConnectionStore()

	if store.IntentCount() != 0 {
		t.Errorf("Fresh store should have 0 intents, got %d", store.IntentCount())
	}

	// Add 3 intents
	for i := 0; i < 3; i++ {
		intent := connection.NewConnectIntent(
			connection.KindEmail,
			connection.ModeMock,
			fixedTime.Add(time.Duration(i)*time.Minute),
			connection.NoteUserInitiated,
		)
		store.AppendIntent(intent)
	}

	if store.IntentCount() != 3 {
		t.Errorf("Expected 3 intents, got %d", store.IntentCount())
	}
}
