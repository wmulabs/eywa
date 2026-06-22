# 🔐 Authentication & Security

Eywa's HTTP surface (the `fiber` sub-module) is mounted by a **single registrar**,
`RegisterRoutes(app, weave, RouteDeps{...})`. Routes fall into three buckets:

- **Open** — health checks and event ingestion (events are webhook-style; see below).
- **Management** — everything sensitive (Spirit CRUD, conversation history, scheduling, chronicle,
  analytics, Vigil, Rites, config, HTTP tools, app-token management, SSE). **Always behind auth.**
- **Internal** — the Keeper/Cloud Tasks callback (`/internal/execute-event`), guarded by its own
  middleware (e.g. OIDC).

There are **two independent auth axes** — management auth and event auth — because they serve
different audiences (humans/operators vs. machine callers).

```go
eywafiber.RegisterRoutes(app, weave, eywafiber.RouteDeps{
    // ── Axis 1: management auth (required to expose any management route) ──
    APIKeys: map[string]string{adminKey: "admin"},   // and/or OperatorAuth / TokenValidator
    AppTokenRepo: appTokenRepo,                       // exposes /api/v1/app-tokens (authed)
    // ... management repos: ChronicleQueryRepo, EchoRepo, VigilRepo, RiteRepo, ...

    // ── Axis 2: event auth (optional; empty = open) ──
    EventAuth:      []eywa.TokenValidator{eywa.NewAppTokenValidator(appTokenRepo)},
    EventVerifiers: []eywa.RequestVerifier{twilio.NewSignatureVerifier(authToken)},
})
```

**Fail-closed:** if you provide a protected repository (e.g. `AppTokenRepo`, `VigilRepo`) without any
management validator, `RegisterRoutes` returns an error rather than exposing it unauthenticated.

---

## Axis 1 — Management auth

Every management route sits behind the auth middleware. Configure one or more modes; a request is
accepted if any validator accepts the `Authorization: Bearer <token>` header. (All modes use the
**Bearer** scheme — including the static API key.)

| Mode | RouteDeps field | Use |
|---|---|---|
| Static API key | `APIKeys map[string]string` (key → role) | machine/admin, simplest |
| Operator JWT | `OperatorAuth *eywa.OperatorAuth` | human operators; adds `POST /api/v1/auth/token` (login) + roles |
| External JWT / JWKS | `TokenValidator eywa.TokenValidator` | SSO / OIDC |

```bash
curl -H "Authorization: Bearer $ADMIN_KEY" https://host/api/v1/chronicle
```

`/api/v1/operators` additionally requires the `admin` role.

---

## Axis 2 — Event auth

Inbound event routes (`POST /api/v1/events/:event_key` and its `/stream`, `/async` variants) are
**open by default** — many channels are webhooks that authenticate by their own scheme. Set either of
the following to require authentication; a request passes if **any** validator or verifier accepts it.

### Bearer — app tokens (`EventAuth`)

Revocable, optionally-expiring tokens for first-party/server callers. Stored as a SHA-256 hash; the
plaintext is shown once at creation. Managed via `/api/v1/app-tokens` (needs `AppTokenRepo`).

```go
repo := eywamongo.NewAppTokenRepository(db)
RouteDeps{
    AppTokenRepo: repo,                                                  // manage tokens (admin-authed)
    EventAuth:    []eywa.TokenValidator{eywa.NewAppTokenValidator(repo)}, // require a token on events
}
```

Mint / list / revoke:
```bash
curl -X POST host/api/v1/app-tokens -H "Authorization: Bearer $ADMIN_KEY" \
  -d '{"name":"mobile-app","ttl_seconds":0}'      # ttl 0 = never expires; returns the plaintext once
curl host/api/v1/app-tokens     -H "Authorization: Bearer $ADMIN_KEY"   # list (never returns the hash)
curl -X DELETE host/api/v1/app-tokens/$ID -H "Authorization: Bearer $ADMIN_KEY"  # revoke (takes effect immediately)
```

The caller then sends `Authorization: Bearer <app-token>` on event requests.

### Signature — `EventVerifiers`

For webhook senders that sign the request (the dominant pattern). A `RequestVerifier` authenticates
from the full request (headers + raw body), which a bearer token cannot.

| Verifier | Scheme | Secret |
|---|---|---|
| `eywa.NewHMACVerifier(secret)` | Stripe-style HMAC-SHA256 over `"<t>.<body>"`, header `t=…,v1=…`, timestamp window (anti-replay) | a secret you share with your caller |
| `twilio.NewSignatureVerifier(authToken)` | `X-Twilio-Signature` (via the official twilio-go validator) | your Twilio **Auth Token** |
| `dialog360.NewBasicAuthVerifier(user, pass)` | HTTP Basic Auth on the webhook URL | the user/pass set in 360dialog |

Providers sign automatically — you don't issue a token, you give Eywa the secret it already uses:

```go
import twilio "github.com/wmulabs/eywa/channels/whatsapp/twilio"

RouteDeps{ EventVerifiers: []eywa.RequestVerifier{
    twilio.NewSignatureVerifier(os.Getenv("TWILIO_AUTH_TOKEN")),
}}
```

> **Twilio behind a proxy:** the signature is over the exact public URL. Reconstruct the external URL
> (`X-Forwarded-Proto` / `X-Forwarded-Host`) or the check fails.
>
> **360dialog:** it does **not** forward Meta's `X-Hub-Signature-256` (the Meta app secret belongs to
> 360dialog as the BSP) — its documented webhook security is HTTP Basic Auth, so use the Basic Auth verifier.
>
> **Custom verifier:** implement `eywa.RequestVerifier` (`Verify(ctx, VerifiableRequest)`) for any other scheme.

---

## Internal callback

When async dispatch or scheduling is enabled, `POST /internal/execute-event` is registered for the
Keeper/Cloud Tasks callback. Protect it with `RouteDeps.InternalMiddleware` (e.g. a Cloud Tasks OIDC
verifier); it is otherwise open.

```go
RouteDeps{ InternalMiddleware: []fiber.Handler{ cloudtasks.NewOIDCMiddleware(serviceURL) } }
```

---

## Other security defaults

These hold regardless of the HTTP layer (see also the project's security invariants):

- **SSRF guard** on all outbound HTTP (HTTPTool, MCP) — blocks private IPs, loopback, link-local, IMDS.
- **Response body limits** (`io.LimitReader`) on every HTTP read.
- **Secrets stored hashed** (SHA-256) and compared in constant time (API keys, app tokens).
- **Rate limiting** on the operator login endpoint.
