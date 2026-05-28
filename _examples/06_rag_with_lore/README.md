# Example 06: RAG with Lore

Demonstrates **Lore** — Eywa's built-in Retrieval-Augmented Generation (RAG) system.

## What this shows

- Ingesting documents into a `LoreRepository` (MongoDB full-text index)
- Implementing the `LoreEmbedder` port for vector embeddings
- Using the `SearchLoreAction` built-in action so a spirit can query its knowledge base
- Spirit referencing the action via `AllowedActions`

## Key concepts

### LoreRepository

Stores document chunks with optional vector embeddings. The MongoDB adapter uses full-text search; swap for `pgvector`, Qdrant, or Pinecone adapters for semantic similarity search.

### LoreEmbedder port

```go
type LoreEmbedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
}
```

This example provides a minimal implementation using OpenAI's `text-embedding-3-small`. In production, use a dedicated embedder adapter.

### SearchLoreAction

A built-in action (`search_lore`) the spirit calls at runtime. The spirit receives retrieved chunks as context before generating its response.

```go
spirit.AllowedActions = []eywa.AllowedAction{
    {Name: "search_lore"},
}
```

## Running

```bash
export OPENAI_API_KEY=sk-...
export MONGO_URL=mongodb://localhost:27017
export REDIS_URL=redis://localhost:6379

go run .
```

## Architecture note

Lore is designed as a **port** (`LoreEmbedder`, `LoreRepository`, `LoreStore`). The MongoDB adapter provides persistence; vector search adapters (pgvector, Qdrant, Pinecone, Weaviate) slot in without changing the spirit or action configuration.
