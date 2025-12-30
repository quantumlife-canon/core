// Package impl_inmem provides an in-memory implementation of the audit interfaces.
// This is for demo and testing purposes only.
//
// CRITICAL: This implementation is NOT for production use.
// Production requires persistent, tamper-evident storage.
package impl_inmem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"quantumlife/internal/audit"
	"quantumlife/pkg/primitives"
)

// Store implements the audit Logger, Reader, and LoopEventEmitter interfaces.
type Store struct {
	mu           sync.RWMutex
	entries      []audit.Entry
	explanations map[string]audit.Explanation
	lastHash     string
	idCounter    int
}

// NewStore creates a new in-memory audit store.
func NewStore() *Store {
	return &Store{
		entries:      make([]audit.Entry, 0),
		explanations: make(map[string]audit.Explanation),
		lastHash:     "",
	}
}

// Log appends an entry to the audit log.
func (s *Store) Log(ctx context.Context, entry audit.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.idCounter++
	entry.ID = fmt.Sprintf("audit-%d", s.idCounter)
	entry.Timestamp = time.Now()
	entry.PreviousHash = s.lastHash
	entry.Hash = s.computeHash(entry)
	s.lastHash = entry.Hash

	s.entries = append(s.entries, entry)
	return nil
}

// LogWithExplanation appends an entry with explanation.
func (s *Store) LogWithExplanation(ctx context.Context, entry audit.Entry, explanation audit.Explanation) error {
	if err := s.Log(ctx, entry); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Link explanation to entry
	explanation.EntryID = entry.ID
	explanation.Timestamp = time.Now()
	s.explanations[entry.ID] = explanation

	return nil
}

// Get retrieves a single audit entry.
func (s *Store) Get(ctx context.Context, entryID string) (*audit.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, e := range s.entries {
		if e.ID == entryID {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("entry not found: %s", entryID)
}

// List retrieves audit entries matching filter.
func (s *Store) List(ctx context.Context, filter audit.Filter) ([]audit.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []audit.Entry
	for _, e := range s.entries {
		if s.matchesFilter(e, filter) {
			results = append(results, e)
		}
	}

	// Apply offset and limit
	if filter.Offset > 0 && filter.Offset < len(results) {
		results = results[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results, nil
}

// GetExplanation retrieves the explanation for an entry.
func (s *Store) GetExplanation(ctx context.Context, entryID string) (*audit.Explanation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if exp, ok := s.explanations[entryID]; ok {
		return &exp, nil
	}
	return nil, fmt.Errorf("explanation not found for entry: %s", entryID)
}

// GetAllEntries returns all entries (for demo/testing).
func (s *Store) GetAllEntries() []audit.Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]audit.Entry, len(s.entries))
	copy(result, s.entries)
	return result
}

// EmitStepStarted emits an event when a loop step begins.
func (s *Store) EmitStepStarted(ctx context.Context, loopCtx primitives.LoopContext, step string) error {
	entry := audit.Entry{
		CircleID:  loopCtx.IssuerCircleID,
		EventType: fmt.Sprintf("loop.step.%s.started", step),
		Action:    "step_started",
		Outcome:   "in_progress",
		Metadata: map[string]string{
			"trace_id": string(loopCtx.TraceID),
			"step":     step,
		},
	}
	return s.Log(ctx, entry)
}

// EmitStepCompleted emits an event when a loop step completes.
func (s *Store) EmitStepCompleted(ctx context.Context, loopCtx primitives.LoopContext, step string, resultSummary string) error {
	entry := audit.Entry{
		CircleID:  loopCtx.IssuerCircleID,
		EventType: fmt.Sprintf("loop.step.%s.completed", step),
		Action:    "step_completed",
		Outcome:   "success",
		Metadata: map[string]string{
			"trace_id": string(loopCtx.TraceID),
			"step":     step,
			"result":   resultSummary,
		},
	}
	return s.Log(ctx, entry)
}

// EmitStepFailed emits an event when a loop step fails.
func (s *Store) EmitStepFailed(ctx context.Context, loopCtx primitives.LoopContext, step string, errMsg string) error {
	entry := audit.Entry{
		CircleID:  loopCtx.IssuerCircleID,
		EventType: fmt.Sprintf("loop.step.%s.failed", step),
		Action:    "step_failed",
		Outcome:   "failure",
		Metadata: map[string]string{
			"trace_id": string(loopCtx.TraceID),
			"step":     step,
			"error":    errMsg,
		},
	}
	return s.Log(ctx, entry)
}

// EmitLoopCompleted emits an event when the entire loop completes.
func (s *Store) EmitLoopCompleted(ctx context.Context, loopCtx primitives.LoopContext, success bool, summary string) error {
	outcome := "success"
	if !success {
		outcome = "failure"
	}
	entry := audit.Entry{
		CircleID:  loopCtx.IssuerCircleID,
		EventType: "loop.completed",
		Action:    "loop_completed",
		Outcome:   outcome,
		Metadata: map[string]string{
			"trace_id": string(loopCtx.TraceID),
			"summary":  summary,
		},
	}
	return s.Log(ctx, entry)
}

// EmitLoopAborted emits an event when a loop is aborted.
func (s *Store) EmitLoopAborted(ctx context.Context, loopCtx primitives.LoopContext, reason string) error {
	entry := audit.Entry{
		CircleID:  loopCtx.IssuerCircleID,
		EventType: "loop.aborted",
		Action:    "loop_aborted",
		Outcome:   "aborted",
		Metadata: map[string]string{
			"trace_id": string(loopCtx.TraceID),
			"reason":   reason,
		},
	}
	return s.Log(ctx, entry)
}

// matchesFilter checks if an entry matches the given filter.
func (s *Store) matchesFilter(entry audit.Entry, filter audit.Filter) bool {
	if filter.CircleID != "" && entry.CircleID != filter.CircleID {
		return false
	}
	if filter.IntersectionID != "" && entry.IntersectionID != filter.IntersectionID {
		return false
	}
	if filter.EventType != "" && entry.EventType != filter.EventType {
		return false
	}
	if filter.SubjectID != "" && entry.SubjectID != filter.SubjectID {
		return false
	}
	if !filter.After.IsZero() && entry.Timestamp.Before(filter.After) {
		return false
	}
	if !filter.Before.IsZero() && entry.Timestamp.After(filter.Before) {
		return false
	}
	return true
}

// computeHash computes the hash for an entry.
func (s *Store) computeHash(entry audit.Entry) string {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		entry.ID, entry.CircleID, entry.EventType,
		entry.Action, entry.Outcome, entry.PreviousHash)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Entry is a convenience type for demos that converts to audit.Entry.
type Entry struct {
	Type                 string
	CircleID             string
	IntersectionID       string
	Action               string
	Outcome              string
	TraceID              string
	AuthorizationProofID string
}

// Append is a convenience method for demos that creates an audit entry.
func (s *Store) Append(ctx context.Context, e Entry) error {
	entry := audit.Entry{
		CircleID:             e.CircleID,
		IntersectionID:       e.IntersectionID,
		EventType:            e.Type,
		Action:               e.Action,
		Outcome:              e.Outcome,
		TraceID:              e.TraceID,
		AuthorizationProofID: e.AuthorizationProofID,
	}
	return s.Log(ctx, entry)
}

// ListAll returns all entries (convenience method without filter).
func (s *Store) ListAll(ctx context.Context) ([]audit.Entry, error) {
	return s.List(ctx, audit.Filter{})
}

// Verify interface compliance at compile time.
var (
	_ audit.Logger           = (*Store)(nil)
	_ audit.Reader           = (*Store)(nil)
	_ audit.LoopEventEmitter = (*Store)(nil)
)
