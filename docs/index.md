# 🌿 Eywa

> Production-grade, event-driven AI agent orchestration for **Go**.

Eywa is the **runtime** for putting conversational agents into production — distributed locking,
idempotency, message coalescing, human-in-the-loop, cost governance, guardrails, and agent versioning
included. Not just an LLM wrapper: the hard parts of running an agent under real traffic, done once.

## Start here

- **[eywa-starter](https://github.com/wmulabs/eywa-starter)** — clone the template for a ready-to-run hexagonal project; a working agent in minutes.
- **[Core concepts](concepts.md)** — Weave, Spirit, Pulse, Oracle, and the rest of the glossary.
- **[Architecture](architecture.md)** — hexagonal ports & adapters, the pipeline, the reasoning loop.
- **[Builder](builder.md)** — wire a Weave with the fluent `WeaveBuilder` API.

## Build with it

- **[Multi-agent](multi-agent.md)** — Pathfinder routing, summon delegation, and peer handoff.
- **[Reasoning loop](reasoning.md)** — the meta-cognitive loop: stall detection, reflection, grounding.
- **[Guardrails](guardrails.md)** — PII redaction, output denylist, jailbreak detection.
- **[Durable execution](durable-execution.md)** — checkpoint and resume agent turns.
- **[Authentication](authentication.md)** & **[REST API](rest-api.md)** — the management surface.

## Run it

- **[Scaling & failure scenarios](scaling.md)** · **[Operations](operations.md)**
- **[Sub-modules](sub-modules.md)** — Mongo, Redis, Fiber, providers, vector stores, MCP.

## Choosing a framework

- **[Eywa vs Eino, Genkit Go, LangChainGo](comparison.md)** — an honest comparison.

---

Source on [GitHub](https://github.com/wmulabs/eywa) · Apache-2.0.
