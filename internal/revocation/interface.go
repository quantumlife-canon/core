// Package revocation provides revocation signal checking for action execution.
// This is a CRITICAL safety component for v6 Execute mode.
//
// The revocation mechanism ensures that:
// - Actions can be cancelled before external writes occur
// - Authority revocation halts pending actions (per Canon)
// - Settlement can be aborted if revocation is received
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.3 Authority & Policy Engine
package revocation

import (
	"context"
	"errors"
	"time"
)

// SignalType identifies the type of revocation signal.
type SignalType string

// Signal types.
const (
	// SignalAuthorityRevoked indicates that the authority grant was revoked.
	SignalAuthorityRevoked SignalType = "authority_revoked"

	// SignalIntersectionDissolved indicates the intersection was dissolved.
	SignalIntersectionDissolved SignalType = "intersection_dissolved"

	// SignalCircleSuspended indicates the circle was suspended.
	SignalCircleSuspended SignalType = "circle_suspended"

	// SignalActionCancelled indicates the specific action was cancelled.
	SignalActionCancelled SignalType = "action_cancelled"

	// SignalCircleRevoked indicates the circle explicitly revoked permission.
	SignalCircleRevoked SignalType = "circle_revoked"
)

// Signal represents a revocation signal.
type Signal struct {
	// ID uniquely identifies this signal.
	ID string

	// Type identifies the kind of revocation.
	Type SignalType

	// TargetID is the ID of the revoked entity (grant, intersection, action, etc.).
	TargetID string

	// TargetType describes what kind of entity was revoked.
	// Examples: "authority_grant", "intersection", "circle", "action"
	TargetType string

	// RevokedAt is when the revocation occurred.
	RevokedAt time.Time

	// RevokedBy identifies who/what triggered the revocation.
	RevokedBy string

	// Reason explains why the revocation occurred.
	Reason string

	// TraceID links to a distributed trace.
	TraceID string
}

// Checker provides revocation status checking for actions.
// CRITICAL: This must be checked before any external write.
type Checker interface {
	// IsRevoked checks if a target has been revoked.
	// Returns the signal if revoked, nil if not revoked.
	IsRevoked(ctx context.Context, targetID string) (*Signal, error)

	// IsActionRevoked checks if a specific action or its authority is revoked.
	// This is the primary check before executing a write.
	// It checks:
	// - The action itself
	// - The intersection authorizing the action
	// - The authority grant
	IsActionRevoked(ctx context.Context, actionID, intersectionID, authorityProofID string) (*Signal, error)

	// CheckBeforeWrite is the final safety check before an external write.
	// CRITICAL: This MUST be called immediately before any external write.
	// Returns nil if safe to proceed, error if revoked.
	CheckBeforeWrite(ctx context.Context, actionID, intersectionID, authorityProofID string) error
}

// Signaler emits revocation signals.
type Signaler interface {
	// Revoke emits a revocation signal.
	Revoke(ctx context.Context, signal Signal) error

	// RevokeAction emits a revocation signal for a specific action.
	RevokeAction(ctx context.Context, actionID, reason, revokedBy string) error

	// RevokeAuthority emits a revocation signal for an authority grant.
	RevokeAuthority(ctx context.Context, grantID, reason, revokedBy string) error

	// RevokeIntersection emits a revocation signal for an intersection.
	RevokeIntersection(ctx context.Context, intersectionID, reason, revokedBy string) error
}

// Registry combines Checker and Signaler for full revocation management.
type Registry interface {
	Checker
	Signaler
}

// Errors for revocation checking.
var (
	// ErrActionRevoked is returned when an action has been revoked.
	ErrActionRevoked = errors.New("action has been revoked")

	// ErrAuthorityRevoked is returned when the authority grant was revoked.
	ErrAuthorityRevoked = errors.New("authority grant has been revoked")

	// ErrIntersectionDissolved is returned when the intersection was dissolved.
	ErrIntersectionDissolved = errors.New("intersection has been dissolved")

	// ErrCircleSuspended is returned when the circle was suspended.
	ErrCircleSuspended = errors.New("circle has been suspended")

	// ErrCircleRevoked is returned when the circle revoked permission.
	ErrCircleRevoked = errors.New("circle has revoked permission")
)
