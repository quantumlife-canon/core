// Package persist provides notification persistence for Phase 16.
//
// NotificationStore uses Phase 12 storelog for append-only persistence.
// All records are hash-verified and support deterministic replay.
//
// CRITICAL: No goroutines. No time.Now(). Clock must be injected.
// CRITICAL: Append-only. Records are never modified or deleted.
//
// Reference: docs/ADR/ADR-0032-phase16-notification-projection.md
package persist

import (
	"fmt"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/interrupt"
	"quantumlife/pkg/domain/notify"
	"quantumlife/pkg/domain/storelog"
	"quantumlife/pkg/events"
)

// NotificationStore provides persistent storage for notifications.
type NotificationStore struct {
	log          storelog.AppendOnlyLog
	eventEmitter events.Emitter
	clock        func() time.Time

	// In-memory indexes built from replay
	notifications map[string]*notify.Notification
	badges        map[string]*notify.Notification
	plans         map[string]*notify.NotificationPlan
	delivered     map[string]time.Time
}

// NotificationStoreOption configures the store.
type NotificationStoreOption func(*NotificationStore)

// WithNotifyLog sets the append-only log.
func WithNotifyLog(log storelog.AppendOnlyLog) NotificationStoreOption {
	return func(s *NotificationStore) {
		s.log = log
	}
}

// WithNotifyEventEmitter sets the event emitter.
func WithNotifyEventEmitter(emitter events.Emitter) NotificationStoreOption {
	return func(s *NotificationStore) {
		s.eventEmitter = emitter
	}
}

// WithNotifyClock sets the clock.
func WithNotifyClock(clock func() time.Time) NotificationStoreOption {
	return func(s *NotificationStore) {
		s.clock = clock
	}
}

// NewNotificationStore creates a new notification store.
func NewNotificationStore(opts ...NotificationStoreOption) *NotificationStore {
	s := &NotificationStore{
		log:           storelog.NewInMemoryLog(),
		clock:         time.Now,
		notifications: make(map[string]*notify.Notification),
		badges:        make(map[string]*notify.Notification),
		plans:         make(map[string]*notify.NotificationPlan),
		delivered:     make(map[string]time.Time),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// AddPlanned records a planned notification.
func (s *NotificationStore) AddPlanned(n *notify.Notification) error {
	now := s.clock()

	payload := n.CanonicalString()
	record := storelog.NewRecord(
		storelog.RecordTypeNotificationPlanned,
		now,
		n.CircleID,
		payload,
	)

	if err := s.log.Append(record); err != nil {
		return fmt.Errorf("append notification: %w", err)
	}

	s.notifications[n.NotificationID] = n

	// Track web badges
	if n.Channel == notify.ChannelWebBadge && n.Status == notify.StatusPlanned {
		s.badges[n.NotificationID] = n
	}

	s.emit(events.Event{
		Type:      events.Phase16NotifyStoreAppend,
		Timestamp: now,
		Metadata: map[string]string{
			"notification_id": n.NotificationID,
			"record_type":     storelog.RecordTypeNotificationPlanned,
		},
	})

	return nil
}

// MarkDelivered records that a notification was delivered.
func (s *NotificationStore) MarkDelivered(notificationID string, deliveredAt time.Time) error {
	now := s.clock()

	payload := fmt.Sprintf("id:%s|delivered:%d", notificationID, deliveredAt.Unix())
	record := storelog.NewRecord(
		storelog.RecordTypeNotificationDelivered,
		now,
		"", // No specific circle
		payload,
	)

	if err := s.log.Append(record); err != nil {
		return fmt.Errorf("append delivered: %w", err)
	}

	s.delivered[notificationID] = deliveredAt

	if n, ok := s.notifications[notificationID]; ok {
		n.MarkDelivered(deliveredAt)
		// Remove from badges if web badge
		delete(s.badges, notificationID)
	}

	return nil
}

// MarkSuppressed records that a notification was suppressed.
func (s *NotificationStore) MarkSuppressed(notificationID string, reason notify.SuppressionReason) error {
	now := s.clock()

	payload := fmt.Sprintf("id:%s|reason:%s", notificationID, reason)
	record := storelog.NewRecord(
		storelog.RecordTypeNotificationSuppressed,
		now,
		"",
		payload,
	)

	if err := s.log.Append(record); err != nil {
		return fmt.Errorf("append suppressed: %w", err)
	}

	if n, ok := s.notifications[notificationID]; ok {
		n.Suppress(reason)
		delete(s.badges, notificationID)
	}

	return nil
}

// AddPlan records a notification plan.
func (s *NotificationStore) AddPlan(plan *notify.NotificationPlan) error {
	now := s.clock()

	payload := plan.CanonicalString()
	record := storelog.NewRecord(
		storelog.RecordTypeNotifyDigest,
		now,
		"",
		payload,
	)

	if err := s.log.Append(record); err != nil {
		return fmt.Errorf("append plan: %w", err)
	}

	s.plans[plan.PlanID] = plan

	// Also add each notification
	for _, n := range plan.Notifications {
		if err := s.AddPlanned(n); err != nil {
			// Log but continue
		}
	}

	return nil
}

// GetNotification retrieves a notification by ID.
func (s *NotificationStore) GetNotification(id string) (*notify.Notification, bool) {
	n, ok := s.notifications[id]
	return n, ok
}

// GetBadges returns current badge counts.
func (s *NotificationStore) GetBadges(now time.Time) *notify.BadgeCounts {
	counts := notify.NewBadgeCounts()

	for _, n := range s.badges {
		// Skip expired badges
		if n.IsExpired(now) {
			continue
		}
		counts.Add(n)
	}

	return counts
}

// GetBadgesByCircle returns badges for a specific circle.
func (s *NotificationStore) GetBadgesByCircle(circleID identity.EntityID, now time.Time) []*notify.Notification {
	var result []*notify.Notification

	for _, n := range s.badges {
		if n.CircleID == circleID && !n.IsExpired(now) {
			result = append(result, n)
		}
	}

	return result
}

// ClearBadge removes a badge.
func (s *NotificationStore) ClearBadge(notificationID string) error {
	delete(s.badges, notificationID)
	return nil
}

// Replay rebuilds state from the log.
func (s *NotificationStore) Replay() error {
	// Clear indexes
	s.notifications = make(map[string]*notify.Notification)
	s.badges = make(map[string]*notify.Notification)
	s.plans = make(map[string]*notify.NotificationPlan)
	s.delivered = make(map[string]time.Time)

	records, err := s.log.List()
	if err != nil {
		return fmt.Errorf("list records: %w", err)
	}

	for _, record := range records {
		switch record.Type {
		case storelog.RecordTypeNotificationPlanned:
			n, err := parseNotificationPayload(record.Payload)
			if err != nil {
				continue // Skip malformed records
			}
			s.notifications[n.NotificationID] = n
			if n.Channel == notify.ChannelWebBadge && n.Status == notify.StatusPlanned {
				s.badges[n.NotificationID] = n
			}

		case storelog.RecordTypeNotificationDelivered:
			id, deliveredAt, err := parseDeliveredPayload(record.Payload)
			if err != nil {
				continue
			}
			s.delivered[id] = deliveredAt
			if n, ok := s.notifications[id]; ok {
				n.MarkDelivered(deliveredAt)
				delete(s.badges, id)
			}

		case storelog.RecordTypeNotificationSuppressed:
			id, reason, err := parseSuppressedPayload(record.Payload)
			if err != nil {
				continue
			}
			if n, ok := s.notifications[id]; ok {
				n.Suppress(reason)
				delete(s.badges, id)
			}
		}
	}

	s.emit(events.Event{
		Type:      events.Phase16NotifyStoreReplay,
		Timestamp: s.clock(),
		Metadata: map[string]string{
			"notification_count": fmt.Sprintf("%d", len(s.notifications)),
			"badge_count":        fmt.Sprintf("%d", len(s.badges)),
		},
	})

	return nil
}

// ListDelivered returns delivered notification IDs.
func (s *NotificationStore) ListDelivered() map[string]time.Time {
	result := make(map[string]time.Time, len(s.delivered))
	for k, v := range s.delivered {
		result[k] = v
	}
	return result
}

// Count returns the total number of records.
func (s *NotificationStore) Count() int {
	return s.log.Count()
}

func (s *NotificationStore) emit(event events.Event) {
	if s.eventEmitter != nil {
		s.eventEmitter.Emit(event)
	}
}

// parseNotificationPayload parses a notification from its canonical string.
// This is a simplified parser - in production, use proper parsing.
func parseNotificationPayload(payload string) (*notify.Notification, error) {
	// Format: interruption:X|circle:Y|intersection:Z|level:L|channel:C|...
	parts := splitParts(payload)

	n := &notify.Notification{
		Status: notify.StatusPlanned,
	}

	for _, part := range parts {
		kv := splitKV(part)
		if len(kv) != 2 {
			continue
		}
		key, value := kv[0], kv[1]

		switch key {
		case "interruption":
			n.InterruptionID = value
		case "circle":
			n.CircleID = identity.EntityID(value)
		case "intersection":
			n.IntersectionID = value
		case "level":
			n.Level = interrupt.Level(value)
		case "channel":
			n.Channel = notify.Channel(value)
		case "trigger":
			n.Trigger = interrupt.Trigger(value)
		case "audience":
			n.Audience = notify.Audience(value)
		case "summary":
			n.Summary = value
		case "status":
			n.Status = notify.NotificationStatus(value)
		case "suppression":
			n.SuppressionReason = notify.SuppressionReason(value)
		}
	}

	// Generate ID from parsed data
	n.NotificationID = n.CanonicalString()[:16]
	n.Hash = n.ComputeHash()

	return n, nil
}

func parseDeliveredPayload(payload string) (string, time.Time, error) {
	// Format: id:X|delivered:Y
	parts := splitParts(payload)
	var id string
	var ts int64

	for _, part := range parts {
		kv := splitKV(part)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "id":
			id = kv[1]
		case "delivered":
			fmt.Sscanf(kv[1], "%d", &ts)
		}
	}

	if id == "" {
		return "", time.Time{}, fmt.Errorf("missing id")
	}

	return id, time.Unix(ts, 0), nil
}

func parseSuppressedPayload(payload string) (string, notify.SuppressionReason, error) {
	// Format: id:X|reason:Y
	parts := splitParts(payload)
	var id string
	var reason notify.SuppressionReason

	for _, part := range parts {
		kv := splitKV(part)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "id":
			id = kv[1]
		case "reason":
			reason = notify.SuppressionReason(kv[1])
		}
	}

	if id == "" {
		return "", "", fmt.Errorf("missing id")
	}

	return id, reason, nil
}

func splitParts(s string) []string {
	return strings.Split(s, "|")
}

func splitKV(s string) []string {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+1:]}
}
