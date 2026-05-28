package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	eywa "github.com/wmulabs/eywa"
	eywamongo "github.com/wmulabs/eywa/mongo"
	eywaopenai "github.com/wmulabs/eywa/providers/openai"
	eywaredis "github.com/wmulabs/eywa/redis"
)

// Example 06: RAG with Lore
//
// Demonstrates Lore — Eywa's built-in knowledge base with vector search.
//
// Key concepts:
// - LoreRepository stores Lore metadata in MongoDB
// - LoreStore stores and searches chunk embeddings (MongoDB Atlas Vector Search)
// - LoreEmbedder converts text to vectors (implemented here with OpenAI)
// - SearchLoreAction lets the Oracle query the knowledge base
// - Lore.SpiritIDs links which spirits have access to which lore bases

func main() {
	ctx := context.Background()
	fmt.Println("=== Eywa — RAG with Lore ===")

	mongoURL := getEnv("MONGO_URL", "mongodb://localhost:27017")
	mongoDatabase := getEnv("MONGO_DATABASE", "eywa_example")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	environment := getEnv("ENVIRONMENT", "lcl")
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

	spiritRepo := eywamongo.NewSpiritRepository(mongoConn.GetDatabase())
	memoryRepo := eywaredis.NewMemoryRepository(redisConn.GetClient(), serviceName, environment, 3600, nil)
	echoRepo := eywamongo.NewEchoRepository(mongoConn.GetDatabase())
	chronicleRepo := eywamongo.NewChronicleRepository(mongoConn.GetDatabase())
	bond := eywaredis.NewBondManager(redisConn.GetClient())

	loreRepo := eywamongo.NewLoreRepository(mongoConn.GetDatabase())
	loreStore := eywamongo.NewLoreStore(mongoConn.GetDatabase())
	embedder := &openAIEmbedder{apiKey: mustEnv("OPENAI_API_KEY")}

	// SearchLoreAction is a built-in action: the Oracle calls it to retrieve
	// relevant chunks from any Lore base the spirit has access to.
	actionRegistry := eywa.NewActionRegistry()
	if err := actionRegistry.Register(eywa.NewSearchLoreAction(loreRepo, loreStore, embedder)); err != nil {
		log.Fatalf("register search_lore: %v", err)
	}

	weave, err := eywa.NewWeaveBuilder(ctx).
		WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
		WithBond(bond).
		WithActionRegistry(actionRegistry).
		WithScoutRegistry(eywa.NewScoutRegistry()).
		WithLoreRepository(loreRepo).
		WithLoreStore(loreStore).
		WithLoreEmbedder(embedder).
		AddOracle(eywaopenai.NewOracle(mustEnv("OPENAI_API_KEY"))).
		WithConfig(eywa.DefaultWeaveConfig()).
		Build()
	if err != nil {
		log.Fatalf("build weave: %v", err)
	}

	// ── Create the knowledge base ─────────────────────────────────────────────

	spirit := &eywa.Spirit{
		Name:         "support_spirit",
		Description:  "Customer support spirit backed by a product knowledge base",
		SystemPrompt: "You are a helpful support agent. Use the search_lore action to find relevant information before answering questions about our product.",
		ModelConfig: eywa.SpiritModel{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.3,
			MaxTokens:   500,
		},
		AllowedActions: []eywa.AllowedAction{{Name: "search_lore"}},
		IsActive:       true,
		CreatedAt:      time.Now(),
	}
	if err := spiritRepo.Create(ctx, spirit); err != nil {
		fmt.Printf("note: spirit already exists: %v\n", err)
	}

	// Lore is a named knowledge base. SpiritIDs controls which spirits can access it.
	lore := eywa.Lore{
		ID:          "product-kb-001",
		Name:        "product_knowledge",
		Description: "Product documentation and FAQs",
		SpiritIDs:   []string{spirit.Name},
		ChunkSize:   500,
		Overlap:     50,
	}
	if err := loreRepo.Create(ctx, lore); err != nil {
		fmt.Printf("note: lore already exists: %v\n", err)
	}

	// Ingest knowledge chunks.
	// In production, these come from your docs pipeline — PDFs, web pages, etc.
	docs := []string{
		"Eywa supports multi-Spirit orchestration with parallel execution. Multiple spirits can work concurrently on the same request, with results aggregated.",
		"The Vigil feature enables human takeover of any active conversation. An operator acquires a seat via Redis, taking full control until they release it.",
		"Lore is Eywa's built-in RAG knowledge base. You define lore bases, ingest chunks with embeddings, and spirits retrieve relevant context automatically via vector search.",
		"Ledger tracks token usage and costs per spirit. You can set monthly budgets, configure automatic model downgrade when limits are approached, and receive alerts.",
		"Conduit connects Eywa to external MCP servers. Tools are auto-discovered on startup and registered as first-class actions — no manual wiring required.",
	}

	fmt.Println("\nIngesting knowledge base...")
	chunks := make([]eywa.LoreChunk, 0, len(docs))
	for i, doc := range docs {
		embeddings, err := embedder.Embed(ctx, []string{doc})
		if err != nil {
			log.Fatalf("embed doc %d: %v", i, err)
		}
		chunks = append(chunks, eywa.LoreChunk{
			ID:        fmt.Sprintf("chunk-%d", i),
			LoreID:    lore.ID,
			Content:   doc,
			Embedding: embeddings[0],
		})
	}
	if err := loreStore.Upsert(ctx, chunks); err != nil {
		log.Fatalf("upsert chunks: %v", err)
	}
	fmt.Printf("Ingested %d chunks\n", len(chunks))

	weave.RegisterEventConfiguration(
		eywa.NewLink("support_query").
			WithDefaultSpirit("support_spirit").
			Build(),
	)

	// ── Query ─────────────────────────────────────────────────────────────────

	queries := []string{
		"How does human takeover work in Eywa?",
		"What is Lore and how do I use it for RAG?",
	}

	for _, q := range queries {
		fmt.Printf("\n--- Query: %s ---\n", q)
		pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: "user_rag"}).
			WithUserMessage(q).
			WithSource("api").
			Build()

		result, err := weave.ProcessEventByKey(ctx, "support_query", pulse)
		if err != nil {
			log.Printf("error: %v", err)
			continue
		}
		fmt.Printf("Actions: %v\n", result.ActionsExecuted)
		fmt.Printf("Reply:   %s\n", result.Message)
	}
}

// ── OpenAI Embedder ───────────────────────────────────────────────────────────
//
// LoreEmbedder implementation using OpenAI text-embedding-3-small.
// In production you can swap this for any embedding model by implementing
// the eywa.LoreEmbedder interface.

type openAIEmbedder struct {
	apiKey string
}

func (e *openAIEmbedder) Dimensions() int { return 1536 }

func (e *openAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	// For this example, return random embeddings so it runs without Atlas Vector Search.
	// Replace with a real embedding API call in production.
	//
	// Real implementation (openai-go):
	//   client := openai.NewClient(option.WithAPIKey(e.apiKey))
	//   resp, err := client.Embeddings.New(ctx, openai.EmbeddingNewParams{
	//       Input: openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: texts},
	//       Model: openai.EmbeddingModelTextEmbedding3Small,
	//   })
	//   // map resp.Data[i].Embedding to []float32
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
