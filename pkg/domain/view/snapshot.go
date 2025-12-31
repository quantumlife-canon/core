// Package view provides view snapshot models for circle state representation.
//
// A CircleViewSnapshot captures the current state of a circle for display.
// It aggregates counts, deadlines, and metadata from ingested events.
//
// CRITICAL: Views are READ-ONLY representations. They do not trigger actions.
//
// Reference: docs/ARCHITECTURE_LIFE_OS_V1.md
package view

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
)

// CircleViewSnapshot captures the current state of a circle.
type CircleViewSnapshot struct {
	// Identity
	CircleID   identity.EntityID `json:"circle_id"`
	CircleName string            `json:"circle_name"`

	// Timing
	CapturedAt time.Time `json:"captured_at"`

	// Counts
	Counts ViewCounts `json:"counts"`

	// Next deadline (if any)
	NextDeadline *Deadline `json:"next_deadline,omitempty"`

	// Pending items requiring attention
	PendingItems []PendingItem `json:"pending_items,omitempty"`

	// Hash for integrity verification (v9.13)
	Hash string `json:"hash"`
}

// ViewCounts holds aggregated counts for a circle view.
type ViewCounts struct {
	// Email counts
	UnreadEmails    int `json:"unread_emails"`
	ImportantEmails int `json:"important_emails"`

	// Calendar counts
	UpcomingEvents int `json:"upcoming_events"` // Next 7 days
	TodayEvents    int `json:"today_events"`

	// Finance counts (if applicable)
	PendingTransactions int    `json:"pending_transactions,omitempty"`
	NewTransactions     int    `json:"new_transactions,omitempty"` // Since last view
	TotalBalanceMinor   int64  `json:"total_balance_minor,omitempty"`
	BalanceCurrency     string `json:"balance_currency,omitempty"`

	// General
	TotalItems         int `json:"total_items"`
	ItemsNeedingAction int `json:"items_needing_action"`
}

// Deadline represents an upcoming obligation deadline.
type Deadline struct {
	Title       string    `json:"title"`
	DueAt       time.Time `json:"due_at"`
	Source      string    `json:"source"` // email, calendar, etc.
	SourceID    string    `json:"source_id"`
	RegretScore float64   `json:"regret_score,omitempty"`
}

// PendingItem represents an item requiring attention.
type PendingItem struct {
	ID         string               `json:"id"`
	Type       string               `json:"type"` // email, event, transaction, form
	Title      string               `json:"title"`
	Summary    string               `json:"summary,omitempty"`
	OccurredAt time.Time            `json:"occurred_at"`
	DueAt      *time.Time           `json:"due_at,omitempty"`
	Priority   Priority             `json:"priority"`
	SourceID   string               `json:"source_id"`
	EntityRefs []identity.EntityRef `json:"entity_refs,omitempty"`
}

// Priority levels for pending items.
type Priority string

const (
	PriorityLow    Priority = "LOW"
	PriorityNormal Priority = "NORMAL"
	PriorityHigh   Priority = "HIGH"
	PriorityUrgent Priority = "URGENT"
)

// ComputeHash generates a deterministic hash of the snapshot for v9.13 binding.
func (s *CircleViewSnapshot) ComputeHash() string {
	// Build canonical string for hashing
	var parts []string

	parts = append(parts, string(s.CircleID))
	parts = append(parts, s.CircleName)
	parts = append(parts, fmt.Sprintf("%d", s.CapturedAt.Unix()))

	// Counts
	parts = append(parts, fmt.Sprintf("unread:%d", s.Counts.UnreadEmails))
	parts = append(parts, fmt.Sprintf("important:%d", s.Counts.ImportantEmails))
	parts = append(parts, fmt.Sprintf("upcoming:%d", s.Counts.UpcomingEvents))
	parts = append(parts, fmt.Sprintf("today:%d", s.Counts.TodayEvents))
	parts = append(parts, fmt.Sprintf("pending_tx:%d", s.Counts.PendingTransactions))
	parts = append(parts, fmt.Sprintf("new_tx:%d", s.Counts.NewTransactions))
	parts = append(parts, fmt.Sprintf("balance:%d:%s", s.Counts.TotalBalanceMinor, s.Counts.BalanceCurrency))
	parts = append(parts, fmt.Sprintf("total:%d", s.Counts.TotalItems))
	parts = append(parts, fmt.Sprintf("action:%d", s.Counts.ItemsNeedingAction))

	// Deadline
	if s.NextDeadline != nil {
		parts = append(parts, fmt.Sprintf("deadline:%s:%d", s.NextDeadline.Title, s.NextDeadline.DueAt.Unix()))
	}

	// Pending items (sorted by ID for determinism)
	sortedItems := make([]PendingItem, len(s.PendingItems))
	copy(sortedItems, s.PendingItems)
	sort.Slice(sortedItems, func(i, j int) bool {
		return sortedItems[i].ID < sortedItems[j].ID
	})
	for _, item := range sortedItems {
		parts = append(parts, fmt.Sprintf("item:%s:%s", item.ID, item.Type))
	}

	canonical := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// IsStale checks if the snapshot is older than maxAge.
func (s *CircleViewSnapshot) IsStale(now time.Time, maxAge time.Duration) bool {
	return now.Sub(s.CapturedAt) > maxAge
}

// NeedsAttention returns true if there are items requiring action.
func (s *CircleViewSnapshot) NeedsAttention() bool {
	return s.Counts.ItemsNeedingAction > 0
}

// NothingNeedsYou returns true if the circle is in "Nothing Needs You" state.
func (s *CircleViewSnapshot) NothingNeedsYou() bool {
	return s.Counts.ItemsNeedingAction == 0 &&
		s.Counts.UnreadEmails == 0 &&
		s.Counts.TodayEvents == 0 &&
		s.NextDeadline == nil
}

// ViewSnapshotBuilder helps construct CircleViewSnapshots.
type ViewSnapshotBuilder struct {
	circleID   identity.EntityID
	circleName string
	capturedAt time.Time
	counts     ViewCounts
	deadline   *Deadline
	items      []PendingItem
}

// NewViewSnapshotBuilder creates a new builder.
func NewViewSnapshotBuilder(circleID identity.EntityID, circleName string, capturedAt time.Time) *ViewSnapshotBuilder {
	return &ViewSnapshotBuilder{
		circleID:   circleID,
		circleName: circleName,
		capturedAt: capturedAt,
	}
}

// WithEmailCounts sets email counts.
func (b *ViewSnapshotBuilder) WithEmailCounts(unread, important int) *ViewSnapshotBuilder {
	b.counts.UnreadEmails = unread
	b.counts.ImportantEmails = important
	return b
}

// WithCalendarCounts sets calendar counts.
func (b *ViewSnapshotBuilder) WithCalendarCounts(upcoming, today int) *ViewSnapshotBuilder {
	b.counts.UpcomingEvents = upcoming
	b.counts.TodayEvents = today
	return b
}

// WithFinanceCounts sets finance counts.
func (b *ViewSnapshotBuilder) WithFinanceCounts(pending, new int, balance int64, currency string) *ViewSnapshotBuilder {
	b.counts.PendingTransactions = pending
	b.counts.NewTransactions = new
	b.counts.TotalBalanceMinor = balance
	b.counts.BalanceCurrency = currency
	return b
}

// WithTotalCounts sets total counts.
func (b *ViewSnapshotBuilder) WithTotalCounts(total, needingAction int) *ViewSnapshotBuilder {
	b.counts.TotalItems = total
	b.counts.ItemsNeedingAction = needingAction
	return b
}

// WithNextDeadline sets the next deadline.
func (b *ViewSnapshotBuilder) WithNextDeadline(title string, dueAt time.Time, source, sourceID string) *ViewSnapshotBuilder {
	b.deadline = &Deadline{
		Title:    title,
		DueAt:    dueAt,
		Source:   source,
		SourceID: sourceID,
	}
	return b
}

// AddPendingItem adds a pending item.
func (b *ViewSnapshotBuilder) AddPendingItem(item PendingItem) *ViewSnapshotBuilder {
	b.items = append(b.items, item)
	return b
}

// Build creates the CircleViewSnapshot with computed hash.
func (b *ViewSnapshotBuilder) Build() *CircleViewSnapshot {
	snapshot := &CircleViewSnapshot{
		CircleID:     b.circleID,
		CircleName:   b.circleName,
		CapturedAt:   b.capturedAt,
		Counts:       b.counts,
		NextDeadline: b.deadline,
		PendingItems: b.items,
	}

	// Compute hash
	snapshot.Hash = snapshot.ComputeHash()

	return snapshot
}

// ViewStore provides storage for view snapshots.
type ViewStore interface {
	// Store saves a view snapshot.
	Store(snapshot *CircleViewSnapshot) error

	// GetLatest retrieves the most recent snapshot for a circle.
	GetLatest(circleID identity.EntityID) (*CircleViewSnapshot, error)

	// GetByHash retrieves a snapshot by its hash.
	GetByHash(hash string) (*CircleViewSnapshot, error)
}

// InMemoryViewStore is a thread-safe in-memory implementation.
type InMemoryViewStore struct {
	snapshots map[identity.EntityID][]*CircleViewSnapshot
	byHash    map[string]*CircleViewSnapshot
}

// NewInMemoryViewStore creates a new in-memory view store.
func NewInMemoryViewStore() *InMemoryViewStore {
	return &InMemoryViewStore{
		snapshots: make(map[identity.EntityID][]*CircleViewSnapshot),
		byHash:    make(map[string]*CircleViewSnapshot),
	}
}

func (s *InMemoryViewStore) Store(snapshot *CircleViewSnapshot) error {
	s.snapshots[snapshot.CircleID] = append(s.snapshots[snapshot.CircleID], snapshot)
	s.byHash[snapshot.Hash] = snapshot
	return nil
}

func (s *InMemoryViewStore) GetLatest(circleID identity.EntityID) (*CircleViewSnapshot, error) {
	snapshots := s.snapshots[circleID]
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no snapshots for circle %s", circleID)
	}
	return snapshots[len(snapshots)-1], nil
}

func (s *InMemoryViewStore) GetByHash(hash string) (*CircleViewSnapshot, error) {
	snapshot, exists := s.byHash[hash]
	if !exists {
		return nil, fmt.Errorf("snapshot not found for hash %s", hash)
	}
	return snapshot, nil
}

// Verify interface compliance.
var _ ViewStore = (*InMemoryViewStore)(nil)
