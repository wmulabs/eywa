package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"

	eywa "github.com/wmulabs/eywa"
	eywamongo "github.com/wmulabs/eywa/mongo"
	eywaopenai "github.com/wmulabs/eywa/providers/openai"
	eywaredis "github.com/wmulabs/eywa/redis"
)

// Example 14: Lore as a queryable store (matching)
//
// Example 06 shows Lore as RAG the Oracle pulls from mid-turn. This example shows the OTHER face of
// Lore: a queryable vector store you drive directly, out of any turn — for matching, deduplication,
// recommendation, or any "find the closest records" lookup.
//
// Key concepts:
//   - IngestObject — index a structured record by verbalizing it into natural language via an Oracle
//     (better for embeddings than raw JSON), keeping every field as filterable metadata.
//   - SearchLore — a direct semantic query that returns scored results (no Spirit, no reasoning loop).
//   - LoreSearchOptions.Filter — constrain the search by metadata (equality + numeric ranges).
//   - LoreSearchOptions.GroupByDocument — collapse chunks back to distinct objects.
func main() {
	ctx := context.Background()
	fmt.Println("=== Eywa — Lore Matching (queryable store) ===")

	mongoURL := getEnv("MONGO_URL", "mongodb://localhost:27017")
	mongoDatabase := getEnv("MONGO_DATABASE", "eywa_example")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	serviceName := getEnv("SERVICE_NAME", "eywa")

	mongoConn, err := eywamongo.NewMongoConnection(ctx, mongoURL, mongoDatabase, serviceName)
	if err != nil {
		log.Fatalf("failed to connect to MongoDB: %v", err)
	}
	defer mongoConn.DisconnectMongoDB(ctx)

	redisConn, err := eywaredis.NewRedisConnection(ctx, redisURL, serviceName)
	if err != nil {
		log.Fatalf("failed to connect to Redis: %v", err)
	}
	defer redisConn.DisconnectRedisDB(ctx)

	loreRepo := eywamongo.NewLoreRepository(mongoConn.GetDatabase())
	loreStore := eywamongo.NewLoreStore(mongoConn.GetDatabase())
	embedder := &openAIEmbedder{apiKey: mustEnv("OPENAI_API_KEY")}

	weave, err := eywa.NewWeaveBuilder(ctx).
		WithRepositories(
			eywamongo.NewSpiritRepository(mongoConn.GetDatabase()),
			eywaredis.NewMemoryRepository(redisConn.GetClient(), serviceName, "lcl", 3600, nil),
			eywamongo.NewEchoRepository(mongoConn.GetDatabase()),
			eywamongo.NewChronicleRepository(mongoConn.GetDatabase()),
		).
		WithBond(eywaredis.NewBondManager(redisConn.GetClient())).
		WithLoreRepository(loreRepo).
		WithLoreStore(loreStore).
		WithLoreEmbedder(embedder).
		AddOracle(eywaopenai.NewOracle(mustEnv("OPENAI_API_KEY"))).
		Build()
	if err != nil {
		log.Fatalf("build weave: %v", err)
	}

	const loreID = "catalog-001"
	if err := loreRepo.Create(ctx, eywa.Lore{ID: loreID, Name: "service_catalog"}); err != nil {
		fmt.Printf("note: lore already exists: %v\n", err)
	}

	// ── Index structured records ──────────────────────────────────────────────
	//
	// Each record is a map of fields. IngestObject verbalizes it via the Oracle into a single
	// canonical description, embeds that, and stores the original fields as filterable metadata.
	// DocumentID makes re-ingesting the same record an in-place upsert.
	records := []map[string]any{
		{"id": "svc-1", "name": "Helios Analytics", "category": "analytics", "region": "eu", "monthly_price": 49, "rating": 4.6},
		{"id": "svc-2", "name": "Nimbus Storage", "category": "storage", "region": "us", "monthly_price": 19, "rating": 4.1},
		{"id": "svc-3", "name": "Aurora Insights", "category": "analytics", "region": "eu", "monthly_price": 120, "rating": 4.8},
		{"id": "svc-4", "name": "Comet Metrics", "category": "analytics", "region": "us", "monthly_price": 35, "rating": 4.3},
	}

	fmt.Println("\nIndexing records...")
	for _, rec := range records {
		err := weave.IngestObject(ctx, loreID, rec, eywa.IngestObjectOptions{
			DocumentID: rec["id"].(string),
			Provider:   "openai",
			Model:      "gpt-4o-mini",
		})
		if err != nil {
			log.Fatalf("ingest %s: %v", rec["id"], err)
		}
	}
	fmt.Printf("Indexed %d records\n", len(records))

	// ── 1. Plain semantic match ───────────────────────────────────────────────
	printMatches(ctx, weave, loreID, "affordable analytics tool", eywa.LoreSearchOptions{TopK: 3})

	// ── 2. Semantic match + metadata filter ───────────────────────────────────
	// Only analytics services in the EU priced at 100 or below.
	maxPrice := 100.0
	printMatches(ctx, weave, loreID, "analytics platform for dashboards", eywa.LoreSearchOptions{
		TopK: 3,
		Filter: &eywa.LoreFilter{
			Equals: map[string]any{"category": "analytics", "region": "eu"},
			Ranges: map[string]eywa.LoreRange{"monthly_price": {Max: &maxPrice}},
		},
	})

	// ── 3. Distinct objects (group chunks by document) ────────────────────────
	printMatches(ctx, weave, loreID, "data tooling", eywa.LoreSearchOptions{TopK: 3, GroupByDocument: true})
}

func printMatches(ctx context.Context, weave *eywa.Weave, loreID, query string, opts eywa.LoreSearchOptions) {
	fmt.Printf("\n--- Query: %q ---\n", query)
	matches, err := weave.SearchLore(ctx, loreID, query, opts)
	if err != nil {
		log.Printf("search error: %v", err)
		return
	}
	for _, m := range matches {
		fmt.Printf("  [%.3f] %v — %s\n", m.Score, m.Metadata["name"], m.Content)
	}
}

// ── OpenAI Embedder ───────────────────────────────────────────────────────────
//
// LoreEmbedder implementation. This stub returns random vectors so the example compiles and runs
// without MongoDB Atlas Vector Search; swap it for a real embedding API (e.g. OpenAI
// text-embedding-3-small) to get meaningful match scores.
type openAIEmbedder struct {
	apiKey string
}

func (e *openAIEmbedder) Dimensions() int { return 1536 }

func (e *openAIEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		vec := make([]float32, e.Dimensions())
		for j := range vec {
			vec[j] = rand.Float32()
		}
		embeddings[i] = vec
	}
	return embeddings, nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is required", key)
	}
	return v
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
