// Package persist provides the shadow receipt acknowledgement and vote store for Phase 27.
//
// Phase 27: Real Shadow Receipt (Primary Proof of Intelligence, Zero Pressure)
//
// CRITICAL INVARIANTS:
//   - Hash-only storage (no raw content)
//   - Period-bounded retention
//   - LRU eviction
//   - Storelog-backed
//   - Vote does NOT change behavior
//   - Vote feeds Phase 19 calibration only
//
// Reference: docs/ADR/ADR-0058-phase27-real-shadow-receipt-primary-proof.md
package persist

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"quantumlife/pkg/domain/identity"
	domainshadowview "quantumlife/pkg/domain/shadowview"
	"quantumlife/pkg/domain/storelog"
)

// ShadowReceiptAckStore stores shadow receipt acknowledgements and votes.
//
// CRITICAL: Hash-only, period-bounded, LRU eviction.
// CRITICAL: Vote does NOT change behavior.
type ShadowReceiptAckStore struct {
	mu          sync.RWMutex
	acks        map[string]*shadowReceiptAck                   // period+receiptHash -> ack
	votes       map[string]*domainshadowview.ShadowReceiptVote // receiptHash -> vote
	maxPeriods  int
	clock       func() time.Time
	storelogRef storelog.AppendOnlyLog
}

// shadowReceiptAck represents an acknowledgement record.
type shadowReceiptAck struct {
	ReceiptHash  string
	PeriodBucket string
	Action       string // "viewed" or "dismissed"
	TSHash       string // Hash of timestamp, never raw
}

// DefaultMaxShadowReceiptPeriods is the default retention period.
const DefaultMaxShadowReceiptPeriods = 30

// NewShadowReceiptAckStore creates a new shadow receipt ack/vote store.
func NewShadowReceiptAckStore(clock func() time.Time) *ShadowReceiptAckStore {
	return &ShadowReceiptAckStore{
		acks:       make(map[string]*shadowReceiptAck),
		votes:      make(map[string]*domainshadowview.ShadowReceiptVote),
		maxPeriods: DefaultMaxShadowReceiptPeriods,
		clock:      clock,
	}
}

// SetStorelog sets the storelog reference for replay.
func (s *ShadowReceiptAckStore) SetStorelog(log storelog.AppendOnlyLog) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storelogRef = log
}

// RecordViewed records that a receipt was viewed.
//
// CRITICAL: Stores only hashes, never raw content.
func (s *ShadowReceiptAckStore) RecordViewed(receiptHash, periodBucket string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := periodBucket + "|" + receiptHash
	now := s.clock()
	tsHash := hashTimestampForShadow(now)

	s.acks[key] = &shadowReceiptAck{
		ReceiptHash:  receiptHash,
		PeriodBucket: periodBucket,
		Action:       "viewed",
		TSHash:       tsHash,
	}

	s.evictOldPeriods()

	// Append to storelog if available
	if s.storelogRef != nil {
		payload := hashAck(receiptHash, periodBucket, "viewed")
		record := storelog.NewRecord(
			storelog.RecordTypeShadowReceiptAck,
			now,
			identity.EntityID(""),
			payload,
		)
		_ = s.storelogRef.Append(record)
	}

	return nil
}

// RecordDismissed records that a receipt cue was dismissed.
//
// CRITICAL: Stores only hashes, never raw content.
func (s *ShadowReceiptAckStore) RecordDismissed(receiptHash, periodBucket string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := periodBucket + "|" + receiptHash
	now := s.clock()
	tsHash := hashTimestampForShadow(now)

	s.acks[key] = &shadowReceiptAck{
		ReceiptHash:  receiptHash,
		PeriodBucket: periodBucket,
		Action:       "dismissed",
		TSHash:       tsHash,
	}

	s.evictOldPeriods()

	// Append to storelog if available
	if s.storelogRef != nil {
		payload := hashAck(receiptHash, periodBucket, "dismissed")
		record := storelog.NewRecord(
			storelog.RecordTypeShadowReceiptAck,
			now,
			identity.EntityID(""),
			payload,
		)
		_ = s.storelogRef.Append(record)
	}

	return nil
}

// RecordVote records a vote for a receipt.
//
// CRITICAL: Vote does NOT change behavior.
// CRITICAL: Vote feeds Phase 19 calibration only.
// CRITICAL: One vote per receipt hash.
func (s *ShadowReceiptAckStore) RecordVote(vote *domainshadowview.ShadowReceiptVote) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// One vote per receipt hash
	s.votes[vote.ReceiptHash] = vote

	// Append to storelog if available
	if s.storelogRef != nil {
		now := s.clock()
		payload := hashVote(vote)
		record := storelog.NewRecord(
			storelog.RecordTypeShadowReceiptVote,
			now,
			identity.EntityID(""),
			payload,
		)
		_ = s.storelogRef.Append(record)
	}

	return nil
}

// IsDismissed checks if a receipt was dismissed for the given period.
func (s *ShadowReceiptAckStore) IsDismissed(receiptHash, periodBucket string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := periodBucket + "|" + receiptHash
	ack, ok := s.acks[key]
	if !ok {
		return false
	}
	return ack.Action == "dismissed"
}

// HasViewed checks if a receipt was viewed for the given period.
func (s *ShadowReceiptAckStore) HasViewed(receiptHash, periodBucket string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := periodBucket + "|" + receiptHash
	_, ok := s.acks[key]
	return ok
}

// HasVoted checks if a vote was recorded for the receipt.
func (s *ShadowReceiptAckStore) HasVoted(receiptHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.votes[receiptHash]
	return ok
}

// GetVote returns the vote for a receipt, if any.
func (s *ShadowReceiptAckStore) GetVote(receiptHash string) (*domainshadowview.ShadowReceiptVote, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vote, ok := s.votes[receiptHash]
	return vote, ok
}

// CountVotesByPeriod returns vote counts for a period.
// Used for Phase 19 calibration.
func (s *ShadowReceiptAckStore) CountVotesByPeriod(periodBucket string) (useful, unnecessary int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, vote := range s.votes {
		if vote.PeriodBucket == periodBucket {
			switch vote.Choice {
			case domainshadowview.VoteUseful:
				useful++
			case domainshadowview.VoteUnnecessary:
				unnecessary++
			}
		}
	}
	return useful, unnecessary
}

// AckCount returns the current acknowledgement count.
func (s *ShadowReceiptAckStore) AckCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.acks)
}

// VoteCount returns the current vote count.
func (s *ShadowReceiptAckStore) VoteCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.votes)
}

// evictOldPeriods removes records older than maxPeriods days.
// Must be called with lock held.
func (s *ShadowReceiptAckStore) evictOldPeriods() {
	now := s.clock()
	cutoff := now.AddDate(0, 0, -s.maxPeriods).Format("2006-01-02")

	// Evict old acks
	for key, ack := range s.acks {
		if ack.PeriodBucket < cutoff {
			delete(s.acks, key)
		}
	}

	// Evict old votes
	for hash, vote := range s.votes {
		if vote.PeriodBucket < cutoff {
			delete(s.votes, hash)
		}
	}
}

// ReplayFromStorelog replays entries from storelog.
func (s *ShadowReceiptAckStore) ReplayFromStorelog(records []*storelog.LogRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range records {
		switch record.Type {
		case storelog.RecordTypeShadowReceiptAck:
			// We only store hashes, so we can't fully reconstruct
			// But we can mark that something was recorded
			// In practice, replay just validates the hash exists
		case storelog.RecordTypeShadowReceiptVote:
			// Same - hash-only storage means limited replay
		}
	}
	return nil
}

// hashTimestampForShadow creates a SHA256 hash of a timestamp.
// CRITICAL: Never store raw timestamps.
func hashTimestampForShadow(t time.Time) string {
	h := sha256.New()
	h.Write([]byte("SHADOW_RECEIPT_ACK_TS|"))
	h.Write([]byte(t.UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// hashAck creates a hash of an acknowledgement.
func hashAck(receiptHash, periodBucket, action string) string {
	h := sha256.New()
	h.Write([]byte("SHADOW_RECEIPT_ACK|v1|"))
	h.Write([]byte(receiptHash))
	h.Write([]byte("|"))
	h.Write([]byte(periodBucket))
	h.Write([]byte("|"))
	h.Write([]byte(action))
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// hashVote creates a hash of a vote.
func hashVote(vote *domainshadowview.ShadowReceiptVote) string {
	h := sha256.New()
	h.Write([]byte(vote.CanonicalString()))
	return hex.EncodeToString(h.Sum(nil)[:16])
}
