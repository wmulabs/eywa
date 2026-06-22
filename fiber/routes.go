package fiber

import (
	"errors"
	"time"

	fiberlib "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	eywa "github.com/wmulabs/eywa"
	"github.com/wmulabs/eywa/fiber/middleware"
)

// RouteDeps configures the single route registrar. Auth is required to expose any sensitive route:
// Spirit management, conversation history, scheduling, and the whole operator/management API all sit
// behind the configured validator(s). Only event ingestion and health checks are served openly —
// events are webhook-style (a channel or Cloud Tasks authenticates them, not a bearer token).
//
// Each management group is mounted only when its backing repository is non-nil. If any protected
// repository is provided without an auth validator, RegisterRoutes fails closed (returns an error)
// rather than exposing it unauthenticated.
type RouteDeps struct {
	// Auth — at least one is required to mount any protected route.
	APIKeys        map[string]string   // Mode 1: static key→role map
	OperatorAuth   *eywa.OperatorAuth  // Mode 2: built-in operator JWT (also enables POST /api/v1/auth/token)
	TokenValidator eywa.TokenValidator // Mode 3: external JWT / JWKS

	// SpiritRepo enables authenticated Spirit CRUD under /api/v1/spirits. Leave nil to not expose it
	// (e.g. when Spirits are defined in code) — there is no unauthenticated Spirit surface.
	SpiritRepo eywa.SpiritRepository

	// Echo repositories: GET /api/v1/messages (EchoRepo) and the /echoes management group (EchoQueryRepo).
	EchoRepo      eywa.EchoRepository
	EchoQueryRepo eywa.EchoQueryRepository

	// Chronicle read-only queries for the audit log and analytics endpoints.
	ChronicleQueryRepo eywa.ChronicleQueryRepository

	// Config management — live event-configuration cache and persistent engine config.
	ConfigCache     *eywa.ConfigCache
	WeaveConfigRepo eywa.WeaveConfigRepository

	// HTTPToolRepo enables CRUD for HTTP tool definitions managed via the API.
	HTTPToolRepo eywa.HTTPToolRepository

	// Vigil — human-takeover seat management.
	VigilRepo   eywa.VigilRepository
	VigilConfig eywa.VigilConfig

	// RiteRepo enables approval flow management.
	RiteRepo eywa.RiteRepository

	// Real-time (SSE) — requires PubSub (Redis recommended). Enables GET /api/v1/sse/*.
	PubSub eywa.PubSub

	// Imprint management — GET/DELETE /api/v1/imprints.
	ImprintRepo eywa.ImprintRepository

	// InternalMiddleware guards POST /internal/execute-event (the Keeper/Cloud Tasks callback), e.g.
	// an OIDC verifier. The callback is registered openly without it — set it before exposing publicly.
	InternalMiddleware []fiberlib.Handler
}

// RegisterRoutes mounts every Eywa HTTP route onto app. Open routes: GET /health, /ready and
// POST /api/v1/events/:event_key (+ /stream, + /async when an async dispatcher is wired). The Keeper
// callback POST /internal/execute-event is registered when async dispatch or scheduling is enabled
// (protect it with RouteDeps.InternalMiddleware). Everything else — Spirit CRUD, messages, scheduling,
// discovery, operators, chronicle/analytics, echoes, event-configurations, engine config, HTTP tools,
// Vigil, Rites, imprints, and SSE — is mounted behind authentication, conditional on its dependency.
//
// At least one auth validator (APIKeys, OperatorAuth, or TokenValidator) is required to expose any
// protected route. Providing a protected repository without a validator fails closed with an error.
func RegisterRoutes(app *fiberlib.App, weave *eywa.Weave, deps RouteDeps) error {
	registerOpenRoutes(app, weave, deps)

	validators := buildValidatorChain(deps)
	if len(validators) == 0 {
		if hasProtectedDeps(deps) {
			return errors.New("RegisterRoutes: protected repositories were provided but no auth validator is configured (set APIKeys, OperatorAuth, or TokenValidator) — refusing to expose Spirit/message/management routes unauthenticated")
		}
		return nil
	}

	if deps.OperatorAuth != nil {
		registerOperatorLogin(app, deps.OperatorAuth)
	}

	api := app.Group("/api/v1", middleware.AuthMiddleware(validators...))
	registerProtectedRoutes(api, weave, deps)
	return nil
}

// registerOpenRoutes mounts the unauthenticated surface: health and event ingestion (webhook-style),
// plus the Keeper callback (guarded by InternalMiddleware).
func registerOpenRoutes(app *fiberlib.App, weave *eywa.Weave, deps RouteDeps) {
	if weave == nil {
		return
	}

	health := NewHealthHandler(weave)
	app.Get("/health", health.Health)
	app.Get("/ready", health.Ready)

	open := app.Group("/api/v1")
	open.Post("/events/:event_key", NewEventHandler(weave).ProcessEvent)
	open.Post("/events/:event_key/stream", NewStreamEventHandler(weave).StreamEvent)
	if weave.GetAsyncDispatcher() != nil {
		open.Post("/events/:event_key/async", NewAsyncEventHandler(weave).IngestAsyncEvent)
	}

	if weave.GetAsyncDispatcher() != nil || weave.GetRitualManager() != nil {
		handlers := append(deps.InternalMiddleware, HandleExecuteEvent(weave))
		app.Post("/internal/execute-event", handlers...)
	}
}

// registerProtectedRoutes mounts every authenticated route onto api (which already carries the auth
// middleware). Each group is conditional on its dependency.
func registerProtectedRoutes(api fiberlib.Router, weave *eywa.Weave, deps RouteDeps) {
	eventBus := NewEventBus(deps.PubSub)

	if weave != nil {
		api.Get("/discovery", newDiscoveryHandler(weave).get)
	}

	if deps.OperatorAuth != nil {
		oh := newOperatorHandler(deps.OperatorAuth)
		ops := api.Group("/operators", middleware.RequireRole(eywa.RoleAdmin))
		ops.Get("", oh.list)
		ops.Post("", oh.create)
		ops.Get("/:id", oh.getByID)
		ops.Put("/:id", oh.update)
		ops.Delete("/:id", oh.deactivate)
	}

	if deps.SpiritRepo != nil {
		sh := NewSpiritManagementHandler(deps.SpiritRepo)
		spirits := api.Group("/spirits")
		spirits.Post("", sh.CreateSpirit)
		spirits.Get("", sh.ListSpirits)
		spirits.Get("/:name", sh.GetSpirit)
		spirits.Put("/:name", sh.UpdateSpirit)
		spirits.Delete("/:name", sh.DeleteSpirit)
		spirits.Post("/:name/activate", sh.ActivateSpirit)
		spirits.Post("/:name/deactivate", sh.DeactivateSpirit)
		spirits.Get("/:name/versions", sh.GetSpiritVersions)
	}

	if deps.EchoRepo != nil {
		api.Get("/messages", NewMessageHandler(deps.EchoRepo).GetMessages)
	}

	if weave != nil && weave.GetRitualManager() != nil {
		sch := NewScheduleHandler(weave)
		api.Post("/events/:event_key/schedule", sch.ScheduleByEventKey)
		schedule := api.Group("/schedule")
		schedule.Get("", sch.ListPending)
		schedule.Delete("/:id", sch.Cancel)
	}

	if deps.ChronicleQueryRepo != nil {
		ch := newChronicleHandler(deps.ChronicleQueryRepo)
		api.Get("/chronicle", ch.list)
		api.Get("/chronicle/:id", ch.findByID)

		analytics := api.Group("/analytics")
		analytics.Get("/tokens", ch.aggregateTokens)
		analytics.Get("/actions", ch.aggregateActions)
		analytics.Get("/spirits", ch.aggregateSpirits)
	}

	if deps.EchoQueryRepo != nil {
		eh := newEchoMgmtHandler(deps.EchoQueryRepo, deps.EchoRepo)
		echoes := api.Group("/echoes")
		echoes.Get("/sessions", eh.listSessions)
		echoes.Get("/sessions/:memoryKey", eh.sessionDetail)
		echoes.Post("/sessions/:memoryKey/messages", eh.sendMessage)
		echoes.Get("", eh.listEchoes)
	}

	if deps.ConfigCache != nil {
		lh := newLinkHandler(deps.ConfigCache)
		cfgs := api.Group("/event-configurations")
		cfgs.Get("", lh.list)
		cfgs.Get("/:eventType", lh.getByKey)
		cfgs.Put("/:eventType", lh.save)
		cfgs.Delete("/:eventType", lh.delete)
	}

	if deps.WeaveConfigRepo != nil {
		wh := newWeaveConfigHandler(deps.WeaveConfigRepo, deps.ConfigCache)
		admin := api.Group("/admin")
		admin.Get("/engine-config", wh.get)
		admin.Put("/engine-config", wh.save)
		admin.Post("/config/reload", wh.reload)
	}

	if deps.HTTPToolRepo != nil {
		hth := newHTTPToolHandler(deps.HTTPToolRepo)
		httpTools := api.Group("/http-tools")
		httpTools.Get("", hth.list)
		httpTools.Post("", hth.create)
		httpTools.Get("/:id", hth.getByID)
		httpTools.Put("/:id", hth.update)
		httpTools.Delete("/:id", hth.delete)
		httpTools.Post("/:id/test", hth.test)
	}

	if deps.VigilRepo != nil {
		vh := newVigilHandler(deps.VigilRepo, deps.VigilConfig, deps.EchoRepo, eventBus)
		api.Get("/vigil", vh.listAll)
		vigil := api.Group("/vigil/:memoryKey")
		vigil.Post("", vh.takeSeat)
		vigil.Delete("", vh.releaseSeat)
		vigil.Get("", vh.getStatus)
		vigil.Post("/echoes", vh.sendMessage)
	}

	if deps.RiteRepo != nil {
		rh := newRiteHandler(deps.RiteRepo, weave, eventBus)
		rites := api.Group("/rites")
		rites.Get("", rh.list)
		rites.Get("/:id", rh.getByID)
		rites.Post("/:id/approve", rh.approve)
		rites.Post("/:id/reject", rh.reject)
	}

	if deps.ImprintRepo != nil {
		ih := newImprintMgmtHandler(deps.ImprintRepo)
		imprints := api.Group("/imprints")
		imprints.Get("", ih.list)
		imprints.Delete("/:id", ih.delete)
	}

	if deps.PubSub != nil {
		sse := newSSEHandler(deps.PubSub, deps.RiteRepo, deps.VigilRepo)
		sseGroup := api.Group("/sse")
		sseGroup.Get("/rites", sse.streamRites)
		sseGroup.Get("/vigil", sse.streamVigil)
		sseGroup.Get("/echoes/:memoryKey", sse.streamEchoes)
	}
}

func registerOperatorLogin(app *fiberlib.App, operatorAuth *eywa.OperatorAuth) {
	oh := newOperatorHandler(operatorAuth)
	authLimiter := limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiberlib.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiberlib.Ctx) error {
			return c.Status(fiberlib.StatusTooManyRequests).JSON(fiberlib.Map{
				"error": "too many login attempts, please try again later",
			})
		},
	})
	app.Post("/api/v1/auth/token", authLimiter, oh.login)
}

func buildValidatorChain(deps RouteDeps) []eywa.TokenValidator {
	var validators []eywa.TokenValidator
	if len(deps.APIKeys) > 0 {
		validators = append(validators, eywa.NewAPIKeyValidator(deps.APIKeys))
	}
	if deps.OperatorAuth != nil {
		validators = append(validators, deps.OperatorAuth)
	}
	if deps.TokenValidator != nil {
		validators = append(validators, deps.TokenValidator)
	}
	return validators
}

// hasProtectedDeps reports whether any dependency would mount a route that must be authenticated.
func hasProtectedDeps(deps RouteDeps) bool {
	return deps.SpiritRepo != nil ||
		deps.EchoRepo != nil ||
		deps.EchoQueryRepo != nil ||
		deps.ChronicleQueryRepo != nil ||
		deps.ConfigCache != nil ||
		deps.WeaveConfigRepo != nil ||
		deps.HTTPToolRepo != nil ||
		deps.VigilRepo != nil ||
		deps.RiteRepo != nil ||
		deps.ImprintRepo != nil ||
		deps.PubSub != nil
}
