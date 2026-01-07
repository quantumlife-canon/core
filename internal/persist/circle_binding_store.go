// Package persist provides persistence for circle-device bindings.
//
// Phase 30A: Circle Binding Store
//
// CRITICAL INVARIANTS:
// - stdlib only
// - No time.Now() - clock injection only
// - No goroutines
// - Hash-only storage (fingerprints, not raw public keys)
// - Bounded: max 5 devices per circle
// - Storelog integration for replay
//
// Reference: docs/ADR/ADR-0061-phase30A-identity-and-replay.md
package persist

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"quantumlife/pkg/domain/deviceidentity"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/storelog"
)

// CircleBindingStore manages device-to-circle bindings.
// Stores only fingerprints (hashes) - never raw public keys.
type CircleBindingStore struct {
	mu sync.RWMutex

	// bindings maps circleIDHash -> list of bindings (max 5)
	bindings map[string][]*deviceidentity.CircleBinding

	// byFingerprint maps fingerprint -> list of circleIDHashes for lookup
	byFingerprint map[string][]string

	// clock provides time injection
	clock func() time.Time

	// storelog for replay integration
	storelogRef storelog.AppendOnlyLog
}

// NewCircleBindingStore creates a new circle binding store.
func NewCircleBindingStore(clock func() time.Time, storelogRef storelog.AppendOnlyLog) *CircleBindingStore {
	return &CircleBindingStore{
		bindings:      make(map[string][]*deviceidentity.CircleBinding),
		byFingerprint: make(map[string][]string),
		clock:         clock,
		storelogRef:   storelogRef,
	}
}

// Bind binds a device fingerprint to a circle.
// Returns error if max devices reached (5 per circle).
// Idempotent: binding same fingerprint again is a no-op.
func (s *CircleBindingStore) Bind(circleID string, fingerprint deviceidentity.Fingerprint) (*deviceidentity.BindResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := fingerprint.Validate(); err != nil {
		return &deviceidentity.BindResult{
			Success: false,
			Error:   fmt.Sprintf("invalid fingerprint: %v", err),
		}, nil
	}

	circleIDHash := deviceidentity.HashString(circleID)

	// Check if already bound (idempotent)
	for _, binding := range s.bindings[circleIDHash] {
		if binding.Fingerprint == fingerprint {
			return &deviceidentity.BindResult{
				Success:    true,
				Binding:    binding,
				BoundCount: len(s.bindings[circleIDHash]),
				AtMaxLimit: len(s.bindings[circleIDHash]) >= deviceidentity.MaxDevicesPerCircle,
			}, nil
		}
	}

	// Check max limit
	if len(s.bindings[circleIDHash]) >= deviceidentity.MaxDevicesPerCircle {
		return &deviceidentity.BindResult{
			Success:    false,
			Error:      fmt.Sprintf("max devices reached (%d)", deviceidentity.MaxDevicesPerCircle),
			BoundCount: len(s.bindings[circleIDHash]),
			AtMaxLimit: true,
		}, nil
	}

	// Create binding
	now := s.clock()
	binding := deviceidentity.NewCircleBinding(circleID, fingerprint, now)

	// Store binding
	s.bindings[circleIDHash] = append(s.bindings[circleIDHash], binding)

	// Update reverse index
	fingerprintStr := string(fingerprint)
	s.byFingerprint[fingerprintStr] = append(s.byFingerprint[fingerprintStr], circleIDHash)

	// Write to storelog if available
	if s.storelogRef != nil {
		record := storelog.NewRecord(
			storelog.RecordTypeCircleBinding,
			now,
			identity.EntityID(circleID),
			binding.CanonicalString(),
		)
		// Best effort - don't fail bind if storelog fails
		_ = s.storelogRef.Append(record)
	}

	return &deviceidentity.BindResult{
		Success:    true,
		Binding:    binding,
		BoundCount: len(s.bindings[circleIDHash]),
		AtMaxLimit: len(s.bindings[circleIDHash]) >= deviceidentity.MaxDevicesPerCircle,
	}, nil
}

// IsBound checks if a fingerprint is bound to a circle.
func (s *CircleBindingStore) IsBound(circleID string, fingerprint deviceidentity.Fingerprint) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	circleIDHash := deviceidentity.HashString(circleID)
	for _, binding := range s.bindings[circleIDHash] {
		if binding.Fingerprint == fingerprint {
			return true
		}
	}
	return false
}

// GetBindings returns all bindings for a circle.
func (s *CircleBindingStore) GetBindings(circleID string) []*deviceidentity.CircleBinding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	circleIDHash := deviceidentity.HashString(circleID)
	bindings := s.bindings[circleIDHash]
	if bindings == nil {
		return nil
	}

	// Return a copy
	result := make([]*deviceidentity.CircleBinding, len(bindings))
	copy(result, bindings)
	return result
}

// GetBoundCount returns the number of devices bound to a circle.
func (s *CircleBindingStore) GetBoundCount(circleID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	circleIDHash := deviceidentity.HashString(circleID)
	return len(s.bindings[circleIDHash])
}

// GetCirclesForFingerprint returns all circles a fingerprint is bound to.
func (s *CircleBindingStore) GetCirclesForFingerprint(fingerprint deviceidentity.Fingerprint) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fingerprintStr := string(fingerprint)
	circleHashes := s.byFingerprint[fingerprintStr]
	if circleHashes == nil {
		return nil
	}

	// Return a copy
	result := make([]string, len(circleHashes))
	copy(result, circleHashes)
	return result
}

// ReplayFromStorelog replays binding records from storelog.
// Used for state reconstruction.
func (s *CircleBindingStore) ReplayFromStorelog() error {
	if s.storelogRef == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.storelogRef.ListByType(storelog.RecordTypeCircleBinding)
	if err != nil {
		return fmt.Errorf("failed to list binding records: %w", err)
	}

	for _, record := range records {
		binding, err := parseCircleBinding(record.Payload)
		if err != nil {
			// Skip invalid records during replay
			continue
		}

		// Check if already exists
		exists := false
		for _, existing := range s.bindings[binding.CircleIDHash] {
			if existing.Fingerprint == binding.Fingerprint {
				exists = true
				break
			}
		}

		if !exists && len(s.bindings[binding.CircleIDHash]) < deviceidentity.MaxDevicesPerCircle {
			s.bindings[binding.CircleIDHash] = append(s.bindings[binding.CircleIDHash], binding)
			fingerprintStr := string(binding.Fingerprint)
			s.byFingerprint[fingerprintStr] = append(s.byFingerprint[fingerprintStr], binding.CircleIDHash)
		}
	}

	return nil
}

// parseCircleBinding parses a binding from its canonical string.
// Format: v1|circle_binding|CIRCLE_ID_HASH|FINGERPRINT|PERIOD_KEY
func parseCircleBinding(payload string) (*deviceidentity.CircleBinding, error) {
	// Simple parsing - split by |
	parts := strings.SplitN(payload, "|", 5)
	if len(parts) < 5 {
		return nil, fmt.Errorf("invalid binding format: expected 5 parts, got %d", len(parts))
	}

	if parts[0] != "v1" || parts[1] != "circle_binding" {
		return nil, fmt.Errorf("invalid binding header: %s|%s", parts[0], parts[1])
	}

	return &deviceidentity.CircleBinding{
		CircleIDHash:  parts[2],
		Fingerprint:   deviceidentity.Fingerprint(parts[3]),
		BoundAtPeriod: deviceidentity.PeriodKey(parts[4]),
		BindingHash:   "", // Computed on demand
	}, nil
}
