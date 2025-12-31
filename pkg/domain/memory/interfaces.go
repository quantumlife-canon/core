// Package memory defines interfaces for the Intelligence Layer.
//
// CRITICAL: The Intelligence Layer operates ONLY in Sense and Model phases.
// The Execute phase remains deterministic and does not use ML/AI.
//
// Reference: docs/INTELLIGENCE_LAYER_V1.md
package memory

import (
	"context"
	"time"
)

// Retriever retrieves relevant memories for a query.
// Used in the Model phase for context-aware assistance.
type Retriever interface {
	// Retrieve finds memories relevant to the query.
	// Returns ranked results with similarity scores.
	Retrieve(ctx context.Context, query Query) ([]RetrievalResult, error)
}

// Query specifies what to retrieve.
type Query struct {
	// Text is the natural language query.
	Text string

	// CircleID scopes retrieval to a specific circle.
	// Empty string means all circles the user can access.
	CircleID string

	// TimeRange optionally constrains by time.
	TimeRange *TimeRange

	// Limit is the maximum number of results.
	Limit int

	// MinScore is the minimum similarity score (0-1).
	MinScore float64

	// Types filters by memory type.
	// Empty means all types.
	Types []MemoryType
}

// TimeRange specifies a time range.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// RetrievalResult is a single retrieval result.
type RetrievalResult struct {
	// Memory is the retrieved memory.
	Memory Memory

	// Score is the similarity score (0-1).
	Score float64

	// Highlights are matching text snippets.
	Highlights []string
}

// MemoryStore stores and manages memories.
// This is the persistence layer for the Intelligence Layer.
type MemoryStore interface {
	// Store adds a memory to the store.
	// If embedding is nil, the store may generate it.
	Store(ctx context.Context, memory Memory) error

	// Get retrieves a memory by ID.
	Get(ctx context.Context, id string) (Memory, error)

	// Delete removes a memory.
	Delete(ctx context.Context, id string) error

	// List retrieves memories matching criteria.
	List(ctx context.Context, filter MemoryFilter) ([]Memory, error)

	// Count returns the number of memories matching criteria.
	Count(ctx context.Context, filter MemoryFilter) (int, error)
}

// MemoryFilter specifies criteria for listing memories.
type MemoryFilter struct {
	// CircleID filters by circle.
	CircleID string

	// Types filters by memory type.
	Types []MemoryType

	// Since filters to memories created after this time.
	Since *time.Time

	// Until filters to memories created before this time.
	Until *time.Time

	// Limit is the maximum number of results.
	Limit int

	// Offset is the number of results to skip.
	Offset int
}

// Memory represents a stored memory.
type Memory struct {
	// ID uniquely identifies this memory.
	ID string

	// CircleID is the owning circle.
	CircleID string

	// Type categorizes this memory.
	Type MemoryType

	// Content is the text content.
	Content string

	// Embedding is the vector embedding.
	// May be nil if not yet computed.
	Embedding []float32

	// Metadata contains additional structured data.
	Metadata map[string]any

	// CreatedAt is when this memory was created.
	CreatedAt time.Time

	// UpdatedAt is when this memory was last updated.
	UpdatedAt time.Time

	// SourceEventID is the originating event ID, if any.
	SourceEventID string

	// ExpiresAt is when this memory should be deleted.
	// Zero means no expiration.
	ExpiresAt time.Time
}

// MemoryType categorizes memories.
type MemoryType string

// Memory types.
const (
	// MemoryTypeEvent is a memory derived from an ingested event.
	MemoryTypeEvent MemoryType = "event"

	// MemoryTypeSummary is a generated summary of events.
	MemoryTypeSummary MemoryType = "summary"

	// MemoryTypeNote is a user-created note.
	MemoryTypeNote MemoryType = "note"

	// MemoryTypeObservation is a system-generated observation.
	MemoryTypeObservation MemoryType = "observation"

	// MemoryTypePattern is a detected pattern across events.
	MemoryTypePattern MemoryType = "pattern"
)

// EmbeddingProvider generates embeddings for text.
// Used to convert text to vectors for similarity search.
type EmbeddingProvider interface {
	// Embed generates an embedding for the text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts.
	// More efficient than calling Embed multiple times.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the embedding dimension.
	Dimensions() int

	// Model returns the model identifier.
	Model() string
}

// VectorStore stores and queries vectors.
// This is the low-level vector storage interface.
type VectorStore interface {
	// Insert adds a vector to the store.
	Insert(ctx context.Context, id string, vector []float32, metadata map[string]any) error

	// Delete removes a vector from the store.
	Delete(ctx context.Context, id string) error

	// Search finds the k most similar vectors.
	Search(ctx context.Context, query []float32, k int, filter VectorFilter) ([]VectorResult, error)

	// Get retrieves a vector by ID.
	Get(ctx context.Context, id string) ([]float32, map[string]any, error)
}

// VectorFilter specifies criteria for vector search.
type VectorFilter struct {
	// CircleID filters by circle.
	CircleID string

	// Types filters by memory type.
	Types []MemoryType

	// MinScore is the minimum similarity score.
	MinScore float64
}

// VectorResult is a vector search result.
type VectorResult struct {
	// ID is the vector ID.
	ID string

	// Score is the similarity score (0-1).
	Score float64

	// Metadata is the stored metadata.
	Metadata map[string]any
}

// MCPResourceProvider provides MCP resources.
// Used for integration with Claude Code and other MCP clients.
type MCPResourceProvider interface {
	// ListResources returns available resources.
	ListResources(ctx context.Context) ([]MCPResource, error)

	// ReadResource reads a specific resource.
	ReadResource(ctx context.Context, uri string) (MCPResourceContent, error)
}

// MCPResource describes an MCP resource.
type MCPResource struct {
	// URI is the resource identifier.
	URI string

	// Name is a human-readable name.
	Name string

	// Description describes the resource.
	Description string

	// MimeType is the content type.
	MimeType string
}

// MCPResourceContent is the content of a resource.
type MCPResourceContent struct {
	// URI is the resource identifier.
	URI string

	// MimeType is the content type.
	MimeType string

	// Text is the text content (if text-based).
	Text string

	// Blob is the binary content (if binary).
	Blob []byte
}

// ChunkStrategy defines how to split text into chunks.
type ChunkStrategy string

// Chunk strategies.
const (
	// ChunkByParagraph splits on paragraph boundaries.
	ChunkByParagraph ChunkStrategy = "paragraph"

	// ChunkBySize splits at fixed size with overlap.
	ChunkBySize ChunkStrategy = "size"

	// ChunkBySentence splits on sentence boundaries.
	ChunkBySentence ChunkStrategy = "sentence"

	// ChunkSemantic uses semantic boundaries.
	ChunkSemantic ChunkStrategy = "semantic"
)

// ChunkConfig configures chunking behavior.
type ChunkConfig struct {
	// Strategy is the chunking strategy.
	Strategy ChunkStrategy

	// MaxChunkSize is the maximum chunk size in characters.
	MaxChunkSize int

	// Overlap is the overlap between chunks in characters.
	Overlap int
}

// Chunker splits text into chunks for embedding.
type Chunker interface {
	// Chunk splits text into chunks.
	Chunk(text string, config ChunkConfig) ([]Chunk, error)
}

// Chunk is a piece of text to be embedded.
type Chunk struct {
	// Text is the chunk content.
	Text string

	// Start is the start offset in the original text.
	Start int

	// End is the end offset in the original text.
	End int

	// Index is the chunk index.
	Index int
}
