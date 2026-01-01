// Package notifyexec provides notification execution boundary enforcement.
//
// This package handles the actual delivery of notifications through
// various channels. It enforces:
// - Policy and view snapshot verification before execution
// - Draft→Approval→ExecIntent flow for email channels
// - Web badge storage for passive UI updates
// - Mock-only implementation for push/sms (ErrNotSupported in real mode)
//
// CRITICAL: No auto-retries. No background execution.
// CRITICAL: All executions are idempotent.
// CRITICAL: Clock must be injected.
//
// Reference: docs/ADR/ADR-0032-phase16-notification-projection.md
package notifyexec

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/notify"
	"quantumlife/pkg/events"
)

// Common errors
var (
	ErrNotSupported     = errors.New("channel not supported")
	ErrPolicyMismatch   = errors.New("policy snapshot mismatch")
	ErrViewMismatch     = errors.New("view snapshot mismatch")
	ErrAlreadyDelivered = errors.New("notification already delivered")
	ErrExpired          = errors.New("notification expired")
	ErrChannelBlocked   = errors.New("channel blocked in real mode")
)

// NotificationEnvelope wraps a notification for execution.
type NotificationEnvelope struct {
	// EnvelopeID is deterministic from contents.
	EnvelopeID string

	// Notification to deliver.
	Notification *notify.Notification

	// PolicySnapshotHash verifies policy hasn't changed.
	PolicySnapshotHash string

	// ViewSnapshotHash verifies view state.
	ViewSnapshotHash string

	// ViewSnapshotAt is when the view was captured.
	ViewSnapshotAt time.Time

	// IdempotencyKey prevents duplicate delivery.
	IdempotencyKey string

	// TraceID for audit.
	TraceID string

	// CreatedAt is envelope creation time.
	CreatedAt time.Time

	// Status of the envelope.
	Status EnvelopeStatus

	// DeliveredAt is when delivery completed.
	DeliveredAt *time.Time

	// Error message if failed.
	Error string

	// Hash of the envelope.
	Hash string
}

// EnvelopeStatus represents envelope lifecycle.
type EnvelopeStatus string

const (
	EnvelopeStatusPending   EnvelopeStatus = "pending"
	EnvelopeStatusDelivered EnvelopeStatus = "delivered"
	EnvelopeStatusFailed    EnvelopeStatus = "failed"
	EnvelopeStatusBlocked   EnvelopeStatus = "blocked"
)

// NewNotificationEnvelope creates an envelope for execution.
func NewNotificationEnvelope(
	notification *notify.Notification,
	policyHash, viewHash string,
	viewAt time.Time,
	traceID string,
	createdAt time.Time,
) *NotificationEnvelope {
	env := &NotificationEnvelope{
		Notification:       notification,
		PolicySnapshotHash: policyHash,
		ViewSnapshotHash:   viewHash,
		ViewSnapshotAt:     viewAt,
		TraceID:            traceID,
		CreatedAt:          createdAt,
		Status:             EnvelopeStatusPending,
	}

	// Compute idempotency key
	env.IdempotencyKey = fmt.Sprintf("notify|%s|%s|%d",
		notification.NotificationID, traceID, createdAt.Unix())

	// Compute envelope ID and hash
	env.EnvelopeID = env.computeID()
	env.Hash = env.ComputeHash()

	return env
}

func (e *NotificationEnvelope) computeID() string {
	canonical := fmt.Sprintf("env|%s|%s|%d",
		e.Notification.NotificationID, e.TraceID, e.CreatedAt.Unix())
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:8])
}

// ComputeHash returns the envelope hash.
func (e *NotificationEnvelope) ComputeHash() string {
	canonical := fmt.Sprintf("envelope|%s|%s|%s|%s|%s|%s|%d",
		e.EnvelopeID,
		e.Notification.NotificationID,
		e.PolicySnapshotHash,
		e.ViewSnapshotHash,
		e.IdempotencyKey,
		e.Status,
		e.CreatedAt.Unix())
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// Executor delivers notifications through various channels.
type Executor struct {
	// badgeStore stores web badges.
	badgeStore BadgeStore

	// emailDraftCreator creates email drafts.
	emailDraftCreator EmailDraftCreator

	// pushProvider delivers push notifications (mock only).
	pushProvider PushProvider

	// smsProvider delivers SMS (mock only).
	smsProvider SMSProvider

	// policyVerifier verifies policy snapshots.
	policyVerifier PolicyVerifier

	// viewVerifier verifies view snapshots.
	viewVerifier ViewVerifier

	// envelopeStore persists envelopes.
	envelopeStore EnvelopeStore

	// eventEmitter emits audit events.
	eventEmitter events.Emitter

	// clock provides current time.
	clock func() time.Time

	// realMode blocks push/sms when true.
	realMode bool
}

// ExecutorOption configures the executor.
type ExecutorOption func(*Executor)

// WithBadgeStore sets the badge store.
func WithBadgeStore(store BadgeStore) ExecutorOption {
	return func(e *Executor) {
		e.badgeStore = store
	}
}

// WithEmailDraftCreator sets the email draft creator.
func WithEmailDraftCreator(creator EmailDraftCreator) ExecutorOption {
	return func(e *Executor) {
		e.emailDraftCreator = creator
	}
}

// WithPushProvider sets the push provider.
func WithPushProvider(provider PushProvider) ExecutorOption {
	return func(e *Executor) {
		e.pushProvider = provider
	}
}

// WithSMSProvider sets the SMS provider.
func WithSMSProvider(provider SMSProvider) ExecutorOption {
	return func(e *Executor) {
		e.smsProvider = provider
	}
}

// WithPolicyVerifier sets the policy verifier.
func WithPolicyVerifier(verifier PolicyVerifier) ExecutorOption {
	return func(e *Executor) {
		e.policyVerifier = verifier
	}
}

// WithViewVerifier sets the view verifier.
func WithViewVerifier(verifier ViewVerifier) ExecutorOption {
	return func(e *Executor) {
		e.viewVerifier = verifier
	}
}

// WithEnvelopeStore sets the envelope store.
func WithEnvelopeStore(store EnvelopeStore) ExecutorOption {
	return func(e *Executor) {
		e.envelopeStore = store
	}
}

// WithEventEmitter sets the event emitter.
func WithEventEmitter(emitter events.Emitter) ExecutorOption {
	return func(e *Executor) {
		e.eventEmitter = emitter
	}
}

// WithClock sets the clock function.
func WithClock(clock func() time.Time) ExecutorOption {
	return func(e *Executor) {
		e.clock = clock
	}
}

// WithRealMode enables real mode (blocks push/sms).
func WithRealMode(real bool) ExecutorOption {
	return func(e *Executor) {
		e.realMode = real
	}
}

// NewExecutor creates a new notification executor.
func NewExecutor(opts ...ExecutorOption) *Executor {
	e := &Executor{
		badgeStore:    NewMemoryBadgeStore(),
		envelopeStore: NewMemoryEnvelopeStore(),
		clock:         time.Now,
		realMode:      false,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Execute delivers a notification envelope.
func (e *Executor) Execute(env *NotificationEnvelope) (*NotificationEnvelope, error) {
	now := e.clock()

	// Emit attempt event
	e.emit(events.Event{
		Type:      events.Phase16NotifyEnvelopeCreated,
		Timestamp: now,
		Metadata: map[string]string{
			"envelope_id":     env.EnvelopeID,
			"notification_id": env.Notification.NotificationID,
			"channel":         string(env.Notification.Channel),
			"circle_id":       string(env.Notification.CircleID),
		},
	})

	// Check idempotency
	if existing, found := e.envelopeStore.GetByIdempotencyKey(env.IdempotencyKey); found {
		return &existing, nil
	}

	// Check expiry
	if env.Notification.IsExpired(now) {
		return e.block(env, ErrExpired.Error(), now)
	}

	// Verify policy snapshot
	if e.policyVerifier != nil {
		if err := e.policyVerifier.Verify(env.PolicySnapshotHash); err != nil {
			return e.block(env, fmt.Sprintf("policy verification failed: %v", err), now)
		}
	}

	// Verify view snapshot
	if e.viewVerifier != nil {
		if err := e.viewVerifier.Verify(env.ViewSnapshotHash, env.ViewSnapshotAt); err != nil {
			return e.block(env, fmt.Sprintf("view verification failed: %v", err), now)
		}
	}

	// Execute based on channel
	var execErr error
	switch env.Notification.Channel {
	case notify.ChannelWebBadge:
		execErr = e.executeWebBadge(env, now)
	case notify.ChannelEmailDigest, notify.ChannelEmailAlert:
		execErr = e.executeEmail(env, now)
	case notify.ChannelPush:
		execErr = e.executePush(env, now)
	case notify.ChannelSMS:
		execErr = e.executeSMS(env, now)
	default:
		execErr = ErrNotSupported
	}

	if execErr != nil {
		return e.fail(env, execErr.Error(), now)
	}

	// Success
	env.Status = EnvelopeStatusDelivered
	deliveredAt := now
	env.DeliveredAt = &deliveredAt
	env.Notification.MarkDelivered(now)
	env.Hash = env.ComputeHash()

	// Store result
	if err := e.envelopeStore.Put(*env); err != nil {
		// Log but don't fail
		e.emit(events.Event{
			Type:      events.Phase16NotifyStoreError,
			Timestamp: now,
			Metadata: map[string]string{
				"envelope_id": env.EnvelopeID,
				"error":       err.Error(),
			},
		})
	}

	// Emit success
	e.emit(events.Event{
		Type:      events.Phase16NotifyDelivered,
		Timestamp: now,
		Metadata: map[string]string{
			"envelope_id":     env.EnvelopeID,
			"notification_id": env.Notification.NotificationID,
			"channel":         string(env.Notification.Channel),
		},
	})

	return env, nil
}

func (e *Executor) executeWebBadge(env *NotificationEnvelope, now time.Time) error {
	badge := Badge{
		NotificationID: env.Notification.NotificationID,
		CircleID:       env.Notification.CircleID,
		Level:          env.Notification.Level,
		Summary:        env.Notification.Summary,
		CreatedAt:      now,
		ExpiresAt:      env.Notification.ExpiresAt,
	}
	return e.badgeStore.Add(badge)
}

func (e *Executor) executeEmail(env *NotificationEnvelope, now time.Time) error {
	if e.emailDraftCreator == nil {
		return errors.New("email draft creator not configured")
	}

	// Create email draft (not auto-sent)
	draft := EmailDraft{
		DraftID:        fmt.Sprintf("notify-draft-%s", env.Notification.NotificationID),
		NotificationID: env.Notification.NotificationID,
		CircleID:       env.Notification.CircleID,
		PersonIDs:      env.Notification.PersonIDs,
		Subject:        e.buildEmailSubject(env.Notification),
		Body:           e.buildEmailBody(env.Notification),
		Channel:        env.Notification.Channel,
		CreatedAt:      now,
	}

	return e.emailDraftCreator.CreateDraft(draft)
}

func (e *Executor) executePush(env *NotificationEnvelope, now time.Time) error {
	if e.realMode {
		return ErrChannelBlocked
	}

	if e.pushProvider == nil {
		return ErrNotSupported
	}

	req := PushRequest{
		NotificationID: env.Notification.NotificationID,
		PersonIDs:      env.Notification.PersonIDs,
		Title:          e.buildPushTitle(env.Notification),
		Body:           env.Notification.Summary,
		Level:          env.Notification.Level,
	}

	return e.pushProvider.Send(req)
}

func (e *Executor) executeSMS(env *NotificationEnvelope, now time.Time) error {
	if e.realMode {
		return ErrChannelBlocked
	}

	if e.smsProvider == nil {
		return ErrNotSupported
	}

	req := SMSRequest{
		NotificationID: env.Notification.NotificationID,
		PersonIDs:      env.Notification.PersonIDs,
		Message:        e.buildSMSMessage(env.Notification),
	}

	return e.smsProvider.Send(req)
}

func (e *Executor) buildEmailSubject(n *notify.Notification) string {
	switch n.Channel {
	case notify.ChannelEmailDigest:
		return fmt.Sprintf("QuantumLife Weekly Digest")
	case notify.ChannelEmailAlert:
		return fmt.Sprintf("QuantumLife: %s", n.Summary)
	default:
		return "QuantumLife Notification"
	}
}

func (e *Executor) buildEmailBody(n *notify.Notification) string {
	// Simple, deterministic template - no dynamic content
	return fmt.Sprintf("%s\n\nOpen QuantumLife to review.", n.Summary)
}

func (e *Executor) buildPushTitle(n *notify.Notification) string {
	return "QuantumLife"
}

func (e *Executor) buildSMSMessage(n *notify.Notification) string {
	// Short, no dynamic content
	return fmt.Sprintf("QuantumLife: %s. Open app to review.", n.Summary)
}

func (e *Executor) block(env *NotificationEnvelope, reason string, now time.Time) (*NotificationEnvelope, error) {
	env.Status = EnvelopeStatusBlocked
	env.Error = reason
	env.Hash = env.ComputeHash()

	if err := e.envelopeStore.Put(*env); err != nil {
		// Log but continue
	}

	e.emit(events.Event{
		Type:      events.Phase16NotifyBlocked,
		Timestamp: now,
		Metadata: map[string]string{
			"envelope_id": env.EnvelopeID,
			"reason":      reason,
		},
	})

	return env, nil
}

func (e *Executor) fail(env *NotificationEnvelope, errorMsg string, now time.Time) (*NotificationEnvelope, error) {
	env.Status = EnvelopeStatusFailed
	env.Error = errorMsg
	env.Hash = env.ComputeHash()

	if err := e.envelopeStore.Put(*env); err != nil {
		// Log but continue
	}

	e.emit(events.Event{
		Type:      events.Phase16NotifyFailed,
		Timestamp: now,
		Metadata: map[string]string{
			"envelope_id": env.EnvelopeID,
			"error":       errorMsg,
		},
	})

	return env, nil
}

func (e *Executor) emit(event events.Event) {
	if e.eventEmitter != nil {
		e.eventEmitter.Emit(event)
	}
}

// GetBadges returns current badge counts.
func (e *Executor) GetBadges(now time.Time) *notify.BadgeCounts {
	return e.badgeStore.GetBadges(now)
}

// GetBadgesByCircle returns badges for a specific circle.
func (e *Executor) GetBadgesByCircle(circleID identity.EntityID, now time.Time) []Badge {
	return e.badgeStore.GetByCircle(circleID, now)
}

// ClearBadge removes a badge.
func (e *Executor) ClearBadge(notificationID string) error {
	return e.badgeStore.Remove(notificationID)
}

// GetEnvelope retrieves an envelope by ID.
func (e *Executor) GetEnvelope(id string) (NotificationEnvelope, bool) {
	return e.envelopeStore.Get(id)
}
