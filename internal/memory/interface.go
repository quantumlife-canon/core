// Package memory provides persistence for circle and intersection state.
//
// Canon Reference: docs/QUANTUMLIFE_CANON_V1.md §Ontology (Memory)
// Technical Split Reference: docs/TECHNICAL_SPLIT_V1.md §3.6 Memory Layer
package memory

import (
	"context"

	"quantumlife/pkg/primitives"
)

// Store provides authoritative state storage.
// Backed by PostgreSQL.
type Store interface {
	// Circle memory operations

	// WriteCircleMemory writes to a circle's memory.
	WriteCircleMemory(ctx context.Context, circleID string, entry MemoryEntry) error

	// ReadCircleMemory reads from a circle's memory.
	ReadCircleMemory(ctx context.Context, circleID string, key string) (*MemoryEntry, error)

	// ListCircleMemory lists entries in a circle's memory.
	ListCircleMemory(ctx context.Context, circleID string, filter Filter) ([]MemoryEntry, error)

	// DeleteCircleMemory deletes from a circle's memory.
	DeleteCircleMemory(ctx context.Context, circleID string, key string) error

	// Intersection memory operations

	// WriteIntersectionMemory writes to an intersection's shared memory.
	WriteIntersectionMemory(ctx context.Context, intersectionID string, entry MemoryEntry) error

	// ReadIntersectionMemory reads from an intersection's shared memory.
	ReadIntersectionMemory(ctx context.Context, intersectionID string, key string) (*MemoryEntry, error)

	// Version operations

	// GetVersion returns the current version of a memory entry.
	GetVersion(ctx context.Context, ownerID string, key string) (int, error)

	// GetHistory returns version history for a memory entry.
	GetHistory(ctx context.Context, ownerID string, key string) ([]MemoryEntry, error)
}

// SemanticIndex provides assistive semantic search.
// Backed by Qdrant. NOT authoritative — regenerable from Store.
type SemanticIndex interface {
	// Index adds or updates an entry in the semantic index.
	Index(ctx context.Context, circleID string, entry IndexEntry) error

	// Search performs semantic search over a circle's memory.
	Search(ctx context.Context, circleID string, query SearchQuery) ([]SearchResult, error)

	// Delete removes an entry from the semantic index.
	Delete(ctx context.Context, circleID string, entryID string) error

	// Rebuild regenerates the index from authoritative store.
	Rebuild(ctx context.Context, circleID string) error
}

// Cache provides ephemeral caching.
// Backed by Redis. Non-authoritative — evictable without data loss.
type Cache interface {
	// Get retrieves a cached value.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value in cache with TTL.
	Set(ctx context.Context, key string, value []byte, ttlSeconds int) error

	// Delete removes a value from cache.
	Delete(ctx context.Context, key string) error

	// Invalidate removes all cached values for an owner.
	Invalidate(ctx context.Context, ownerID string) error
}

// LoopMemoryUpdater provides loop-aware memory updates.
// Used by the orchestrator at step 7 (Memory Update) of the Irreducible Loop.
type LoopMemoryUpdater interface {
	// RecordLoopOutcome records the outcome of a complete loop traversal.
	// This creates an immutable audit record of what happened.
	RecordLoopOutcome(ctx context.Context, loopCtx LoopContext, outcome LoopOutcome) (*MemoryRecord, error)
}

// LoopContext is imported from primitives for loop threading.
type LoopContext = primitives.LoopContext

// LoopOutcome contains the outcome of a loop to be recorded.
type LoopOutcome struct {
	TraceID       string
	Success       bool
	FinalStep     string
	IntentID      string
	ActionID      string
	SettlementID  string
	FailureReason string
	Metadata      map[string]string
}

// MemoryRecord contains confirmation of a memory update.
type MemoryRecord struct {
	RecordID string
	TraceID  string
	StoredAt string
}
