package fiber

import (
	"errors"
	"time"

	fiberlib "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	eywa "github.com/wmulabs/eywa"
	"github.com/wmulabs/eywa/fiber/middleware"
)

// ManagementDeps wires optional repositories and auth config into RegisterManagementRoutes.
// Each route group is only registered when its required dependency is non-nil.
type ManagementDeps struct {
	// Auth — at least one required for management routes to be secured.
	APIKeys        map[string]string   // Mode 1: static key→role map
	OperatorAuth   *eywa.OperatorAuth  // Mode 2: built-in operator JWT
	TokenValidator eywa.TokenValidator // Mode 3: external JWT / JWKS

	// Chronicle read-only queries for the audit log and analytics endpoints.
	ChronicleQueryRepo eywa.ChronicleQueryRepository

	// Echo repositories for conversation history and real-time operator messaging.
	EchoRepo      eywa.EchoRepository
	EchoQueryRepo eywa.EchoQueryRepository

	// Config management — live event-configuration cache and persistent engine config.
	ConfigCache     *eywa.ConfigCache
	WeaveConfigRepo eywa.WeaveConfigRepository

	// HTTPToolRepo enables CRUD for HTTP tool definitions managed via the API.
	HTTPToolRepo eywa.HTTPToolRepository

	// Vigil — human-takeover seat management (operator takes over a conversation).
	VigilRepo   eywa.VigilRepository
	VigilConfig eywa.VigilConfig

	// RiteRepo enables approval flow management (create, list, approve/reject Rites).
	RiteRepo eywa.RiteRepository

	// Real-time (SSE) — requires PubSub (Redis recommended).
	// When set, enables GET /api/v1/sse/* endpoints and live fanout from
	// Vigil and Rite handlers.
	PubSub eywa.PubSub

	// Imprint management — exposes GET/DELETE /api/v1/imprints.
	ImprintRepo eywa.ImprintRepository
}

// RegisterManagementRoutes mounts the management API onto app under /api/v1.
// All management routes require authentication. Route groups are registered
// conditionally based on which ManagementDeps fields are non-nil.
func RegisterManagementRoutes(app *fiberlib.App, weave *eywa.Weave, deps ManagementDeps) error {
	validators := buildValidatorChain(deps)
	if len(validators) == 0 {
		return errors.New("RegisterManagementRoutes: at least one auth validator must be configured (set APIKeys, OperatorAuth, or TokenValidator in ManagementDeps)")
	}
	authMW := middleware.AuthMiddleware(validators...)

	var oh *operatorHandler
	if deps.OperatorAuth != nil {
		oh = newOperatorHandler(deps.OperatorAuth)
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

	// EventBus — nil when no PubSub configured; all publish calls are no-ops.
	eventBus := NewEventBus(deps.PubSub)

	api := app.Group("/api/v1", authMW)

	// Discovery — always registered when weave is provided.
	if weave != nil {
		dh := newDiscoveryHandler(weave)
		api.Get("/discovery", dh.get)
	}

	if oh != nil {
		ops := api.Group("/operators", middleware.RequireRole(eywa.RoleAdmin))
		ops.Get("", oh.list)
		ops.Post("", oh.create)
		ops.Get("/:id", oh.getByID)
		ops.Put("/:id", oh.update)
		ops.Delete("/:id", oh.deactivate)
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

	// SSE — registered only when PubSub is configured.
	if deps.PubSub != nil {
		sse := newSSEHandler(deps.PubSub, deps.RiteRepo, deps.VigilRepo)
		sseGroup := api.Group("/sse")
		sseGroup.Get("/rites", sse.streamRites)
		sseGroup.Get("/vigil", sse.streamVigil)
		sseGroup.Get("/echoes/:memoryKey", sse.streamEchoes)
	}

	return nil
}

func buildValidatorChain(deps ManagementDeps) []eywa.TokenValidator {
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
