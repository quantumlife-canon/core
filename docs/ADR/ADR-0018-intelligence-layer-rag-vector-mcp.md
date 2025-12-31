# ADR-0018: Intelligence Layer - RAG, Vector Database, and MCP

## Status

Accepted

## Context

QuantumLife Canon needs context-aware assistance to provide relevant observations and help users understand patterns in their data. This requires:

1. **Retrieval-Augmented Generation (RAG)**: Find relevant past events/memories when generating observations
2. **Vector Database**: Store and query embeddings for semantic similarity
3. **Model Context Protocol (MCP)**: Integrate with Claude Code and other AI assistants

We must balance these capabilities against Canon's core principle: deterministic execution with no ML in the Execute phase.

## Decision

### 1. Scope Restriction

The Intelligence Layer operates ONLY in Sense and Model phases:

```
Sense → Model → [INTELLIGENCE BOUNDARY] → Decide → Propose → Approve → Execute
         ↑                                    ↓
    RAG/Embedding                        Deterministic
    allowed here                         rules only
```

### 2. Interface-First Design

Define interfaces without implementation to allow pluggable backends:

```go
// pkg/domain/memory/interfaces.go

type Retriever interface {
    Retrieve(ctx context.Context, query Query) ([]RetrievalResult, error)
}

type MemoryStore interface {
    Store(ctx context.Context, memory Memory) error
    Get(ctx context.Context, id string) (Memory, error)
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, filter MemoryFilter) ([]Memory, error)
}

type EmbeddingProvider interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    Dimensions() int
    Model() string
}
```

### 3. Local-First Strategy

Prefer local processing over cloud:

| Component | Local Option | Cloud Fallback |
|-----------|--------------|----------------|
| Embeddings | sentence-transformers | OpenAI/Anthropic |
| Vector Store | SQLite-VSS | Pinecone |
| Retrieval | In-process | Managed service |

### 4. MCP Integration

Implement MCP server for Claude Code integration:

- **Resources**: Expose view snapshots, circle summaries
- **Tools**: None (read-only principle maintained)
- **Prompts**: Context-aware templates

### 5. Memory Types

Define structured memory types:

```go
type MemoryType string

const (
    MemoryTypeEvent      MemoryType = "event"       // From ingested events
    MemoryTypeSummary    MemoryType = "summary"     // Generated summaries
    MemoryTypeNote       MemoryType = "note"        // User notes
    MemoryTypeObservation MemoryType = "observation" // System observations
)
```

## Consequences

### Positive

1. **Flexibility**: Interface-based design allows swapping implementations
2. **Privacy**: Local-first respects user data sovereignty
3. **Canon Compliance**: Clear boundary prevents ML in Execute phase
4. **Integration**: MCP enables Claude Code and other assistants

### Negative

1. **Complexity**: Multiple interfaces to implement
2. **Performance**: Local embedding may be slower than cloud
3. **Maintenance**: Must keep up with MCP specification changes

### Neutral

1. **MVP Scope**: Initial implementation is in-memory only
2. **Embedding Choice**: Deferred to implementation phase

## Implementation

### Phase 1: Interfaces (This ADR)

Create `pkg/domain/memory/interfaces.go` with:
- `Retriever` interface
- `MemoryStore` interface
- `EmbeddingProvider` interface
- `MCPResourceProvider` interface
- Supporting types (`Memory`, `Query`, `RetrievalResult`, etc.)

### Phase 2: In-Memory Implementation

Create `pkg/domain/memory/impl_inmem/`:
- Basic vector store with cosine similarity
- No external dependencies
- Suitable for development and testing

### Phase 3: Persistence

Add SQLite-VSS backend:
- Persistent storage
- Cross-session retrieval
- Backup/restore support

### Phase 4: MCP Server

Implement MCP server:
- Resource listing
- Resource reading
- Integration tests with Claude Code

## Alternatives Considered

### 1. Full Cloud RAG

**Rejected**: Violates privacy-first principle and requires constant connectivity.

### 2. No Intelligence Layer

**Rejected**: Limits usefulness of the system for pattern recognition and context.

### 3. ML in Execute Phase

**Rejected**: Violates Canon determinism requirement. Execution must be predictable.

### 4. Custom Vector Format

**Rejected**: Standard embedding formats enable interoperability.

## References

- [INTELLIGENCE_LAYER_V1.md](../INTELLIGENCE_LAYER_V1.md) - Full specification
- [Model Context Protocol](https://modelcontextprotocol.io/) - MCP specification
- [CANON_CORE_V1.md](../CANON_CORE_V1.md) - Canon principles
