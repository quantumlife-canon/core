// Package persist provides storage for rule pack records.
//
// Phase 19.6: Rule Pack Export (Promotion Pipeline)
//
// CRITICAL INVARIANTS:
//   - Append-only storage
//   - Replayable (Phase 12 compliant)
//   - No goroutines, no I/O except storelog
//   - RulePack does NOT affect behavior
//
// Reference: docs/ADR/ADR-0047-phase19-6-rulepack-export.md
package persist

import (
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/rulepack"
	"quantumlife/pkg/domain/shadowgate"
	"quantumlife/pkg/domain/shadowllm"
	"quantumlife/pkg/domain/storelog"
)

// =============================================================================
// Record Types for Storelog
// =============================================================================

// Record type constants for storelog.
const (
	RecordTypeRulePackExported = "RULEPACK_EXPORTED"
	RecordTypeRulePackAck      = "RULEPACK_ACK"
)

// =============================================================================
// RulePack Store
// =============================================================================

// RulePackStore stores rule packs and their acknowledgments.
//
// CRITICAL: Append-only. Does NOT affect any runtime behavior.
type RulePackStore struct {
	mu      sync.RWMutex
	nowFunc func() time.Time

	// Packs indexed by ID
	packs map[string]*rulepack.RulePack

	// Packs by period for lookup
	packsByPeriod map[string][]string // period -> []packID

	// Acks indexed by ID
	acks map[string]*rulepack.PackAck

	// Acks by pack ID
	acksByPack map[string][]string // packID -> []ackID
}

// NewRulePackStore creates a new rule pack store.
// nowFunc is used for clock injection (deterministic testing).
func NewRulePackStore(nowFunc func() time.Time) *RulePackStore {
	return &RulePackStore{
		nowFunc:       nowFunc,
		packs:         make(map[string]*rulepack.RulePack),
		packsByPeriod: make(map[string][]string),
		acks:          make(map[string]*rulepack.PackAck),
		acksByPack:    make(map[string][]string),
	}
}

// =============================================================================
// Pack Operations
// =============================================================================

// AppendPack stores a rule pack.
// Returns error if a pack with the same ID already exists.
func (s *RulePackStore) AppendPack(pack *rulepack.RulePack) error {
	if pack == nil {
		return errors.New("nil pack")
	}
	if err := pack.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Compute ID and hash if not set
	if pack.PackID == "" {
		pack.PackID = pack.ComputeID()
	}
	if pack.PackHash == "" {
		pack.PackHash = pack.ComputeHash()
	}

	if _, exists := s.packs[pack.PackID]; exists {
		return storelog.ErrRecordExists
	}

	s.packs[pack.PackID] = pack
	s.packsByPeriod[pack.PeriodKey] = append(
		s.packsByPeriod[pack.PeriodKey],
		pack.PackID,
	)

	return nil
}

// GetPack retrieves a pack by ID.
func (s *RulePackStore) GetPack(packID string) (*rulepack.RulePack, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pack, ok := s.packs[packID]
	return pack, ok
}

// ListPacks returns all packs for a period in deterministic order.
func (s *RulePackStore) ListPacks(periodKey string) []rulepack.RulePack {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.packsByPeriod[periodKey]
	if len(ids) == 0 {
		return nil
	}

	// Collect packs
	packs := make([]rulepack.RulePack, 0, len(ids))
	for _, id := range ids {
		if pack, ok := s.packs[id]; ok {
			packs = append(packs, *pack)
		}
	}

	// Sort for determinism (most recent first)
	rulepack.SortRulePacks(packs)

	return packs
}

// ListAllPacks returns all packs sorted by period (most recent first).
func (s *RulePackStore) ListAllPacks() []rulepack.RulePack {
	s.mu.RLock()
	defer s.mu.RUnlock()

	packs := make([]rulepack.RulePack, 0, len(s.packs))
	for _, pack := range s.packs {
		packs = append(packs, *pack)
	}

	// Sort for determinism (most recent first)
	rulepack.SortRulePacks(packs)

	return packs
}

// GetPackCount returns the total number of packs.
func (s *RulePackStore) GetPackCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.packs)
}

// =============================================================================
// Ack Operations
// =============================================================================

// AckPack records an acknowledgment for a pack.
func (s *RulePackStore) AckPack(packID string, ackKind rulepack.AckKind) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify pack exists
	pack, ok := s.packs[packID]
	if !ok {
		return errors.New("pack not found")
	}

	// Create ack
	now := s.nowFunc()
	ack := &rulepack.PackAck{
		PackID:        packID,
		PackHash:      pack.PackHash,
		AckKind:       ackKind,
		CreatedBucket: rulepack.FiveMinuteBucket(now),
		CreatedAt:     now,
	}
	ack.AckID = ack.ComputeID()
	ack.AckHash = ack.ComputeHash()

	if err := ack.Validate(); err != nil {
		return err
	}

	s.acks[ack.AckID] = ack
	s.acksByPack[packID] = append(s.acksByPack[packID], ack.AckID)

	return nil
}

// GetAcksForPack returns all acks for a pack.
func (s *RulePackStore) GetAcksForPack(packID string) []rulepack.PackAck {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.acksByPack[packID]
	if len(ids) == 0 {
		return nil
	}

	acks := make([]rulepack.PackAck, 0, len(ids))
	for _, id := range ids {
		if ack, ok := s.acks[id]; ok {
			acks = append(acks, *ack)
		}
	}

	// Sort by created bucket desc
	sort.Slice(acks, func(i, j int) bool {
		return acks[i].CreatedBucket > acks[j].CreatedBucket
	})

	return acks
}

// HasAckKind checks if a pack has a specific ack kind.
func (s *RulePackStore) HasAckKind(packID string, ackKind rulepack.AckKind) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.acksByPack[packID]
	for _, id := range ids {
		if ack, ok := s.acks[id]; ok && ack.AckKind == ackKind {
			return true
		}
	}
	return false
}

// =============================================================================
// Storelog Integration (Replay Support)
// =============================================================================

// packRecord is the JSON structure for persisting packs.
type packRecord struct {
	PackID              string         `json:"pack_id"`
	PackHash            string         `json:"pack_hash"`
	PeriodKey           string         `json:"period_key"`
	CircleID            string         `json:"circle_id"`
	CreatedAtBucket     string         `json:"created_at_bucket"`
	ExportFormatVersion string         `json:"export_format_version"`
	ChangeCount         int            `json:"change_count"`
	Changes             []changeRecord `json:"changes"`
	CreatedAt           string         `json:"created_at"`
}

// changeRecord is the JSON structure for persisting changes.
type changeRecord struct {
	ChangeID             string `json:"change_id"`
	CandidateHash        string `json:"candidate_hash"`
	IntentHash           string `json:"intent_hash"`
	CircleID             string `json:"circle_id"`
	ChangeKind           string `json:"change_kind"`
	TargetScope          string `json:"target_scope"`
	TargetHash           string `json:"target_hash"`
	Category             string `json:"category"`
	SuggestedDelta       string `json:"suggested_delta"`
	UsefulnessBucket     string `json:"usefulness_bucket"`
	VoteConfidenceBucket string `json:"vote_confidence_bucket"`
	NoveltyBucket        string `json:"novelty_bucket"`
	AgreementBucket      string `json:"agreement_bucket"`
}

// ackRecord is the JSON structure for persisting acks.
type ackRecord struct {
	AckID         string `json:"ack_id"`
	AckHash       string `json:"ack_hash"`
	PackID        string `json:"pack_id"`
	PackHash      string `json:"pack_hash"`
	AckKind       string `json:"ack_kind"`
	CreatedBucket string `json:"created_bucket"`
	CreatedAt     string `json:"created_at"`
}

// PackToStorelogRecord converts a pack to a storelog record.
func (s *RulePackStore) PackToStorelogRecord(pack *rulepack.RulePack) *storelog.LogRecord {
	changes := make([]changeRecord, len(pack.Changes))
	for i, c := range pack.Changes {
		changes[i] = changeRecord{
			ChangeID:             c.ChangeID,
			CandidateHash:        c.CandidateHash,
			IntentHash:           c.IntentHash,
			CircleID:             string(c.CircleID),
			ChangeKind:           string(c.ChangeKind),
			TargetScope:          string(c.TargetScope),
			TargetHash:           c.TargetHash,
			Category:             string(c.Category),
			SuggestedDelta:       string(c.SuggestedDelta),
			UsefulnessBucket:     string(c.UsefulnessBucket),
			VoteConfidenceBucket: string(c.VoteConfidenceBucket),
			NoveltyBucket:        string(c.NoveltyBucket),
			AgreementBucket:      string(c.AgreementBucket),
		}
	}

	payload := packRecord{
		PackID:              pack.PackID,
		PackHash:            pack.PackHash,
		PeriodKey:           pack.PeriodKey,
		CircleID:            string(pack.CircleID),
		CreatedAtBucket:     pack.CreatedAtBucket,
		ExportFormatVersion: pack.ExportFormatVersion,
		ChangeCount:         len(pack.Changes),
		Changes:             changes,
		CreatedAt:           pack.CreatedAt.UTC().Format(time.RFC3339),
	}

	data, _ := json.Marshal(payload)

	return &storelog.LogRecord{
		Type:      RecordTypeRulePackExported,
		Version:   "v1",
		Timestamp: pack.CreatedAt,
		Payload:   string(data),
	}
}

// AckToStorelogRecord converts an ack to a storelog record.
func (s *RulePackStore) AckToStorelogRecord(ack *rulepack.PackAck) *storelog.LogRecord {
	payload := ackRecord{
		AckID:         ack.AckID,
		AckHash:       ack.AckHash,
		PackID:        ack.PackID,
		PackHash:      ack.PackHash,
		AckKind:       string(ack.AckKind),
		CreatedBucket: ack.CreatedBucket,
		CreatedAt:     ack.CreatedAt.UTC().Format(time.RFC3339),
	}

	data, _ := json.Marshal(payload)

	return &storelog.LogRecord{
		Type:      RecordTypeRulePackAck,
		Version:   "v1",
		Timestamp: ack.CreatedAt,
		Payload:   string(data),
	}
}

// =============================================================================
// Replay Support
// =============================================================================

// ReplayPackRecord replays a pack record from storelog.
func (s *RulePackStore) ReplayPackRecord(record *storelog.LogRecord) error {
	if record.Type != RecordTypeRulePackExported {
		return errors.New("invalid record type for pack")
	}

	var pr packRecord
	if err := json.Unmarshal([]byte(record.Payload), &pr); err != nil {
		return err
	}

	createdAt, _ := time.Parse(time.RFC3339, pr.CreatedAt)

	changes := make([]rulepack.RuleChange, len(pr.Changes))
	for i, cr := range pr.Changes {
		changes[i] = rulepack.RuleChange{
			ChangeID:             cr.ChangeID,
			CandidateHash:        cr.CandidateHash,
			IntentHash:           cr.IntentHash,
			CircleID:             identity.EntityID(cr.CircleID),
			ChangeKind:           rulepack.ChangeKind(cr.ChangeKind),
			TargetScope:          rulepack.TargetScope(cr.TargetScope),
			TargetHash:           cr.TargetHash,
			Category:             shadowllm.AbstractCategory(cr.Category),
			SuggestedDelta:       rulepack.SuggestedDelta(cr.SuggestedDelta),
			UsefulnessBucket:     shadowgate.UsefulnessBucket(cr.UsefulnessBucket),
			VoteConfidenceBucket: shadowgate.VoteConfidenceBucket(cr.VoteConfidenceBucket),
			NoveltyBucket:        rulepack.NoveltyBucket(cr.NoveltyBucket),
			AgreementBucket:      rulepack.AgreementBucket(cr.AgreementBucket),
		}
	}

	pack := &rulepack.RulePack{
		PackID:              pr.PackID,
		PackHash:            pr.PackHash,
		PeriodKey:           pr.PeriodKey,
		CircleID:            identity.EntityID(pr.CircleID),
		CreatedAtBucket:     pr.CreatedAtBucket,
		ExportFormatVersion: pr.ExportFormatVersion,
		Changes:             changes,
		CreatedAt:           createdAt,
	}

	// Use internal add to avoid duplicate check during replay
	s.mu.Lock()
	defer s.mu.Unlock()

	s.packs[pack.PackID] = pack
	s.packsByPeriod[pack.PeriodKey] = append(s.packsByPeriod[pack.PeriodKey], pack.PackID)

	return nil
}

// ReplayAckRecord replays an ack record from storelog.
func (s *RulePackStore) ReplayAckRecord(record *storelog.LogRecord) error {
	if record.Type != RecordTypeRulePackAck {
		return errors.New("invalid record type for ack")
	}

	var ar ackRecord
	if err := json.Unmarshal([]byte(record.Payload), &ar); err != nil {
		return err
	}

	createdAt, _ := time.Parse(time.RFC3339, ar.CreatedAt)

	ack := &rulepack.PackAck{
		AckID:         ar.AckID,
		AckHash:       ar.AckHash,
		PackID:        ar.PackID,
		PackHash:      ar.PackHash,
		AckKind:       rulepack.AckKind(ar.AckKind),
		CreatedBucket: ar.CreatedBucket,
		CreatedAt:     createdAt,
	}

	// Use internal add to avoid duplicate check during replay
	s.mu.Lock()
	defer s.mu.Unlock()

	s.acks[ack.AckID] = ack
	s.acksByPack[ack.PackID] = append(s.acksByPack[ack.PackID], ack.AckID)

	return nil
}
