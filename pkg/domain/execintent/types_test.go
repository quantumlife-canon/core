package execintent

import (
	"strings"
	"testing"
	"time"

	"quantumlife/pkg/domain/draft"
)

func TestExecutionIntent_CanonicalString_Determinism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	intent1 := &ExecutionIntent{
		DraftID:            draft.DraftID("draft-123"),
		CircleID:           "circle-abc",
		Action:             ActionEmailSend,
		EmailThreadID:      "thread-001",
		EmailMessageID:     "msg-001",
		EmailTo:            "test@example.com",
		EmailSubject:       "Test Subject",
		EmailBody:          "Test body content",
		PolicySnapshotHash: "policy-hash-abc",
		ViewSnapshotHash:   "view-hash-xyz",
		CreatedAt:          now,
	}

	intent2 := &ExecutionIntent{
		DraftID:            draft.DraftID("draft-123"),
		CircleID:           "circle-abc",
		Action:             ActionEmailSend,
		EmailThreadID:      "thread-001",
		EmailMessageID:     "msg-001",
		EmailTo:            "test@example.com",
		EmailSubject:       "Test Subject",
		EmailBody:          "Test body content",
		PolicySnapshotHash: "policy-hash-abc",
		ViewSnapshotHash:   "view-hash-xyz",
		CreatedAt:          now,
	}

	if intent1.CanonicalString() != intent2.CanonicalString() {
		t.Error("canonical strings should be identical for same inputs")
	}

	if intent1.Hash() != intent2.Hash() {
		t.Error("hashes should be identical for same inputs")
	}

	intent1.Finalize()
	intent2.Finalize()

	if intent1.IntentID != intent2.IntentID {
		t.Errorf("IntentIDs should match: %s vs %s", intent1.IntentID, intent2.IntentID)
	}
}

func TestExecutionIntent_CanonicalString_Format(t *testing.T) {
	intent := &ExecutionIntent{
		DraftID:            draft.DraftID("draft-123"),
		CircleID:           "circle-abc",
		Action:             ActionEmailSend,
		EmailThreadID:      "thread-001",
		PolicySnapshotHash: "policy-hash",
		ViewSnapshotHash:   "view-hash",
	}

	canonical := intent.CanonicalString()

	if !strings.HasPrefix(canonical, "execintent|") {
		t.Error("canonical string should start with execintent|")
	}

	expectedParts := []string{
		"draft:draft-123",
		"circle:circle-abc",
		"action:email_send",
		"policy_hash:policy-hash",
		"view_hash:view-hash",
	}

	for _, part := range expectedParts {
		if !strings.Contains(canonical, part) {
			t.Errorf("canonical string missing: %s", part)
		}
	}
}

func TestExecutionIntent_CanonicalString_Normalization(t *testing.T) {
	intent := &ExecutionIntent{
		DraftID:            draft.DraftID("draft-123"),
		CircleID:           "circle-abc",
		Action:             ActionEmailSend,
		EmailSubject:       "UPPERCASE Subject",
		EmailBody:          "Line1\nLine2\r\nLine3",
		PolicySnapshotHash: "hash",
		ViewSnapshotHash:   "hash",
	}

	canonical := intent.CanonicalString()

	// Should be lowercased
	if strings.Contains(canonical, "UPPERCASE") {
		t.Error("canonical string should be lowercased")
	}

	// Newlines should be replaced
	if strings.Contains(canonical, "\n") {
		t.Error("canonical string should not contain newlines")
	}
}

func TestExecutionIntent_DifferentInputs_DifferentHashes(t *testing.T) {
	base := &ExecutionIntent{
		DraftID:            draft.DraftID("draft-123"),
		CircleID:           "circle-abc",
		Action:             ActionEmailSend,
		EmailThreadID:      "thread-001",
		PolicySnapshotHash: "policy-hash",
		ViewSnapshotHash:   "view-hash",
	}

	modified := &ExecutionIntent{
		DraftID:            draft.DraftID("draft-456"), // Different
		CircleID:           "circle-abc",
		Action:             ActionEmailSend,
		EmailThreadID:      "thread-001",
		PolicySnapshotHash: "policy-hash",
		ViewSnapshotHash:   "view-hash",
	}

	if base.Hash() == modified.Hash() {
		t.Error("different inputs should produce different hashes")
	}
}

func TestExecutionIntent_Validate_EmailAction(t *testing.T) {
	tests := []struct {
		name    string
		intent  *ExecutionIntent
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid email intent",
			intent: &ExecutionIntent{
				DraftID:            draft.DraftID("draft-123"),
				CircleID:           "circle-abc",
				Action:             ActionEmailSend,
				EmailThreadID:      "thread-001",
				PolicySnapshotHash: "hash",
				ViewSnapshotHash:   "hash",
			},
			wantErr: false,
		},
		{
			name: "missing DraftID",
			intent: &ExecutionIntent{
				CircleID:           "circle-abc",
				Action:             ActionEmailSend,
				EmailThreadID:      "thread-001",
				PolicySnapshotHash: "hash",
				ViewSnapshotHash:   "hash",
			},
			wantErr: true,
			errMsg:  "missing DraftID",
		},
		{
			name: "missing PolicySnapshotHash",
			intent: &ExecutionIntent{
				DraftID:          draft.DraftID("draft-123"),
				CircleID:         "circle-abc",
				Action:           ActionEmailSend,
				EmailThreadID:    "thread-001",
				ViewSnapshotHash: "hash",
			},
			wantErr: true,
			errMsg:  "missing PolicySnapshotHash",
		},
		{
			name: "missing ViewSnapshotHash",
			intent: &ExecutionIntent{
				DraftID:            draft.DraftID("draft-123"),
				CircleID:           "circle-abc",
				Action:             ActionEmailSend,
				EmailThreadID:      "thread-001",
				PolicySnapshotHash: "hash",
			},
			wantErr: true,
			errMsg:  "missing ViewSnapshotHash",
		},
		{
			name: "email action missing thread",
			intent: &ExecutionIntent{
				DraftID:            draft.DraftID("draft-123"),
				CircleID:           "circle-abc",
				Action:             ActionEmailSend,
				PolicySnapshotHash: "hash",
				ViewSnapshotHash:   "hash",
			},
			wantErr: true,
			errMsg:  "ThreadID or MessageID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.intent.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error should contain %q, got %q", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestExecutionIntent_Validate_CalendarAction(t *testing.T) {
	tests := []struct {
		name    string
		intent  *ExecutionIntent
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid calendar intent",
			intent: &ExecutionIntent{
				DraftID:            draft.DraftID("draft-123"),
				CircleID:           "circle-abc",
				Action:             ActionCalendarRespond,
				CalendarEventID:    "event-001",
				CalendarResponse:   "accepted",
				PolicySnapshotHash: "hash",
				ViewSnapshotHash:   "hash",
			},
			wantErr: false,
		},
		{
			name: "calendar action missing EventID",
			intent: &ExecutionIntent{
				DraftID:            draft.DraftID("draft-123"),
				CircleID:           "circle-abc",
				Action:             ActionCalendarRespond,
				CalendarResponse:   "accepted",
				PolicySnapshotHash: "hash",
				ViewSnapshotHash:   "hash",
			},
			wantErr: true,
			errMsg:  "EventID",
		},
		{
			name: "calendar action missing Response",
			intent: &ExecutionIntent{
				DraftID:            draft.DraftID("draft-123"),
				CircleID:           "circle-abc",
				Action:             ActionCalendarRespond,
				CalendarEventID:    "event-001",
				PolicySnapshotHash: "hash",
				ViewSnapshotHash:   "hash",
			},
			wantErr: true,
			errMsg:  "Response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.intent.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error should contain %q, got %q", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestExecutionIntent_Finalize(t *testing.T) {
	intent := &ExecutionIntent{
		DraftID:            draft.DraftID("draft-123"),
		CircleID:           "circle-abc",
		Action:             ActionEmailSend,
		EmailThreadID:      "thread-001",
		PolicySnapshotHash: "hash",
		ViewSnapshotHash:   "hash",
	}

	if intent.IntentID != "" {
		t.Error("IntentID should be empty before Finalize")
	}
	if intent.DeterministicHash != "" {
		t.Error("DeterministicHash should be empty before Finalize")
	}

	intent.Finalize()

	if intent.IntentID == "" {
		t.Error("IntentID should be set after Finalize")
	}
	if intent.DeterministicHash == "" {
		t.Error("DeterministicHash should be set after Finalize")
	}

	// Verify IntentID format
	if !strings.HasPrefix(string(intent.IntentID), "intent-") {
		t.Errorf("IntentID should have intent- prefix: %s", intent.IntentID)
	}
}

func TestActionClass_Constants(t *testing.T) {
	if ActionEmailSend != "email_send" {
		t.Errorf("ActionEmailSend = %s, want email_send", ActionEmailSend)
	}
	if ActionCalendarRespond != "calendar_respond" {
		t.Errorf("ActionCalendarRespond = %s, want calendar_respond", ActionCalendarRespond)
	}
}
