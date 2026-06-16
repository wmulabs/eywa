package fiber

import (
	"github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

type RouteOption func(*routeOptions)

type routeOptions struct {
	internalMiddleware []fiber.Handler
}

// WithInternalMiddleware adds Fiber handlers that run before POST /internal/execute-event.
// Use this for OIDC token verification, e.g.:
//
//	RegisterRoutes(app, weave, spiritRepo, echoRepo,
//	    WithInternalMiddleware(gcp.NewCloudTasksOIDCMiddleware(audience)))
func WithInternalMiddleware(handlers ...fiber.Handler) RouteOption {
	return func(o *routeOptions) {
		o.internalMiddleware = append(o.internalMiddleware, handlers...)
	}
}

// RegisterRoutes registers all REST API routes onto app.
//
// Routes:
//   - GET  /health, GET /ready
//   - POST /api/v1/events/:event_key
//   - POST /api/v1/events/:event_key/stream  (Server-Sent Events: streamed agent turn)
//   - POST /api/v1/events/:event_key/async  (requires async dispatcher on weave)
//   - POST /api/v1/events/:event_key/schedule, GET/DELETE /api/v1/schedule
//   - CRUD /api/v1/spirits
//   - GET  /api/v1/messages
//   - POST /internal/execute-event  (Keeper callback)
//
// Security: Spirit mutation endpoints (POST/PUT/DELETE /api/v1/spirits) are registered
// without authentication by this function. Spirit system prompts are a critical attack
// surface — an attacker who can write Spirits can inject arbitrary instructions into your
// AI agents (prompt injection at the infrastructure level). You MUST protect these routes
// before exposing this server publicly. Options:
//
//  1. Use [RegisterManagementRoutes] instead, which applies auth middleware to all
//     management endpoints including Spirit CRUD.
//  2. Place this server behind a network boundary (VPC/firewall) and never expose it
//     to the public internet.
//  3. Add your own auth middleware to the Fiber app before calling this function.
func RegisterRoutes(
	app *fiber.App,
	weave *eywa.Weave,
	spiritRepo eywa.SpiritRepository,
	echoRepo eywa.EchoRepository,
	opts ...RouteOption,
) {
	cfg := &routeOptions{}
	for _, o := range opts {
		o(cfg)
	}

	healthHandler := NewHealthHandler(weave)
	eventHandler := NewEventHandler(weave)
	spiritHandler := NewSpiritManagementHandler(spiritRepo)
	messageHandler := NewMessageHandler(echoRepo)

	app.Get("/health", healthHandler.Health)
	app.Get("/ready", healthHandler.Ready)

	api := app.Group("/api/v1")
	api.Post("/events/:event_key", eventHandler.ProcessEvent)
	api.Post("/events/:event_key/stream", NewStreamEventHandler(weave).StreamEvent)

	if weave.GetAsyncDispatcher() != nil {
		asyncHandler := NewAsyncEventHandler(weave)
		api.Post("/events/:event_key/async", asyncHandler.IngestAsyncEvent)
	}

	spirits := api.Group("/spirits")
	spirits.Post("", spiritHandler.CreateSpirit)
	spirits.Get("", spiritHandler.ListSpirits)
	spirits.Get("/:name", spiritHandler.GetSpirit)
	spirits.Put("/:name", spiritHandler.UpdateSpirit)
	spirits.Delete("/:name", spiritHandler.DeleteSpirit)
	spirits.Post("/:name/activate", spiritHandler.ActivateSpirit)
	spirits.Post("/:name/deactivate", spiritHandler.DeactivateSpirit)
	spirits.Get("/:name/versions", spiritHandler.GetSpiritVersions)

	api.Get("/messages", messageHandler.GetMessages)

	if weave.GetRitualManager() != nil {
		scheduleHandler := NewScheduleHandler(weave)
		api.Post("/events/:event_key/schedule", scheduleHandler.ScheduleByEventKey)
		schedule := api.Group("/schedule")
		schedule.Get("", scheduleHandler.ListPending)
		schedule.Delete("/:id", scheduleHandler.Cancel)
	}

	if weave.GetAsyncDispatcher() != nil || weave.GetRitualManager() != nil {
		handlers := append(cfg.internalMiddleware, HandleExecuteEvent(weave))
		app.Post("/internal/execute-event", handlers...)
	}
}
