package pagination

import (
	"testing"
	"time"
)

func TestEncodeDecode_Roundtrip(t *testing.T) {
	original := CursorData{
		Offset:     100,
		WindowHash: "abc123",
		SortKey:    "2024-01-15",
		PageNumber: 3,
	}

	encoded, err := EncodeCursor(original)
	if err != nil {
		t.Fatalf("EncodeCursor failed: %v", err)
	}

	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor failed: %v", err)
	}

	if decoded.Offset != original.Offset {
		t.Errorf("Offset mismatch: got %d, want %d", decoded.Offset, original.Offset)
	}
	if decoded.WindowHash != original.WindowHash {
		t.Errorf("WindowHash mismatch: got %s, want %s", decoded.WindowHash, original.WindowHash)
	}
	if decoded.SortKey != original.SortKey {
		t.Errorf("SortKey mismatch: got %s, want %s", decoded.SortKey, original.SortKey)
	}
	if decoded.PageNumber != original.PageNumber {
		t.Errorf("PageNumber mismatch: got %d, want %d", decoded.PageNumber, original.PageNumber)
	}
}

func TestDecodeCursor_Empty(t *testing.T) {
	data, err := DecodeCursor("")
	if err != nil {
		t.Fatalf("DecodeCursor failed: %v", err)
	}

	if data.Offset != 0 {
		t.Errorf("expected Offset=0 for empty cursor, got %d", data.Offset)
	}
	if data.PageNumber != 1 {
		t.Errorf("expected PageNumber=1 for empty cursor, got %d", data.PageNumber)
	}
}

func TestDecodeCursor_Invalid(t *testing.T) {
	_, err := DecodeCursor("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid cursor")
	}
}

func TestComputeWindowHash_Deterministic(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	hash1 := ComputeWindowHash(start, end)
	hash2 := ComputeWindowHash(start, end)

	if hash1 != hash2 {
		t.Error("window hash should be deterministic")
	}
}

func TestComputeWindowHash_DifferentWindows(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end1 := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	end2 := time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC)

	hash1 := ComputeWindowHash(start, end1)
	hash2 := ComputeWindowHash(start, end2)

	if hash1 == hash2 {
		t.Error("different windows should have different hashes")
	}
}

func TestValidateCursor_EmptyValid(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	err := ValidateCursor("", start, end)
	if err != nil {
		t.Errorf("empty cursor should be valid: %v", err)
	}
}

func TestValidateCursor_StaleCursor(t *testing.T) {
	start1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end1 := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	// Create cursor with original window
	cursorData := CursorData{
		Offset:     50,
		WindowHash: ComputeWindowHash(start1, end1),
		PageNumber: 2,
	}
	cursor, _ := EncodeCursor(cursorData)

	// Validate with different window
	start2 := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	end2 := time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC)

	err := ValidateCursor(cursor, start2, end2)
	if err == nil {
		t.Error("expected error for stale cursor")
	}
}

func TestNormalizeLimit(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, DefaultLimit},
		{-1, DefaultLimit},
		{50, 50},
		{100, 100},
		{500, 500},
		{1000, MaxLimit},
	}

	for _, tt := range tests {
		result := NormalizeLimit(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeLimit(%d) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

type testItem struct {
	ID   string
	Date string
}

func TestPaginator_FirstPage(t *testing.T) {
	items := make([]testItem, 250)
	for i := range items {
		items[i] = testItem{ID: string(rune('A' + i%26))}
	}

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	p := NewPaginator(items, start, end).WithLimit(100)

	page, resp, err := p.GetPage("")
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}

	if len(page) != 100 {
		t.Errorf("expected 100 items, got %d", len(page))
	}

	if !resp.HasMore {
		t.Error("expected HasMore=true")
	}

	if resp.TotalCount != 250 {
		t.Errorf("expected TotalCount=250, got %d", resp.TotalCount)
	}

	if resp.NextCursor == "" {
		t.Error("expected non-empty NextCursor")
	}
}

func TestPaginator_AllPages(t *testing.T) {
	items := make([]testItem, 250)
	for i := range items {
		items[i] = testItem{ID: string(rune('A' + i%26))}
	}

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	p := NewPaginator(items, start, end).WithLimit(100)

	var allItems []testItem
	cursor := ""
	pageCount := 0

	for {
		page, resp, err := p.GetPage(cursor)
		if err != nil {
			t.Fatalf("GetPage failed: %v", err)
		}

		allItems = append(allItems, page...)
		pageCount++

		if !resp.HasMore {
			break
		}
		cursor = resp.NextCursor
	}

	if len(allItems) != 250 {
		t.Errorf("expected 250 total items, got %d", len(allItems))
	}

	if pageCount != 3 {
		t.Errorf("expected 3 pages (100+100+50), got %d", pageCount)
	}
}

func TestPaginator_Determinism(t *testing.T) {
	items := make([]testItem, 50)
	for i := range items {
		items[i] = testItem{ID: string(rune('A' + i))}
	}

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	p := NewPaginator(items, start, end).WithLimit(20)

	// Get first two pages
	page1a, resp1a, _ := p.GetPage("")
	page2a, _, _ := p.GetPage(resp1a.NextCursor)

	// Get same pages again
	page1b, resp1b, _ := p.GetPage("")
	page2b, _, _ := p.GetPage(resp1b.NextCursor)

	// Results should be identical
	if len(page1a) != len(page1b) {
		t.Error("page 1 length differs between calls")
	}

	if len(page2a) != len(page2b) {
		t.Error("page 2 length differs between calls")
	}

	for i := range page1a {
		if page1a[i].ID != page1b[i].ID {
			t.Error("page 1 content differs between calls")
		}
	}

	for i := range page2a {
		if page2a[i].ID != page2b[i].ID {
			t.Error("page 2 content differs between calls")
		}
	}
}
