# Eywa Starter

A minimal, runnable [Eywa](https://github.com/wmulabs/eywa) agent — a terminal chat backed by MongoDB,
Redis, and an LLM, with one example tool. **Fork it and make it yours.**

## Quickstart (3 steps)

```bash
# 1. Start MongoDB + Redis
make up

# 2. Configure your key
cp .env.example .env && $EDITOR .env   # set OPENAI_API_KEY

# 3. Chat with your agent
make run
```

```
Eywa starter — type a message (exit to quit)

you> what time is it?
bot> It is 2026-06-19T18:40:00Z.
```

That's a full agent: distributed locking, conversation memory, an audit log, a tool-calling reasoning
loop — already wired.

## What's inside

| File | Purpose |
|------|---------|
| `main.go` | The whole agent: infra wiring, one Spirit, one Action, a chat loop |
| `docker-compose.yml` | MongoDB + Redis for local dev |
| `.env.example` | Configuration template |
| `Dockerfile` | Container build for deployment |
| `Makefile` | `up` / `down` / `run` / `build` / `tidy` |

## Make it yours

- **Change the agent** — edit the `spirit` in `main.go`: its `SystemPrompt`, `Model`, `Temperature`.
- **Add a tool** — implement the `eywa.Action` interface (copy `timeAction`), register it, and list it in
  the Spirit's `AllowedActions`.
- **Add a channel** — swap the CLI loop for a WhatsApp/HTTP receptor (see the eywa channels sub-modules).
- **Turn on more of the loop** — stall detection, reflection, grounding, plan, durable execution: see
  [the reasoning docs](https://github.com/wmulabs/eywa/blob/main/docs/reasoning.md).

## Deploy

```bash
docker build -t eywa-starter .
docker run --env-file .env eywa-starter
```

Point `MONGO_URL` / `REDIS_URL` at managed instances in production.

## Learn more

- [Eywa repository](https://github.com/wmulabs/eywa) · [examples](https://github.com/wmulabs/eywa/tree/main/_examples) · [docs](https://github.com/wmulabs/eywa/tree/main/docs)
