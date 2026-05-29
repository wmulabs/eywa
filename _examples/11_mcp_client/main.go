package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	eywa "github.com/wmulabs/eywa"
	eywamcp "github.com/wmulabs/eywa/mcp"
	eywamongo "github.com/wmulabs/eywa/mongo"
	eywaopenai "github.com/wmulabs/eywa/providers/openai"
	eywaredis "github.com/wmulabs/eywa/redis"
)

// Example 11: MCP Client (Conduit)
//
// Demonstrates Conduit — Eywa's MCP (Model Context Protocol) client.
//
// Key concepts:
// - Conduit connects to an external MCP server over HTTP (StreamableHTTP)
// - On Build(), tools are auto-discovered via MCP's tools/list endpoint
// - Tools are registered as eywa Actions with the prefix "<conduit_name>__<tool_name>"
// - Spirits reference MCP tools via AllowedActions using the prefixed name
// - Multiple MCP servers can be connected to the same Weave
//
// To test this example, run an MCP server locally:
//   npx @modelcontextprotocol/server-everything  # sample MCP server on :3001
// Or point MCP_SERVER_URL at any compatible HTTP MCP server.

func main() {
	ctx := context.Background()
	fmt.Println("=== Eywa — MCP Client (Conduit) ===")

	mongoURL := getEnv("MONGO_URL", "mongodb://localhost:27017")
	mongoDatabase := getEnv("MONGO_DATABASE", "eywa_example")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	environment := getEnv("ENVIRONMENT", "lcl")
	serviceName := getEnv("SERVICE_NAME", "eywa")
	mcpServerURL := getEnv("MCP_SERVER_URL", "http://localhost:3001")

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

	// Conduit: connects to an MCP server and discovers its tools.
	// Tool names are automatically prefixed: "tools_server__<tool_name>"
	conduit := eywamcp.NewConduit(eywamcp.ConduitConfig{
		Name:      "tools_server",
		Transport: "http",
		URL:       mcpServerURL,
		Timeout:   15 * time.Second,
		// Add auth headers if your MCP server requires them:
		// Headers: map[string]string{"Authorization": "Bearer " + os.Getenv("MCP_API_KEY")},
	})

	weave, err := eywa.NewWeaveBuilder(ctx).
		WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
		WithBond(bond).
		WithActionRegistry(eywa.NewActionRegistry()).
		WithScoutRegistry(eywa.NewScoutRegistry()).
		WithConduit(conduit). // tools auto-registered on Build()
		AddOracle(eywaopenai.NewOracle(mustEnv("OPENAI_API_KEY"))).
		WithConfig(eywa.DefaultWeaveConfig()).
		Build()
	if err != nil {
		// Build() connects to all conduits and discovers tools.
		// If the MCP server is unreachable, it logs a warning but doesn't fail.
		log.Printf("build weave (MCP may be unavailable): %v", err)
	}

	// The spirit references MCP tools via AllowedActions.
	// Tool name format: "<conduit_name>__<mcp_tool_name>"
	spirit := &eywa.Spirit{
		Name:        "mcp_spirit",
		Description: "Spirit with access to external MCP tools",
		SystemPrompt: `You are a helpful assistant with access to external tools via MCP.
Use the available tools to answer questions that require real-world data or actions.`,
		ModelConfig: eywa.SpiritModel{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.3,
			MaxTokens:   500,
		},
		// AllowedActions references the MCP tool by its prefixed name.
		// Use "*" as Name to allow all tools from all conduits (not recommended for production).
		AllowedActions: []eywa.AllowedAction{
			{Name: "tools_server__echo"},
			{Name: "tools_server__add"},
		},
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	if err := spiritRepo.Create(ctx, spirit); err != nil {
		fmt.Printf("note: spirit already exists\n")
	}

	weave.RegisterEventConfiguration(
		eywa.NewLink("user_message").WithDefaultSpirit("mcp_spirit").Build(),
	)

	// ── Test ──────────────────────────────────────────────────────────────────

	fmt.Printf("\nMCP server: %s\n", mcpServerURL)
	fmt.Println("Tip: run 'npx @modelcontextprotocol/server-everything' to start a sample MCP server")

	questions := []string{
		"Echo the message: hello from eywa",
		"What is 42 + 58?",
	}

	for _, q := range questions {
		fmt.Printf("\nQ: %s\n", q)
		pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: "user_mcp"}).
			WithUserMessage(q).
			WithSource("api").
			Build()

		result, err := weave.ProcessEventByKey(ctx, "user_message", pulse)
		if err != nil {
			log.Printf("error: %v", err)
			continue
		}
		fmt.Printf("Actions: %v\n", result.ActionsExecuted)
		fmt.Printf("Reply:   %s\n", result.Message)
	}
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
