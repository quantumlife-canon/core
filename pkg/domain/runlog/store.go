package runlog

import (
	"sort"
	"strings"
	"sync"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/storelog"
)

// RunRecordType is the record type for run snapshots in the log.
const RunRecordType = "RUN"

// FileRunStore implements RunStore with file-backed persistence.
type FileRunStore struct {
	mu        sync.RWMutex
	log       storelog.AppendOnlyLog
	snapshots map[string]*RunSnapshot
	byCircle  map[identity.EntityID][]*RunSnapshot
}

// NewFileRunStore creates a new file-backed run store.
func NewFileRunStore(log storelog.AppendOnlyLog) (*FileRunStore, error) {
	store := &FileRunStore{
		log:       log,
		snapshots: make(map[string]*RunSnapshot),
		byCircle:  make(map[identity.EntityID][]*RunSnapshot),
	}

	// Replay existing records
	if err := store.replay(); err != nil {
		return nil, err
	}

	return store, nil
}

// replay loads run snapshots from the log.
func (s *FileRunStore) replay() error {
	records, err := s.log.ListByType(RunRecordType)
	if err != nil {
		return err
	}

	for _, record := range records {
		snapshot, err := parseRunPayload(record.Payload)
		if err != nil {
			continue // Skip corrupted records
		}
		snapshot.StartTime = record.Timestamp
		s.index(snapshot)
	}

	return nil
}

// index adds a snapshot to all indexes.
func (s *FileRunStore) index(snapshot *RunSnapshot) {
	s.snapshots[snapshot.RunID] = snapshot
	s.byCircle[snapshot.CircleID] = append(s.byCircle[snapshot.CircleID], snapshot)
}

// Store saves a run snapshot.
func (s *FileRunStore) Store(snapshot *RunSnapshot) error {
	if err := snapshot.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Create log record
	payload := formatRunPayload(snapshot)
	record := storelog.NewRecord(
		RunRecordType,
		snapshot.StartTime,
		snapshot.CircleID,
		payload,
	)

	// Append to log
	if err := s.log.Append(record); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	s.index(snapshot)
	return nil
}

// Get retrieves a run snapshot by ID.
func (s *FileRunStore) Get(runID string) (*RunSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, exists := s.snapshots[runID]
	if !exists {
		return nil, ErrRunNotFound
	}
	return snapshot, nil
}

// List returns all run snapshots in chronological order.
func (s *FileRunStore) List() ([]*RunSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*RunSnapshot, 0, len(s.snapshots))
	for _, snapshot := range s.snapshots {
		result = append(result, snapshot)
	}

	// Sort by start time
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartTime.Before(result[j].StartTime)
	})

	return result, nil
}

// ListByCircle returns run snapshots for a specific circle.
func (s *FileRunStore) ListByCircle(circleID identity.EntityID) ([]*RunSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots := s.byCircle[circleID]
	result := make([]*RunSnapshot, len(snapshots))
	copy(result, snapshots)

	// Sort by start time
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartTime.Before(result[j].StartTime)
	})

	return result, nil
}

// Count returns the total number of run snapshots.
func (s *FileRunStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.snapshots)
}

// InMemoryRunStore implements RunStore with in-memory storage.
type InMemoryRunStore struct {
	mu        sync.RWMutex
	snapshots map[string]*RunSnapshot
	byCircle  map[identity.EntityID][]*RunSnapshot
}

// NewInMemoryRunStore creates a new in-memory run store.
func NewInMemoryRunStore() *InMemoryRunStore {
	return &InMemoryRunStore{
		snapshots: make(map[string]*RunSnapshot),
		byCircle:  make(map[identity.EntityID][]*RunSnapshot),
	}
}

// Store saves a run snapshot.
func (s *InMemoryRunStore) Store(snapshot *RunSnapshot) error {
	if err := snapshot.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshots[snapshot.RunID] = snapshot
	s.byCircle[snapshot.CircleID] = append(s.byCircle[snapshot.CircleID], snapshot)
	return nil
}

// Get retrieves a run snapshot by ID.
func (s *InMemoryRunStore) Get(runID string) (*RunSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, exists := s.snapshots[runID]
	if !exists {
		return nil, ErrRunNotFound
	}
	return snapshot, nil
}

// List returns all run snapshots in chronological order.
func (s *InMemoryRunStore) List() ([]*RunSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*RunSnapshot, 0, len(s.snapshots))
	for _, snapshot := range s.snapshots {
		result = append(result, snapshot)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartTime.Before(result[j].StartTime)
	})

	return result, nil
}

// ListByCircle returns run snapshots for a specific circle.
func (s *InMemoryRunStore) ListByCircle(circleID identity.EntityID) ([]*RunSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots := s.byCircle[circleID]
	result := make([]*RunSnapshot, len(snapshots))
	copy(result, snapshots)

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartTime.Before(result[j].StartTime)
	})

	return result, nil
}

// Count returns the total number of run snapshots.
func (s *InMemoryRunStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.snapshots)
}

// Clear removes all snapshots.
func (s *InMemoryRunStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = make(map[string]*RunSnapshot)
	s.byCircle = make(map[identity.EntityID][]*RunSnapshot)
}

// formatRunPayload creates a canonical payload for a run snapshot.
func formatRunPayload(s *RunSnapshot) string {
	var b strings.Builder
	b.WriteString("run")
	b.WriteString("|id:")
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
	b.WriteString("|result_hash:")
	b.WriteString(s.ResultHash)
	return b.String()
}

// parseRunPayload parses a canonical payload into a run snapshot.
func parseRunPayload(payload string) (*RunSnapshot, error) {
	s := &RunSnapshot{}

	parts := strings.Split(payload, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "id:") {
			s.RunID = part[3:]
		} else if strings.HasPrefix(part, "end:") {
			t, _ := time.Parse(time.RFC3339Nano, part[4:])
			s.EndTime = t
		} else if strings.HasPrefix(part, "duration:") {
			d, _ := time.ParseDuration(part[9:])
			s.Duration = d
		} else if strings.HasPrefix(part, "events:") {
			s.EventsIngested = atoi(part[7:])
		} else if strings.HasPrefix(part, "interruptions:") {
			s.InterruptionsCreated = atoi(part[14:])
		} else if strings.HasPrefix(part, "deduped:") {
			s.InterruptionsDeduplicated = atoi(part[8:])
		} else if strings.HasPrefix(part, "drafts:") {
			s.DraftsCreated = atoi(part[7:])
		} else if strings.HasPrefix(part, "needs_you:") {
			s.NeedsYouItems = atoi(part[10:])
		} else if strings.HasPrefix(part, "needs_you_hash:") {
			s.NeedsYouHash = part[15:]
		} else if strings.HasPrefix(part, "config_hash:") {
			s.ConfigHash = part[12:]
		} else if strings.HasPrefix(part, "result_hash:") {
			s.ResultHash = part[12:]
		}
	}

	return s, nil
}

// atoi converts string to int without strconv.
func atoi(s string) int {
	n := 0
	negative := false
	for i, c := range s {
		if i == 0 && c == '-' {
			negative = true
			continue
		}
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	if negative {
		n = -n
	}
	return n
}

// Verify interface compliance.
var _ RunStore = (*FileRunStore)(nil)
var _ RunStore = (*InMemoryRunStore)(nil)
