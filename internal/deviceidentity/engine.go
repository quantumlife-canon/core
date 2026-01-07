// Package deviceidentity provides the identity engine for Phase 30A.
// This engine manages device identity, signed requests, and circle bindings.
//
// CRITICAL INVARIANTS:
// - stdlib only
// - No time.Now() - clock injection only
// - No goroutines
// - Verify signatures before any sensitive operation
// - Require bound device for replay export/import
//
// Reference: docs/ADR/ADR-0061-phase30A-identity-and-replay.md
package deviceidentity

import (
	"fmt"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/pkg/domain/deviceidentity"
)

// Engine manages device identity operations.
type Engine struct {
	clock        func() time.Time
	keyStore     *persist.DeviceKeyStore
	bindingStore *persist.CircleBindingStore
}

// NewEngine creates a new device identity engine.
func NewEngine(
	clock func() time.Time,
	keyStore *persist.DeviceKeyStore,
	bindingStore *persist.CircleBindingStore,
) *Engine {
	return &Engine{
		clock:        clock,
		keyStore:     keyStore,
		bindingStore: bindingStore,
	}
}

// EnsureDeviceIdentity ensures the device has an identity.
// Creates keypair if missing, returns public key and fingerprint.
func (e *Engine) EnsureDeviceIdentity() (deviceidentity.DevicePublicKey, deviceidentity.Fingerprint, error) {
	return e.keyStore.EnsureKeypair()
}

// GetFingerprint returns the device fingerprint.
func (e *Engine) GetFingerprint() (deviceidentity.Fingerprint, error) {
	return e.keyStore.GetFingerprint()
}

// GetPublicKey returns the device public key.
func (e *Engine) GetPublicKey() (deviceidentity.DevicePublicKey, error) {
	return e.keyStore.GetPublicKey()
}

// BindToCircle binds the current device to a circle.
// Returns error if max devices reached.
func (e *Engine) BindToCircle(circleID string) (*deviceidentity.BindResult, error) {
	fingerprint, err := e.GetFingerprint()
	if err != nil {
		return nil, fmt.Errorf("failed to get fingerprint: %w", err)
	}

	return e.bindingStore.Bind(circleID, fingerprint)
}

// IsBoundToCircle checks if current device is bound to a circle.
func (e *Engine) IsBoundToCircle(circleID string) (bool, error) {
	fingerprint, err := e.GetFingerprint()
	if err != nil {
		return false, fmt.Errorf("failed to get fingerprint: %w", err)
	}

	return e.bindingStore.IsBound(circleID, fingerprint), nil
}

// GetBoundCount returns number of devices bound to a circle.
func (e *Engine) GetBoundCount(circleID string) int {
	return e.bindingStore.GetBoundCount(circleID)
}

// VerifySignedRequest verifies a signed request.
// Checks signature validity and period key freshness.
func (e *Engine) VerifySignedRequest(req *deviceidentity.SignedRequest) *deviceidentity.VerificationResult {
	// Validate request format
	if err := req.Validate(); err != nil {
		return &deviceidentity.VerificationResult{
			Valid: false,
			Error: fmt.Sprintf("invalid request: %v", err),
		}
	}

	// Check period key is current (within tolerance)
	now := e.clock()
	currentPeriod := deviceidentity.NewPeriodKey(now)

	// Allow current period and previous period (30-minute window)
	prevPeriod := deviceidentity.NewPeriodKey(now.Add(-15 * time.Minute))

	if req.PeriodKey != currentPeriod && req.PeriodKey != prevPeriod {
		return &deviceidentity.VerificationResult{
			Valid: false,
			Error: "period key expired or invalid",
		}
	}

	// Verify signature
	valid, err := persist.VerifyRequest(req)
	if err != nil {
		return &deviceidentity.VerificationResult{
			Valid: false,
			Error: fmt.Sprintf("verification failed: %v", err),
		}
	}

	if !valid {
		return &deviceidentity.VerificationResult{
			Valid: false,
			Error: "signature verification failed",
		}
	}

	fingerprint := req.PublicKey.Fingerprint()

	return &deviceidentity.VerificationResult{
		Valid:       true,
		Fingerprint: fingerprint,
	}
}

// RequireBoundDevice verifies the request and checks device is bound to circle.
// This is required for sensitive operations like replay export/import.
func (e *Engine) RequireBoundDevice(circleID string, req *deviceidentity.SignedRequest) *deviceidentity.VerificationResult {
	// First verify the signed request
	result := e.VerifySignedRequest(req)
	if !result.Valid {
		return result
	}

	// Check device is bound to circle
	isBound := e.bindingStore.IsBound(circleID, result.Fingerprint)
	result.IsBoundToCircle = isBound

	if !isBound {
		result.Valid = false
		result.Error = "device not bound to this circle"
	}

	return result
}

// SignRequest signs a request with the device's private key.
func (e *Engine) SignRequest(req *deviceidentity.SignedRequest) error {
	// Set period key if not set
	if req.PeriodKey == "" {
		req.PeriodKey = deviceidentity.NewPeriodKey(e.clock())
	}

	// Get public key
	pubKey, err := e.GetPublicKey()
	if err != nil {
		return fmt.Errorf("failed to get public key: %w", err)
	}
	req.PublicKey = pubKey

	// Sign
	return e.keyStore.SignRequest(req)
}

// BuildIdentityPage builds the identity page for display.
func (e *Engine) BuildIdentityPage(circleID string) (*deviceidentity.DeviceIdentityPage, error) {
	fingerprint, err := e.GetFingerprint()
	if err != nil {
		return nil, fmt.Errorf("failed to get fingerprint: %w", err)
	}

	boundCount := e.GetBoundCount(circleID)
	isBound := e.bindingStore.IsBound(circleID, fingerprint)

	return deviceidentity.NewDeviceIdentityPage(fingerprint, boundCount, isBound), nil
}

// GetCurrentPeriodKey returns the current period key.
func (e *Engine) GetCurrentPeriodKey() deviceidentity.PeriodKey {
	return deviceidentity.NewPeriodKey(e.clock())
}
