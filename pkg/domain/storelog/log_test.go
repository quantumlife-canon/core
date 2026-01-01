package storelog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"quantumlife/pkg/domain/identity"
)

func TestLogRecord_ComputeHash(t *testing.T) {
	record := &LogRecord{
		Type:    RecordTypeEvent,
		Version: SchemaVersion,
		Payload: "email|google|msg-123|test@example.com",
	}

	hash1 := record.ComputeHash()
	hash2 := record.ComputeHash()

	if hash1 != hash2 {
		t.Error("hash should be deterministic")
	}

	if len(hash1) != 64 {
		t.Errorf("hash should be 64 hex chars, got %d", len(hash1))
	}
}

func TestLogRecord_ToCanonicalLine_ParseCanonicalLine(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	original := NewRecord(RecordTypeEvent, ts, "work", "email|google|msg-123|test@example.com")

	line := original.ToCanonicalLine()

	parsed, err := ParseCanonicalLine(line)
	if err != nil {
		t.Fatalf("ParseCanonicalLine failed: %v", err)
	}

	if parsed.Type != original.Type {
		t.Errorf("Type mismatch: %q != %q", parsed.Type, original.Type)
	}
	if parsed.Version != original.Version {
		t.Errorf("Version mismatch: %q != %q", parsed.Version, original.Version)
	}
	if !parsed.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp mismatch: %v != %v", parsed.Timestamp, original.Timestamp)
	}
	if parsed.Hash != original.Hash {
		t.Errorf("Hash mismatch: %q != %q", parsed.Hash, original.Hash)
	}
	if parsed.CircleID != original.CircleID {
		t.Errorf("CircleID mismatch: %q != %q", parsed.CircleID, original.CircleID)
	}
	if parsed.Payload != original.Payload {
		t.Errorf("Payload mismatch: %q != %q", parsed.Payload, original.Payload)
	}
}

func TestLogRecord_PayloadWithPipes(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	payload := "email|google|msg-123|subject:Hello|World|cc:a@b.com"
	original := NewRecord(RecordTypeEvent, ts, "personal", payload)

	line := original.ToCanonicalLine()

	parsed, err := ParseCanonicalLine(line)
	if err != nil {
		t.Fatalf("ParseCanonicalLine failed: %v", err)
	}

	if parsed.Payload != payload {
		t.Errorf("Payload with pipes not preserved: %q != %q", parsed.Payload, payload)
	}
}

func TestLogRecord_Validate(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		record  *LogRecord
		wantErr bool
	}{
		{
			name:    "valid record",
			record:  NewRecord(RecordTypeEvent, ts, "work", "test payload"),
			wantErr: false,
		},
		{
			name: "missing type",
			record: &LogRecord{
				Version: SchemaVersion,
				Payload: "test",
				Hash:    "abc",
			},
			wantErr: true,
		},
		{
			name: "hash mismatch",
			record: &LogRecord{
				Type:    RecordTypeEvent,
				Version: SchemaVersion,
				Payload: "test",
				Hash:    "wronghash",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.record.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInMemoryLog_Append(t *testing.T) {
	log := NewInMemoryLog()
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	record1 := NewRecord(RecordTypeEvent, ts, "work", "email|msg-1")
	record2 := NewRecord(RecordTypeEvent, ts, "personal", "email|msg-2")

	if err := log.Append(record1); err != nil {
		t.Fatalf("Append record1 failed: %v", err)
	}

	if err := log.Append(record2); err != nil {
		t.Fatalf("Append record2 failed: %v", err)
	}

	// Duplicate should fail
	if err := log.Append(record1); err != ErrRecordExists {
		t.Errorf("expected ErrRecordExists, got %v", err)
	}

	if log.Count() != 2 {
		t.Errorf("Count() = %d, want 2", log.Count())
	}
}

func TestInMemoryLog_Contains(t *testing.T) {
	log := NewInMemoryLog()
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	record := NewRecord(RecordTypeEvent, ts, "work", "email|msg-1")
	log.Append(record)

	if !log.Contains(record.Hash) {
		t.Error("should contain record hash")
	}

	if log.Contains("nonexistent") {
		t.Error("should not contain nonexistent hash")
	}
}

func TestInMemoryLog_Get(t *testing.T) {
	log := NewInMemoryLog()
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	record := NewRecord(RecordTypeEvent, ts, "work", "email|msg-1")
	log.Append(record)

	got, err := log.Get(record.Hash)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.Payload != record.Payload {
		t.Errorf("Payload mismatch: %q != %q", got.Payload, record.Payload)
	}

	_, err = log.Get("nonexistent")
	if err != ErrRecordNotFound {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestInMemoryLog_ListByType(t *testing.T) {
	log := NewInMemoryLog()
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	log.Append(NewRecord(RecordTypeEvent, ts, "work", "event1"))
	log.Append(NewRecord(RecordTypeEvent, ts, "work", "event2"))
	log.Append(NewRecord(RecordTypeDraft, ts, "work", "draft1"))

	events, _ := log.ListByType(RecordTypeEvent)
	if len(events) != 2 {
		t.Errorf("ListByType(EVENT) = %d records, want 2", len(events))
	}

	drafts, _ := log.ListByType(RecordTypeDraft)
	if len(drafts) != 1 {
		t.Errorf("ListByType(DRAFT) = %d records, want 1", len(drafts))
	}
}

func TestInMemoryLog_ListByCircle(t *testing.T) {
	log := NewInMemoryLog()
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	log.Append(NewRecord(RecordTypeEvent, ts, "work", "event1"))
	log.Append(NewRecord(RecordTypeEvent, ts, "work", "event2"))
	log.Append(NewRecord(RecordTypeEvent, ts, "personal", "event3"))

	workRecords, _ := log.ListByCircle("work")
	if len(workRecords) != 2 {
		t.Errorf("ListByCircle(work) = %d records, want 2", len(workRecords))
	}

	personalRecords, _ := log.ListByCircle("personal")
	if len(personalRecords) != 1 {
		t.Errorf("ListByCircle(personal) = %d records, want 1", len(personalRecords))
	}
}

func TestInMemoryLog_Verify(t *testing.T) {
	log := NewInMemoryLog()
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	log.Append(NewRecord(RecordTypeEvent, ts, "work", "event1"))
	log.Append(NewRecord(RecordTypeEvent, ts, "work", "event2"))

	if err := log.Verify(); err != nil {
		t.Errorf("Verify() failed: %v", err)
	}
}

func TestFileLog_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create log and add records
	log1, err := NewFileLog(logPath)
	if err != nil {
		t.Fatalf("NewFileLog failed: %v", err)
	}

	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	record1 := NewRecord(RecordTypeEvent, ts, "work", "email|msg-1")
	record2 := NewRecord(RecordTypeDraft, ts, "personal", "draft|email|to:test@example.com")

	if err := log1.Append(record1); err != nil {
		t.Fatalf("Append record1 failed: %v", err)
	}
	if err := log1.Append(record2); err != nil {
		t.Fatalf("Append record2 failed: %v", err)
	}

	// Reopen log and verify records
	log2, err := NewFileLog(logPath)
	if err != nil {
		t.Fatalf("NewFileLog (reopen) failed: %v", err)
	}

	if log2.Count() != 2 {
		t.Errorf("Count after reopen = %d, want 2", log2.Count())
	}

	got, err := log2.Get(record1.Hash)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.Payload != record1.Payload {
		t.Errorf("Payload mismatch after reopen: %q != %q", got.Payload, record1.Payload)
	}
}

func TestFileLog_Flush_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	log, err := NewFileLog(logPath)
	if err != nil {
		t.Fatalf("NewFileLog failed: %v", err)
	}

	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	// Add multiple records
	for i := 0; i < 10; i++ {
		record := NewRecord(RecordTypeEvent, ts, identity.EntityID("work"), "event-"+itoa(i))
		if err := log.Append(record); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Verify file exists and contains data
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	if info.Size() == 0 {
		t.Error("log file should not be empty")
	}

	// Verify integrity
	if err := log.Verify(); err != nil {
		t.Errorf("Verify failed: %v", err)
	}
}

func TestFileLog_DuplicateRejection(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	log, err := NewFileLog(logPath)
	if err != nil {
		t.Fatalf("NewFileLog failed: %v", err)
	}

	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	record := NewRecord(RecordTypeEvent, ts, "work", "email|msg-1")

	if err := log.Append(record); err != nil {
		t.Fatalf("First append failed: %v", err)
	}

	// Same record should be rejected
	if err := log.Append(record); err != ErrRecordExists {
		t.Errorf("expected ErrRecordExists, got %v", err)
	}

	// Reopen and try again
	log2, err := NewFileLog(logPath)
	if err != nil {
		t.Fatalf("NewFileLog (reopen) failed: %v", err)
	}

	if err := log2.Append(record); err != ErrRecordExists {
		t.Errorf("expected ErrRecordExists after reopen, got %v", err)
	}
}

func TestNewRecord_Deterministic(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	payload := "email|google|msg-123|test@example.com"

	record1 := NewRecord(RecordTypeEvent, ts, "work", payload)
	record2 := NewRecord(RecordTypeEvent, ts, "work", payload)

	if record1.Hash != record2.Hash {
		t.Errorf("hash should be deterministic: %q != %q", record1.Hash, record2.Hash)
	}
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
