# Intelligence Layer Architecture

**Version**: 1.0
**Status**: Approved
**Last Updated**: 2024-01-15

## Executive Summary

This document defines the Intelligence Layer architecture for QuantumLife Canon. The Intelligence Layer enables context-aware assistance through Retrieval-Augmented Generation (RAG), vector embeddings, and Model Context Protocol (MCP) integration.

**Critical Constraint**: The Intelligence Layer operates in the Sense and Model phases only. The Execute phase remains deterministic and does not use ML/AI for decision-making.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Intelligence Layer                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐              │
│  │  Embedding   │    │   Vector    │    │  Retrieval  │              │
│  │  Provider    │───▶│   Store     │───▶│   Engine    │              │
│  └─────────────┘    └─────────────┘    └─────────────┘              │
│         │                                      │                      │
│         │                                      ▼                      │
│         │           ┌─────────────────────────────────┐              │
│         │           │      Memory Store (RAG)          │              │
│         │           │  ┌─────────┐  ┌─────────────┐   │              │
│         └──────────▶│  │ Chunks  │  │ Summaries   │   │              │
│                     │  └─────────┘  └─────────────┘   │              │
│                     └─────────────────────────────────┘              │
│                                      │                                │
│                                      ▼                                │
│                     ┌─────────────────────────────────┐              │
│                     │      MCP Context Provider        │              │
│                     └─────────────────────────────────┘              │
│                                                                       │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
                    ┌─────────────────────────────┐
                    │     Decide Layer (Rules)     │
                    │   (Deterministic, No ML)     │
                    └─────────────────────────────┘
```

## Design Decisions

### 1. RAG Strategy

**Decision**: Use local embedding and retrieval with optional cloud fallback.

| Aspect | Decision | Rationale |
|--------|----------|-----------|
| **Embedding Model** | Local-first (e.g., sentence-transformers) | Privacy, offline capability |
| **Vector Store** | In-process (e.g., Hnswlib, SQLite-VSS) | No external dependencies for MVP |
| **Chunk Strategy** | Semantic chunking with overlap | Better retrieval quality |
| **Re-ranking** | Optional, disabled by default | Simplicity first |

### 2. Vector Database

**Decision**: Interface-based design allowing pluggable backends.

| Option | Use Case | Trade-offs |
|--------|----------|------------|
| **In-Memory** | Development, testing | Fast, but no persistence |
| **SQLite-VSS** | Single-user, local | Persistent, no server |
| **PostgreSQL+pgvector** | Multi-user, cloud | Scalable, requires server |
| **Pinecone/Weaviate** | Enterprise, managed | Cloud-only, cost |

**MVP Implementation**: In-memory with optional SQLite persistence.

### 3. MCP Integration

**Decision**: Implement MCP server for Claude Code integration.

| Component | Purpose |
|-----------|---------|
| **MCP Server** | Expose QuantumLife context to Claude |
| **Resources** | View snapshots, circle summaries |
| **Tools** | None (read-only principle) |
| **Prompts** | Context-aware prompt templates |

### 4. Canon Boundary Enforcement

**CRITICAL**: The Intelligence Layer must NOT influence Execute phase decisions.

| Phase | Intelligence Usage | Enforcement |
|-------|-------------------|-------------|
| **Sense** | Embedding for similarity | Allowed |
| **Model** | RAG for context | Allowed |
| **Decide** | Rule engine only | No ML, deterministic |
| **Propose** | Template-based | No ML generation |
| **Execute** | Registry locks, caps | No ML, deterministic |

## Interface Definitions

### Memory Retrieval

```go
// Retriever retrieves relevant memories for a query.
type Retriever interface {
    // Retrieve finds memories relevant to the query.
    // Returns ranked results with similarity scores.
    Retrieve(ctx context.Context, query Query) ([]RetrievalResult, error)
}

// Query specifies what to retrieve.
type Query struct {
    Text      string            // Natural language query
    CircleID  string            // Scope to specific circle
    TimeRange *TimeRange        // Optional time constraints
    Limit     int               // Maximum results
    MinScore  float64           // Minimum similarity score
    Types     []MemoryType      // Filter by memory type
}

// RetrievalResult is a single retrieval result.
type RetrievalResult struct {
    Memory     Memory    // The retrieved memory
    Score      float64   // Similarity score (0-1)
    Highlights []string  // Matching text snippets
}
```

### Memory Storage

```go
// MemoryStore stores and manages memories.
type MemoryStore interface {
    // Store adds a memory to the store.
    Store(ctx context.Context, memory Memory) error

    // Get retrieves a memory by ID.
    Get(ctx context.Context, id string) (Memory, error)

    // Delete removes a memory.
    Delete(ctx context.Context, id string) error

    // List retrieves memories matching criteria.
    List(ctx context.Context, filter MemoryFilter) ([]Memory, error)
}

// Memory represents a stored memory.
type Memory struct {
    ID          string          // Unique identifier
    CircleID    string          // Owning circle
    Type        MemoryType      // Type of memory
    Content     string          // Text content
    Embedding   []float32       // Vector embedding
    Metadata    map[string]any  // Additional metadata
    CreatedAt   time.Time       // When created
    SourceEvent string          // Originating event ID
}

// MemoryType categorizes memories.
type MemoryType string

const (
    MemoryTypeEvent      MemoryType = "event"       // From ingested events
    MemoryTypeSummary    MemoryType = "summary"     // Generated summaries
    MemoryTypeNote       MemoryType = "note"        // User notes
    MemoryTypeObservation MemoryType = "observation" // System observations
)
```

### Embedding Provider

```go
// EmbeddingProvider generates embeddings for text.
type EmbeddingProvider interface {
    // Embed generates an embedding for the text.
    Embed(ctx context.Context, text string) ([]float32, error)

    // EmbedBatch generates embeddings for multiple texts.
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

    // Dimensions returns the embedding dimension.
    Dimensions() int

    // Model returns the model identifier.
    Model() string
}
```

### MCP Resources

```go
// MCPResourceProvider provides MCP resources.
type MCPResourceProvider interface {
    // ListResources returns available resources.
    ListResources(ctx context.Context) ([]MCPResource, error)

    // ReadResource reads a specific resource.
    ReadResource(ctx context.Context, uri string) (MCPResourceContent, error)
}

// MCPResource describes an MCP resource.
type MCPResource struct {
    URI         string // Resource URI
    Name        string // Human-readable name
    Description string // Resource description
    MimeType    string // Content type
}

// MCPResourceContent is the content of a resource.
type MCPResourceContent struct {
    URI      string // Resource URI
    MimeType string // Content type
    Text     string // Text content (if text)
    Blob     []byte // Binary content (if binary)
}
```

## Implementation Roadmap

### Phase 1: Interfaces (Current)

- Define `pkg/domain/memory/interfaces.go`
- No implementation, interfaces only
- ADR-0018 documentation

### Phase 2: In-Memory Implementation

- `pkg/domain/memory/impl_inmem/` - In-memory store
- Basic vector similarity (cosine distance)
- No external dependencies

### Phase 3: Embedding Integration

- Local embedding provider (future)
- Cloud fallback option (future)
- Batch processing

### Phase 4: MCP Server

- MCP server implementation
- Resource providers for views
- Integration with Claude Code

## Security Considerations

### Data Isolation

- Embeddings are circle-scoped
- No cross-circle retrieval without explicit permission
- Memory deletion respects data retention policies

### Privacy

- Local-first embedding to avoid sending data externally
- Optional cloud fallback must be explicit opt-in
- No training on user data

### Audit Trail

- All retrievals are logged
- Embedding operations are audited
- MCP access is tracked

## Non-Goals

The Intelligence Layer explicitly does NOT:

1. **Execute Actions**: No ML-driven execution
2. **Override Rules**: Rule engine remains deterministic
3. **Generate Proposals**: Proposals are template-based
4. **Access Credentials**: No access to token broker
5. **Modify Events**: Events are immutable

## References

- [ADR-0018: Intelligence Layer - RAG, Vector, MCP](ADR/ADR-0018-intelligence-layer-rag-vector-mcp.md)
- [CANON_CORE_V1.md](CANON_CORE_V1.md) - Canon principles
- [Model Context Protocol](https://modelcontextprotocol.io/) - MCP specification
