package storelog

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"quantumlife/pkg/domain/identity"
)

// FileLog implements AppendOnlyLog with file-based persistence.
// Records are stored one per line in canonical format.
//
// CRITICAL: This implementation uses atomic writes (temp file + rename)
// to ensure data integrity even during crashes.
//
// GUARDRAIL: No goroutines. All operations are synchronous.
type FileLog struct {
	mu sync.RWMutex

	// path is the file path for the log
	path string

	// records is the in-memory cache of all records
	records []*LogRecord

	// hashIndex maps hash to record for O(1) lookup
	hashIndex map[string]*LogRecord

	// typeIndex maps record type to records
	typeIndex map[string][]*LogRecord

	// circleIndex maps circle ID to records
	circleIndex map[identity.EntityID][]*LogRecord

	// dirty indicates if there are unpersisted changes
	dirty bool
}

// NewFileLog creates a new FileLog at the given path.
// If the file exists, it loads existing records.
func NewFileLog(path string) (*FileLog, error) {
	fl := &FileLog{
		path:        path,
		records:     make([]*LogRecord, 0),
		hashIndex:   make(map[string]*LogRecord),
		typeIndex:   make(map[string][]*LogRecord),
		circleIndex: make(map[identity.EntityID][]*LogRecord),
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	// Load existing records if file exists
	if _, err := os.Stat(path); err == nil {
		if err := fl.load(); err != nil {
			return nil, err
		}
	}

	return fl, nil
}

// load reads all records from the file.
func (fl *FileLog) load() error {
	file, err := os.Open(fl.path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		record, err := ParseCanonicalLine(line)
		if err != nil {
			// Log corrupted line but continue
			continue
		}

		fl.addToIndexes(record)
	}

	return scanner.Err()
}

// addToIndexes adds a record to all in-memory indexes.
func (fl *FileLog) addToIndexes(record *LogRecord) {
	fl.records = append(fl.records, record)
	fl.hashIndex[record.Hash] = record
	fl.typeIndex[record.Type] = append(fl.typeIndex[record.Type], record)
	if record.CircleID != "" {
		fl.circleIndex[record.CircleID] = append(fl.circleIndex[record.CircleID], record)
	}
}

// Append adds a new record to the log.
func (fl *FileLog) Append(record *LogRecord) error {
	if err := record.Validate(); err != nil {
		return err
	}

	fl.mu.Lock()
	defer fl.mu.Unlock()

	// Check for duplicates
	if _, exists := fl.hashIndex[record.Hash]; exists {
		return ErrRecordExists
	}

	// Add to indexes
	fl.addToIndexes(record)
	fl.dirty = true

	// Append to file immediately for durability
	return fl.appendToFile(record)
}

// appendToFile appends a single record to the file.
func (fl *FileLog) appendToFile(record *LogRecord) error {
	file, err := os.OpenFile(fl.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	line := record.ToCanonicalLine() + "\n"
	_, err = file.WriteString(line)
	if err != nil {
		return err
	}

	fl.dirty = false
	return file.Sync()
}

// Contains checks if a record with the given hash exists.
func (fl *FileLog) Contains(hash string) bool {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	_, exists := fl.hashIndex[hash]
	return exists
}

// Get retrieves a record by hash.
func (fl *FileLog) Get(hash string) (*LogRecord, error) {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	record, exists := fl.hashIndex[hash]
	if !exists {
		return nil, ErrRecordNotFound
	}
	return record, nil
}

// List returns all records in append order.
func (fl *FileLog) List() ([]*LogRecord, error) {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	result := make([]*LogRecord, len(fl.records))
	copy(result, fl.records)
	return result, nil
}

// ListByType returns all records of a given type.
func (fl *FileLog) ListByType(recordType string) ([]*LogRecord, error) {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	records := fl.typeIndex[recordType]
	result := make([]*LogRecord, len(records))
	copy(result, records)
	return result, nil
}

// ListByCircle returns all records for a given circle.
func (fl *FileLog) ListByCircle(circleID identity.EntityID) ([]*LogRecord, error) {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	records := fl.circleIndex[circleID]
	result := make([]*LogRecord, len(records))
	copy(result, records)
	return result, nil
}

// Count returns the total number of records.
func (fl *FileLog) Count() int {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	return len(fl.records)
}

// Verify checks that all records have valid hashes.
func (fl *FileLog) Verify() error {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	for _, record := range fl.records {
		computed := record.ComputeHash()
		if computed != record.Hash {
			return ErrLogCorrupted
		}
	}
	return nil
}

// Flush ensures all records are persisted to disk.
// Uses atomic write (temp file + rename) for safety.
func (fl *FileLog) Flush() error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if !fl.dirty {
		return nil
	}

	// Write to temp file first
	tmpPath := fl.path + ".tmp." + randomSuffix()
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(file)
	for _, record := range fl.records {
		line := record.ToCanonicalLine() + "\n"
		if _, err := writer.WriteString(line); err != nil {
			file.Close()
			os.Remove(tmpPath)
			return err
		}
	}

	if err := writer.Flush(); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return err
	}

	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return err
	}

	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Atomic rename
	if err := os.Rename(tmpPath, fl.path); err != nil {
		os.Remove(tmpPath)
		return err
	}

	fl.dirty = false
	return nil
}

// randomSuffix generates a random hex suffix for temp files.
func randomSuffix() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// InMemoryLog implements AppendOnlyLog with in-memory storage.
// Useful for testing.
type InMemoryLog struct {
	mu          sync.RWMutex
	records     []*LogRecord
	hashIndex   map[string]*LogRecord
	typeIndex   map[string][]*LogRecord
	circleIndex map[identity.EntityID][]*LogRecord
}

// NewInMemoryLog creates a new in-memory log.
func NewInMemoryLog() *InMemoryLog {
	return &InMemoryLog{
		records:     make([]*LogRecord, 0),
		hashIndex:   make(map[string]*LogRecord),
		typeIndex:   make(map[string][]*LogRecord),
		circleIndex: make(map[identity.EntityID][]*LogRecord),
	}
}

// Append adds a new record to the log.
func (ml *InMemoryLog) Append(record *LogRecord) error {
	if err := record.Validate(); err != nil {
		return err
	}

	ml.mu.Lock()
	defer ml.mu.Unlock()

	if _, exists := ml.hashIndex[record.Hash]; exists {
		return ErrRecordExists
	}

	ml.records = append(ml.records, record)
	ml.hashIndex[record.Hash] = record
	ml.typeIndex[record.Type] = append(ml.typeIndex[record.Type], record)
	if record.CircleID != "" {
		ml.circleIndex[record.CircleID] = append(ml.circleIndex[record.CircleID], record)
	}

	return nil
}

// Contains checks if a record with the given hash exists.
func (ml *InMemoryLog) Contains(hash string) bool {
	ml.mu.RLock()
	defer ml.mu.RUnlock()
	_, exists := ml.hashIndex[hash]
	return exists
}

// Get retrieves a record by hash.
func (ml *InMemoryLog) Get(hash string) (*LogRecord, error) {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	record, exists := ml.hashIndex[hash]
	if !exists {
		return nil, ErrRecordNotFound
	}
	return record, nil
}

// List returns all records in append order.
func (ml *InMemoryLog) List() ([]*LogRecord, error) {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	result := make([]*LogRecord, len(ml.records))
	copy(result, ml.records)
	return result, nil
}

// ListByType returns all records of a given type.
func (ml *InMemoryLog) ListByType(recordType string) ([]*LogRecord, error) {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	records := ml.typeIndex[recordType]
	result := make([]*LogRecord, len(records))
	copy(result, records)
	return result, nil
}

// ListByCircle returns all records for a given circle.
func (ml *InMemoryLog) ListByCircle(circleID identity.EntityID) ([]*LogRecord, error) {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	records := ml.circleIndex[circleID]
	result := make([]*LogRecord, len(records))
	copy(result, records)
	return result, nil
}

// Count returns the total number of records.
func (ml *InMemoryLog) Count() int {
	ml.mu.RLock()
	defer ml.mu.RUnlock()
	return len(ml.records)
}

// Verify checks that all records have valid hashes.
func (ml *InMemoryLog) Verify() error {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	for _, record := range ml.records {
		computed := record.ComputeHash()
		if computed != record.Hash {
			return ErrLogCorrupted
		}
	}
	return nil
}

// Flush is a no-op for in-memory log.
func (ml *InMemoryLog) Flush() error {
	return nil
}

// Clear removes all records (for testing).
func (ml *InMemoryLog) Clear() {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	ml.records = make([]*LogRecord, 0)
	ml.hashIndex = make(map[string]*LogRecord)
	ml.typeIndex = make(map[string][]*LogRecord)
	ml.circleIndex = make(map[identity.EntityID][]*LogRecord)
}

// Hashes returns all record hashes in sorted order (for deterministic comparison).
func (ml *InMemoryLog) Hashes() []string {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	hashes := make([]string, len(ml.records))
	for i, r := range ml.records {
		hashes[i] = r.Hash
	}
	sort.Strings(hashes)
	return hashes
}
