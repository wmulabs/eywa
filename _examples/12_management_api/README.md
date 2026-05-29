# Example 12: Management API

Demonstrates the **eywa/fiber** sub-module — a full REST management layer for Eywa.

## What this shows

- Wiring all repositories into `ManagementDeps`
- Registering all management routes with `RegisterManagementRoutes`
- Auth modes: static API keys and built-in operator JWT (`OperatorAuth`)
- Graceful shutdown via OS signals

## Routes registered (all under `/api/v1`, auth required)

| Resource | Methods |
|---|---|
| Spirits | `GET/POST /spirits`, `GET/PUT/DELETE /spirits/:name` |
| Chronicle | `GET /chronicle`, `GET /chronicle/:id` |
| Analytics | `GET /analytics/tokens`, `/analytics/actions`, `/analytics/spirits` |
| Conversations | `GET /echoes/sessions`, `GET/POST /echoes/sessions/:key/messages` |
| Config | `GET/PUT /weave-config`, `POST /weave-config/reload` |
| HTTP Tools | `GET/POST /http-tools`, `GET/PUT/DELETE /http-tools/:id` |
| Vigil | `GET/POST/DELETE /vigil/:memoryKey`, `POST /vigil/:memoryKey/echoes` |
| Rites | `GET /rites`, `GET /rites/:id`, `POST /rites/:id/decide` |
| Operators | `GET/POST /operators`, `GET/PUT/DELETE /operators/:id` |
| Auth | `POST /auth/token` (public — no auth required) |

## Auth configuration

```go
deps := eywafiber.ManagementDeps{
    // Option 1: Static API key (header: X-API-Key)
    APIKeys: map[string]string{
        "dev-key-change-me": "admin",
    },
    // Option 2: Built-in operator JWT
    OperatorAuth: operatorAuth,
    // Option 3: External JWT / JWKS — set ExternalJWTConfig
}
```

## Running

```bash
export OPENAI_API_KEY=sk-...
export MONGO_URL=mongodb://localhost:27017
export REDIS_URL=redis://localhost:6379
export JWT_SECRET=change-in-production
export API_KEY=dev-key-change-me

go run .
```

Test the API:

```bash
# Health check (no auth)
curl http://localhost:8080/health

# List spirits (API key auth)
curl -H "X-API-Key: dev-key-change-me" http://localhost:8080/api/v1/spirits

# Login as operator (returns JWT)
curl -X POST http://localhost:8080/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"secret"}'
```
