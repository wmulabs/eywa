# Example 14: Lore Matching (queryable store)

Example 06 shows **Lore as RAG** — knowledge the Oracle pulls in mid-turn. This example shows the
other face of Lore: a **queryable vector store you drive directly**, outside any turn, for matching,
deduplication, recommendation, or any "find the closest records" lookup.

## What this shows

- `IngestObject` — index a **structured record** by verbalizing it into natural language via an Oracle,
  keeping every field as filterable metadata
- `SearchLore` — a direct semantic query that returns **scored** results (no Spirit, no reasoning loop)
- `LoreSearchOptions.Filter` — constrain the search by metadata (equality + numeric ranges)
- `LoreSearchOptions.GroupByDocument` — collapse chunks back to **distinct objects**

## Key concepts

### IngestObject — structured records as searchable text

Embedding raw JSON matches poorly. `IngestObject` first asks an Oracle to turn the record into a
canonical, factual description (data-to-text *verbalization*, the technique behind Anthropic's
Contextual Retrieval), embeds that, and stores the original fields as metadata you can filter on.

```go
weave.IngestObject(ctx, "catalog-001", map[string]any{
    "id": "svc-1", "name": "Helios Analytics", "category": "analytics",
    "region": "eu", "monthly_price": 49, "rating": 4.6,
}, eywa.IngestObjectOptions{
    DocumentID: "svc-1",   // re-ingesting the same ID upserts in place
    Provider:   "openai",
    Model:      "gpt-4o-mini",
})
```

### SearchLore — direct, scored, filtered matching

```go
maxPrice := 100.0
matches, _ := weave.SearchLore(ctx, "catalog-001", "analytics platform for dashboards",
    eywa.LoreSearchOptions{
        TopK: 3,
        Filter: &eywa.LoreFilter{
            Equals: map[string]any{"category": "analytics", "region": "eu"},
            Ranges: map[string]eywa.LoreRange{"monthly_price": {Max: &maxPrice}},
        },
    })
// each match carries .Score and .Metadata
```

Metadata filtering needs a store that implements `FilterableLoreStore` (pgvector, Qdrant, Pinecone).
On a store without it the filter is ignored and a plain vector search runs.

### GroupByDocument — distinct objects, not chunks

When a document is chunked, the top-K can be several chunks of the *same* object. `GroupByDocument`
over-fetches and collapses by document so you get the K best **distinct** records.

```go
weave.SearchLore(ctx, "catalog-001", "data tooling",
    eywa.LoreSearchOptions{TopK: 3, GroupByDocument: true})
```

## Running

```bash
export OPENAI_API_KEY=sk-...
export MONGO_URL=mongodb://localhost:27017
export REDIS_URL=redis://localhost:6379

go run .
```

> The embedder here returns random vectors so the example runs without MongoDB Atlas Vector Search.
> Swap it for a real embedding API and a vector store (pgvector/Qdrant/Pinecone) for meaningful scores.

## When to use which

| You want… | Use |
|---|---|
| The agent to pull knowledge mid-conversation | RAG via `search_lore` action (example 06) |
| To match/dedupe/recommend records yourself | `SearchLore` direct query (this example) |
| To index structured records, not prose | `IngestObject` |
