# eywa — Weaviate provider

A `LoreStore` adapter backed by [Weaviate](https://weaviate.io) for semantic Lore retrieval.

```go
import "github.com/wmulabs/eywa/providers/weaviate"

store, err := weaviate.NewLoreStore(ctx, weaviate.Config{
    Host:      "localhost:8080", // or "xyz.weaviate.network"
    ClassName: "LoreChunk",
    // APIKey / UseTLS for Weaviate Cloud
})
weave, _ := eywa.NewWeaveBuilder(ctx).WithLoreStore(store)./* … */Build()
```

Multiple Lore knowledge bases share one class; `lore_id` is stored per object and used to scope search.

## Metadata filtering

This adapter does **not** implement `FilterableLoreStore`. Chunk metadata is stored as a single JSON
`text` property, while Weaviate's `Where` filters operate on typed class properties — so per-field
filtering of arbitrary (dynamic) metadata isn't supported without a class-schema redesign.

`Weave.SearchLore` still works against Weaviate: when a `LoreFilter` is supplied it is **ignored**
(the capability layer falls back to plain vector search). For metadata-filtered matching
(`LoreFilter`, candidate-style queries), use a store that implements `FilterableLoreStore` —
pgvector, Qdrant, or Pinecone.
