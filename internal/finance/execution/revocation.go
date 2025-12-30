package execution

import (
	"fmt"
	"sync"
	"time"
)

// RevocationChecker checks and processes revocation signals.
// Per Technical Split v9 §6 and §10.3:
// - MUST check for revocation before execution
// - MUST check for revocation during execution
// - MUST halt execution upon revocation detection
// - MUST NOT ignore revocation signals
// - MUST NOT delay revocation processing
// - MUST NOT allow "finish what you started"
type RevocationChecker struct {
	mu          sync.RWMutex
	revocations map[string]*RevocationSignal // envelopeID -> signal
	idGenerator func() string
}

// NewRevocationChecker creates a new revocation checker.
func NewRevocationChecker(idGen func() string) *RevocationChecker {
	return &RevocationChecker{
		revocations: make(map[string]*RevocationSignal),
		idGenerator: idGen,
	}
}

// Revoke records a revocation signal for an envelope.
// This is immediate and authoritative per Technical Split v9 §6.4.
func (c *RevocationChecker) Revoke(
	envelopeID string,
	revokerCircleID string,
	revokerID string,
	reason string,
	now time.Time,
) *RevocationSignal {
	c.mu.Lock()
	defer c.mu.Unlock()

	signal := &RevocationSignal{
		SignalID:        c.idGenerator(),
		EnvelopeID:      envelopeID,
		RevokerCircleID: revokerCircleID,
		RevokerID:       revokerID,
		RevokedAt:       now,
		Reason:          reason,
	}

	c.revocations[envelopeID] = signal
	return signal
}

// IsRevoked checks if an envelope has been revoked.
// This check MUST be performed before and during execution.
func (c *RevocationChecker) IsRevoked(envelopeID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.revocations[envelopeID]
	return exists
}

// GetRevocation returns the revocation signal for an envelope.
func (c *RevocationChecker) GetRevocation(envelopeID string) *RevocationSignal {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.revocations[envelopeID]
}

// CheckResult represents the result of a revocation check.
type RevocationCheckResult struct {
	// Revoked is true if revocation signal exists.
	Revoked bool

	// Signal is the revocation signal (if revoked).
	Signal *RevocationSignal

	// CheckedAt is when the check was performed.
	CheckedAt time.Time
}

// Check performs a revocation check and returns the result.
func (c *RevocationChecker) Check(envelopeID string, now time.Time) RevocationCheckResult {
	signal := c.GetRevocation(envelopeID)
	return RevocationCheckResult{
		Revoked:   signal != nil,
		Signal:    signal,
		CheckedAt: now,
	}
}

// ApplyRevocationToEnvelope applies a revocation to an envelope.
// This marks the envelope as revoked and records who/when.
func ApplyRevocationToEnvelope(env *ExecutionEnvelope, signal *RevocationSignal) error {
	if env.Revoked {
		return fmt.Errorf("envelope already revoked")
	}

	env.Revoked = true
	env.RevokedAt = signal.RevokedAt
	env.RevokedBy = signal.RevokerCircleID

	return nil
}

// RevocationWindowManager manages revocation window lifecycle.
type RevocationWindowManager struct {
	idGenerator func() string
}

// NewRevocationWindowManager creates a new revocation window manager.
func NewRevocationWindowManager(idGen func() string) *RevocationWindowManager {
	return &RevocationWindowManager{
		idGenerator: idGen,
	}
}

// RevocationWindowState represents the state of a revocation window.
type RevocationWindowState string

const (
	// WindowNotStarted means the window has not yet opened.
	WindowNotStarted RevocationWindowState = "not_started"

	// WindowOpen means the revocation window is currently open.
	WindowOpen RevocationWindowState = "open"

	// WindowClosed means the revocation window has closed without revocation.
	WindowClosed RevocationWindowState = "closed"

	// WindowWaived means the revocation window was explicitly waived.
	WindowWaived RevocationWindowState = "waived"
)

// GetWindowState returns the current state of the revocation window.
func (m *RevocationWindowManager) GetWindowState(env *ExecutionEnvelope, now time.Time) RevocationWindowState {
	if env.RevocationWaived {
		return WindowWaived
	}

	if now.Before(env.RevocationWindowStart) {
		return WindowNotStarted
	}

	if now.Before(env.RevocationWindowEnd) {
		return WindowOpen
	}

	return WindowClosed
}

// CanRevoke returns true if revocation is currently possible.
func (m *RevocationWindowManager) CanRevoke(env *ExecutionEnvelope, now time.Time) bool {
	state := m.GetWindowState(env, now)
	// Can revoke during open window or if window hasn't started yet
	return state == WindowOpen || state == WindowNotStarted
}

// TimeUntilWindowCloses returns the duration until the window closes.
// Returns 0 if window is closed or waived.
func (m *RevocationWindowManager) TimeUntilWindowCloses(env *ExecutionEnvelope, now time.Time) time.Duration {
	if env.RevocationWaived {
		return 0
	}

	if now.After(env.RevocationWindowEnd) {
		return 0
	}

	return env.RevocationWindowEnd.Sub(now)
}
