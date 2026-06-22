package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	eywafiber "github.com/wmulabs/eywa/fiber"
	eywamongo "github.com/wmulabs/eywa/mongo"
	eywaopenai "github.com/wmulabs/eywa/providers/openai"
	eywaredis "github.com/wmulabs/eywa/redis"
)

// Example 12: Management API
//
// Demonstrates the eywa/fiber sub-module — a full REST management layer.
//
// Routes registered (all under /api/v1, auth required):
//   Spirits:      GET/POST /spirits, GET/PUT/DELETE /spirits/:name
//   Chronicle:    GET /chronicle, GET /chronicle/:id
//   Analytics:    GET /analytics/tokens, /analytics/actions, /analytics/spirits
//   Conversations: GET /echoes/sessions, GET /echoes/sessions/:key, GET /echoes/sessions/:key/messages
//   Send message: POST /echoes/sessions/:memoryKey/messages  (operator → user)
//   Config:       GET/PUT /weave-config, POST /weave-config/reload
//   HTTP Tools:   GET/POST /http-tools, GET/PUT/DELETE /http-tools/:id
//   Vigil:        GET /vigil/:memoryKey, POST /vigil/:memoryKey/acquire, DELETE /vigil/:memoryKey
//                 POST /vigil/:memoryKey/echoes  (operator direct message)
//   Rites:        GET /rites, GET /rites/:id, POST /rites/:id/decide
//   Operators:    GET/POST /operators, GET/PUT/DELETE /operators/:id
//   Auth:         POST /auth/token  (operator login — public, no auth)
//
// Auth modes (configure one or more):
//   - Static API keys (simplest — good for internal tools)
//   - Built-in operator JWT (OperatorAuth)
//   - External JWT / JWKS (for OIDC/SSO)

func main() {
	ctx := context.Background()
	fmt.Println("=== Eywa — Management API ===")

	mongoURL := getEnv("MONGO_URL", "mongodb://localhost:27017")
	mongoDatabase := getEnv("MONGO_DATABASE", "eywa_example")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	environment := getEnv("ENVIRONMENT", "lcl")
	serviceName := getEnv("SERVICE_NAME", "eywa")
	addr := getEnv("ADDR", ":8080")

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

	db := mongoConn.GetDatabase()

	// Repositories
	spiritRepo := eywamongo.NewSpiritRepository(db)
	memoryRepo := eywaredis.NewMemoryRepository(redisConn.GetClient(), serviceName, environment, 3600, nil)
	echoRepo := eywamongo.NewEchoRepository(db)
	chronicleRepo := eywamongo.NewChronicleRepository(db)
	httpToolRepo := eywamongo.NewHTTPToolRepository(db)
	vigilRepo := eywaredis.NewVigilRepository(redisConn.GetClient(), serviceName, environment)
	riteRepo := eywamongo.NewRiteRepository(db)
	operatorRepo := eywamongo.NewOperatorRepository(db)
	bond := eywaredis.NewBondManager(redisConn.GetClient())

	// OperatorAuth: built-in operator management + JWT issuer.
	// JWTSecret is used to sign operator tokens.
	jwtSecret := getEnv("JWT_SECRET", "dev-secret-change-in-production")
	operatorAuth := eywa.NewOperatorAuth(operatorRepo, []byte(jwtSecret))

	// WeaveConfig: hot-reload event configurations from MongoDB.
	// LinkRepository stores event→spirit configurations; ConfigCache caches them in memory.
	weaveConfigRepo := eywamongo.NewWeaveConfigRepository(db)
	linkRepo := eywamongo.NewLinkRepository(db)
	configCache := eywa.NewConfigCache(linkRepo, nil, nil)

	weave, err := eywa.NewWeaveBuilder(ctx).
		WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
		WithBond(bond).
		WithActionRegistry(eywa.NewActionRegistry()).
		WithScoutRegistry(eywa.NewScoutRegistry()).
		WithVigilRepository(vigilRepo).
		WithVigilConfig(eywa.VigilConfig{InactivityTimeout: 30 * time.Minute}).
		WithRiteRepository(riteRepo).
		AddOracle(eywaopenai.NewOracle(mustEnv("OPENAI_API_KEY"))).
		WithConfig(eywa.DefaultWeaveConfig()).
		Build()
	if err != nil {
		log.Fatalf("build weave: %v", err)
	}

	// Register a default event configuration
	weave.RegisterEventConfiguration(
		eywa.NewLink("user_message").
			WithDefaultSpirit("assistant").
			Build(),
	)

	// Create a sample spirit
	spirit := &eywa.Spirit{
		Name:         "assistant",
		Description:  "General purpose assistant",
		SystemPrompt: "You are a helpful assistant.",
		ModelConfig: eywa.SpiritModel{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.7,
			MaxTokens:   300,
		},
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	spiritRepo.Create(ctx, spirit) //nolint:errcheck

	// ── Fiber + Management routes ─────────────────────────────────────────────

	app := fiber.New(fiber.Config{
		AppName:      "eywa-management",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	})

	// RouteDeps wires repositories into the single route registrar. Event ingestion and health are
	// open; every management group below is mounted only when its dep is non-nil — and behind auth.
	deps := eywafiber.RouteDeps{
		// Auth: using static API key + built-in operator JWT
		APIKeys: map[string]string{
			getEnv("API_KEY", "dev-key-change-me"): "admin",
		},
		OperatorAuth: operatorAuth,

		// Spirit management (authed)
		SpiritRepo: spiritRepo,

		// Observability
		ChronicleQueryRepo: chronicleRepo,

		// Conversations
		EchoRepo:      echoRepo,
		EchoQueryRepo: echoRepo,

		// Config as a Service
		ConfigCache:     configCache,
		WeaveConfigRepo: weaveConfigRepo,

		// HTTP Tools
		HTTPToolRepo: httpToolRepo,

		// Human Takeover
		VigilRepo:   vigilRepo,
		VigilConfig: eywa.VigilConfig{InactivityTimeout: 30 * time.Minute},

		// Approval Flows
		RiteRepo: riteRepo,
	}

	// One registrar: open event/health routes + token-protected management API.
	if err := eywafiber.RegisterRoutes(app, weave, deps); err != nil {
		log.Fatalf("failed to register routes: %v", err)
	}

	fmt.Printf("\nManagement API listening on %s\n", addr)
	fmt.Println("\nOpen endpoints:")
	fmt.Println("  GET  /health, /ready")
	fmt.Println("  POST /api/v1/events/:event_key")
	fmt.Println("  POST /api/v1/auth/token  (operator login)")
	fmt.Println("\nAuthenticated (Authorization: Bearer dev-key-change-me):")
	fmt.Println("  GET  /api/v1/spirits")
	fmt.Println("  GET  /api/v1/chronicle")
	fmt.Println("  GET  /api/v1/echoes/sessions")
	fmt.Println("  GET  /api/v1/rites")

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := app.Listen(addr); err != nil {
			log.Printf("server error: %v", err)
		}
	}()

	<-quit
	fmt.Println("\nShutting down...")
	app.Shutdown() //nolint:errcheck
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
