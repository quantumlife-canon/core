// Package pagination provides v8.5 deterministic pagination for provider readers.
//
// CRITICAL INVARIANT: Pagination is deterministic.
// Same cursor + same request = same result.
// No time-based logic, no random offsets.
//
// Reference: docs/TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md
package pagination

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// PageRequest contains pagination parameters for fetching data.
type PageRequest struct {
	// Cursor is an opaque cursor from a previous page response.
	// Empty for the first page.
	Cursor string

	// Limit is the maximum number of items to return.
	// Default: 100, Max: 500
	Limit int

	// WindowStart is the start of the data window.
	WindowStart time.Time

	// WindowEnd is the end of the data window.
	WindowEnd time.Time
}

// PageResponse contains pagination metadata for a response.
type PageResponse struct {
	// NextCursor is the cursor for the next page.
	// Empty if this is the last page.
	NextCursor string

	// HasMore indicates if more pages exist.
	HasMore bool

	// TotalCount is the total number of items (if known).
	// -1 if unknown.
	TotalCount int

	// PageNumber is the current page number (1-indexed).
	PageNumber int
}

// CursorData contains the data encoded in a cursor.
// This is an internal structure - cursors are opaque to clients.
type CursorData struct {
	// Offset is the position in the result set.
	Offset int `json:"o"`

	// WindowHash is a hash of the window parameters.
	// Used to detect if the window changed between requests.
	WindowHash string `json:"w"`

	// SortKey is the last item's sort key from the previous page.
	// Used for keyset pagination when available.
	SortKey string `json:"k,omitempty"`

	// PageNumber is the page number.
	PageNumber int `json:"p"`

	// Version is the cursor format version.
	Version int `json:"v"`
}

const (
	// DefaultLimit is the default page size.
	DefaultLimit = 100

	// MaxLimit is the maximum page size.
	MaxLimit = 500

	// CursorVersion is the current cursor format version.
	CursorVersion = 1
)

// EncodeCursor encodes cursor data into an opaque string.
func EncodeCursor(data CursorData) (string, error) {
	data.Version = CursorVersion

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", errors.New("failed to encode cursor data")
	}

	return base64.URLEncoding.EncodeToString(jsonBytes), nil
}

// DecodeCursor decodes an opaque cursor string into cursor data.
func DecodeCursor(cursor string) (CursorData, error) {
	if cursor == "" {
		return CursorData{
			Offset:     0,
			PageNumber: 1,
			Version:    CursorVersion,
		}, nil
	}

	jsonBytes, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return CursorData{}, errors.New("invalid cursor format")
	}

	var data CursorData
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return CursorData{}, errors.New("invalid cursor data")
	}

	if data.Version != CursorVersion {
		return CursorData{}, errors.New("cursor version mismatch")
	}

	return data, nil
}

// ComputeWindowHash computes a deterministic hash of window parameters.
// This ensures the cursor is invalidated if window parameters change.
func ComputeWindowHash(start, end time.Time) string {
	input := start.UTC().Format(time.RFC3339) + "|" + end.UTC().Format(time.RFC3339)
	hash := sha256.Sum256([]byte(input))
	return base64.URLEncoding.EncodeToString(hash[:8])
}

// ValidateCursor validates that a cursor is valid for the given window.
func ValidateCursor(cursor string, windowStart, windowEnd time.Time) error {
	if cursor == "" {
		return nil // First page, no cursor to validate
	}

	data, err := DecodeCursor(cursor)
	if err != nil {
		return err
	}

	expectedHash := ComputeWindowHash(windowStart, windowEnd)
	if data.WindowHash != expectedHash {
		return errors.New("cursor is stale: window parameters changed")
	}

	return nil
}

// NormalizeLimit ensures the limit is within valid bounds.
func NormalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}

// Paginator handles deterministic pagination of slices.
type Paginator[T any] struct {
	items       []T
	limit       int
	windowHash  string
	sortKeyFunc func(T) string
}

// NewPaginator creates a new paginator for a slice of items.
func NewPaginator[T any](items []T, windowStart, windowEnd time.Time) *Paginator[T] {
	return &Paginator[T]{
		items:      items,
		limit:      DefaultLimit,
		windowHash: ComputeWindowHash(windowStart, windowEnd),
	}
}

// WithLimit sets the page size limit.
func (p *Paginator[T]) WithLimit(limit int) *Paginator[T] {
	p.limit = NormalizeLimit(limit)
	return p
}

// WithSortKey sets the function to extract sort keys for keyset pagination.
func (p *Paginator[T]) WithSortKey(fn func(T) string) *Paginator[T] {
	p.sortKeyFunc = fn
	return p
}

// GetPage retrieves a page of items based on the cursor.
func (p *Paginator[T]) GetPage(cursor string) ([]T, PageResponse, error) {
	cursorData, err := DecodeCursor(cursor)
	if err != nil {
		return nil, PageResponse{}, err
	}

	// Validate window hash if cursor is not empty
	if cursor != "" && cursorData.WindowHash != p.windowHash {
		return nil, PageResponse{}, errors.New("cursor is stale: window parameters changed")
	}

	// Apply offset pagination
	offset := cursorData.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(p.items) {
		return []T{}, PageResponse{
			HasMore:    false,
			TotalCount: len(p.items),
			PageNumber: cursorData.PageNumber,
		}, nil
	}

	// Calculate end index
	endIdx := offset + p.limit
	if endIdx > len(p.items) {
		endIdx = len(p.items)
	}

	// Extract page items
	pageItems := p.items[offset:endIdx]

	// Build response
	response := PageResponse{
		HasMore:    endIdx < len(p.items),
		TotalCount: len(p.items),
		PageNumber: cursorData.PageNumber,
	}

	// Generate next cursor if more pages exist
	if response.HasMore {
		nextCursor := CursorData{
			Offset:     endIdx,
			WindowHash: p.windowHash,
			PageNumber: cursorData.PageNumber + 1,
			Version:    CursorVersion,
		}

		// Add sort key if available
		if p.sortKeyFunc != nil && len(pageItems) > 0 {
			lastItem := pageItems[len(pageItems)-1]
			nextCursor.SortKey = p.sortKeyFunc(lastItem)
		}

		response.NextCursor, err = EncodeCursor(nextCursor)
		if err != nil {
			return nil, PageResponse{}, err
		}
	}

	return pageItems, response, nil
}
