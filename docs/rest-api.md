# 🌐 REST API Reference

Eywa exposes two distinct API surfaces via the `fiber` sub-module, each serving a different audience.

| API | Function | Audience |
|-----|----------|----------|
| **Event API** | `RegisterRoutes` | Your application backend — sends Pulses into the Weave |
| **Management API** | `RegisterManagementRoutes` | Operators and dashboards — inspect, configure, take over |

---

## Event API — `RegisterRoutes`

For receiving events from external sources and routing them into the Weave.

```go
eywafiber.RegisterRoutes(app, weave, spiritRepo, echoRepo)
```

No authentication by default — secure these routes at the infrastructure level (VPC, Cloud Armor, etc.) or add middleware manually.

> **Security:** `RegisterRoutes` also mounts Spirit CRUD endpoints (`POST/PUT/DELETE /api/v1/spirits`) without authentication. Spirit system prompts are a critical attack surface — an unauthenticated write is prompt injection at the infrastructure level. **Never expose `RegisterRoutes` to the public internet without auth.** For operator-facing management, use `RegisterManagementRoutes` instead.

---

### `GET /health`

Liveness check. Always 200 if the process is alive.

```json
{ "status": "healthy", "app": "myapp", "version": "v1.0.0" }
```

---

### `GET /ready`

Readiness check. Verifies repositories and Oracle factory.

**200 — ready:**
```json
{ "status": "ready" }
```

**503 — not ready:**
```json
{ "status": "not_ready", "error": "spirit repository unavailable" }
```

---

### `POST /api/v1/events/:event_key`

Process a Pulse synchronously. Blocks until the Oracle loop completes.

**Headers:**
```
Content-Type: application/json
```

**Body:**
```json
{
  "user_id":         "user_123",
  "channel":         "api",
  "message":         "What is the status of my order?",
  "idempotency_key": "evt_abc123",
  "subject_key":     "order:4821",
  "knowledge": {
    "user_tier": "VIP"
  },
  "metadata": {
    "source": "mobile_app"
  },
  "payload": {}
}
```

> [!NOTE]
> `knowledge`, `metadata`, and `payload` are optional maps. When a Receptor is configured for the event type, `payload` is passed to it for raw webhook conversion.

**200 — success:**
```json
{
  "success":    true,
  "message":    "Your order #4821 is out for delivery.",
  "memory_key": "api:user_123",
  "spirit":     "support_spirit",
  "chronicle_id": "chron_xyz789"
}
```

**4xx / 5xx — failure:**
```json
{
  "success": false,
  "error":   "rate_limited",
  "message": "Too many requests"
}
```

---

### `POST /api/v1/events/:event_key/async`

Dispatch a Pulse to the Keeper for background processing. Returns immediately — the Pulse is processed by a Cloud Tasks callback.

> [!NOTE]
> Only available when `WithAsyncDispatch` is configured on the Weave.

**Body:** same as synchronous endpoint.

**202 — accepted:**
```json
{ "success": true, "task_id": "task_abc123" }
```

---

### `POST /api/v1/events/:event_key/schedule`

Schedule a future Ritual. The Keeper fires it at `execute_at`.

> [!NOTE]
> Only available when `WithRitualManager` is configured.

**Body:**
```json
{
  "user_id":    "user_123",
  "channel":    "api",
  "message":    "Send follow-up",
  "execute_at": "2026-06-01T10:00:00Z",
  "recurrence": {
    "cron":     "0 9 * * 1",
    "timezone": "America/Sao_Paulo",
    "max_runs": 12,
    "ends_at":  "2026-12-31T00:00:00Z"
  }
}
```

**201 — created:**
```json
{ "success": true, "ritual_id": "ritual_xyz" }
```

---

### `GET /api/v1/schedule`

List pending Rituals for a MemoryKey.

**Query params:**
| Param | Type | Description |
|-------|------|-------------|
| `memory_key` | string | Required — e.g. `api:user_123` |

**200:**
```json
{
  "rituals": [
    {
      "id":          "ritual_xyz",
      "memory_key":  "api:user_123",
      "execute_at":  "2026-06-01T10:00:00Z",
      "status":      "pending",
      "recurrence":  { "cron": "0 9 * * 1", ... }
    }
  ]
}
```

---

### `DELETE /api/v1/schedule/:id`

Cancel a pending Ritual.

**Query params:** `memory_key` (required).

**200:**
```json
{ "success": true }
```

---

### `POST /internal/execute-event`

Keeper callback — called by Cloud Tasks to execute a scheduled or async Pulse. Not for external use.

Protected with `WithInternalMiddleware` (OIDC verification in production):

```go
eywafiber.RegisterRoutes(app, weave, spiritRepo, echoRepo,
    eywafiber.WithInternalMiddleware(
        cloudtasks.NewCloudTasksOIDCMiddleware(os.Getenv("SERVICE_URL")),
    ),
)
```

---

## Management API — `RegisterManagementRoutes`

For operators, dashboards, and tooling that inspect and control the Weave at runtime.

```go
eywafiber.RegisterManagementRoutes(app, weave, eywafiber.ManagementDeps{...})
```

All routes require authentication. Three auth modes — configure one or more:

| Mode | Config field | Token format |
|------|-------------|--------------|
| Static API key | `APIKeys map[string]string` | `Authorization: ApiKey <key>` |
| Built-in operator JWT | `OperatorAuth *eywa.OperatorAuth` | `Authorization: Bearer <jwt>` |
| External JWT / JWKS | `TokenValidator eywa.TokenValidator` | `Authorization: Bearer <jwt>` |

---

## Auth

### Authentication Flow

The Management API uses JWT Bearer tokens. Get a token with your credentials, then include it
in all subsequent requests as `Authorization: Bearer <token>`.

```bash
# Step 1: Get a token
TOKEN=$(curl -s -X POST https://your-service.run.app/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "your-password"}' \
  | jq -r '.token')

# Step 2: Use the token in subsequent requests
curl -H "Authorization: Bearer $TOKEN" \
  https://your-service.run.app/api/v1/spirits

# Tokens expire after the configured TTL (default: 24h).
# Re-authenticate when you receive 401 Unauthorized.
```

**API Key alternative:** If configured with `WithAPIKey`, pass the key as:
```
Authorization: ApiKey your-api-key
```

---

### `POST /api/v1/auth/token` (public — no auth required)

Obtain a JWT from the built-in operator system.

> [!NOTE]
> Only available when `OperatorAuth` is set in `ManagementDeps`.

**Body:**
```json
{ "username": "admin", "password": "secret" }
```

**200:**
```json
{
  "token":      "eyJhbGci...",
  "expires_at": "2026-05-20T12:00:00Z",
  "operator": {
    "id":       "op_123",
    "username": "admin",
    "role":     "admin"
  }
}
```

---

## Discovery

### `GET /api/v1/discovery`

Returns all registered engine components. Use this to populate configuration forms — which Actions, Scouts, channels, and routers are available at runtime.

**200:**
```json
{
  "actions": [
    {
      "name":        "track_order",
      "description": "Track a customer order by ID.",
      "parameters":  { "type": "object", "properties": {...} },
      "is_critical": false,
      "category":    "retrieval"
    }
  ],
  "scouts":      ["user_profile", "order_context"],
  "classifiers": ["intent_classifier"],
  "channels":    [{ "name": "whatsapp", "auto_respond": true }],
  "routers":     ["tier_router", "content_router"],
  "receptors":   ["whatsapp_360dialog", "telegram"]
}
```

---

## Spirits

### `GET /api/v1/spirits`

List all active Spirits (paginated).

**Query params:** `limit` (default 50), `offset` (default 0).

**200:**
```json
{
  "items":  [{ Spirit }],
  "total":  42,
  "limit":  50,
  "offset": 0
}
```

---

### `POST /api/v1/spirits`

Create a new Spirit.

**Body:**
```json
{
  "name":           "support_spirit",
  "description":    "Customer support specialist",
  "specialization": "e-commerce support",
  "system_prompt":  "You are a helpful support agent...",
  "enforce_voice_delivery":      false,
  "voice_delivery_instructions": "",
  "business_error_instructions": "",
  "allowed_actions": [
    { "name": "track_order",   "is_critical": false },
    { "name": "request_rite",  "is_critical": true  }
  ],
  "model_config": {
    "provider":    "anthropic",
    "model":       "claude-sonnet-4-6",
    "temperature": 0.5,
    "max_tokens":  2000
  },
  "metadata": {}
}
```

**201:**
```json
{ "success": true, "spirit": { Spirit } }
```

---

### `GET /api/v1/spirits/:name`

Get the active Spirit by name.

**200:**
```json
{ "success": true, "spirit": { Spirit } }
```

**404:**
```json
{ "success": false, "error": "spirit not found" }
```

---

### `PUT /api/v1/spirits/:name`

Update a Spirit. Creates a new version — the old version is preserved and can be restored.

**Headers:** `X-Change-Log: "Updated system prompt for v2"` (optional)

**Body:** same shape as `POST`, minus `name`.

**200:**
```json
{ "success": true, "spirit": { Spirit } }
```

---

### `DELETE /api/v1/spirits/:name`

Deactivate a Spirit (soft delete).

**200:**
```json
{ "success": true, "message": "Spirit deactivated successfully" }
```

---

### `POST /api/v1/spirits/:name/activate`

Activate a specific version of a Spirit.

**Body:**
```json
{ "version": 3 }
```

**200:**
```json
{ "success": true, "message": "Spirit activated successfully" }
```

---

### `POST /api/v1/spirits/:name/deactivate`

Deactivate a Spirit.

**200:**
```json
{ "success": true, "message": "Spirit deactivated successfully" }
```

---

### `GET /api/v1/spirits/:name/versions`

Full version history, newest first.

**200:**
```json
{
  "name":     "support_spirit",
  "versions": [{ Spirit }, { Spirit }, ...],
  "total":    5
}
```

---

## Operators

> [!NOTE]
> Only available when `OperatorAuth` is set. Admin role required for write operations.

### `GET /api/v1/operators`

List all operators.

**200:**
```json
{
  "operators": [{ "id": "...", "username": "...", "role": "admin|operator", "is_active": true }],
  "total": 3
}
```

---

### `POST /api/v1/operators`

Create a new operator.

**Body:**
```json
{ "username": "alice", "password": "secure123", "role": "operator" }
```

**201:**
```json
{ "operator": { Operator } }
```

---

### `GET /api/v1/operators/:id` · `PUT /api/v1/operators/:id` · `DELETE /api/v1/operators/:id`

Standard CRUD. `DELETE` deactivates (soft delete).

---

## Chronicle (Audit Log)

### `GET /api/v1/chronicle`

List interaction records.

**Query params:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | int | 50 | Max results |
| `offset` | int | 0 | Pagination offset |
| `status` | string | — | Filter: `success`, `error`, `rate_limited`, etc. |
| `spirit` | string | — | Filter by Spirit name |
| `memory_key` | string | — | Filter by MemoryKey |
| `from` | RFC3339 | — | Start of time range |
| `to` | RFC3339 | — | End of time range |

**200:**
```json
{
  "items": [
    {
      "id":          "chron_xyz",
      "memory_key":  "whatsapp:+5511999999999",
      "spirit":      "support_spirit",
      "status":      "success",
      "tokens_in":   450,
      "tokens_out":  123,
      "duration_ms": 1840,
      "created_at":  "2026-05-19T10:00:00Z"
    }
  ],
  "total": 1250
}
```

---

### `GET /api/v1/chronicle/:id`

Full interaction detail including thread messages, Actions called, and step timings.

---

## Analytics

### `GET /api/v1/analytics/tokens`

Token usage aggregated by period.

**Query params:** `period` (`today`, `7d`, `30d`), `group_by` (`day`, `hour`, `spirit`).

**200:**
```json
{
  "total_tokens_in":  45000,
  "total_tokens_out": 12000,
  "total_cost_usd":   0.85,
  "series": [
    { "date": "2026-05-19", "tokens_in": 6200, "tokens_out": 1800, "cost_usd": 0.12 }
  ]
}
```

---

### `GET /api/v1/analytics/actions`

Most-called Actions.

**Query params:** `period`, `limit` (default 10).

**200:**
```json
{
  "actions": [
    { "name": "track_order", "call_count": 842, "error_rate": 0.02 }
  ]
}
```

---

### `GET /api/v1/analytics/spirits`

Usage breakdown by Spirit.

**Query params:** `period`.

**200:**
```json
{
  "spirits": [
    {
      "name":         "support_spirit",
      "interactions": 1240,
      "tokens_in":    38000,
      "tokens_out":   9500,
      "avg_duration_ms": 1600
    }
  ]
}
```

---

## Echoes (Conversations)

### `GET /api/v1/echoes/sessions`

List conversation sessions with pagination and filtering.

**Query params:** `limit`, `offset`, `spirit`, `status` (`active`, `ended`), `cursor`.

**200:**
```json
{
  "sessions": [
    {
      "memory_key":    "whatsapp:+5511999999999",
      "spirit":        "support_spirit",
      "message_count": 12,
      "last_message":  "2026-05-19T10:00:00Z",
      "has_vigil":     false
    }
  ],
  "total": 340,
  "cursor": "..."
}
```

---

### `GET /api/v1/echoes/sessions/:memoryKey`

Session detail with recent messages.

**200:**
```json
{
  "memory_key": "whatsapp:+5511999999999",
  "messages": [
    { "role": "user",      "content": "What's my order status?", "created_at": "..." },
    { "role": "assistant", "content": "Order #4821 is out for delivery.", "created_at": "..." }
  ],
  "vigil": null
}
```

---

### `POST /api/v1/echoes/sessions/:memoryKey/messages`

Inject a message into a session (used by Vigil — operator sends message directly).

**Body:**
```json
{ "content": "Hi, I'm taking over from the AI. How can I help?" }
```

**200:**
```json
{ "success": true }
```

---

### `GET /api/v1/echoes`

List individual echo messages with filters. Useful for cross-session message search.

**Query params:** `memory_key`, `limit`, `offset`.

---

## Imprints (Long-term Memory)

### `GET /api/v1/imprints`

List stored user facts with optional filtering.

**Query params:**

| Param | Type | Description |
|-------|------|-------------|
| `user_key` | string | Filter by user identifier |
| `spirit_id` | string | Filter by the Spirit that stored the fact |
| `category` | string | Filter by fact category (e.g. `preference`, `goal`, `personal`) |
| `limit` | int | Default 50 |
| `offset` | int | Default 0 |

**200:**
```json
{
  "imprints": [
    {
      "id":         "imp_abc",
      "user_key":   "whatsapp:+5511999999999",
      "spirit_id":  "support_spirit",
      "fact":       "Prefers to be addressed in English",
      "category":   "preference",
      "source":     "extracted",
      "created_at": "2026-05-19T10:00:00Z"
    }
  ],
  "total":  18,
  "limit":  50,
  "offset": 0
}
```

---

### `DELETE /api/v1/imprints/:id`

Delete a single stored fact.

**200:**
```json
{ "success": true }
```

---

## Vigil (Human Takeover)

### `GET /api/v1/vigil`

List all active Vigil seats across every session.

**200:**
```json
{
  "vigils": [
    {
      "memory_key":  "whatsapp:+5511999999999",
      "operator_id": "op_alice",
      "seat_since":  "2026-05-19T09:45:00Z",
      "expires_at":  "2026-05-19T10:15:00Z"
    }
  ]
}
```

---

### `POST /api/v1/vigil/:memoryKey`

Acquire a Vigil seat. Blocks the AI from processing this session — incoming Pulses return `ErrSessionHeld`.

**Body:** `{}`

**200:**
```json
{
  "vigil": {
    "operator_id": "op_alice",
    "seat_since":  "2026-05-19T09:45:00Z",
    "expires_at":  "2026-05-19T10:15:00Z"
  }
}
```

**409 — seat already taken:**
```json
{ "success": false, "error": "session already held by another operator" }
```

---

### `GET /api/v1/vigil/:memoryKey`

Check Vigil seat status for a session.

**200 — seat active:**
```json
{
  "vigil": { "operator_id": "op_alice", "seat_since": "...", "expires_at": "..." }
}
```

**200 — no active seat:**
```json
{ "vigil": null }
```

---

### `DELETE /api/v1/vigil/:memoryKey`

Release the Vigil seat. The AI resumes processing incoming Pulses.

**200:**
```json
{ "success": true }
```

---

### `POST /api/v1/vigil/:memoryKey/echoes`

Send a message as the operator (Vigil must be active on this session).

**Body:**
```json
{ "content": "We're working on your issue, please hold." }
```

**200:**
```json
{ "success": true }
```

---

## Rites (Approval Workflows)

### `GET /api/v1/rites`

List Rites with optional status filter.

**Query params:** `status` (`pending`, `approved`, `rejected`, `expired`), `limit`, `offset`, `cursor`.

**200:**
```json
{
  "rites": [
    {
      "id":          "rite_xyz",
      "memory_key":  "whatsapp:+5511999999999",
      "spirit":      "support_spirit",
      "reason":      "Requesting refund of $49.99 for order #4821",
      "context":     { "order_id": "4821", "amount": 49.99 },
      "status":      "pending",
      "created_at":  "2026-05-19T10:00:00Z",
      "expires_at":  "2026-05-19T10:30:00Z"
    }
  ],
  "total": 5
}
```

---

### `GET /api/v1/rites/:id`

Full Rite detail.

---

### `POST /api/v1/rites/:id/approve`

Approve a Rite. The Spirit resumes execution and proceeds with the approved action.

**Body:**
```json
{ "note": "Verified — refund approved" }
```

**200:**
```json
{ "success": true }
```

---

### `POST /api/v1/rites/:id/reject`

Reject a Rite. The Spirit receives the decision and responds to the user accordingly.

**Body:**
```json
{ "note": "Outside return window — rejected" }
```

**200:**
```json
{ "success": true }
```

---

## Event Configurations (Pulse Flows)

### `GET /api/v1/event-configurations`

List all registered event type → Spirit links.

**200:**
```json
{
  "configurations": [
    {
      "event_type":     "customer_message",
      "default_agent":  "support_spirit",
      "allowed_agents": ["support_spirit", "billing_spirit"],
      "scouts":         ["user_profile", "order_context"],
      "pathfinder":     "content_pathfinder",
      "voice":          "whatsapp"
    }
  ]
}
```

---

### `GET /api/v1/event-configurations/:eventType`

Get configuration for a specific event type.

---

### `PUT /api/v1/event-configurations/:eventType`

Update a Link at runtime. Changes take effect on the next Pulse — no redeploy needed.

**Body:**
```json
{
  "default_agent":  "support_spirit_v2",
  "allowed_agents": ["support_spirit_v2"],
  "scouts":         ["user_profile"]
}
```

---

### `DELETE /api/v1/event-configurations/:eventType`

Remove a Link. Pulses for that event type will return `404` until re-registered.

---

## HTTP Tools (Conduit)

### `GET /api/v1/http-tools`

List all configured HTTP tools.

**200:**
```json
{
  "tools": [
    {
      "id":          "tool_abc",
      "name":        "get_order_status",
      "description": "Fetches order status from OMS",
      "url":         "https://api.myservice.com/orders/{order_id}",
      "method":      "GET",
      "headers":     { "Authorization": "Bearer {{SECRET}}" }
    }
  ]
}
```

---

### `POST /api/v1/http-tools`

Create a new HTTP tool that Spirits can invoke as an Action.

**Body:**
```json
{
  "name":        "create_ticket",
  "description": "Creates a support ticket in the helpdesk",
  "url":         "https://helpdesk.myservice.com/tickets",
  "method":      "POST",
  "headers":     { "Content-Type": "application/json" },
  "body_template": "{\"subject\": \"{{subject}}\", \"priority\": \"{{priority}}\"}"
}
```

---

### `GET /api/v1/http-tools/:id` · `PUT /api/v1/http-tools/:id` · `DELETE /api/v1/http-tools/:id`

Standard CRUD.

---

### `POST /api/v1/http-tools/:id/test`

Run a live test of the HTTP tool with sample arguments.

**Body:**
```json
{ "args": { "order_id": "4821" } }
```

**200:**
```json
{
  "success":     true,
  "status_code": 200,
  "response":    "{ \"status\": \"delivered\" }",
  "duration_ms": 142
}
```

---

## Admin

> [!NOTE]
> These routes require `admin` role.

### `GET /api/v1/admin/engine-config`

Get the active WeaveConfig as stored in the database.

---

### `PUT /api/v1/admin/engine-config`

Update the WeaveConfig at runtime.

**Body:**
```json
{
  "reasoning_timeout_seconds": 90,
  "max_reasoning_iterations":  15,
  "parallel_action_execution": true
}
```

---

### `POST /api/v1/admin/config/reload`

Reload event configurations from the database into the in-memory ConfigCache. Use after bulk Link updates.

**200:**
```json
{ "success": true, "loaded": 12 }
```

---

## SSE (Server-Sent Events)

Real-time event streams backed by Redis PubSub. All instances publish to the same channels — every subscriber on any instance receives the event.

> [!NOTE]
> Only available when `PubSub` is set in `ManagementDeps`.

**Heartbeat:** `: ping` comment line every 30 seconds — keeps connections alive through proxies.

**Browser usage:**
```javascript
const token = localStorage.getItem('eywa_token')
const es = new EventSource(`/api/v1/sse/rites?token=${token}`)
es.onmessage = (e) => {
  const event = JSON.parse(e.data)
  console.log(event.event, event) // e.g. "rite_created" { rite: {...} }
}
```

---

### `GET /api/v1/sse/rites`

Stream Rite lifecycle events across all sessions.

**Events:**

| Event | Payload |
|-------|---------|
| `rite_created` | `{ "event": "rite_created", "rite": { Rite } }` |
| `rite_decided` | `{ "event": "rite_decided", "rite_id": "...", "status": "approved\|rejected" }` |
| `rite_expired` | `{ "event": "rite_expired", "rite_id": "..." }` |

---

### `GET /api/v1/sse/vigil`

Stream Vigil seat events across all sessions.

**Events:**

| Event | Payload |
|-------|---------|
| `vigil_acquired` | `{ "event": "vigil_acquired", "memory_key": "...", "operator_id": "..." }` |
| `vigil_released` | `{ "event": "vigil_released", "memory_key": "..." }` |

---

### `GET /api/v1/sse/echoes/:memoryKey`

Stream per-session events for a specific conversation.

**Events:**

| Event | Payload |
|-------|---------|
| `message_added` | `{ "event": "message_added", "message": { "role": "...", "content": "...", "created_at": "..." } }` |
| `vigil_acquired` | `{ "event": "vigil_acquired", "operator_id": "...", "seat_since": "..." }` |
| `vigil_released` | `{ "event": "vigil_released" }` |
| `rite_created` | `{ "event": "rite_created", "rite": { Rite } }` |

---

## Error Responses

All endpoints return consistent error envelopes:

```json
{
  "success": false,
  "error":   "machine-readable code",
  "message": "human-readable description"
}
```

Common HTTP status codes:

| Status | Meaning |
|--------|---------|
| `400` | Bad request — invalid body or missing required fields |
| `401` | Unauthorized — missing or invalid token |
| `403` | Forbidden — role insufficient for this operation |
| `404` | Not found — resource does not exist |
| `409` | Conflict — e.g. Vigil seat already held |
| `429` | Rate limited |
| `500` | Internal server error |
