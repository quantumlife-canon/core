package memory

import (
	"time"
)

// MemoryEntry represents a versioned memory entry.
type MemoryEntry struct {
	ID        string
	OwnerID   string // Circle ID or Intersection ID
	OwnerType string // "circle" or "intersection"
	Key       string
	Value     []byte
	Version   int
	Encrypted bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Filter specifies criteria for listing memory entries.
type Filter struct {
	Prefix string
	After  time.Time
	Before time.Time
	Limit  int
	Offset int
}

// IndexEntry represents an entry for semantic indexing.
type IndexEntry struct {
	EntryID   string
	Content   string
	Embedding []float32
	Category  string
	Timestamp time.Time
	Metadata  map[string]string
}

// SearchQuery specifies a semantic search query.
type SearchQuery struct {
	QueryText      string
	QueryEmbedding []float32
	IntersectionID string // Optional: scope to intersection
	Category       string // Optional: filter by category
	After          time.Time
	Before         time.Time
	Limit          int
}

// SearchResult represents a semantic search result.
type SearchResult struct {
	EntryID   string
	Score     float32
	Content   string
	Category  string
	Timestamp time.Time
	Metadata  map[string]string
}
