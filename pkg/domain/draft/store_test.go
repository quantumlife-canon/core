package draft

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/identity"
)

func TestInMemoryStore_PutGet(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	draft := Draft{
		DraftID:   "draft-123",
		DraftType: DraftTypeEmailReply,
		CircleID:  identity.EntityID("circle-1"),
		CreatedAt: now,
		ExpiresAt: now.Add(48 * time.Hour),
		Status:    StatusProposed,
		Content: EmailDraftContent{
			To:      "alice@example.com",
			Subject: "Test",
			Body:    "Hello",
		},
	}

	err := store.Put(draft)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	retrieved, ok := store.Get("draft-123")
	if !ok {
		t.Fatalf("Get failed to find draft")
	}

	if retrieved.DraftID != draft.DraftID {
		t.Errorf("Retrieved draft has wrong ID: %s", retrieved.DraftID)
	}

	// Hash should be computed on Put
	if retrieved.DeterministicHash == "" {
		t.Errorf("DeterministicHash should be computed on Put")
	}
}

func TestInMemoryStore_GetNotFound(t *testing.T) {
	store := NewInMemoryStore()

	_, ok := store.Get("nonexistent")
	if ok {
		t.Errorf("Get should return false for nonexistent draft")
	}
}

func TestInMemoryStore_List_FilterByCircle(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	circle1 := identity.EntityID("circle-1")
	circle2 := identity.EntityID("circle-2")

	_ = store.Put(Draft{DraftID: "d1", CircleID: circle1, Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d2", CircleID: circle2, Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d3", CircleID: circle1, Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})

	result := store.List(ListFilter{CircleID: circle1})

	if len(result) != 2 {
		t.Errorf("Expected 2 drafts for circle1, got %d", len(result))
	}

	for _, d := range result {
		if d.CircleID != circle1 {
			t.Errorf("List returned draft from wrong circle: %s", d.CircleID)
		}
	}
}

func TestInMemoryStore_List_FilterByStatus(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	_ = store.Put(Draft{DraftID: "d1", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d2", Status: StatusApproved, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d3", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})

	result := store.List(ListFilter{Status: StatusProposed})

	if len(result) != 2 {
		t.Errorf("Expected 2 proposed drafts, got %d", len(result))
	}
}

func TestInMemoryStore_List_FilterByType(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	_ = store.Put(Draft{DraftID: "d1", DraftType: DraftTypeEmailReply, Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d2", DraftType: DraftTypeCalendarResponse, Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d3", DraftType: DraftTypeEmailReply, Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})

	result := store.List(ListFilter{DraftType: DraftTypeEmailReply})

	if len(result) != 2 {
		t.Errorf("Expected 2 email drafts, got %d", len(result))
	}
}

func TestInMemoryStore_List_ExcludeExpiredByDefault(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	_ = store.Put(Draft{DraftID: "d1", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d2", Status: StatusExpired, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d3", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})

	result := store.List(ListFilter{})

	if len(result) != 2 {
		t.Errorf("Expected 2 non-expired drafts by default, got %d", len(result))
	}

	// With IncludeExpired
	result = store.List(ListFilter{IncludeExpired: true})

	if len(result) != 3 {
		t.Errorf("Expected 3 drafts with IncludeExpired, got %d", len(result))
	}
}

func TestInMemoryStore_List_Limit(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		_ = store.Put(Draft{
			DraftID:   DraftID("d" + string(rune('0'+i))),
			Status:    StatusProposed,
			CreatedAt: now,
			ExpiresAt: now.Add(time.Duration(i+1) * time.Hour),
		})
	}

	result := store.List(ListFilter{Limit: 3})

	if len(result) != 3 {
		t.Errorf("Expected 3 drafts with limit, got %d", len(result))
	}
}

func TestInMemoryStore_List_DeterministicOrder(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	// Add in random order
	_ = store.Put(Draft{DraftID: "d3", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(48 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d1", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d2", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(36 * time.Hour)})

	result1 := store.List(ListFilter{})
	result2 := store.List(ListFilter{})

	// Same order both times
	for i := range result1 {
		if result1[i].DraftID != result2[i].DraftID {
			t.Errorf("List not deterministic at position %d", i)
		}
	}

	// Sorted by ExpiresAt (soonest first)
	expected := []DraftID{"d1", "d2", "d3"}
	for i, exp := range expected {
		if result1[i].DraftID != exp {
			t.Errorf("Position %d: got %s, want %s", i, result1[i].DraftID, exp)
		}
	}
}

func TestInMemoryStore_UpdateStatus(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	_ = store.Put(Draft{DraftID: "d1", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})

	changedAt := now.Add(1 * time.Hour)
	err := store.UpdateStatus("d1", StatusApproved, "Looks good", "person-1", changedAt)
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	draft, _ := store.Get("d1")
	if draft.Status != StatusApproved {
		t.Errorf("Status not updated: %s", draft.Status)
	}
	if draft.StatusReason != "Looks good" {
		t.Errorf("StatusReason not updated: %s", draft.StatusReason)
	}
	if draft.StatusChangedBy != "person-1" {
		t.Errorf("StatusChangedBy not updated: %s", draft.StatusChangedBy)
	}
	if !draft.StatusChangedAt.Equal(changedAt) {
		t.Errorf("StatusChangedAt not updated")
	}
}

func TestInMemoryStore_UpdateStatus_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	err := store.UpdateStatus("nonexistent", StatusApproved, "", "", now)
	if err == nil {
		t.Errorf("UpdateStatus should fail for nonexistent draft")
	}
}

func TestInMemoryStore_UpdateStatus_InvalidTransition(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	_ = store.Put(Draft{DraftID: "d1", Status: StatusApproved, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})

	err := store.UpdateStatus("d1", StatusRejected, "", "", now)
	if err == nil {
		t.Errorf("UpdateStatus should fail for invalid transition")
	}
}

func TestInMemoryStore_MarkExpired(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	// d1: expires in 24h (not expired at now+25h)
	// d2: expires in 12h (expired at now+25h)
	// d3: already approved (should not be marked expired)
	_ = store.Put(Draft{DraftID: "d1", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d2", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(12 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d3", Status: StatusApproved, CreatedAt: now, ExpiresAt: now.Add(6 * time.Hour)})

	checkTime := now.Add(13 * time.Hour)
	count := store.MarkExpired(checkTime)

	if count != 1 {
		t.Errorf("Expected 1 draft marked expired, got %d", count)
	}

	d2, _ := store.Get("d2")
	if d2.Status != StatusExpired {
		t.Errorf("d2 should be expired, got %s", d2.Status)
	}
	if d2.StatusReason != "TTL expired" {
		t.Errorf("StatusReason should be 'TTL expired', got %s", d2.StatusReason)
	}
	if d2.StatusChangedBy != "system" {
		t.Errorf("StatusChangedBy should be 'system', got %s", d2.StatusChangedBy)
	}

	d1, _ := store.Get("d1")
	if d1.Status != StatusProposed {
		t.Errorf("d1 should still be proposed, got %s", d1.Status)
	}

	d3, _ := store.Get("d3")
	if d3.Status != StatusApproved {
		t.Errorf("d3 should still be approved, got %s", d3.Status)
	}
}

func TestInMemoryStore_MarkExpired_Deterministic(t *testing.T) {
	store1 := NewInMemoryStore()
	store2 := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	// Same data in both stores
	for _, s := range []*InMemoryStore{store1, store2} {
		_ = s.Put(Draft{DraftID: "d1", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(1 * time.Hour)})
		_ = s.Put(Draft{DraftID: "d2", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(2 * time.Hour)})
	}

	checkTime := now.Add(3 * time.Hour)

	count1 := store1.MarkExpired(checkTime)
	count2 := store2.MarkExpired(checkTime)

	if count1 != count2 {
		t.Errorf("MarkExpired not deterministic: %d vs %d", count1, count2)
	}

	// Both drafts should have same final state
	d1a, _ := store1.Get("d1")
	d1b, _ := store2.Get("d1")
	if d1a.Status != d1b.Status {
		t.Errorf("d1 status differs: %s vs %s", d1a.Status, d1b.Status)
	}
}

func TestInMemoryStore_Count(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	if store.Count() != 0 {
		t.Errorf("Empty store should have count 0")
	}

	_ = store.Put(Draft{DraftID: "d1", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d2", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})

	if store.Count() != 2 {
		t.Errorf("Expected count 2, got %d", store.Count())
	}
}

func TestInMemoryStore_CountByCircleAndDay(t *testing.T) {
	store := NewInMemoryStore()

	circle1 := identity.EntityID("circle-1")
	circle2 := identity.EntityID("circle-2")

	day1 := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2024, 3, 16, 10, 0, 0, 0, time.UTC)

	_ = store.Put(Draft{DraftID: "d1", CircleID: circle1, Status: StatusProposed, CreatedAt: day1, ExpiresAt: day1.Add(24 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d2", CircleID: circle1, Status: StatusProposed, CreatedAt: day1.Add(2 * time.Hour), ExpiresAt: day1.Add(26 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d3", CircleID: circle1, Status: StatusProposed, CreatedAt: day2, ExpiresAt: day2.Add(24 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d4", CircleID: circle2, Status: StatusProposed, CreatedAt: day1, ExpiresAt: day1.Add(24 * time.Hour)})

	count := store.CountByCircleAndDay(circle1, DayKey(day1))
	if count != 2 {
		t.Errorf("Expected 2 drafts for circle1 on day1, got %d", count)
	}

	count = store.CountByCircleAndDay(circle1, DayKey(day2))
	if count != 1 {
		t.Errorf("Expected 1 draft for circle1 on day2, got %d", count)
	}

	count = store.CountByCircleAndDay(circle2, DayKey(day1))
	if count != 1 {
		t.Errorf("Expected 1 draft for circle2 on day1, got %d", count)
	}
}

func TestInMemoryStore_GetByDedupKey(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	draft := Draft{
		DraftID:            "d1",
		DraftType:          DraftTypeEmailReply,
		CircleID:           identity.EntityID("circle-1"),
		SourceObligationID: "obl-123",
		Status:             StatusProposed,
		CreatedAt:          now,
		ExpiresAt:          now.Add(24 * time.Hour),
		Content: EmailDraftContent{
			ThreadID: "thread-456",
		},
	}

	_ = store.Put(draft)

	found, ok := store.GetByDedupKey(draft.DedupKey())
	if !ok {
		t.Fatalf("GetByDedupKey failed to find draft")
	}
	if found.DraftID != "d1" {
		t.Errorf("GetByDedupKey returned wrong draft: %s", found.DraftID)
	}

	// Nonexistent key
	_, ok = store.GetByDedupKey("nonexistent")
	if ok {
		t.Errorf("GetByDedupKey should return false for nonexistent key")
	}
}

func TestInMemoryStore_Delete(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	_ = store.Put(Draft{DraftID: "d1", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d2", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})

	store.Delete("d1")

	if store.Count() != 1 {
		t.Errorf("Expected count 1 after delete, got %d", store.Count())
	}

	_, ok := store.Get("d1")
	if ok {
		t.Errorf("Deleted draft should not be found")
	}
}

func TestInMemoryStore_Clear(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	_ = store.Put(Draft{DraftID: "d1", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	_ = store.Put(Draft{DraftID: "d2", Status: StatusProposed, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})

	store.Clear()

	if store.Count() != 0 {
		t.Errorf("Expected count 0 after clear, got %d", store.Count())
	}
}
