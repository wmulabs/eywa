package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	eywa "github.com/wmulabs/eywa"
	eywamongo "github.com/wmulabs/eywa/mongo"
	eywaopenai "github.com/wmulabs/eywa/providers/openai"
	eywaredis "github.com/wmulabs/eywa/redis"
)

// Example 02: Custom Actions
//
// How to implement and register custom Actions that a Spirit can call.
//
// Key concepts:
// - Implement the Action interface
// - Register Actions in an ActionRegistry
// - Spirit.AllowedActions controls which Actions the Oracle can invoke
// - BusinessError vs InfrastructureError for correct retry behavior

func main() {
	ctx := context.Background()
	fmt.Println("=== Eywa — Custom Actions ===")

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

	// Register custom Actions
	actionRegistry := eywa.NewActionRegistry()
	if err := actionRegistry.Register(NewGetWeatherAction()); err != nil {
		log.Fatalf("register get_weather: %v", err)
	}
	if err := actionRegistry.Register(NewCalculatorAction()); err != nil {
		log.Fatalf("register calculator: %v", err)
	}
	fmt.Println("Actions registered: get_weather, calculator")

	// Weave
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	config := eywa.DefaultWeaveConfig()
	config.ReasoningTimeout = 90 * time.Second
	config.MaxReasoningIterations = 10

	weave, err := eywa.NewWeaveBuilder(ctx).
		WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
		WithBond(bond).
		WithActionRegistry(actionRegistry).
		WithScoutRegistry(eywa.NewScoutRegistry()).
		AddOracle(eywaopenai.NewOracle(apiKey)).
		WithConfig(config).
		Build()
	if err != nil {
		log.Fatalf("failed to build Weave: %v", err)
	}

	// Spirit — AllowedActions gates which Actions the Oracle can invoke
	spirit := &eywa.Spirit{
		Name:        "action_assistant",
		Description: "An assistant that uses actions to answer questions",
		SystemPrompt: `You are a helpful assistant with access to actions.
Use get_weather for weather questions. Use calculator for math.`,
		ModelConfig: eywa.SpiritModel{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.7,
			MaxTokens:   1000,
		},
		AllowedActions: []eywa.AllowedAction{{Name: "get_weather"}, {Name: "calculator"}},
		IsActive:       true,
		CreatedAt:      time.Now(),
	}
	if err := spiritRepo.Create(ctx, spirit); err != nil {
		fmt.Printf("note: spirit already exists: %v\n", err)
	}

	weave.RegisterEventConfiguration(
		eywa.NewLink("user_message").
			WithDefaultSpirit("action_assistant").
			Build(),
	)

	// Test 1: retrieval action
	fmt.Println("\n--- Test 1: Weather ---")
	processAndPrint(ctx, weave, "What's the weather in São Paulo?")

	// Test 2: general action
	fmt.Println("\n--- Test 2: Calculator ---")
	processAndPrint(ctx, weave, "What is 156 * 789?")

	// Test 3: multi-action in single turn
	fmt.Println("\n--- Test 3: Multi-action ---")
	processAndPrint(ctx, weave, "What's the weather in Tokyo and what is 25 + 75?")
}

func processAndPrint(ctx context.Context, weave *eywa.Weave, message string) {
	pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: "actions_demo"}).
		WithUserMessage(message).
		WithSource("api").
		Build()

	result, err := weave.ProcessEventByKey(ctx, "user_message", pulse)
	if err != nil {
		log.Printf("error: %v", err)
		return
	}
	fmt.Printf("Status:  %v\n", result.Status)
	fmt.Printf("Actions: %v\n", result.ActionsExecuted)
	fmt.Printf("Reply:   %s\n", result.Message)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ── GetWeatherAction ───────────────────────────────────────────────────────

type GetWeatherAction struct{}

func NewGetWeatherAction() eywa.Action { return &GetWeatherAction{} }

func (a *GetWeatherAction) GetName() string                  { return "get_weather" }
func (a *GetWeatherAction) IsCritical() bool                 { return false }
func (a *GetWeatherAction) GetCategory() eywa.ActionCategory { return eywa.ActionRetrieval }

func (a *GetWeatherAction) GetDescription() string {
	return "Get current weather for a city (temperature, conditions, humidity)."
}

func (a *GetWeatherAction) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{"type": "string", "description": "City name"},
		},
		"required": []string{"city"},
	}
}

func (a *GetWeatherAction) Validate(args map[string]any) error {
	city, ok := args["city"].(string)
	if !ok || city == "" {
		return eywa.NewBusinessError("city must be a non-empty string")
	}
	return nil
}

func (a *GetWeatherAction) Execute(_ context.Context, args map[string]any) (string, error) {
	city := args["city"].(string)
	time.Sleep(100 * time.Millisecond) // simulate API call

	data := map[string]string{
		"são paulo": "Sunny 28°C, humidity 65%",
		"tokyo":     "Rainy 18°C, humidity 85%",
		"london":    "Cloudy 15°C, humidity 80%",
	}
	if w, ok := data[city]; ok {
		return fmt.Sprintf("Weather in %s: %s", city, w), nil
	}
	return fmt.Sprintf("Weather in %s: Clear 25°C, humidity 60%%", city), nil
}

// ── CalculatorAction ──────────────────────────────────────────────────────

type CalculatorAction struct{}

func NewCalculatorAction() eywa.Action { return &CalculatorAction{} }

func (a *CalculatorAction) GetName() string                  { return "calculator" }
func (a *CalculatorAction) IsCritical() bool                 { return false }
func (a *CalculatorAction) GetCategory() eywa.ActionCategory { return eywa.ActionGeneral }

func (a *CalculatorAction) GetDescription() string {
	return "Perform basic math: add, subtract, multiply, divide."
}

func (a *CalculatorAction) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type": "string",
				"enum": []string{"add", "subtract", "multiply", "divide"},
			},
			"a": map[string]any{"type": "number"},
			"b": map[string]any{"type": "number"},
		},
		"required": []string{"operation", "a", "b"},
	}
}

func (a *CalculatorAction) Validate(args map[string]any) error {
	ops := map[string]bool{"add": true, "subtract": true, "multiply": true, "divide": true}
	op, _ := args["operation"].(string)
	if !ops[op] {
		return eywa.NewBusinessError("operation must be add|subtract|multiply|divide")
	}
	return nil
}

func (a *CalculatorAction) Execute(_ context.Context, args map[string]any) (string, error) {
	op := args["operation"].(string)
	av, _ := args["a"].(float64)
	bv, _ := args["b"].(float64)

	var result float64
	switch op {
	case "add":
		result = av + bv
	case "subtract":
		result = av - bv
	case "multiply":
		result = av * bv
	case "divide":
		if bv == 0 {
			return "", eywa.NewBusinessError("cannot divide by zero")
		}
		result = av / bv
	}
	return fmt.Sprintf("%.2f %s %.2f = %.2f", av, op, bv, result), nil
}
