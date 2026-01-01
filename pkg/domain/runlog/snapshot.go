// Package runlog provides run snapshot and replay for QuantumLife.
//
// CRITICAL: Run snapshots capture the complete state of a quiet loop run.
// Replay mode verifies that replaying from logs produces identical results.
//
// GUARDRAIL: No goroutines. All operations are synchronous.
// No time.Now() - clock must be injected.
//
// Reference: docs/ADR/ADR-0027-phase12-persistence-replay.md
package runlog

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
)

// RunSnapshot captures the complete state of a quiet loop run.
type RunSnapshot struct {
	// RunID uniquely identifies this run.
	RunID string

	// StartTime is when the run started.
	StartTime time.Time

	// EndTime is when the run completed.
	EndTime time.Time

	// Duration is the total run duration.
	Duration time.Duration

	// CircleID is the circle this run was for (empty = all circles).
	CircleID identity.EntityID

	// EventsIngested is the count of events ingested.
	EventsIngested int

	// InterruptionsCreated is the count of interruptions created.
	InterruptionsCreated int

	// InterruptionsDeduplicated is the count of deduplicated interruptions.
	InterruptionsDeduplicated int

	// DraftsCreated is the count of drafts created.
	DraftsCreated int

	// NeedsYouItems is the count of items in NeedsYou view.
	NeedsYouItems int

	// EventHashes contains hashes of all events in deterministic order.
	EventHashes []string

	// InterruptionHashes contains hashes of all interruptions.
	InterruptionHashes []string

	// DraftHashes contains hashes of all drafts.
	DraftHashes []string

	// NeedsYouHash is the hash of the NeedsYou view snapshot.
	NeedsYouHash string

	// ResultHash is the deterministic hash of the entire run result.
	ResultHash string

	// ConfigHash is the hash of the configuration used.
	ConfigHash string
}

// ComputeResultHash computes the deterministic hash of the run result.
func (s *RunSnapshot) ComputeResultHash() string {
	var b strings.Builder
	b.WriteString("run_snapshot")
	b.WriteString("|id:")
	b.WriteString(s.RunID)
	b.WriteString("|start:")
	b.WriteString(s.StartTime.UTC().Format(time.RFC3339Nano))
	b.WriteString("|end:")
	b.WriteString(s.EndTime.UTC().Format(time.RFC3339Nano))
	b.WriteString("|circle:")
	b.WriteString(string(s.CircleID))
	b.WriteString("|events:")
	b.WriteString(itoa(s.EventsIngested))
	b.WriteString("|interruptions:")
	b.WriteString(itoa(s.InterruptionsCreated))
	b.WriteString("|deduped:")
	b.WriteString(itoa(s.InterruptionsDeduplicated))
	b.WriteString("|drafts:")
	b.WriteString(itoa(s.DraftsCreated))
	b.WriteString("|needs_you:")
	b.WriteString(itoa(s.NeedsYouItems))

	// Include sorted hashes
	for _, h := range s.EventHashes {
		b.WriteString("|event_hash:")
		b.WriteString(h)
	}
	for _, h := range s.InterruptionHashes {
		b.WriteString("|interruption_hash:")
		b.WriteString(h)
	}
	for _, h := range s.DraftHashes {
		b.WriteString("|draft_hash:")
		b.WriteString(h)
	}

	b.WriteString("|needs_you_hash:")
	b.WriteString(s.NeedsYouHash)
	b.WriteString("|config_hash:")
	b.WriteString(s.ConfigHash)

	hash := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(hash[:])
}

// Validate checks that the snapshot is valid.
func (s *RunSnapshot) Validate() error {
	if s.RunID == "" {
		return errors.New("run ID is required")
	}
	if s.StartTime.IsZero() {
		return errors.New("start time is required")
	}
	if s.ResultHash == "" {
		return errors.New("result hash is required")
	}

	// Verify hash matches
	computed := s.ComputeResultHash()
	if computed != s.ResultHash {
		return ErrHashMismatch
	}

	return nil
}

// ToCanonicalLine converts the snapshot to a canonical line format.
func (s *RunSnapshot) ToCanonicalLine() string {
	var b strings.Builder
	b.WriteString("RUN_SNAPSHOT|v1|")
	b.WriteString(s.StartTime.UTC().Format(time.RFC3339Nano))
	b.WriteString("|")
	b.WriteString(s.ResultHash)
	b.WriteString("|")
	b.WriteString(string(s.CircleID))
	b.WriteString("|")
	b.WriteString(s.toPayload())
	return b.String()
}

// toPayload creates the canonical payload.
func (s *RunSnapshot) toPayload() string {
	var b strings.Builder
	b.WriteString("run_id:")
	b.WriteString(s.RunID)
	b.WriteString("|end:")
	b.WriteString(s.EndTime.UTC().Format(time.RFC3339Nano))
	b.WriteString("|duration:")
	b.WriteString(s.Duration.String())
	b.WriteString("|events:")
	b.WriteString(itoa(s.EventsIngested))
	b.WriteString("|interruptions:")
	b.WriteString(itoa(s.InterruptionsCreated))
	b.WriteString("|deduped:")
	b.WriteString(itoa(s.InterruptionsDeduplicated))
	b.WriteString("|drafts:")
	b.WriteString(itoa(s.DraftsCreated))
	b.WriteString("|needs_you:")
	b.WriteString(itoa(s.NeedsYouItems))
	b.WriteString("|needs_you_hash:")
	b.WriteString(s.NeedsYouHash)
	b.WriteString("|config_hash:")
	b.WriteString(s.ConfigHash)
	b.WriteString("|event_hashes:")
	b.WriteString(strings.Join(s.EventHashes, ","))
	b.WriteString("|interruption_hashes:")
	b.WriteString(strings.Join(s.InterruptionHashes, ","))
	b.WriteString("|draft_hashes:")
	b.WriteString(strings.Join(s.DraftHashes, ","))
	return b.String()
}

// Errors.
var (
	ErrHashMismatch    = errors.New("hash mismatch")
	ErrRunNotFound     = errors.New("run not found")
	ErrReplayFailed    = errors.New("replay verification failed")
	ErrInvalidSnapshot = errors.New("invalid snapshot")
)

// ReplayResult contains the result of a replay verification.
type ReplayResult struct {
	// Success indicates if the replay matched the original.
	Success bool

	// OriginalHash is the hash from the original run.
	OriginalHash string

	// ReplayHash is the hash from the replay.
	ReplayHash string

	// Differences lists any differences found.
	Differences []string

	// ReplaySnapshot is the snapshot from the replay.
	ReplaySnapshot *RunSnapshot
}

// VerifyReplay compares an original snapshot with a replay snapshot.
func VerifyReplay(original, replay *RunSnapshot) *ReplayResult {
	result := &ReplayResult{
		OriginalHash:   original.ResultHash,
		ReplayHash:     replay.ResultHash,
		ReplaySnapshot: replay,
		Differences:    make([]string, 0),
	}

	// Check if hashes match
	if original.ResultHash == replay.ResultHash {
		result.Success = true
		return result
	}

	// Find differences
	if original.EventsIngested != replay.EventsIngested {
		result.Differences = append(result.Differences,
			"events_ingested: "+itoa(original.EventsIngested)+" vs "+itoa(replay.EventsIngested))
	}
	if original.InterruptionsCreated != replay.InterruptionsCreated {
		result.Differences = append(result.Differences,
			"interruptions_created: "+itoa(original.InterruptionsCreated)+" vs "+itoa(replay.InterruptionsCreated))
	}
	if original.DraftsCreated != replay.DraftsCreated {
		result.Differences = append(result.Differences,
			"drafts_created: "+itoa(original.DraftsCreated)+" vs "+itoa(replay.DraftsCreated))
	}
	if original.NeedsYouHash != replay.NeedsYouHash {
		result.Differences = append(result.Differences,
			"needs_you_hash: "+original.NeedsYouHash+" vs "+replay.NeedsYouHash)
	}

	// Check event hashes
	if len(original.EventHashes) != len(replay.EventHashes) {
		result.Differences = append(result.Differences,
			"event_hash_count: "+itoa(len(original.EventHashes))+" vs "+itoa(len(replay.EventHashes)))
	} else {
		for i := range original.EventHashes {
			if original.EventHashes[i] != replay.EventHashes[i] {
				result.Differences = append(result.Differences,
					"event_hash["+itoa(i)+"]: "+original.EventHashes[i]+" vs "+replay.EventHashes[i])
				break // Only report first difference
			}
		}
	}

	return result
}

// RunStore provides storage for run snapshots.
type RunStore interface {
	// Store saves a run snapshot.
	Store(snapshot *RunSnapshot) error

	// Get retrieves a run snapshot by ID.
	Get(runID string) (*RunSnapshot, error)

	// List returns all run snapshots in chronological order.
	List() ([]*RunSnapshot, error)

	// ListByCircle returns run snapshots for a specific circle.
	ListByCircle(circleID identity.EntityID) ([]*RunSnapshot, error)

	// Count returns the total number of run snapshots.
	Count() int
}

// ComputeRunID generates a deterministic run ID from start time and config.
func ComputeRunID(startTime time.Time, configHash string) string {
	data := startTime.UTC().Format(time.RFC3339Nano) + "|" + configHash
	hash := sha256.Sum256([]byte(data))
	return "run-" + hex.EncodeToString(hash[:])[:16]
}

// NewRunSnapshot creates a new run snapshot with computed hash.
func NewRunSnapshot(
	runID string,
	startTime, endTime time.Time,
	circleID identity.EntityID,
	configHash string,
) *RunSnapshot {
	s := &RunSnapshot{
		RunID:      runID,
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   endTime.Sub(startTime),
		CircleID:   circleID,
		ConfigHash: configHash,
	}
	return s
}

// FinalizeSnapshot completes the snapshot with hashes and computes result hash.
func (s *RunSnapshot) FinalizeSnapshot() {
	// Sort all hashes for determinism
	sort.Strings(s.EventHashes)
	sort.Strings(s.InterruptionHashes)
	sort.Strings(s.DraftHashes)

	// Compute result hash
	s.ResultHash = s.ComputeResultHash()
}

// itoa converts int to string without strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
