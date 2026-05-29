package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	eywa "github.com/wmulabs/eywa"
	eywamongo "github.com/wmulabs/eywa/mongo"
	eywaopenai "github.com/wmulabs/eywa/providers/openai"
	eywaredis "github.com/wmulabs/eywa/redis"
)

// Example 03: Advanced Routing
//
// Demonstrates Scouts (context enrichment) and Pathfinders (Spirit selection).
//
// Pipeline steps highlighted here:
//  1–2. Validation → Lock
//  3.   Enrichment  — Scouts run concurrently, populate Pulse.Knowledge
//  4.   SpiritSelection — Pathfinder picks the Spirit from the allowed spirits list
//  5–9. Load → SessionSetup → Reasoning → Persistence → Chronicle
//
// Scenario: Customer service with 3 Spirits (support, sales, billing)

func main() {
	ctx := context.Background()
	fmt.Println("=== Eywa — Advanced Routing ===")

	// Infrastructure
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

	// Scout registry — Scouts enrich Pulse.Knowledge before Spirit selection
	scoutRegistry := eywa.NewScoutRegistry()
	if err := scoutRegistry.Register(NewUserContextScout()); err != nil {
		log.Fatalf("register user_context scout: %v", err)
	}
	if err := scoutRegistry.Register(NewSessionHistoryScout()); err != nil {
		log.Fatalf("register session_history scout: %v", err)
	}

	// Pathfinder registry — Pathfinder selects the Spirit when multiple are allowed
	pathfinderRegistry := eywa.NewPathfinderRegistry()
	if err := pathfinderRegistry.Register(NewKeywordPathfinder()); err != nil {
		log.Fatalf("register keyword_pathfinder: %v", err)
	}

	// Weave
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	config := eywa.DefaultWeaveConfig()
	config.ScoutTimeout = 15 * time.Second
	config.ReasoningTimeout = 60 * time.Second

	weave, err := eywa.NewWeaveBuilder(ctx).
		WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
		WithBond(bond).
		WithActionRegistry(eywa.NewActionRegistry()).
		WithScoutRegistry(scoutRegistry).
		WithPathfinderRegistry(pathfinderRegistry).
		AddOracle(eywaopenai.NewOracle(apiKey)).
		WithConfig(config).
		Build()
	if err != nil {
		log.Fatalf("failed to build Weave: %v", err)
	}

	// Three specialized Spirits — each with its own system prompt and model config
	for _, s := range []*eywa.Spirit{
		{
			Name:         "support_spirit",
			Description:  "Technical support specialist — handles crashes, errors, and bugs",
			SystemPrompt: "You are a technical support specialist. Help users troubleshoot problems step by step.",
			ModelConfig:  eywa.SpiritModel{Provider: "openai", Model: "gpt-4o-mini", Temperature: 0.5, MaxTokens: 1000},
			IsActive:     true, CreatedAt: time.Now(),
		},
		{
			Name:         "sales_spirit",
			Description:  "Sales specialist — handles product questions, pricing, and plans",
			SystemPrompt: "You are a friendly sales specialist. Help customers find the right products and plans.",
			ModelConfig:  eywa.SpiritModel{Provider: "openai", Model: "gpt-4o-mini", Temperature: 0.7, MaxTokens: 1000},
			IsActive:     true, CreatedAt: time.Now(),
		},
		{
			Name:         "billing_spirit",
			Description:  "Billing specialist — handles payments, invoices, and charges",
			SystemPrompt: "You are a billing specialist. Help with payment questions clearly and accurately.",
			ModelConfig:  eywa.SpiritModel{Provider: "openai", Model: "gpt-4o-mini", Temperature: 0.3, MaxTokens: 1000},
			IsActive:     true, CreatedAt: time.Now(),
		},
	} {
		if err := spiritRepo.Create(ctx, s); err != nil {
			fmt.Printf("note: spirit %s already exists: %v\n", s.Name, err)
		}
	}

	// Link: Scouts run first, then Pathfinder picks from the allowed spirits list
	weave.RegisterEventConfiguration(
		eywa.NewLink("customer_message").
			WithScouts("user_context", "session_history").
			WithPathfinder("keyword_pathfinder").
			WithSpirits("support_spirit", "sales_spirit", "billing_spirit").
			WithDefaultSpirit("support_spirit").
			Build(),
	)

	// Test scenarios — each should route to a different Spirit
	tests := []struct{ label, user, msg string }{
		{"Support routing", "user_001", "My app keeps crashing when I upload files. Can you help?"},
		{"Sales routing", "user_002", "I'm interested in the premium plan. What features does it include?"},
		{"Billing routing", "user_003", "I was charged twice for my subscription. Can you check my invoice?"},
	}

	for _, t := range tests {
		fmt.Printf("\n--- %s ---\n", t.label)
		result := processPulse(ctx, weave, t.user, t.msg)
		if result != nil {
			fmt.Printf("Spirit: %s\n", result.SpiritUsed)
			fmt.Printf("Reply:  %s\n", truncate(result.Message, 120))
		}
	}
}

func processPulse(ctx context.Context, weave *eywa.Weave, userID, message string) *eywa.Response {
	pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: userID}).
		WithUserMessage(message).
		WithSource("api").
		Build()

	result, err := weave.ProcessEventByKey(ctx, "customer_message", pulse)
	if err != nil {
		log.Printf("error: %v", err)
		return nil
	}
	return result
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ── UserContextScout ──────────────────────────────────────────────────────
// Simulates loading user profile data and injecting into Pulse.Knowledge.

type UserContextScout struct{}

func NewUserContextScout() eywa.Scout { return &UserContextScout{} }

func (s *UserContextScout) GetName() string { return "user_context" }

func (s *UserContextScout) IsApplicable(_ *eywa.Pulse) bool {
	return true // run for every Pulse in this example
}

func (s *UserContextScout) Harvest(_ context.Context, pulse *eywa.Pulse) error {
	time.Sleep(30 * time.Millisecond) // simulate DB lookup

	// In a real system you'd fetch the user from a DB using pulse.MemoryKey or ContactPhone.
	// Here we simulate: user_002 and user_003 get "standard", others get "standard" too.
	pulse.Knowledge["user_tier"] = "standard"
	pulse.Knowledge["user_priority"] = "normal"
	return nil
}

// ── SessionHistoryScout ───────────────────────────────────────────────────
// Simulates injecting session metadata (message count, session start time).

type SessionHistoryScout struct{}

func NewSessionHistoryScout() eywa.Scout { return &SessionHistoryScout{} }

func (s *SessionHistoryScout) GetName() string                 { return "session_history" }
func (s *SessionHistoryScout) IsApplicable(_ *eywa.Pulse) bool { return true }

func (s *SessionHistoryScout) Harvest(_ context.Context, pulse *eywa.Pulse) error {
	time.Sleep(20 * time.Millisecond)
	pulse.Knowledge["session_start"] = time.Now().Add(-10 * time.Minute).Format(time.RFC3339)
	pulse.Knowledge["message_count"] = 3
	return nil
}

// ── KeywordPathfinder ─────────────────────────────────────────────────────
// Selects the Spirit based on keyword scoring in the user message.

type KeywordPathfinder struct{}

func NewKeywordPathfinder() eywa.Pathfinder { return &KeywordPathfinder{} }

func (p *KeywordPathfinder) GetName() string { return "keyword_pathfinder" }

func (p *KeywordPathfinder) SelectSpirit(_ context.Context, pulse *eywa.Pulse, available []string) string {
	msg := strings.ToLower(pulse.UserMessage)

	scores := map[string]int{"support_spirit": 0, "sales_spirit": 0, "billing_spirit": 0}

	for _, kw := range []string{"crash", "error", "bug", "problem", "fix", "help", "technical"} {
		if strings.Contains(msg, kw) {
			scores["support_spirit"]++
		}
	}
	for _, kw := range []string{"buy", "purchase", "price", "plan", "upgrade", "product", "feature", "interested"} {
		if strings.Contains(msg, kw) {
			scores["sales_spirit"]++
		}
	}
	for _, kw := range []string{"invoice", "payment", "charge", "charged", "bill", "refund", "subscription"} {
		if strings.Contains(msg, kw) {
			scores["billing_spirit"]++
		}
	}

	best, bestScore := "", -1
	for _, name := range available {
		if scores[name] > bestScore {
			best, bestScore = name, scores[name]
		}
	}
	return best // empty string → WeaveBuilder falls back to DefaultSpirit
}
