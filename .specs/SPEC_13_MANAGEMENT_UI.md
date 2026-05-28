# SPEC 13 — eywa-cockpit: The Living Management Console

> "The Na'vi do not simply use Eywa — they listen to her. The cockpit is how operators listen."

---

## Vision

eywa-cockpit is a standalone web application that connects to any running eywa engine instance.
No code, no terminal, no MongoDB queries. Configure spirits, monitor conversations in real time,
approve Rites, review cost intelligence, manage Lore knowledge bases, and control the Weave — all
from a single interface that speaks the eywa mythology.

This is not a demo tool or an internal admin panel.
This is the **best agent management interface in the Go ecosystem** — production-grade, real-time,
and built around the metaphors that make eywa memorable.

---

## Comparison: Why Better Than Agno Playground

| Capability | Agno | eywa-cockpit |
|---|---|---|
| Agent CRUD | ✓ | ✓ Spirit Grove (typed: conversational / executor / orchestrator / notifier) |
| Chat playground | ✓ | ✓ Echo Chamber (with role badges + iteration timeline) |
| Session history | ✓ | ✓ + full Chronicle drill-down per interaction |
| Operator takeover (live) | ✗ | ✓ Vigil Watch — acquire seat, direct message, real-time indicator |
| Approval workflows | ✗ | ✓ Rite Chamber — approve/reject with context, SSE notifications |
| Long-term memory viewer | ✗ | ✓ Imprint Records — per-user facts, categories, source |
| Cost intelligence | ✗ | ✓ Ledger — per-spirit cost, token budget alerts, model routing |
| Knowledge base management | Basic | ✓ Lore Sanctum — ingest, chunk viewer, vector query tester |
| Multi-spirit orchestration view | ✗ | ✓ Orchestration graph per interaction in Chronicle |
| Event routing config | ✗ | ✓ Pulse Flows — live CRUD of EventConfiguration without deploy |
| HTTP Tools visual test runner | ✗ | ✓ Conduit Gateway — build, test, debug HTTP tools in UI |
| Real-time engine config hot-reload | ✗ | ✓ Weave Config — change timeouts, summarization, lock TTL live |
| Operator management | ✗ | ✓ full CRUD, role assignment, JWT issuance |
| SSE / live updates | ✗ | ✓ conversations, Rites, Vigil, analytics |
| Mythology UI theme | ✗ | ✓ bioluminescent dark theme, Na'vi forest aesthetic |

---

## Repository

Separate repo: `eywa-cockpit` (not part of `eywa` library).
Consumes the eywa REST API only — no direct DB access, no Go imports.

---

## Distribution Model

### Self-hosted (open source, free)

Clone, deploy anywhere, point at engine URL. All features. No registration.
Tokens stored in `localStorage` — no server needed.

### Hosted SaaS (future — `cockpit.eywa.dev`)

Lightweight backend for multi-user, multi-engine profiles and billing.
Engine data never passes through the SaaS layer.

---

## Technology Stack

| Layer | Choice | Reason |
|---|---|---|
| Framework | **Next.js 14** (App Router) | SSR optional, mostly client-side; great ecosystem |
| UI Components | **shadcn/ui** + **Radix UI** | Accessible, unstyled primitives; full control |
| Styling | **Tailwind CSS** | Utility-first; easy to implement custom Na'vi theme |
| State / Server Data | **TanStack Query v5** | Caching, polling, optimistic updates, SSE integration |
| Charts | **Recharts** | Composable, works well with Tailwind; lightweight |
| Code Editor | **Monaco Editor** (via `@monaco-editor/react`) | System prompt and body template editing |
| Real-time | **SSE** (EventSource API) with polling fallback | Native browser support, no WS infra needed |
| Auth | JWT in `localStorage`, `Authorization: Bearer` on every request | Stateless; engine owns auth |
| Icons | **Lucide React** | Clean, consistent |

---

## Visual Design Language

### Theme: Pandoran Night

Primary aesthetic: a bioluminescent forest at night. Dark backgrounds with cyan/teal/indigo glowing accents.
Every entity has a color and glyph identity rooted in the mythology.

```
Background:    #0a0f14  (deep Pandoran night)
Surface:       #111820  (elevated panels)
Border:        #1e2d3d  (subtle separation)
Text primary:  #e2eaf4  (moonlit)
Text muted:    #6b8299  (shadow)

Accent cyan:   #00d4d4  (Eywa glow — active state, links, highlights)
Accent teal:   #0d9488  (secondary accent)
Accent amber:  #f59e0b  (warnings, Rite pending)
Accent red:    #ef4444  (errors, rejections)
Accent green:  #22c55e  (success, approvals)
Accent purple: #8b5cf6  (orchestrator spirits, special state)
```

### Entity Color Map

| Entity | Color | Glyph |
|---|---|---|
| Spirit | cyan | `⟡` (crystalline, alive) |
| Echo / Conversation | indigo | `≋` (waves, voices) |
| Lore | emerald | `⌬` (triangular, accumulated knowledge) |
| Imprint | violet | `◎` (bond, tsaheylu) |
| Rite | amber | `⊕` (sacred circle, pending action) |
| Vigil | teal | `⊘` (watchful eye, takeover) |
| Chronicle | slate | `⌖` (crosshair, observation) |
| Ledger | yellow | `◈` (diamond, value) |
| Conduit | orange | `⇌` (bridge, bidirectional) |
| Pulse | blue | `⬡` (hexagonal, heartbeat) |
| Operator | rose | `⬡` (human node) |

### Typography

- Display: `Inter` or `Geist` — clean, technical
- Monospace (prompts, payloads): `JetBrains Mono` — readable, precise
- Mythology headings use `letter-spacing: 0.1em` in SMALL CAPS for section labels

---

## Application Shell

```
┌─────────────────────────────────────────────────────────────┐
│  ⟡ eywa-cockpit     [engine indicator ● connected]    [Ops] │
├──────────┬──────────────────────────────────────────────────┤
│          │                                                   │
│ NAV      │   MAIN CONTENT AREA                              │
│          │                                                   │
│ ⬡ Home  │                                                   │
│ ⟡ Spirits│                                                   │
│ ≋ Echoes │                                                   │
│ ⌬ Lore  │                                                   │
│ ◎ Imprint│                                                   │
│ ⊕ Rites  │                                                   │
│ ⊘ Vigil  │                                                   │
│ ⌖ Chron  │                                                   │
│ ◈ Ledger │                                                   │
│ ⇌ Conduit│                                                   │
│ ⬡ Pulses │                                                   │
│ ⚙ Weave  │                                                   │
│ ⬡ Ops    │                                                   │
│          │                                                   │
│ ──────── │                                                   │
│ [engine] │                                                   │
└──────────┴──────────────────────────────────────────────────┘
```

### Engine Connection Banner

Top bar shows engine URL, connection status (live health check every 30s via `GET /health`),
engine version, and operator name decoded from JWT claims.

Engine status chip:
- `● connected` (green) — health OK
- `● degraded` (amber) — health endpoint slow or returning 2xx with warnings
- `● unreachable` (red) — health check failed

On first visit (no engine configured): full-screen connection setup modal.

---

## Auth & Connection Setup

### Connect Screen (first run)

```
┌─────────────────────────────────────────────┐
│            ⟡ Connect to Eywa Engine         │
│                                             │
│  Engine URL                                 │
│  ┌─────────────────────────────────────┐   │
│  │ https://my-agent-engine.run.app     │   │
│  └─────────────────────────────────────┘   │
│                                             │
│  Auth Method  ○ API Key  ● Operator JWT     │
│                                             │
│  Token / API Key                            │
│  ┌─────────────────────────────────────┐   │
│  │ ••••••••••••••••••••••••           │   │
│  └─────────────────────────────────────┘   │
│                                             │
│  — or —  [ Login with username + password ] │
│                                             │
│              [ Connect ]                    │
└─────────────────────────────────────────────┘
```

**Login flow** (OperatorAuth mode):
```
POST /api/v1/auth/token
Body:  { "username": "string", "password": "string" }
Response: { "token": "eyJ...", "operator_id": "...", "role": "admin|operator" }
```

Token stored in `localStorage["eywa_token"]` and `localStorage["eywa_engine_url"]`.

Role decoded from JWT `role` claim. UI adapts: admin sees all screens, operator sees
Echoes / Rites / Vigil / Chronicle (read-only config).

---

## Screen 1 — Hometree (Dashboard)

> The dashboard is the heartbeat of your Weave.

### Layout

```
┌──────────┬──────────┬──────────┬──────────┐
│ Sessions │  Tokens  │  Errors  │ Avg Iters│
│  1,248   │ 4.2M     │  0.8%    │  2.3     │
│  today   │  today   │  today   │  today   │
└──────────┴──────────┴──────────┴──────────┘

┌─────────────────────────────┬─────────────┐
│  Token Usage (7d)           │ Top Spirits │
│  [area chart]               │ by sessions │
│                             │ [bar chart] │
└─────────────────────────────┴─────────────┘

┌──────────────────────────┬──────────────────┐
│  Recent Errors            │  Pending Rites   │
│  [list]                   │  [list]          │
└──────────────────────────┴──────────────────┘

┌────────────────────────────────────────────┐
│  Top Actions by Call Count (24h)           │
│  [horizontal bar chart]                    │
└────────────────────────────────────────────┘
```

### Data Sources

**KPI cards** — polled every 60s:
```
GET /api/v1/analytics/tokens?period=today
Response:
{
  "total_tokens": 4200000,
  "reasoning": { "prompt": 3100000, "completion": 800000, "total": 3900000 },
  "media": { "prompt": 250000, "completion": 50000, "total": 300000 },
  "estimated_cost_usd": 12.40
}

GET /api/v1/analytics/spirits?period=today
Response:
{
  "spirits": [
    {
      "spirit_name": "support-bot",
      "interactions": 842,
      "avg_iterations": 2.1,
      "error_rate": 0.006,
      "total_tokens": 2100000
    }
  ]
}
```

**Token Usage chart** — `GET /api/v1/analytics/tokens?period=7d&group_by=day`
Returns array of `{ date, reasoning_tokens, media_tokens, estimated_cost_usd }`.

**Top Actions chart** — `GET /api/v1/analytics/actions?period=24h&limit=10`
Returns array of `{ action_name, call_count, error_count, avg_duration_ms }`.

**Recent Errors** — `GET /api/v1/chronicle?status=error&limit=5`

**Pending Rites** — `GET /api/v1/rites?status=pending&limit=5`
Shown as amber cards with approve/reject quick actions inline.

---

## Screen 2 — Spirit Grove (Spirit Management)

> Every Spirit is a soul you sculpted. Configure it here, and it comes alive in the Weave.

### List View

```
┌────────────────────────────────────────────────────────────┐
│  Spirit Grove                              [ + New Spirit ] │
│  Filter: [all types ▾]  [active ▾]  [search...]           │
├────────┬──────────────┬────────────┬───────┬───────────────┤
│  Name  │  Type        │  Model     │ Ver.  │  Status       │
├────────┼──────────────┼────────────┼───────┼───────────────┤
│ ⟡ sup.. │ conversatio. │ gpt-4o     │ v12   │ ● active      │
│ ⟡ not.. │ notifier     │ claude-3-5 │ v3    │ ● active      │
│ ⟡ orch  │ orchestrator │ gemini-2.0 │ v1    │ ○ inactive    │
└────────┴──────────────┴────────────┴───────┴───────────────┘
```

Each row: click → detail/edit drawer. Row actions: activate / deactivate / duplicate / delete.

**Endpoints:**
```
GET /api/v1/spirits
Response: { "spirits": [Spirit] }

GET /api/v1/spirits/:name
Response: { "spirit": Spirit }

POST /api/v1/spirits/:name/activate
POST /api/v1/spirits/:name/deactivate
DELETE /api/v1/spirits/:name
```

### Spirit Detail / Edit Drawer

Full-width right drawer with tabs:

#### Tab 1 — Identity

```
Name *          [__________________________]
Description     [__________________________]
Type *          [ conversational ▾ ]
                  conversational | executor | orchestrator | notifier
Specialization  [__________________________]
Status          [● Active]  [Deactivate]
Version         v12  (created 2024-01-15 by willian)
Change log      [last change note here...]
```

#### Tab 2 — Oracle (Model)

```
Provider *   [ anthropic ▾ ]
Model *      [ claude-sonnet-4-5 ▾ ]  (populated from known models per provider)
Temperature  [━━━━━●━━━━━━] 0.7
Max Tokens   [____] 4096
Top P        [____] 1.0

Extra Config (JSON)
┌────────────────────────────────┐
│ {                              │
│   "thinking": true             │
│ }                              │
└────────────────────────────────┘
```

#### Tab 3 — Prompts

```
System Prompt *
┌─────────────────────────────────────────────────────┐  Monaco editor
│ You are a support specialist for {{company_name}}.  │  Full height
│ Always respond in Portuguese.                       │  Monospace font
│ ...                                                 │  Syntax highlight
└─────────────────────────────────────────────────────┘  for {{variables}}

Business Error Instructions
┌─────────────────────────────────────────────────────┐
│ When a business rule prevents completion, apologize │
│ and offer to escalate.                              │
└─────────────────────────────────────────────────────┘

Voice Delivery Instructions
[ ] Enforce Voice Delivery
┌─────────────────────────────────────────────────────┐
│ Always end responses with a clear question.         │
└─────────────────────────────────────────────────────┘
```

#### Tab 4 — Actions

All registered actions from engine shown as toggleable chips.
For each enabled action, expand to configure:

```
Actions registered in engine:  [from GET /api/v1/discovery → actions[]]

[ ✓ ] search_knowledge_base    IsCritical: [ ] Yes  Override description: [____]
[ ✓ ] request_rite             IsCritical: [✓] Yes  Override description: [____]
[   ] send_email               IsCritical: [ ] Yes  Override description: [____]
[ ✓ ] get_order_status         IsCritical: [ ] Yes  Override description: [____]
```

#### Tab 5 — Lore (Knowledge)

```
Associated Lore:
┌───────────────┬─────────┬─────────────────────────────┐
│  Lore Name    │  Chunks │  Config                     │
├───────────────┼─────────┼─────────────────────────────┤
│ ⌬ product-faq │  1,240  │ topK: 5  minScore: 0.75     │
│ ⌬ returns-pol │    328  │ topK: 3  minScore: 0.80     │
└───────────────┴─────────┴─────────────────────────────┘
[ + Add Lore ]
```

#### Tab 6 — Type Config

Shown based on `type` field:

**Conversational:**
```
[ ] Cross-channel memory (share context across channels)
```

**Executor:**
```
[ ] With session memory
[ ] With media processing
```

**Orchestrator:**
```
Mode             [ serial ▾ ]  serial | parallel | logic_router
Sub-Spirits      [select spirits...]  (multi-select from list)
Logic Router     [____]  (name of registered router function)
Max Depth        [3]
[ ] Parallel summon
```

**Notifier:**
```
Mode       [ template ▾ ]
Template   [__________________________________________________]
Conditions  [ + Add Condition ]
  Field: [____]  Operator: [equals ▾]  Value: [____]  [×]
```

#### Tab 7 — History

Version timeline:
```
v12  2024-05-18  by willian  "Updated system prompt for new product line"
v11  2024-05-10  by system   "Temperature adjusted to 0.7"
v10  2024-04-30  by willian  "Added request_rite action"
...
[ Compare v12 ↔ v10 ]  (diff view of system_prompt)
```

**Endpoint:**
```
GET /api/v1/spirits/:name/versions
Response: { "name": "...", "versions": [Spirit], "total": N }  (sorted desc by version)
```

### Create Spirit

Full-page form (same tabs structure). Required: name, type, provider, model, system_prompt.

```
POST /api/v1/spirits
Body:
{
  "name": "support-bot",
  "description": "Customer support specialist",
  "type": "conversational",
  "specialization": "e-commerce support",
  "system_prompt": "You are...",
  "enforce_voice_delivery": false,
  "voice_delivery_instructions": "",
  "business_error_instructions": "",
  "allowed_actions": [
    { "name": "get_order_status", "is_critical": false },
    { "name": "request_rite", "is_critical": true }
  ],
  "model_config": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-5",
    "temperature": 0.7,
    "max_tokens": 4096
  },
  "lore_ids": ["lore-abc123"],
  "conversational_config": { "cross_channel_memory": false },
  "metadata": {}
}
Response: { "success": true, "spirit": Spirit }
```

---

## Screen 3 — Echo Chamber (Conversation Studio)

> Every message is an Echo in Eywa's Web of Voices. Monitor them in real time.

### Session List

```
┌─────────────────────────────────────────────────────────────┐
│  Echo Chamber                                               │
│  Filter: [spirit ▾] [date ▾] [status ▾] [search memoryKey] │
├──────────────┬─────────────┬──────────┬────────┬───────────┤
│  Memory Key  │  Spirit     │  Last    │  Msgs  │  Status   │
├──────────────┼─────────────┼──────────┼────────┼───────────┤
│ 55511112222  │ support-bot │  2m ago  │  12    │ ● active  │
│ 55599998888  │ support-bot │  15m ago │   5    │ ○ idle    │
│ 55522223333  │ orch-spirit │  1h ago  │  34    │ 🔒 vigil  │
└──────────────┴─────────────┴──────────┴────────┴───────────┘
```

**Endpoints:**
```
GET /api/v1/echoes/sessions?limit=50&cursor=&spirit=&status=
Response:
{
  "sessions": [
    {
      "memory_key": "55511112222",
      "spirit_name": "support-bot",
      "last_message_at": "2024-05-18T14:30:00Z",
      "message_count": 12,
      "vigil_active": false
    }
  ],
  "next_cursor": "..."
}

GET /api/v1/echoes/sessions/:memoryKey
Response:
{
  "memory_key": "55511112222",
  "spirit_name": "support-bot",
  "started_at": "2024-05-18T14:00:00Z",
  "message_count": 12,
  "vigil": null | { "operator_id": "...", "seat_since": "...", "expires_at": "..." }
}
```

### Session Detail View

Split view: left = message thread, right = metadata + actions.

```
┌─────────────────────────────┬─────────────────────────────┐
│  ≋ 55511112222              │  Session Info               │
│  support-bot • v12          │  Started: 14:00             │
│  ──────────────────────     │  Spirit: support-bot v12    │
│                             │  Model: claude-sonnet-4-5   │
│  [user]  14:00              │  Iterations: 12             │
│  I need to track my order   │                             │
│                             │  ─────────────────────────  │
│  [assistant]  14:00         │  Vigil Status               │
│  Sure! Please provide your  │  ○ No operator seated       │
│  order number.              │                             │
│                             │  [ Acquire Seat ]           │
│  [user]  14:01              │                             │
│  #ORD-99182                 │  ─────────────────────────  │
│                             │  Recent Rites               │
│  [assistant]  14:01         │  ⊕ PENDING — approve return │
│  Your order #ORD-99182 is   │    [Approve] [Reject]       │
│  in transit...              │                             │
└─────────────────────────────┴─────────────────────────────┘
```

**Message thread:**
```
GET /api/v1/echoes/sessions/:memoryKey/messages?limit=100
Response: (implied — returns Echo records)
{
  "messages": [
    {
      "id": "...",
      "memory_key": "55511112222",
      "role": "user" | "assistant" | "operator",
      "content": "...",
      "created_at": "..."
    }
  ]
}
```

**Real-time updates (SSE):**
```
GET /api/v1/sse/echoes/:memoryKey
Event types:
  message_added   { "message": EchoMessage }
  vigil_acquired  { "operator_id": "...", "seat_since": "..." }
  vigil_released  {}
  rite_created    { "rite": Rite }
```
**Vigil actions:**
```
POST /api/v1/vigil/:memoryKey
Body: {}
Response: { "vigil": { "operator_id": "...", "seat_since": "...", "expires_at": "..." } }

DELETE /api/v1/vigil/:memoryKey
Response: { "success": true }

GET /api/v1/vigil/:memoryKey
Response: { "vigil": Vigil | null }
```

**Operator direct message (only when seat held):**
```
POST /api/v1/vigil/:memoryKey/echoes
Body: { "content": "Hello, how can I help you directly?" }
Response: { "success": true }
```

**Send as management (no seat required):**
```
POST /api/v1/echoes/sessions/:memoryKey/messages
Body: { "content": "...", "role": "operator" }
Response: { "success": true }
```

**Vigil seat indicator in thread:**
When operator holds seat, thread shows a `[VIGIL ACTIVE — you are controlling this conversation]`
banner. Input field appears at the bottom. Messages sent by operator get `[operator]` role badge.

---

## Screen 4 — Lore Sanctum (Knowledge Base Management)

> Ancestral wisdom stored in the roots of the Tree of Souls. Lore is what the Spirits remember.

### Lore List

```
┌──────────────────────────────────────────────────┐
│  Lore Sanctum                    [ + New Lore ]  │
├────────────────┬────────┬──────────┬─────────────┤
│  Name          │ Chunks │ Spirits  │  Updated    │
├────────────────┼────────┼──────────┼─────────────┤
│ ⌬ product-faq  │  1,240 │  2       │  2d ago     │
│ ⌬ returns-pol  │    328 │  1       │  1w ago     │
└────────────────┴────────┴──────────┴─────────────┘
```

**New endpoints needed:**
```
GET /api/v1/lore
Response: { "lore": [Lore] }

POST /api/v1/lore
Body: { "name": "...", "description": "...", "chunk_size": 500, "overlap": 50 }
Response: { "lore": Lore }

GET /api/v1/lore/:id
Response: { "lore": Lore, "chunk_count": 1240 }

DELETE /api/v1/lore/:id
Response: { "success": true }
```

### Lore Detail

Tabs: Documents | Chunks | Query Tester | Settings

#### Documents Tab

```
[ Upload Document ]  (drag-and-drop zone, PDF / TXT / MD)
Status: indexing 3 documents...

┌────────────────────┬──────┬───────────┬──────────────┐
│  File              │ Size │ Chunks    │ Status       │
├────────────────────┼──────┼───────────┼──────────────┤
│ product-catalog.pdf│ 2MB  │ 824       │ ✓ ready      │
│ faq-2024.txt       │ 45KB │ 416       │ ✓ ready      │
│ new-policy.pdf     │ 1MB  │ indexing… │ ⟳ processing │
└────────────────────┴──────┴───────────┴──────────────┘
```

```
POST /api/v1/lore/:id/ingest
Content-Type: multipart/form-data
Body: file (binary), metadata (JSON string)
Response: { "job_id": "...", "status": "processing" }

GET /api/v1/lore/:id/ingest/:job_id/status
Response: { "status": "processing|ready|error", "chunks_created": 416 }
```

#### Chunks Tab

```
Search chunks: [________________]   Page [1] of 248

┌────────────────────────────────────────────────────┬──────────────────┐
│  Content (truncated)                               │  Metadata        │
├────────────────────────────────────────────────────┼──────────────────┤
│  "The return policy applies to all items purchased │  source: faq.txt │
│  within 30 days..."                                │  page: 3         │
└────────────────────────────────────────────────────┴──────────────────┘
```

```
GET /api/v1/lore/:id/chunks?limit=20&cursor=&search=
Response: { "chunks": [LoreChunk], "next_cursor": "..." }
```

#### Query Tester Tab

```
Query text:  [What is the return policy for electronics?         ]
TopK:        [ 5 ]    Min Score:  [ 0.75 ]    [ Search ]

Results:
┌──────┬───────────────────────────────────────────────────────┐
│ 0.91 │ "Electronics may be returned within 15 days with..."  │
│ 0.88 │ "For premium electronics, extended warranty..."       │
│ 0.84 │ "Defective items are eligible for immediate..."       │
└──────┴───────────────────────────────────────────────────────┘
```

```
POST /api/v1/lore/:id/query
Body: { "query": "...", "top_k": 5, "min_score": 0.75 }
Response:
{
  "results": [
    { "chunk_id": "...", "content": "...", "score": 0.91, "metadata": {} }
  ]
}
```

---

## Screen 5 — Imprint Records (Long-Term Memory)

> Every bond forged between a Spirit and a user leaves an Imprint. Here you can read them.

### Layout

```
┌──────────────────────────────────────────────────┐
│  Imprint Records                                 │
│  User Key: [________________]  [ Search ]        │
│  Spirit:   [ all spirits ▾ ]                     │
├──────────┬─────────────┬──────────────┬──────────┤
│  User    │  Spirit     │  Category    │  Fact    │
├──────────┼─────────────┼──────────────┼──────────┤
│ 55511112 │ support-bot │ preference   │ "Prefer…"│
│ 55511112 │ support-bot │ personal     │ "Name: …"│
│ 55599998 │ support-bot │ business     │ "VIP cu…"│
└──────────┴─────────────┴──────────────┴──────────┘
```

Per-user imprints grouped by category:

```
User: 55511112222 — 8 imprints across 1 spirit

  preference  (3)
  ┌─────────────────────────────────────────────────────┐
  │ ◎ "Prefers to communicate in English"               │
  │    spirit: support-bot • source: extracted • 3d ago │
  │    [ Delete ]                                       │
  └─────────────────────────────────────────────────────┘

  personal  (2)
  ...
```

**Endpoints:**
```
GET /api/v1/imprints?user_key=&spirit_id=&category=&limit=50&offset=0
Response:
{
  "imprints": [
    {
      "id": "...",
      "user_key": "55511112222",
      "spirit_id": "...",
      "fact": "Prefers English",
      "category": "preference",
      "source": "extracted",
      "created_at": "...",
      "updated_at": "..."
    }
  ],
  "total": N,
  "limit": 50,
  "offset": 0
}

DELETE /api/v1/imprints/:id
Response: { "success": true }
```

---

## Screen 6 — Rite Chamber (Approval Workflows)

> A Rite is a sacred pause. The Spirit has stopped, waiting for a human decision before it acts.

### List View

```
┌─────────────────────────────────────────────────────────────┐
│  Rite Chamber                                               │
│  Filter: [ pending ▾ ]  [ spirit ▾ ]  [ date ▾ ]          │
├────────────┬──────────┬────────────┬────────┬──────────────┤
│  Requested │  Session │  Spirit    │ Reason │  Status      │
├────────────┼──────────┼────────────┼────────┼──────────────┤
│  2m ago    │ 55511112 │ support-bot│Refund…│ ⊕ PENDING    │
│  1h ago    │ 55599998 │ support-bot│Access…│ ✓ APPROVED   │
│  3h ago    │ 55522223 │ exec-bot   │Delete…│ ✗ REJECTED   │
└────────────┴──────────┴────────────┴────────┴──────────────┘
```

Status filter chips: PENDING (amber badge count) | APPROVED | REJECTED | EXPIRED

**Endpoints:**
```
GET /api/v1/rites?status=pending&limit=50&cursor=
Response:
{
  "rites": [
    {
      "id": "rite-abc123",
      "memory_key": "55511112222",
      "subject_key": "order-99182",
      "event_key": "support",
      "context": { "order_id": "99182", "refund_amount": 299.00 },
      "reason": "Customer requests refund above $200 threshold",
      "status": "pending",
      "requested_at": "2024-05-18T14:30:00Z",
      "expires_at": "2024-05-18T15:30:00Z"
    }
  ],
  "next_cursor": "..."
}

GET /api/v1/rites/:id
Response: { "rite": Rite }
```

### Rite Detail / Decision

```
┌─────────────────────────────────────────────────────────────┐
│  ⊕ Rite — PENDING                              Expires: 45m │
│                                                             │
│  Session:   55511112222   Spirit: support-bot               │
│  Requested: 2024-05-18 14:30                                │
│                                                             │
│  Reason                                                     │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ Customer requests refund above $200 threshold.      │   │
│  │ Spirit requires human approval before processing.   │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  Context                                                    │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ {                                                   │   │
│  │   "order_id": "99182",                              │   │
│  │   "refund_amount": 299.00,                          │   │
│  │   "customer_tier": "VIP",                           │   │
│  │   "purchase_date": "2024-04-15"                     │   │
│  │ }                                                   │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  Decision note (optional)                                   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ Approved per VIP policy.                            │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  [    ✗ Reject    ]              [    ✓ Approve    ]        │
└─────────────────────────────────────────────────────────────┘
```

**Endpoints:**
```
POST /api/v1/rites/:id/approve
Body: { "note": "Approved per VIP policy." }
Response: { "success": true, "rite": Rite }

POST /api/v1/rites/:id/reject
Body: { "note": "Exceeds manual approval limit." }
Response: { "success": true, "rite": Rite }
```

**Real-time Rite notifications (SSE):**
```
GET /api/v1/sse/rites
Event: rite_created { "rite": Rite }
Event: rite_expired  { "rite_id": "..." }
```
Pending Rite count shown as amber badge on nav item.
Desktop toast notification when new Rite arrives.

---

## Screen 7 — Vigil Watch (Active Takeovers)

> When an operator acquires a Vigil seat, they become part of the conversation. Watch all active seats here.

### Layout

```
┌──────────────────────────────────────────────────┐
│  Vigil Watch                                     │
│  ● 3 active seats                               │
├──────────────┬─────────────┬────────┬────────────┤
│  Session     │  Operator   │  Since │  Expires   │
├──────────────┼─────────────┼────────┼────────────┤
│ 55511112222  │ willian     │  5m    │  in 55m    │
│ 55599998888  │ ana.garcia  │  22m   │  in 38m    │
│ 55522223333  │ willian     │  1m    │  in 59m    │
└──────────────┴─────────────┴────────┴────────────┘
```

Click row → opens Echo Chamber for that session, seat already held.

**Endpoints:**
```
GET /api/v1/vigil
Response: { "vigils": [{ "memory_key": "...", "operator_id": "...", "seat_since": "...", "expires_at": "..." }] }

GET /api/v1/vigil/:memoryKey
Response: { "vigil": { "operator_id": "...", "seat_since": "...", "expires_at": "..." } }
```

---

## Screen 8 — Chronicle (Audit Log & Interaction Detail)

> The living memory of Eywa. Every interaction, every decision, every token — recorded.

### List View

```
┌──────────────────────────────────────────────────────────────┐
│  Chronicle                                                   │
│  Filter: [spirit ▾] [status ▾] [event ▾] [date range ▾]   │
│          [search memory_key or contact_phone]                │
├────────┬──────────────┬────────────┬────────┬───────────────┤
│  When  │  Session     │  Spirit    │  Iters │  Status       │
├────────┼──────────────┼────────────┼────────┼───────────────┤
│  2m    │ 55511112222  │ support-bot│  2     │ ✓ success     │
│  5m    │ 55599998888  │ exec-bot   │  4     │ ✗ error       │
│  18m   │ 55522223333  │ orch-spirit│  6     │ ✓ success     │
└────────┴──────────────┴────────────┴────────┴───────────────┘
```

```
GET /api/v1/chronicle?spirit=&status=&event_key=&from=&to=&limit=50&cursor=
Response:
{
  "chronicles": [Chronicle],
  "next_cursor": "..."
}
```

### Chronicle Detail

Full-screen view of one interaction — the most powerful diagnostic tool in the cockpit.

```
┌──────────────────────────────────────────────────────────────────────┐
│  Chronicle — 2024-05-18T14:30:00Z                                    │
│  support-bot v12 • claude-sonnet-4-5 • 55511112222                  │
├──────────────────────────────────────────────┬───────────────────────┤
│  TIMELINE                                    │  TOKEN USAGE          │
│                                              │  Reasoning: 3,241 tok │
│  ▶ Pulse received (14:30:00.000)             │  Media:        150 tok │
│    type: customer_message                    │  Total:      3,391 tok │
│    user_input: "Track my order #ORD-99182"   │  Est. cost:   $0.008  │
│                                              │                       │
│  ▶ Pipeline (14:30:00.050)                   │  PROCESSING           │
│    Pathfinder: topic_classifier              │  Status:    success   │
│    Archivist:  applied (summarized 8 msgs)   │  Duration:  1,243ms   │
│    Scouts:     lore_scout, imprint_scout     │  Iterations: 2        │
│                                              │                       │
│  ▶ Iteration 1  (14:30:00.100)              │  FINAL RESPONSE       │
│    prompt_tokens: 2,100                      │  "Your order #ORD-... │
│    completion_tokens: 400                    │  is currently in      │
│    duration: 843ms                           │  transit and will     │
│    Oracle response: "I'll look up..."        │  arrive tomorrow."    │
│                                              │                       │
│    ◈ Action: get_order_status               │  SESSION INFO         │
│      args: { "order_id": "99182" }          │  Messages before: 8   │
│      result: "Order in transit..."           │  Coalesced: 1         │
│      duration: 120ms                         │                       │
│      is_critical: false                      │  SCOUT KNOWLEDGE      │
│                                              │  lore_scout: 3 chunks │
│  ▶ Iteration 2  (14:30:01.100)              │  imprint_scout: 2 fct │
│    prompt_tokens: 1,141                      │                       │
│    completion_tokens: 312                    │  ATTACHMENTS          │
│    duration: 400ms                           │  (none)               │
│    Oracle response: "Your order..."          │                       │
│    (final — no more action calls)            │  EVENT PAYLOAD        │
│                                              │  { ... }  [expand]   │
└──────────────────────────────────────────────┴───────────────────────┘
```

**Endpoint:**
```
GET /api/v1/chronicle/:id
Response: { "chronicle": Chronicle }  (full Chronicle entity as documented above)
```

---

## Screen 9 — Ledger (Cost Intelligence)

> Tokens are the resource. The Ledger is the reckoning.

### Layout

```
┌──────────────────────────────────────────────────────────────┐
│  Ledger                              Period: [Last 30d ▾]    │
├────────────┬────────────┬────────────┬────────────┬──────────┤
│ Total cost │ Reasoning  │   Media    │ Interactions│ Avg/int │
│  $42.80    │   $38.20   │   $4.60    │    8,420   │ $0.005  │
└────────────┴────────────┴────────────┴────────────┴──────────┘

┌──────────────────────────────────┬───────────────────────────┐
│  Cost by Spirit (30d)            │  Cost by Model (30d)      │
│  [horizontal bar chart]          │  [donut chart]            │
│  support-bot  ████████░░ $28.10  │                           │
│  exec-bot     ████░░░░░░ $10.40  │  claude-sonnet ●● 68%    │
│  orch-spirit  █░░░░░░░░░  $4.30  │  gpt-4o        ●● 24%    │
│                                  │  gemini-2.0    ●● 8%     │
└──────────────────────────────────┴───────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│  Daily Cost Trend                                            │
│  [area chart — reasoning vs media stacked]                   │
└──────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│  Top 10 Actions by Token Consumption                         │
│  [table: action_name | calls | total_tokens | avg_duration]  │
└──────────────────────────────────────────────────────────────┘
```

**Endpoints:**
```
GET /api/v1/analytics/tokens?period=30d&group_by=day
Response:
{
  "period": "30d",
  "total": {
    "prompt_tokens": 28000000,
    "completion_tokens": 9000000,
    "total_tokens": 37000000,
    "estimated_cost_usd": 42.80
  },
  "by_spirit": [
    { "spirit_name": "support-bot", "total_tokens": 25000000, "estimated_cost_usd": 28.10 }
  ],
  "by_model": [
    { "model": "claude-sonnet-4-5", "total_tokens": 25160000, "share": 0.68 }
  ],
  "daily": [
    { "date": "2024-05-18", "reasoning_tokens": 1200000, "media_tokens": 150000, "estimated_cost_usd": 1.42 }
  ]
}

GET /api/v1/analytics/actions?period=30d&limit=10
Response:
{
  "actions": [
    {
      "action_name": "get_order_status",
      "call_count": 4210,
      "error_count": 12,
      "avg_duration_ms": 145,
      "total_tokens_attributed": 180000
    }
  ]
}

GET /api/v1/analytics/spirits?period=30d
Response:
{
  "spirits": [
    {
      "spirit_name": "support-bot",
      "interactions": 6420,
      "avg_iterations": 2.1,
      "error_rate": 0.004,
      "total_tokens": 25000000,
      "estimated_cost_usd": 28.10
    }
  ]
}
```

---

## Screen 10 — Conduit Gateway (Tools Management)

> Conduits are the bridges between Spirits and the world outside. Build, test, and configure them here.

### Tab 1 — HTTP Tools

```
┌──────────────────────────────────────────────────┐
│  HTTP Tools                    [ + New Tool ]    │
├──────────────────┬──────────┬────────┬───────────┤
│  Name            │ Method   │  URL   │  Spirits  │
├──────────────────┼──────────┼────────┼───────────┤
│ get_order_status │ GET      │ /order │ 2         │
│ send_notification│ POST     │ /notify│ 1         │
└──────────────────┴──────────┴────────┴───────────┘
```

**Endpoints:**
```
GET /api/v1/http-tools
Response: { "tools": [HTTPTool] }

POST /api/v1/http-tools
Body:
{
  "name": "get_order_status",
  "description": "Retrieves order status by order ID",
  "method": "GET",
  "url": "https://api.company.com/orders/{{order_id}}",
  "headers": { "Authorization": "Bearer {{api_key}}" },
  "body_template": "",
  "response_mapping": { "status": "$.data.status" },
  "error_rules": [
    { "http_status": 404, "error_type": "business", "message": "Order not found" },
    { "http_status": 500, "error_type": "infra", "message": "Service unavailable" }
  ]
}
Response: { "tool": HTTPTool }

PUT /api/v1/http-tools/:id
DELETE /api/v1/http-tools/:id
```

**HTTP Tool Create/Edit form:**
- Name + description (visible to Oracle)
- Method dropdown
- URL with `{{variable}}` highlighting
- Headers key-value editor (add/remove rows)
- Body template editor (Monaco, JSON mode with `{{variable}}` highlighting)
- Response mapping (JSONPath expressions)
- Error classification rules table
- **Test Runner** — embedded, see below

**Test Runner:**
```
┌───────────────────────────────────────────────────────────┐
│  Test Runner                                              │
│  Arguments                                                │
│  ┌─────────────────────────────────────────────────────┐ │
│  │ {                                                   │ │
│  │   "order_id": "99182",                             │ │
│  │   "api_key": "sk-test-..."                         │ │
│  │ }                                                   │ │
│  └─────────────────────────────────────────────────────┘ │
│                                    [ Run Test ]          │
│  ─────────────────────────────────────────────────       │
│  Status: 200 OK • 145ms                                   │
│  ┌─────────────────────────────────────────────────────┐ │
│  │ {                                                   │ │
│  │   "data": {                                         │ │
│  │     "status": "in_transit",                        │ │
│  │     "eta": "2024-05-19T09:00:00Z"                 │ │
│  │   }                                                │ │
│  │ }                                                   │ │
│  └─────────────────────────────────────────────────────┘ │
│  Mapped result: { "status": "in_transit" }               │
└───────────────────────────────────────────────────────────┘
```

```
POST /api/v1/http-tools/:id/test
Body: { "arguments": { "order_id": "99182", "api_key": "sk-..." } }
Response:
{
  "http_status": 200,
  "duration_ms": 145,
  "raw_response": { ... },
  "mapped_result": { "status": "in_transit" },
  "error": null
}
```

### Tab 2 — Registered Actions (Read-only)

All Go actions registered in the engine at startup. Read-only — they live in code.

```
┌──────────────────────────────────────────────────────────────┐
│  Registered Actions                   [ search... ]         │
├────────────────────────┬──────────────────────────┬──────────┤
│  Name                  │  Description             │  Type    │
├────────────────────────┼──────────────────────────┼──────────┤
│ get_order_status       │ Retrieves order by ID    │ HTTP     │
│ request_rite           │ Built-in: request human  │ builtin  │
│ search_knowledge_base  │ Built-in: Lore scout     │ builtin  │
│ send_whatsapp_message  │ Send via WhatsApp API    │ custom   │
└────────────────────────┴──────────────────────────┴──────────┘
```

**Endpoint:**
```
GET /api/v1/discovery
Response:
{
  "actions":     [{ "name": "...", "description": "...", "parameters": {...}, "is_critical": bool, "category": "..." }],
  "scouts":      ["scout_name", ...],
  "classifiers": ["classifier_name", ...],
  "channels":    [{ "name": "...", "auto_respond": bool }],
  "routers":     ["router_name", ...],
  "receptors":   ["receptor_name", ...]
}
```

---

## Screen 11 — Pulse Flows (Event Configuration)

> Every message that arrives is a Pulse. Pulse Flows define which Spirit receives it and how.

### List View

```
┌────────────────────────────────────────────────────────────┐
│  Pulse Flows                               [ + New Flow ] │
├────────────────┬─────────────────┬────────────┬───────────┤
│  Event Type    │  Default Spirit │  Enrichers │  Status   │
├────────────────┼─────────────────┼────────────┼───────────┤
│ customer_msg   │ support-bot     │  3         │ ● active  │
│ order_event    │ exec-bot        │  1         │ ● active  │
│ notification   │ notifier-spirit │  0         │ ● active  │
└────────────────┴─────────────────┴────────────┴───────────┘
```

### Flow Edit

```
Event Key *          [customer_message]
Inbound Converter    [ json_converter ▾ ]

Enrichers (drag to reorder)
  ═══ lore_scout_enricher         [×]
  ═══ imprint_scout_enricher      [×]
  ═══ contact_enricher            [×]
  [ + Add Enricher ]

Agent Routing
  Default Spirit     [ support-bot ▾ ]
  Classifier         [ topic_classifier ▾ ]
  Agent routes:
    "billing"  →  billing-bot    [×]
    "returns"  →  returns-bot    [×]
    [ + Add Route ]

Response Channel     [ whatsapp_channel ▾ ]

Timeouts
  Inbox min window   [ 500 ] ms
  Lock TTL           [ 60 ] s
```

**Endpoints:**
```
GET /api/v1/event-configurations
Response: { "configurations": [EventConfiguration] }

GET /api/v1/event-configurations/:eventType
Response: { "configuration": EventConfiguration }

PUT /api/v1/event-configurations/:eventType
Body: EventConfiguration
Response: { "configuration": EventConfiguration }

DELETE /api/v1/event-configurations/:eventType
Response: { "success": true }
```

Changes take effect immediately via ConfigCache pub/sub (no redeploy needed).

---

## Screen 12 — Weave Config (Engine Configuration)

> The Weave itself has parameters. Change them here, and they ripple through every Spirit instantly.

```
┌──────────────────────────────────────────────────────────────┐
│  Weave Config                                                │
│                                 [ Save + Reload ]           │
├──────────────────────────────────────────────────────────────┤
│  Processing                                                  │
│  Max Iterations           [ 5 ]                             │
│  Inbox Min Window         [ 500 ]  ms                       │
│  Lock TTL                 [ 60 ]   s                        │
│  Interaction Timeout      [ 30 ]   s                        │
│                                                             │
│  Memory                                                      │
│  Summarization Threshold  [ 10 ]   messages                 │
│  Session TTL              [ 3600 ] s                        │
│                                                             │
│  Observability                                               │
│  [ ✓ ] Chronicle enabled                                    │
│  [ ✓ ] Token tracking enabled                               │
│  [ ✓ ] Action logging enabled                               │
└──────────────────────────────────────────────────────────────┘

Config History (last 10 changes)
┌────────────────┬────────────────────┬────────────────────────┐
│  When          │  Changed by        │  Summary               │
├────────────────┼────────────────────┼────────────────────────┤
│  2h ago        │  willian           │  max_iterations 3 → 5  │
│  3d ago        │  system            │  Initial config        │
└────────────────┴────────────────────┴────────────────────────┘
```

**Endpoints:**
```
GET /api/v1/admin/engine-config
Response: { "config": WeaveConfig }

PUT /api/v1/admin/engine-config
Body: WeaveConfig
Response: { "config": WeaveConfig }

POST /api/v1/admin/config/reload
Response: { "success": true, "reloaded_at": "..." }
```

Reload propagates via Redis pub/sub to all running engine instances.
UI shows success toast: "Config reloaded across all engine instances."

---

## Screen 13 — Operators

> The people who interact with the cockpit are Operators. Manage who has access and what they can do.

```
┌──────────────────────────────────────────────────────────────┐
│  Operators                             [ + New Operator ]    │
├────────────────┬─────────────┬──────────────┬───────────────┤
│  Username      │  Role       │  Status      │  Last Login   │
├────────────────┼─────────────┼──────────────┼───────────────┤
│ willian        │ admin       │ ● active     │  2m ago       │
│ ana.garcia     │ operator    │ ● active     │  1h ago       │
│ carlos.tech    │ operator    │ ○ inactive   │  3d ago       │
└────────────────┴─────────────┴──────────────┴───────────────┘
```

```
POST /api/v1/auth/token
Body: { "username": "...", "password": "..." }
Response: { "token": "eyJ...", "operator_id": "...", "role": "admin|operator" }

GET /api/v1/operators
Response: { "operators": [Operator] }

POST /api/v1/operators
Body: { "username": "...", "password": "...", "role": "admin|operator" }
Response: { "operator": Operator }

GET /api/v1/operators/:id
PUT /api/v1/operators/:id
Body: { "username": "...", "role": "...", "is_active": true }

DELETE /api/v1/operators/:id   (soft deactivate)
```

---

## Real-time Architecture

### SSE Endpoints

The cockpit uses Server-Sent Events for real-time updates.
Implemented in `fiber/sse_handler.go` — Redis PubSub backed, 30s heartbeat ping, nginx buffering disabled.

```
GET /api/v1/sse/rites
  Auth: Bearer token required
  Events:
    rite_created  { "rite": Rite }
    rite_decided  { "rite_id": "...", "status": "approved|rejected" }
    rite_expired  { "rite_id": "..." }

GET /api/v1/sse/echoes/:memoryKey
  Auth: Bearer token required
  Events:
    message_added   { "message": { "role": "...", "content": "...", "created_at": "..." } }
    vigil_acquired  { "operator_id": "...", "seat_since": "..." }
    vigil_released  {}
    rite_created    { "rite": Rite }

GET /api/v1/sse/vigil
  Auth: Bearer token required
  Events:
    vigil_acquired  { "memory_key": "...", "operator_id": "..." }
    vigil_released  { "memory_key": "..." }
```

**Implementation notes:**
- Handler writes `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `X-Accel-Buffering: no`
- Heartbeat every 30s (`: ping\n\n` comment line — standard SSE keep-alive)
- Subscribe goroutine + buffered channel (32); drops messages if client too slow
- Connection cleanup via `context.WithCancel` on `StreamWriter` exit

**TanStack Query integration (cockpit side):**
```typescript
// SSE as TanStack Query subscription
const useRiteSSE = () => {
  return useQuery({
    queryKey: ['rites', 'stream'],
    queryFn: () => new Promise(() => {
      const es = new EventSource('/api/v1/sse/rites', { withCredentials: false })
      es.addEventListener('rite_created', (e) => {
        queryClient.invalidateQueries({ queryKey: ['rites'] })
        toast.amber(`New Rite — ${JSON.parse(e.data).rite.reason}`)
      })
    }),
    staleTime: Infinity,
  })
}
```

**Polling fallback:** when SSE not available, TanStack Query refetchInterval:
- Rites: 15s
- Echo session detail: 5s while active
- Dashboard KPIs: 60s
- Vigil status: 10s

---

## New Engine Endpoints — Implementation Status

| Status | Method | Path | Description |
|---|---|---|---|
| ✅ | `GET` | `/api/v1/discovery` | Engine registry dump: actions, scouts, classifiers, channels, routers, receptors |
| ✅ | `GET` | `/api/v1/spirits/:name/versions` | Spirit version history |
| ✅ | `GET` | `/api/v1/imprints` | List imprints (filtered by user/spirit/category) |
| ✅ | `DELETE` | `/api/v1/imprints/:id` | Delete a single imprint |
| ✅ | `GET` | `/api/v1/vigil` | List all active Vigil seats |
| ✅ | `GET` | `/api/v1/sse/rites` | SSE: Rite lifecycle events |
| ✅ | `GET` | `/api/v1/sse/echoes/:memoryKey` | SSE: Echo session events |
| ✅ | `GET` | `/api/v1/sse/vigil` | SSE: Vigil seat events |
| ⬜ | `GET` | `/api/v1/lore` | List all Lore knowledge bases |
| ⬜ | `POST` | `/api/v1/lore` | Create new Lore |
| ⬜ | `GET` | `/api/v1/lore/:id` | Lore detail + chunk count |
| ⬜ | `DELETE` | `/api/v1/lore/:id` | Delete Lore and all chunks |
| ⬜ | `POST` | `/api/v1/lore/:id/ingest` | Ingest document (multipart) |
| ⬜ | `GET` | `/api/v1/lore/:id/ingest/:jobID/status` | Ingestion job status |
| ⬜ | `GET` | `/api/v1/lore/:id/chunks` | List chunks with pagination |
| ⬜ | `POST` | `/api/v1/lore/:id/query` | Vector search test |

---

## Complete API Reference for eywa-cockpit

### Base URL
`{ENGINE_URL}/api/v1`

### Auth Header
`Authorization: Bearer {token}`

### All Endpoints

```
# Health (no auth)
GET  /health
GET  /ready

# Auth
POST /api/v1/auth/token

# Discovery — engine registry dump
GET    /api/v1/discovery

# Spirits
GET    /api/v1/spirits
POST   /api/v1/spirits
GET    /api/v1/spirits/:name
PUT    /api/v1/spirits/:name
DELETE /api/v1/spirits/:name
POST   /api/v1/spirits/:name/activate
POST   /api/v1/spirits/:name/deactivate
GET    /api/v1/spirits/:name/versions

# Operators (admin only)
GET    /api/v1/operators
POST   /api/v1/operators
GET    /api/v1/operators/:id
PUT    /api/v1/operators/:id
DELETE /api/v1/operators/:id

# Chronicle + Analytics
GET    /api/v1/chronicle
GET    /api/v1/chronicle/:id
GET    /api/v1/analytics/tokens
GET    /api/v1/analytics/actions
GET    /api/v1/analytics/spirits

# Echoes (Conversations)
GET    /api/v1/echoes/sessions
GET    /api/v1/echoes/sessions/:memoryKey
POST   /api/v1/echoes/sessions/:memoryKey/messages
GET    /api/v1/echoes

# Lore (Knowledge Bases) — ⬜ pending
GET    /api/v1/lore
POST   /api/v1/lore
GET    /api/v1/lore/:id
DELETE /api/v1/lore/:id
POST   /api/v1/lore/:id/ingest
GET    /api/v1/lore/:id/ingest/:jobID/status
GET    /api/v1/lore/:id/chunks
POST   /api/v1/lore/:id/query

# Imprints (Long-term Memory)
GET    /api/v1/imprints
DELETE /api/v1/imprints/:id

# Vigil (Operator Takeover)
GET    /api/v1/vigil
POST   /api/v1/vigil/:memoryKey
DELETE /api/v1/vigil/:memoryKey
GET    /api/v1/vigil/:memoryKey
POST   /api/v1/vigil/:memoryKey/echoes

# Rites (Approvals)
GET    /api/v1/rites
GET    /api/v1/rites/:id
POST   /api/v1/rites/:id/approve
POST   /api/v1/rites/:id/reject

# Event Configurations (Pulse Flows)
GET    /api/v1/event-configurations
GET    /api/v1/event-configurations/:eventType
PUT    /api/v1/event-configurations/:eventType
DELETE /api/v1/event-configurations/:eventType

# HTTP Tools (Conduit)
GET    /api/v1/http-tools
POST   /api/v1/http-tools
GET    /api/v1/http-tools/:id
PUT    /api/v1/http-tools/:id
DELETE /api/v1/http-tools/:id
POST   /api/v1/http-tools/:id/test

# Admin (admin only)
GET    /api/v1/admin/engine-config
PUT    /api/v1/admin/engine-config
POST   /api/v1/admin/config/reload

# SSE (Real-time — Redis PubSub)
GET    /api/v1/sse/rites
GET    /api/v1/sse/echoes/:memoryKey
GET    /api/v1/sse/vigil
```

---

## Role-Based Access

| Screen | admin | operator |
|---|---|---|
| Hometree (Dashboard) | ✓ | ✓ |
| Spirit Grove — read | ✓ | ✓ |
| Spirit Grove — write | ✓ | — |
| Echo Chamber | ✓ | ✓ |
| Lore Sanctum — read | ✓ | ✓ |
| Lore Sanctum — write | ✓ | — |
| Imprint Records | ✓ | ✓ |
| Rite Chamber (decide) | ✓ | ✓ |
| Vigil Watch | ✓ | ✓ |
| Chronicle | ✓ | ✓ |
| Ledger | ✓ | ✓ |
| Conduit Gateway — read | ✓ | ✓ |
| Conduit Gateway — write | ✓ | — |
| Pulse Flows | ✓ | — |
| Weave Config | ✓ | — |
| Operators | ✓ | — |

---

## Responsive Layout

- **Desktop (≥1280px):** Full sidebar nav + split views
- **Tablet (768–1279px):** Collapsible sidebar (icon-only mode)
- **Mobile (< 768px):** Bottom tab bar with 5 most-used screens (Dashboard, Echoes, Rites, Chronicle, Settings)

---

## MVP Scope

**Phase 1 — Core (ship first):**
- Connection setup + auth
- Hometree dashboard
- Spirit Grove full CRUD
- Echo Chamber session list + detail + Vigil takeover
- Rite Chamber list + approve/reject
- SSE for Rites and Echoes

**Phase 2 — Observability:**
- Chronicle full drill-down
- Ledger cost intelligence
- Analytics charts

**Phase 3 — Knowledge & Memory:**
- Lore Sanctum full CRUD + query tester
- Imprint Records viewer

**Phase 4 — Advanced Config:**
- Conduit Gateway (HTTP Tools + test runner)
- Pulse Flows editor
- Weave Config

**Phase 5 — Polish:**
- Multi-engine profile switcher
- Keyboard shortcuts
- Command palette (⌘K)
- Dark/light theme toggle (dark default)
- Export chronicle as CSV

---

## Dependencies

All P0 specs are complete — the cockpit can be built now:

- ✅ SPEC_00 (Auth) — JWT, OperatorAuth, APIKey modes
- ✅ SPEC_02 (HTTP Tools) — full CRUD + test endpoint
- ✅ SPEC_03 (Config as a Service) — event configs + engine config
- ✅ SPEC_04 (Human Takeover) — Vigil + Rite
- ✅ SPEC_06 (Conversations API) — sessions + echoes
- ✅ SPEC_07 (Observability API) — chronicle + analytics
- ✅ SPEC_08 (Lore) — RAG knowledge base
- ✅ SPEC_10 (Per-Agent Tool Config) — AllowedAction overrides
- ✅ SPEC_11 (Imprint) — long-term memory
- ✅ Discovery, Vigil list, Imprints CRUD, Spirit versions, SSE, EventBus — implemented in eywa engine
- ⬜ Lore management endpoints (CRUD + ingest + query) — pending

---

## Non-Goals

- Mobile native app (responsive web covers it)
- Embedded chat widget
- Multi-tenant billing backend (phase 2 SaaS)
- Video analytics
- Model playground (separate from session testing)
