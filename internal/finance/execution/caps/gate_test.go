package caps

import (
	"context"
	"testing"
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/events"
)

// testClock returns a fixed clock for deterministic tests.
func testClock() clock.Clock {
	return clock.NewFixed(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
}

// testClockNextDay returns a clock for the next day.
func testClockNextDay() clock.Clock {
	return clock.NewFixed(time.Date(2025, 1, 16, 10, 0, 0, 0, time.UTC))
}

// noopEmitter discards events.
func noopEmitter(e events.Event) {}

func TestDayKey_Deterministic(t *testing.T) {
	t.Run("same clock produces same day key", func(t *testing.T) {
		c := testClock()
		key1 := DayKey(c)
		key2 := DayKey(c)

		if key1 != key2 {
			t.Errorf("expected same key, got %s and %s", key1, key2)
		}

		if key1 != "2025-01-15" {
			t.Errorf("expected 2025-01-15, got %s", key1)
		}
	})

	t.Run("different days produce different keys", func(t *testing.T) {
		key1 := DayKey(testClock())
		key2 := DayKey(testClockNextDay())

		if key1 == key2 {
			t.Errorf("expected different keys, got %s for both", key1)
		}
	})
}

func TestGate_Check_PolicyDisabled(t *testing.T) {
	policy := Policy{Enabled: false}
	gate := NewDefaultGate(policy, noopEmitter)

	result, err := gate.Check(context.Background(), Context{
		Clock:       testClock(),
		CircleID:    "circle-1",
		PayeeID:     "payee-1",
		Currency:    "GBP",
		AmountCents: 1000000, // Way over any reasonable cap
		AttemptID:   "attempt-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Allowed {
		t.Error("expected Allowed=true when policy disabled")
	}
}

func TestGate_Check_CircleDailyCap(t *testing.T) {
	policy := Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 100, // 100 cents = £1.00
		},
	}

	store := NewStore()
	gate := NewDefaultGateWithStore(policy, store, noopEmitter)
	ctx := context.Background()

	t.Run("first payment within cap passes", func(t *testing.T) {
		result, err := gate.Check(ctx, Context{
			Clock:       testClock(),
			CircleID:    "circle-1",
			PayeeID:     "payee-1",
			Currency:    "GBP",
			AmountCents: 50,
			AttemptID:   "attempt-1",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.Allowed {
			t.Errorf("expected Allowed=true, got reasons: %v", result.Reasons)
		}

		if result.RemainingCents != 50 {
			t.Errorf("expected RemainingCents=50, got %d", result.RemainingCents)
		}
	})

	// Simulate first payment completing with money moved
	store.IncrementSpend(DayKey(testClock()), ScopeCircle, "circle-1", "GBP", 50)

	t.Run("payment exceeding cap is blocked", func(t *testing.T) {
		result, err := gate.Check(ctx, Context{
			Clock:       testClock(),
			CircleID:    "circle-1",
			PayeeID:     "payee-1",
			Currency:    "GBP",
			AmountCents: 60, // 50 + 60 = 110 > 100 cap
			AttemptID:   "attempt-2",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Allowed {
			t.Error("expected Allowed=false when exceeding cap")
		}

		if len(result.Reasons) == 0 {
			t.Error("expected blocking reason")
		}

		if result.RemainingCents != 50 {
			t.Errorf("expected RemainingCents=50, got %d", result.RemainingCents)
		}
	})
}

func TestGate_Check_AttemptLimit(t *testing.T) {
	policy := Policy{
		Enabled:                 true,
		MaxAttemptsPerDayCircle: 3,
	}

	store := NewStore()
	gate := NewDefaultGateWithStore(policy, store, noopEmitter)
	ctx := context.Background()
	dayKey := DayKey(testClock())

	// Record 3 attempts
	store.IncrementAttempt(dayKey, ScopeCircle, "circle-1", "GBP", "attempt-1")
	store.IncrementAttempt(dayKey, ScopeCircle, "circle-1", "GBP", "attempt-2")
	store.IncrementAttempt(dayKey, ScopeCircle, "circle-1", "GBP", "attempt-3")

	result, err := gate.Check(ctx, Context{
		Clock:       testClock(),
		CircleID:    "circle-1",
		PayeeID:     "payee-1",
		Currency:    "GBP",
		AmountCents: 10,
		AttemptID:   "attempt-4",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Allowed {
		t.Error("expected Allowed=false when attempt limit reached")
	}

	if result.RemainingAttempts != 0 {
		t.Errorf("expected RemainingAttempts=0, got %d", result.RemainingAttempts)
	}
}

func TestGate_Check_PayeeCap(t *testing.T) {
	policy := Policy{
		Enabled: true,
		PerPayeeDailyCapCents: map[string]int64{
			"GBP": 50,
		},
	}

	store := NewStore()
	gate := NewDefaultGateWithStore(policy, store, noopEmitter)
	ctx := context.Background()
	dayKey := DayKey(testClock())

	// First payment to payee-1
	store.IncrementSpend(dayKey, ScopePayee, "payee-1", "GBP", 50)

	// Second payment to same payee should be blocked
	result, err := gate.Check(ctx, Context{
		Clock:       testClock(),
		CircleID:    "circle-1",
		PayeeID:     "payee-1",
		Currency:    "GBP",
		AmountCents: 10,
		AttemptID:   "attempt-2",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Allowed {
		t.Error("expected Allowed=false when payee cap reached")
	}

	// Different payee should be allowed
	result2, err := gate.Check(ctx, Context{
		Clock:       testClock(),
		CircleID:    "circle-1",
		PayeeID:     "payee-2",
		Currency:    "GBP",
		AmountCents: 50,
		AttemptID:   "attempt-3",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result2.Allowed {
		t.Errorf("expected Allowed=true for different payee, got reasons: %v", result2.Reasons)
	}
}

func TestGate_Check_IntersectionCap(t *testing.T) {
	policy := Policy{
		Enabled: true,
		PerIntersectionDailyCapCents: map[string]int64{
			"GBP": 75,
		},
	}

	store := NewStore()
	gate := NewDefaultGateWithStore(policy, store, noopEmitter)
	ctx := context.Background()
	dayKey := DayKey(testClock())

	store.IncrementSpend(dayKey, ScopeIntersection, "intersection-1", "GBP", 50)

	result, err := gate.Check(ctx, Context{
		Clock:          testClock(),
		CircleID:       "circle-1",
		IntersectionID: "intersection-1",
		PayeeID:        "payee-1",
		Currency:       "GBP",
		AmountCents:    30, // 50 + 30 = 80 > 75 cap
		AttemptID:      "attempt-2",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Allowed {
		t.Error("expected Allowed=false when intersection cap reached")
	}
}

func TestGate_Check_CurrencySeparation(t *testing.T) {
	policy := Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 100,
			"EUR": 100,
		},
	}

	store := NewStore()
	gate := NewDefaultGateWithStore(policy, store, noopEmitter)
	ctx := context.Background()
	dayKey := DayKey(testClock())

	// Max out GBP
	store.IncrementSpend(dayKey, ScopeCircle, "circle-1", "GBP", 100)

	// EUR should still be allowed
	result, err := gate.Check(ctx, Context{
		Clock:       testClock(),
		CircleID:    "circle-1",
		PayeeID:     "payee-1",
		Currency:    "EUR",
		AmountCents: 50,
		AttemptID:   "attempt-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Allowed {
		t.Errorf("expected Allowed=true for different currency, got reasons: %v", result.Reasons)
	}
}

func TestGate_OnAttemptStarted_Idempotent(t *testing.T) {
	policy := Policy{Enabled: true}
	store := NewStore()
	gate := NewDefaultGateWithStore(policy, store, noopEmitter)
	ctx := context.Background()

	c := Context{
		Clock:       testClock(),
		CircleID:    "circle-1",
		PayeeID:     "payee-1",
		Currency:    "GBP",
		AmountCents: 50,
		AttemptID:   "attempt-1",
	}

	// Call OnAttemptStarted multiple times
	if err := gate.OnAttemptStarted(ctx, c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := gate.OnAttemptStarted(ctx, c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := gate.OnAttemptStarted(ctx, c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only count once
	dayKey := DayKey(testClock())
	count := store.GetAttemptCount(dayKey, ScopeCircle, "circle-1")
	if count != 1 {
		t.Errorf("expected 1 attempt after idempotent calls, got %d", count)
	}
}

func TestGate_OnAttemptFinalized_MoneyMoved(t *testing.T) {
	policy := Policy{Enabled: true}
	store := NewStore()
	gate := NewDefaultGateWithStore(policy, store, noopEmitter)
	ctx := context.Background()

	c := Context{
		Clock:       testClock(),
		CircleID:    "circle-1",
		PayeeID:     "payee-1",
		Currency:    "GBP",
		AmountCents: 50,
		AttemptID:   "attempt-1",
	}

	// Finalize with money moved
	if err := gate.OnAttemptFinalized(ctx, c, Finalized{
		Status:           StatusSucceeded,
		MoneyMoved:       true,
		AmountMovedCents: 50,
		Currency:         "GBP",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dayKey := DayKey(testClock())
	counters := store.GetCounters(dayKey, ScopeCircle, "circle-1", "GBP")

	if counters.MoneyMovedCents != 50 {
		t.Errorf("expected MoneyMovedCents=50, got %d", counters.MoneyMovedCents)
	}
}

func TestGate_OnAttemptFinalized_Simulated(t *testing.T) {
	policy := Policy{Enabled: true}
	store := NewStore()
	gate := NewDefaultGateWithStore(policy, store, noopEmitter)
	ctx := context.Background()

	c := Context{
		Clock:       testClock(),
		CircleID:    "circle-1",
		PayeeID:     "payee-1",
		Currency:    "GBP",
		AmountCents: 50,
		AttemptID:   "attempt-1",
	}

	// Finalize simulated (no money moved)
	if err := gate.OnAttemptFinalized(ctx, c, Finalized{
		Status:           StatusSimulated,
		MoneyMoved:       false,
		AmountMovedCents: 0,
		Currency:         "GBP",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dayKey := DayKey(testClock())
	counters := store.GetCounters(dayKey, ScopeCircle, "circle-1", "GBP")

	if counters.MoneyMovedCents != 0 {
		t.Errorf("expected MoneyMovedCents=0 for simulated, got %d", counters.MoneyMovedCents)
	}
}

func TestGate_NewDay_ResetsCaps(t *testing.T) {
	policy := Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 100,
		},
		MaxAttemptsPerDayCircle: 3,
	}

	store := NewStore()
	gate := NewDefaultGateWithStore(policy, store, noopEmitter)
	ctx := context.Background()

	// Day 1: max out caps
	day1Key := DayKey(testClock())
	store.IncrementSpend(day1Key, ScopeCircle, "circle-1", "GBP", 100)
	store.IncrementAttempt(day1Key, ScopeCircle, "circle-1", "GBP", "attempt-1")
	store.IncrementAttempt(day1Key, ScopeCircle, "circle-1", "GBP", "attempt-2")
	store.IncrementAttempt(day1Key, ScopeCircle, "circle-1", "GBP", "attempt-3")

	// Day 2: should have fresh limits
	result, err := gate.Check(ctx, Context{
		Clock:       testClockNextDay(),
		CircleID:    "circle-1",
		PayeeID:     "payee-1",
		Currency:    "GBP",
		AmountCents: 50,
		AttemptID:   "attempt-4",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Allowed {
		t.Errorf("expected Allowed=true on new day, got reasons: %v", result.Reasons)
	}
}

func TestGate_EventsEmitted(t *testing.T) {
	var capturedEvents []events.Event
	emitter := func(e events.Event) {
		capturedEvents = append(capturedEvents, e)
	}

	policy := Policy{Enabled: true}
	gate := NewDefaultGate(policy, emitter)

	// The gate itself doesn't emit events directly in this implementation
	// Events are emitted by the executor which calls the gate
	// This test verifies the emitter is properly stored
	if gate.emitter == nil {
		t.Error("expected emitter to be set")
	}
}

func TestStore_PurgeDaysBefore(t *testing.T) {
	store := NewStore()

	// Add data for multiple days
	store.IncrementSpend("2025-01-14", ScopeCircle, "circle-1", "GBP", 100)
	store.IncrementSpend("2025-01-15", ScopeCircle, "circle-1", "GBP", 50)
	store.IncrementSpend("2025-01-16", ScopeCircle, "circle-1", "GBP", 25)
	store.IncrementAttempt("2025-01-14", ScopeCircle, "circle-1", "GBP", "attempt-old")
	store.IncrementAttempt("2025-01-15", ScopeCircle, "circle-1", "GBP", "attempt-today")

	// Purge before 2025-01-15
	store.PurgeDaysBefore("2025-01-15")

	// Old data should be gone
	old := store.GetCounters("2025-01-14", ScopeCircle, "circle-1", "GBP")
	if old.MoneyMovedCents != 0 {
		t.Errorf("expected old data purged, got %d", old.MoneyMovedCents)
	}

	// Current data should remain
	current := store.GetCounters("2025-01-15", ScopeCircle, "circle-1", "GBP")
	if current.MoneyMovedCents != 50 {
		t.Errorf("expected current data retained, got %d", current.MoneyMovedCents)
	}

	future := store.GetCounters("2025-01-16", ScopeCircle, "circle-1", "GBP")
	if future.MoneyMovedCents != 25 {
		t.Errorf("expected future data retained, got %d", future.MoneyMovedCents)
	}
}
