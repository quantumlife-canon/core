// Package persist provides storage for shadow gating records.
//
// Phase 19.5: Shadow Gating + Promotion Candidates (NO behavior change)
//
// CRITICAL INVARIANTS:
//   - Append-only storage
//   - Hash-only persistence (no raw content with identifiers)
//   - Replayable (Phase 12 compliant)
//   - No goroutines, no I/O except storelog
//   - Shadow does NOT affect behavior
//
// Reference: docs/ADR/ADR-0046-phase19-5-shadow-gating-and-promotion-candidates.md
package persist

import (
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowgate"
	"quantumlife/pkg/domain/shadowllm"
	"quantumlife/pkg/domain/storelog"
)

// =============================================================================
// Record Types for Storelog
// =============================================================================

// Record type constants for storelog.
const (
	RecordTypeShadowCandidate       = "SHADOW_CANDIDATE"
	RecordTypeShadowPromotionIntent = "SHADOW_PROMOTION_INTENT"
)

// =============================================================================
// Shadow Gate Store
// =============================================================================

// ShadowGateStore stores shadow gating candidates and promotion intents.
//
// CRITICAL: Append-only. Does NOT affect any runtime behavior.
type ShadowGateStore struct {
	mu      sync.RWMutex
	nowFunc func() time.Time

	// Candidates indexed by ID
	candidates map[string]*shadowgate.Candidate

	// Candidates by period for lookup
	candidatesByPeriod map[string][]string // period -> []candidateID

	// Promotion intents indexed by IntentID
	intents map[string]*shadowgate.PromotionIntent

	// Intents by period
	intentsByPeriod map[string][]string // period -> []intentID

	// Intents by candidate for quick lookup
	intentsByCandidate map[string][]string // candidateID -> []intentID
}

// NewShadowGateStore creates a new shadow gate store.
// nowFunc is used for clock injection (deterministic testing).
func NewShadowGateStore(nowFunc func() time.Time) *ShadowGateStore {
	return &ShadowGateStore{
		nowFunc:            nowFunc,
		candidates:         make(map[string]*shadowgate.Candidate),
		candidatesByPeriod: make(map[string][]string),
		intents:            make(map[string]*shadowgate.PromotionIntent),
		intentsByPeriod:    make(map[string][]string),
		intentsByCandidate: make(map[string][]string),
	}
}

// =============================================================================
// Candidate Operations
// =============================================================================

// AppendCandidate stores a single candidate.
// If a candidate with the same ID exists, it is updated (upsert behavior for refresh).
func (s *ShadowGateStore) AppendCandidate(candidate *shadowgate.Candidate) error {
	if candidate == nil {
		return errors.New("nil candidate")
	}
	if err := candidate.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Compute ID and hash if not set
	if candidate.ID == "" {
		candidate.ID = candidate.ComputeID()
	}
	if candidate.Hash == "" {
		candidate.Hash = candidate.ComputeHash()
	}

	// Check if already exists
	existingID := ""
	for id, c := range s.candidates {
		if c.ID == candidate.ID && c.PeriodKey == candidate.PeriodKey {
			existingID = id
			break
		}
	}

	if existingID != "" {
		// Update existing
		s.candidates[existingID] = candidate
	} else {
		// Add new
		s.candidates[candidate.ID] = candidate
		s.candidatesByPeriod[candidate.PeriodKey] = append(
			s.candidatesByPeriod[candidate.PeriodKey],
			candidate.ID,
		)
	}

	return nil
}

// AppendCandidates stores multiple candidates for a period.
// Clears existing candidates for the period first (refresh semantics).
func (s *ShadowGateStore) AppendCandidates(periodKey string, candidates []shadowgate.Candidate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove old candidates for this period
	oldIDs := s.candidatesByPeriod[periodKey]
	for _, id := range oldIDs {
		delete(s.candidates, id)
	}
	s.candidatesByPeriod[periodKey] = nil

	// Add new candidates
	for i := range candidates {
		c := &candidates[i]

		if err := c.Validate(); err != nil {
			return err
		}

		// Compute ID and hash if not set
		if c.ID == "" {
			c.ID = c.ComputeID()
		}
		if c.Hash == "" {
			c.Hash = c.ComputeHash()
		}

		s.candidates[c.ID] = c
		s.candidatesByPeriod[periodKey] = append(s.candidatesByPeriod[periodKey], c.ID)
	}

	return nil
}

// GetCandidate retrieves a candidate by ID.
func (s *ShadowGateStore) GetCandidate(candidateID string) (*shadowgate.Candidate, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.candidates[candidateID]
	return c, ok
}

// GetCandidates returns all candidates for a period in deterministic order.
func (s *ShadowGateStore) GetCandidates(periodKey string) []shadowgate.Candidate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.candidatesByPeriod[periodKey]
	if len(ids) == 0 {
		return nil
	}

	// Collect candidates
	candidates := make([]shadowgate.Candidate, 0, len(ids))
	for _, id := range ids {
		if c, ok := s.candidates[id]; ok {
			candidates = append(candidates, *c)
		}
	}

	// Sort for determinism
	shadowgate.SortCandidates(candidates)

	return candidates
}

// GetCandidateCount returns the number of candidates for a period.
func (s *ShadowGateStore) GetCandidateCount(periodKey string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.candidatesByPeriod[periodKey])
}

// =============================================================================
// Promotion Intent Operations
// =============================================================================

// AppendPromotionIntent stores a promotion intent.
// Returns error if an intent with the same ID already exists.
func (s *ShadowGateStore) AppendPromotionIntent(intent *shadowgate.PromotionIntent) error {
	if intent == nil {
		return errors.New("nil promotion intent")
	}
	if err := intent.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Compute ID and hash if not set
	if intent.IntentID == "" {
		intent.IntentID = intent.ComputeID()
	}
	if intent.IntentHash == "" {
		intent.IntentHash = intent.ComputeHash()
	}

	if _, exists := s.intents[intent.IntentID]; exists {
		return storelog.ErrRecordExists
	}

	s.intents[intent.IntentID] = intent
	s.intentsByPeriod[intent.PeriodKey] = append(
		s.intentsByPeriod[intent.PeriodKey],
		intent.IntentID,
	)
	s.intentsByCandidate[intent.CandidateID] = append(
		s.intentsByCandidate[intent.CandidateID],
		intent.IntentID,
	)

	return nil
}

// GetPromotionIntent retrieves an intent by ID.
func (s *ShadowGateStore) GetPromotionIntent(intentID string) (*shadowgate.PromotionIntent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	intent, ok := s.intents[intentID]
	return intent, ok
}

// GetPromotionIntents returns all intents for a period in deterministic order.
func (s *ShadowGateStore) GetPromotionIntents(periodKey string) []shadowgate.PromotionIntent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.intentsByPeriod[periodKey]
	if len(ids) == 0 {
		return nil
	}

	// Collect intents
	intents := make([]shadowgate.PromotionIntent, 0, len(ids))
	for _, id := range ids {
		if intent, ok := s.intents[id]; ok {
			intents = append(intents, *intent)
		}
	}

	// Sort for determinism
	shadowgate.SortPromotionIntents(intents)

	return intents
}

// GetIntentsForCandidate returns all intents for a specific candidate.
func (s *ShadowGateStore) GetIntentsForCandidate(candidateID string) []shadowgate.PromotionIntent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.intentsByCandidate[candidateID]
	if len(ids) == 0 {
		return nil
	}

	intents := make([]shadowgate.PromotionIntent, 0, len(ids))
	for _, id := range ids {
		if intent, ok := s.intents[id]; ok {
			intents = append(intents, *intent)
		}
	}

	sort.Slice(intents, func(i, j int) bool {
		return intents[i].IntentHash < intents[j].IntentHash
	})

	return intents
}

// HasIntentForCandidate checks if any intent exists for a candidate.
func (s *ShadowGateStore) HasIntentForCandidate(candidateID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.intentsByCandidate[candidateID]) > 0
}

// =============================================================================
// Storelog Integration (Replay Support)
// =============================================================================

// candidateRecord is the JSON structure for persisting candidates.
type candidateRecord struct {
	ID                   string `json:"id"`
	Hash                 string `json:"hash"`
	PeriodKey            string `json:"period_key"`
	CircleID             string `json:"circle_id"`
	Origin               string `json:"origin"`
	Category             string `json:"category"`
	HorizonBucket        string `json:"horizon_bucket"`
	MagnitudeBucket      string `json:"magnitude_bucket"`
	WhyGeneric           string `json:"why_generic"`
	UsefulnessPct        int    `json:"usefulness_pct"`
	UsefulnessBucket     string `json:"usefulness_bucket"`
	VoteConfidenceBucket string `json:"vote_confidence_bucket"`
	VotesUseful          int    `json:"votes_useful"`
	VotesUnnecessary     int    `json:"votes_unnecessary"`
	FirstSeenBucket      string `json:"first_seen_bucket"`
	LastSeenBucket       string `json:"last_seen_bucket"`
	CreatedAt            string `json:"created_at"`
}

// intentRecord is the JSON structure for persisting intents.
type intentRecord struct {
	IntentID      string `json:"intent_id"`
	IntentHash    string `json:"intent_hash"`
	CandidateID   string `json:"candidate_id"`
	CandidateHash string `json:"candidate_hash"`
	PeriodKey     string `json:"period_key"`
	NoteCode      string `json:"note_code"`
	CreatedBucket string `json:"created_bucket"`
	CreatedAt     string `json:"created_at"`
}

// CandidateToStorelogRecord converts a candidate to a storelog record.
func (s *ShadowGateStore) CandidateToStorelogRecord(c *shadowgate.Candidate) *storelog.LogRecord {
	payload := candidateRecord{
		ID:                   c.ID,
		Hash:                 c.Hash,
		PeriodKey:            c.PeriodKey,
		CircleID:             string(c.CircleID),
		Origin:               string(c.Origin),
		Category:             string(c.Category),
		HorizonBucket:        string(c.HorizonBucket),
		MagnitudeBucket:      string(c.MagnitudeBucket),
		WhyGeneric:           c.WhyGeneric,
		UsefulnessPct:        c.UsefulnessPct,
		UsefulnessBucket:     string(c.UsefulnessBucket),
		VoteConfidenceBucket: string(c.VoteConfidenceBucket),
		VotesUseful:          c.VotesUseful,
		VotesUnnecessary:     c.VotesUnnecessary,
		FirstSeenBucket:      c.FirstSeenBucket,
		LastSeenBucket:       c.LastSeenBucket,
		CreatedAt:            c.CreatedAt.UTC().Format(time.RFC3339),
	}

	data, _ := json.Marshal(payload)

	return &storelog.LogRecord{
		Type:      RecordTypeShadowCandidate,
		Version:   "v1",
		Timestamp: c.CreatedAt,
		Payload:   string(data),
	}
}

// IntentToStorelogRecord converts an intent to a storelog record.
func (s *ShadowGateStore) IntentToStorelogRecord(intent *shadowgate.PromotionIntent) *storelog.LogRecord {
	payload := intentRecord{
		IntentID:      intent.IntentID,
		IntentHash:    intent.IntentHash,
		CandidateID:   intent.CandidateID,
		CandidateHash: intent.CandidateHash,
		PeriodKey:     intent.PeriodKey,
		NoteCode:      string(intent.NoteCode),
		CreatedBucket: intent.CreatedBucket,
		CreatedAt:     intent.CreatedAt.UTC().Format(time.RFC3339),
	}

	data, _ := json.Marshal(payload)

	return &storelog.LogRecord{
		Type:      RecordTypeShadowPromotionIntent,
		Version:   "v1",
		Timestamp: intent.CreatedAt,
		Payload:   string(data),
	}
}

// =============================================================================
// Replay Support
// =============================================================================

// ReplayCandidateRecord replays a candidate record from storelog.
func (s *ShadowGateStore) ReplayCandidateRecord(record *storelog.LogRecord) error {
	if record.Type != RecordTypeShadowCandidate {
		return errors.New("invalid record type for candidate")
	}

	var cr candidateRecord
	if err := json.Unmarshal([]byte(record.Payload), &cr); err != nil {
		return err
	}

	createdAt, _ := time.Parse(time.RFC3339, cr.CreatedAt)

	candidate := &shadowgate.Candidate{
		ID:                   cr.ID,
		Hash:                 cr.Hash,
		PeriodKey:            cr.PeriodKey,
		CircleID:             identity.EntityID(cr.CircleID),
		Origin:               shadowgate.CandidateOrigin(cr.Origin),
		Category:             shadowllm.AbstractCategory(cr.Category),
		HorizonBucket:        shadowllm.Horizon(cr.HorizonBucket),
		MagnitudeBucket:      shadowllm.MagnitudeBucket(cr.MagnitudeBucket),
		WhyGeneric:           cr.WhyGeneric,
		UsefulnessPct:        cr.UsefulnessPct,
		UsefulnessBucket:     shadowgate.UsefulnessBucket(cr.UsefulnessBucket),
		VoteConfidenceBucket: shadowgate.VoteConfidenceBucket(cr.VoteConfidenceBucket),
		VotesUseful:          cr.VotesUseful,
		VotesUnnecessary:     cr.VotesUnnecessary,
		FirstSeenBucket:      cr.FirstSeenBucket,
		LastSeenBucket:       cr.LastSeenBucket,
		CreatedAt:            createdAt,
	}

	return s.AppendCandidate(candidate)
}

// ReplayIntentRecord replays an intent record from storelog.
func (s *ShadowGateStore) ReplayIntentRecord(record *storelog.LogRecord) error {
	if record.Type != RecordTypeShadowPromotionIntent {
		return errors.New("invalid record type for intent")
	}

	var ir intentRecord
	if err := json.Unmarshal([]byte(record.Payload), &ir); err != nil {
		return err
	}

	createdAt, _ := time.Parse(time.RFC3339, ir.CreatedAt)

	intent := &shadowgate.PromotionIntent{
		IntentID:      ir.IntentID,
		IntentHash:    ir.IntentHash,
		CandidateID:   ir.CandidateID,
		CandidateHash: ir.CandidateHash,
		PeriodKey:     ir.PeriodKey,
		NoteCode:      shadowgate.NoteCode(ir.NoteCode),
		CreatedBucket: ir.CreatedBucket,
		CreatedAt:     createdAt,
	}

	// Use internal add to avoid duplicate check during replay
	s.mu.Lock()
	defer s.mu.Unlock()

	s.intents[intent.IntentID] = intent
	s.intentsByPeriod[intent.PeriodKey] = append(
		s.intentsByPeriod[intent.PeriodKey],
		intent.IntentID,
	)
	s.intentsByCandidate[intent.CandidateID] = append(
		s.intentsByCandidate[intent.CandidateID],
		intent.IntentID,
	)

	return nil
}
