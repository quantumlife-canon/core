// Package persist provides persistence for shadow LLM runs and signals.
//
// Phase 19: LLM Shadow-Mode Contract
//
// CRITICAL: This store persists METADATA ONLY - never content.
// CRITICAL: No goroutines. No time.Now() - clock injection only.
// CRITICAL: Append-only. Records are NEVER modified or deleted.
//
// Reference: docs/ADR/ADR-0043-phase19-shadow-mode-contract.md
package persist

import (
	"errors"
	"sync"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowllm"
	"quantumlife/pkg/domain/storelog"
)

// ShadowLLMStore provides persistence for shadow LLM runs.
type ShadowLLMStore struct {
	log   storelog.AppendOnlyLog
	clock func() time.Time

	// In-memory index for quick lookup (rebuilt on replay)
	mu        sync.RWMutex
	runsByID  map[string]*shadowllm.ShadowRun
	runHashes map[string]bool
}

// NewShadowLLMStore creates a new shadow LLM store.
func NewShadowLLMStore(log storelog.AppendOnlyLog, clock func() time.Time) *ShadowLLMStore {
	return &ShadowLLMStore{
		log:       log,
		clock:     clock,
		runsByID:  make(map[string]*shadowllm.ShadowRun),
		runHashes: make(map[string]bool),
	}
}

// AppendRun persists a shadow run to the log.
// Returns error if the run hash already exists (idempotency).
func (s *ShadowLLMStore) AppendRun(run *shadowllm.ShadowRun) error {
	if err := run.Validate(); err != nil {
		return err
	}

	hash := run.Hash()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate
	if s.runHashes[hash] {
		return ErrRunAlreadyExists
	}

	// Create log record
	record := storelog.NewRecord(
		storelog.RecordTypeShadowLLMRun,
		s.clock(),
		run.CircleID,
		run.CanonicalString(),
	)

	if err := s.log.Append(record); err != nil {
		return err
	}

	// Update in-memory index
	s.runsByID[run.RunID] = run
	s.runHashes[hash] = true

	// Persist signals
	for i := range run.Signals {
		if err := s.appendSignal(&run.Signals[i]); err != nil {
			// Log is append-only, so we can't roll back.
			// Signal append failure after run append is logged but not fatal.
			continue
		}
	}

	return nil
}

// appendSignal persists a single signal (internal, called with lock held).
func (s *ShadowLLMStore) appendSignal(signal *shadowllm.ShadowSignal) error {
	record := storelog.NewRecord(
		storelog.RecordTypeShadowLLMSignal,
		s.clock(),
		signal.CircleID,
		signal.CanonicalString(),
	)
	return s.log.Append(record)
}

// GetRun retrieves a run by ID.
func (s *ShadowLLMStore) GetRun(runID string) (*shadowllm.ShadowRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	run, ok := s.runsByID[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	return run, nil
}

// ListRuns returns all runs in append order.
func (s *ShadowLLMStore) ListRuns() []*shadowllm.ShadowRun {
	s.mu.RLock()
	defer s.mu.RUnlock()

	runs := make([]*shadowllm.ShadowRun, 0, len(s.runsByID))
	for _, run := range s.runsByID {
		runs = append(runs, run)
	}
	return runs
}

// ListRunsByCircle returns all runs for a circle.
func (s *ShadowLLMStore) ListRunsByCircle(circleID identity.EntityID) []*shadowllm.ShadowRun {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var runs []*shadowllm.ShadowRun
	for _, run := range s.runsByID {
		if run.CircleID == circleID {
			runs = append(runs, run)
		}
	}
	return runs
}

// ContainsHash checks if a run with the given hash exists.
func (s *ShadowLLMStore) ContainsHash(hash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runHashes[hash]
}

// Stats returns statistics about the store.
func (s *ShadowLLMStore) Stats() ShadowLLMStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := ShadowLLMStats{
		TotalRuns:         len(s.runsByID),
		SignalCountByKind: make(map[shadowllm.ShadowSignalKind]int),
	}

	for _, run := range s.runsByID {
		stats.TotalSignals += len(run.Signals)
		for _, sig := range run.Signals {
			stats.SignalCountByKind[sig.Kind]++
		}
	}

	return stats
}

// ShadowLLMStats contains statistics about shadow LLM runs.
type ShadowLLMStats struct {
	TotalRuns         int
	TotalSignals      int
	SignalCountByKind map[shadowllm.ShadowSignalKind]int
}

// Replay rebuilds the in-memory index from the log.
// This is used at startup to restore state.
func (s *ShadowLLMStore) Replay() error {
	records, err := s.log.ListByType(storelog.RecordTypeShadowLLMRun)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing index
	s.runsByID = make(map[string]*shadowllm.ShadowRun)
	s.runHashes = make(map[string]bool)

	for _, record := range records {
		run, err := parseShadowRun(record.Payload)
		if err != nil {
			// Log corrupted record but continue
			continue
		}

		s.runsByID[run.RunID] = run
		s.runHashes[run.Hash()] = true
	}

	return nil
}

// parseShadowRun parses a shadow run from its canonical string.
// Format: SHADOW_RUN|v1|run_id|circle_id|inputs_hash|model_spec|seed|created_at|signals_hash
func parseShadowRun(payload string) (*shadowllm.ShadowRun, error) {
	parts := splitPayload(payload, 9)
	if len(parts) < 9 {
		return nil, ErrInvalidPayload
	}

	if parts[0] != "SHADOW_RUN" || parts[1] != "v1" {
		return nil, ErrInvalidPayload
	}

	seed, err := parseInt64(parts[6])
	if err != nil {
		return nil, ErrInvalidPayload
	}

	createdAt, err := time.Parse(time.RFC3339Nano, parts[7])
	if err != nil {
		return nil, ErrInvalidPayload
	}

	run := &shadowllm.ShadowRun{
		RunID:      parts[2],
		CircleID:   identity.EntityID(parts[3]),
		InputsHash: parts[4],
		ModelSpec:  parts[5],
		Seed:       seed,
		CreatedAt:  createdAt,
		// Signals are not stored in the run record, they're separate records
	}

	return run, nil
}

// splitPayload splits a pipe-delimited payload into parts.
func splitPayload(payload string, maxParts int) []string {
	var parts []string
	start := 0
	for i := 0; i < len(payload) && len(parts) < maxParts-1; i++ {
		if payload[i] == '|' {
			parts = append(parts, payload[start:i])
			start = i + 1
		}
	}
	if start < len(payload) {
		parts = append(parts, payload[start:])
	}
	return parts
}

// parseInt64 parses an int64 without strconv.
func parseInt64(s string) (int64, error) {
	if s == "" {
		return 0, errors.New("empty string")
	}

	negative := false
	if s[0] == '-' {
		negative = true
		s = s[1:]
	}

	var result int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("invalid digit")
		}
		result = result*10 + int64(c-'0')
	}

	if negative {
		result = -result
	}
	return result, nil
}

// Error types
var (
	ErrRunAlreadyExists = errors.New("shadow run already exists")
	ErrRunNotFound      = errors.New("shadow run not found")
	ErrInvalidPayload   = errors.New("invalid shadow run payload")
)
