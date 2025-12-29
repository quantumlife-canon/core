// Package primitives defines the immutable data structures for all canon primitives.
// These primitives flow through the irreducible loop defined in Canon v1.
//
// Reference: docs/QUANTUMLIFE_CANON_V1.md §Ontology
package primitives

import (
	"time"
)

// Intent represents a desire or goal expressed by a circle.
// It is the entry point to the irreducible loop.
//
// Canon Reference: The Irreducible Loop — Step 1 (Intent)
type Intent struct {
	// ID uniquely identifies this intent.
	ID string

	// Version tracks the schema version of this intent.
	Version int

	// CreatedAt is the timestamp when this intent was created.
	CreatedAt time.Time

	// Issuer identifies the circle that expressed this intent.
	Issuer string

	// Description is a human-readable description of the intent.
	Description string

	// Context contains additional metadata for intent processing.
	Context map[string]string
}

// Validate checks that the intent has all required fields.
// Returns an error if validation fails.
func (i *Intent) Validate() error {
	if i.ID == "" {
		return ErrMissingID
	}
	if i.Issuer == "" {
		return ErrMissingIssuer
	}
	if i.CreatedAt.IsZero() {
		return ErrMissingTimestamp
	}
	return nil
}
