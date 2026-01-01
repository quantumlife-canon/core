package draft

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/identity"
)

func TestEmailDraftContent_CanonicalString(t *testing.T) {
	content := EmailDraftContent{
		To:                 "alice@example.com",
		Cc:                 []string{"bob@example.com", "carol@example.com"},
		Subject:            "Re: Meeting",
		Body:               "Sounds good!",
		ThreadID:           "thread-123",
		ProviderHint:       "gmail",
		InReplyToMessageID: "msg-456",
	}

	canonical := content.CanonicalString()

	// Verify determinism - same input = same output
	for i := 0; i < 10; i++ {
		if content.CanonicalString() != canonical {
			t.Errorf("CanonicalString not deterministic")
		}
	}

	// Verify Cc is sorted
	contentReverseCc := EmailDraftContent{
		To:                 "alice@example.com",
		Cc:                 []string{"carol@example.com", "bob@example.com"},
		Subject:            "Re: Meeting",
		Body:               "Sounds good!",
		ThreadID:           "thread-123",
		ProviderHint:       "gmail",
		InReplyToMessageID: "msg-456",
	}

	if content.CanonicalString() != contentReverseCc.CanonicalString() {
		t.Errorf("Cc order should not affect canonical string")
	}
}

func TestCalendarDraftContent_CanonicalString(t *testing.T) {
	proposedStart := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)
	proposedEnd := time.Date(2024, 3, 15, 11, 0, 0, 0, time.UTC)

	content := CalendarDraftContent{
		EventID:       "event-123",
		Response:      CalendarResponseAccept,
		Message:       "Looking forward to it!",
		ProposedStart: &proposedStart,
		ProposedEnd:   &proposedEnd,
		ProviderHint:  "google",
		CalendarID:    "cal-456",
	}

	canonical := content.CanonicalString()

	// Verify determinism
	for i := 0; i < 10; i++ {
		if content.CanonicalString() != canonical {
			t.Errorf("CanonicalString not deterministic")
		}
	}

	// Verify nil proposed times work
	contentNoProposed := CalendarDraftContent{
		EventID:      "event-123",
		Response:     CalendarResponseDecline,
		Message:      "Cannot attend",
		ProviderHint: "google",
		CalendarID:   "cal-456",
	}

	if contentNoProposed.CanonicalString() == "" {
		t.Errorf("CanonicalString should not be empty with nil proposed times")
	}
}

func TestDraft_Hash_Determinism(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	expires := now.Add(48 * time.Hour)

	draft := Draft{
		DraftID:            "draft-123",
		DraftType:          DraftTypeEmailReply,
		CircleID:           identity.EntityID("circle-1"),
		SourceObligationID: "obl-456",
		SourceEventIDs:     []string{"evt-1", "evt-2"},
		CreatedAt:          now,
		ExpiresAt:          expires,
		Status:             StatusProposed,
		Content: EmailDraftContent{
			To:       "alice@example.com",
			Subject:  "Re: Meeting",
			Body:     "Confirmed!",
			ThreadID: "thread-123",
		},
		SafetyNotes:      []string{"Note 1", "Note 2"},
		GenerationRuleID: "rule-1",
	}

	hash := draft.Hash()

	// Same draft = same hash
	for i := 0; i < 10; i++ {
		if draft.Hash() != hash {
			t.Errorf("Hash not deterministic")
		}
	}

	// Order of SourceEventIDs should not matter
	draftReversedEvents := draft
	draftReversedEvents.SourceEventIDs = []string{"evt-2", "evt-1"}
	if draft.Hash() != draftReversedEvents.Hash() {
		t.Errorf("SourceEventIDs order should not affect hash")
	}

	// Order of SafetyNotes should not matter
	draftReversedNotes := draft
	draftReversedNotes.SafetyNotes = []string{"Note 2", "Note 1"}
	if draft.Hash() != draftReversedNotes.Hash() {
		t.Errorf("SafetyNotes order should not affect hash")
	}
}

func TestDraft_Hash_DifferentInputs(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	expires := now.Add(48 * time.Hour)

	baseDraft := Draft{
		DraftID:            "draft-123",
		DraftType:          DraftTypeEmailReply,
		CircleID:           identity.EntityID("circle-1"),
		SourceObligationID: "obl-456",
		CreatedAt:          now,
		ExpiresAt:          expires,
		Status:             StatusProposed,
		Content: EmailDraftContent{
			To:       "alice@example.com",
			Subject:  "Re: Meeting",
			Body:     "Confirmed!",
			ThreadID: "thread-123",
		},
	}

	baseHash := baseDraft.Hash()

	// Different body = different hash
	modifiedDraft := baseDraft
	modifiedDraft.Content = EmailDraftContent{
		To:       "alice@example.com",
		Subject:  "Re: Meeting",
		Body:     "Different body",
		ThreadID: "thread-123",
	}
	if modifiedDraft.Hash() == baseHash {
		t.Errorf("Different body should produce different hash")
	}

	// Different circle = different hash
	modifiedDraft = baseDraft
	modifiedDraft.CircleID = identity.EntityID("circle-2")
	if modifiedDraft.Hash() == baseHash {
		t.Errorf("Different circle should produce different hash")
	}
}

func TestComputeDraftID_Determinism(t *testing.T) {
	id1 := ComputeDraftID(
		DraftTypeEmailReply,
		identity.EntityID("circle-1"),
		"obl-123",
		"content-hash-abc",
	)

	id2 := ComputeDraftID(
		DraftTypeEmailReply,
		identity.EntityID("circle-1"),
		"obl-123",
		"content-hash-abc",
	)

	if id1 != id2 {
		t.Errorf("ComputeDraftID not deterministic: %s != %s", id1, id2)
	}

	// 16 hex characters expected
	if len(id1) != 16 {
		t.Errorf("DraftID should be 16 hex characters, got %d", len(id1))
	}
}

func TestDraft_IsExpired(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	draft := Draft{
		ExpiresAt: now.Add(24 * time.Hour),
	}

	// Before expiry
	if draft.IsExpired(now) {
		t.Errorf("Draft should not be expired before ExpiresAt")
	}

	// At expiry (not expired - must be after)
	if draft.IsExpired(now.Add(24 * time.Hour)) {
		t.Errorf("Draft should not be expired exactly at ExpiresAt")
	}

	// After expiry
	if !draft.IsExpired(now.Add(25 * time.Hour)) {
		t.Errorf("Draft should be expired after ExpiresAt")
	}
}

func TestDraft_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name      string
		from      DraftStatus
		to        DraftStatus
		canChange bool
	}{
		{"proposed to approved", StatusProposed, StatusApproved, true},
		{"proposed to rejected", StatusProposed, StatusRejected, true},
		{"proposed to expired", StatusProposed, StatusExpired, true},
		{"proposed to proposed", StatusProposed, StatusProposed, false},
		{"approved to rejected", StatusApproved, StatusRejected, false},
		{"approved to expired", StatusApproved, StatusExpired, false},
		{"rejected to approved", StatusRejected, StatusApproved, false},
		{"expired to approved", StatusExpired, StatusApproved, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			draft := Draft{Status: tt.from}
			if draft.CanTransitionTo(tt.to) != tt.canChange {
				t.Errorf("CanTransitionTo(%s) from %s = %v, want %v",
					tt.to, tt.from, !tt.canChange, tt.canChange)
			}
		})
	}
}

func TestDraft_IsTerminal(t *testing.T) {
	tests := []struct {
		status     DraftStatus
		isTerminal bool
	}{
		{StatusProposed, false},
		{StatusApproved, true},
		{StatusRejected, true},
		{StatusExpired, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			draft := Draft{Status: tt.status}
			if draft.IsTerminal() != tt.isTerminal {
				t.Errorf("IsTerminal() = %v, want %v", !tt.isTerminal, tt.isTerminal)
			}
		})
	}
}

func TestDraft_DedupKey(t *testing.T) {
	emailDraft := Draft{
		DraftType:          DraftTypeEmailReply,
		CircleID:           identity.EntityID("circle-1"),
		SourceObligationID: "obl-123",
		Content: EmailDraftContent{
			ThreadID: "thread-456",
		},
	}

	key1 := emailDraft.DedupKey()

	// Same draft = same key
	key2 := emailDraft.DedupKey()
	if key1 != key2 {
		t.Errorf("DedupKey not deterministic")
	}

	// Calendar draft with same fields but different event
	calendarDraft := Draft{
		DraftType:          DraftTypeCalendarResponse,
		CircleID:           identity.EntityID("circle-1"),
		SourceObligationID: "obl-123",
		Content: CalendarDraftContent{
			EventID: "event-789",
		},
	}

	if emailDraft.DedupKey() == calendarDraft.DedupKey() {
		t.Errorf("Different draft types should have different dedup keys")
	}
}

func TestDraft_EmailContent(t *testing.T) {
	emailDraft := Draft{
		DraftType: DraftTypeEmailReply,
		Content: EmailDraftContent{
			To:      "alice@example.com",
			Subject: "Test",
		},
	}

	content, ok := emailDraft.EmailContent()
	if !ok {
		t.Errorf("EmailContent() should return true for email draft")
	}
	if content.To != "alice@example.com" {
		t.Errorf("EmailContent() returned wrong To address")
	}

	// Calendar draft should return false
	calendarDraft := Draft{
		DraftType: DraftTypeCalendarResponse,
		Content: CalendarDraftContent{
			EventID: "event-123",
		},
	}

	_, ok = calendarDraft.EmailContent()
	if ok {
		t.Errorf("EmailContent() should return false for calendar draft")
	}
}

func TestDraft_CalendarContent(t *testing.T) {
	calendarDraft := Draft{
		DraftType: DraftTypeCalendarResponse,
		Content: CalendarDraftContent{
			EventID:  "event-123",
			Response: CalendarResponseAccept,
		},
	}

	content, ok := calendarDraft.CalendarContent()
	if !ok {
		t.Errorf("CalendarContent() should return true for calendar draft")
	}
	if content.EventID != "event-123" {
		t.Errorf("CalendarContent() returned wrong EventID")
	}

	// Email draft should return false
	emailDraft := Draft{
		DraftType: DraftTypeEmailReply,
		Content: EmailDraftContent{
			To: "alice@example.com",
		},
	}

	_, ok = emailDraft.CalendarContent()
	if ok {
		t.Errorf("CalendarContent() should return false for email draft")
	}
}

func TestSortDrafts_Deterministic(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	drafts := []Draft{
		{DraftID: "d3", Status: StatusApproved, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now},
		{DraftID: "d1", Status: StatusProposed, ExpiresAt: now.Add(48 * time.Hour), CreatedAt: now},
		{DraftID: "d4", Status: StatusExpired, ExpiresAt: now.Add(12 * time.Hour), CreatedAt: now},
		{DraftID: "d2", Status: StatusProposed, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now},
	}

	SortDrafts(drafts)

	// Expected order:
	// 1. d2 (proposed, expires sooner)
	// 2. d1 (proposed, expires later)
	// 3. d3 (approved)
	// 4. d4 (expired)
	expected := []DraftID{"d2", "d1", "d3", "d4"}
	for i, exp := range expected {
		if drafts[i].DraftID != exp {
			t.Errorf("Position %d: got %s, want %s", i, drafts[i].DraftID, exp)
		}
	}

	// Sorting again should produce same order
	SortDrafts(drafts)
	for i, exp := range expected {
		if drafts[i].DraftID != exp {
			t.Errorf("After re-sort, position %d: got %s, want %s", i, drafts[i].DraftID, exp)
		}
	}
}

func TestSortDrafts_StableTiebreaker(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	// All same status, expiresAt, createdAt - should sort by DraftID
	drafts := []Draft{
		{DraftID: "zzz", Status: StatusProposed, ExpiresAt: expires, CreatedAt: now},
		{DraftID: "aaa", Status: StatusProposed, ExpiresAt: expires, CreatedAt: now},
		{DraftID: "mmm", Status: StatusProposed, ExpiresAt: expires, CreatedAt: now},
	}

	SortDrafts(drafts)

	expected := []DraftID{"aaa", "mmm", "zzz"}
	for i, exp := range expected {
		if drafts[i].DraftID != exp {
			t.Errorf("Position %d: got %s, want %s", i, drafts[i].DraftID, exp)
		}
	}
}

func TestStatusOrder(t *testing.T) {
	// Proposed should be highest priority (lowest number)
	if StatusOrder(StatusProposed) >= StatusOrder(StatusApproved) {
		t.Errorf("Proposed should have higher priority than approved")
	}
	if StatusOrder(StatusApproved) >= StatusOrder(StatusRejected) {
		t.Errorf("Approved should have higher priority than rejected")
	}
	if StatusOrder(StatusRejected) >= StatusOrder(StatusExpired) {
		t.Errorf("Rejected should have higher priority than expired")
	}
}
