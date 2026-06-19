# 🧠 The Reasoning Loop — Meta-Cognition

Most agent frameworks run a fixed ReAct loop: call the model, run the tools it asks for, repeat until
it stops or a cap is hit. Eywa's loop does that too — but layers **meta-cognition** on top: it can be
made aware of its own progress, context size, confidence, and grounding, and can re-plan, self-critique,
escalate, or stop deliberately.

Every capability below is **opt-in and off by default**. A Spirit that enables none behaves exactly
like a classic ReAct loop. You turn each one on globally via `WeaveBuilder`, and any Spirit can override
any subset via [per-Spirit overrides](#per-spirit-overrides).

| Capability | Builder | What it buys you |
|---|---|---|
| Tool-result shaping | `WithToolResultLimits` | A huge tool result can't blow the context window |
| Progress / stall detection | `WithProgressPolicy` | Loop notices it's spinning and forces a final answer |
| Arg-aware ban | *(always on)* | A call that failed critically isn't retried with the same args |
| Context compression | `WithCompressionPolicy` | Long turns stay within budget via an evidence ledger |
| Reflection | `WithReflectionPolicy` | Self-critique pass before delivery, bounded retries |
| Grounding | `WithGroundingPolicy` | RAG answers must cite retrieved sources |
| Plan / scratchpad | `WithPlanPolicy` | Persistent plan across iterations, no premature stop |
| Model tiering | `Spirit.DraftModel` | Cheap model for tool steps, strong model for the answer |
| Confidence → handoff | `WithHandoffPolicy` | Low-confidence turns escalate to a human instead of guessing |

---

## Tool-result shaping

A single tool that returns a 50k-character blob can evict everything else from the context window.
`ToolResultLimits` bounds how much of each Action result re-enters the reasoning context. The **full**
result is always kept in the audit log; only the message handed back to the Oracle is shaped.

```go
eywa.NewWeaveBuilder(ctx).
    WithToolResultLimits(eywa.ToolResultLimits{
        MaxChars: 8000,
        Strategy: eywa.ToolShapeTruncate, // or ToolShapeSummarize (an LLM condenses it)
        KeepHead: 2000,
        KeepTail: 1000,
    })
```

`MaxChars: 0` (the default) disables shaping. An Action can override the global limit for its own
results by implementing the `ToolResultShaper` interface.

## Progress / stall detection

Without it, a loop that keeps requesting the same tool just burns iterations until the cap. With
`ProgressPolicy` the loop detects oscillation (the same tool+args repeated within a window) and forces a
final synthesis from the context already gathered — the user gets a real answer instead of a canned
"max iterations reached" message.

```go
WithProgressPolicy(eywa.ProgressPolicy{Enabled: true, StallWindow: 3})
```

`StallWindow` is how many recent iterations are compared for repetition. When enabled, hitting the
iteration cap also yields a forced synthesis rather than the fallback message.

## Arg-aware ban (always on)

When a tool call fails with a **critical business error**, its exact `(name, arguments)` signature is
banned for the rest of the turn — re-running it would just fail again. The tool stays available, so the
model can retry with **corrected** arguments. This is built into the loop; no policy needed.

## Context compression

For long, tool-heavy turns the working context grows unbounded. `CompressionPolicy` summarizes the
oldest completed iterations into a compact "evidence ledger" once the context exceeds a threshold, while
keeping the most recent iterations verbatim — the in-loop counterpart to the [Archivist](concepts.md).

```go
WithCompressionPolicy(eywa.CompressionPolicy{
    Enabled:         true,
    MaxContextChars: 24000, // compress once the working context grows past this
    KeepRecent:      3,     // most recent iterations kept verbatim
})
```

## Reflection

Before delivering a draft answer, the model reviews its own output against criteria; on a "revise"
verdict it gets one more iteration to fix it, bounded by `MaxRounds`. Reflection always fails open — a
failed critique never blocks delivery.

```go
WithReflectionPolicy(eywa.ReflectionPolicy{
    Enabled:   true,
    MaxRounds: 1,
    Criteria:  []string{"fully answers the question", "no unsupported claims"},
    Model:     "gpt-4o", // optional; defaults to the Spirit's model
})
```

## Grounding (citation enforcement for RAG)

For Spirits that retrieve [Lore](concepts.md), `GroundingPolicy` requires the answer to cite the
retrieved chunks (`[chunk:<id>]`). On a violation it can revise once, annotate the answer, or block it.

```go
WithGroundingPolicy(eywa.GroundingPolicy{
    Enabled:      true,
    MinCitations: 1,
    OnViolation:  eywa.GroundingReviseOnce, // or GroundingAnnotate, GroundingBlock
})
```

Validated citations are returned on the result (`ReasoningResult.Citations`).

## Plan / scratchpad

`PlanPolicy` gives the model a turn-scoped plan it maintains via an injected `update_plan` action
(TodoWrite-style). The current plan is re-injected each iteration so the model never re-derives it, and
an incomplete plan blocks a premature stop.

```go
WithPlanPolicy(eywa.PlanPolicy{
    Enabled:  true,
    MaxItems: 8,
    Required: true, // instruct the model to maintain a plan
})
```

The final plan is returned on the result (`ReasoningResult.Plan`).

## Model tiering

Set a Spirit's `DraftModel` to run tool-selection iterations on a cheap model and re-synthesize the
final, user-facing answer on the Spirit's strong model. The draft and strong tiers may even live on
different providers.

```go
spirit.ModelConfig = eywa.SpiritModel{Provider: "openai", Model: "gpt-4o"}
spirit.DraftModel  = "gpt-4o-mini" // tool steps run here; the answer is re-synthesized on gpt-4o
```

Per-model token usage is broken out in `ReasoningResult.TokensByModel`.

## Confidence → handoff

Rather than ship a weak answer, `HandoffPolicy` assesses a coarse, rule-based confidence for the turn
and, below the threshold, escalates to a human — either raising a [Vigil](concepts.md) takeover or
annotating the answer.

```go
WithHandoffPolicy(eywa.HandoffPolicy{
    Enabled:        true,
    MinConfidence:  eywa.ConfidenceMedium,
    Mode:           eywa.HandoffRaiseVigil, // needs WithVigilRepository; or HandoffAnnotateOnly
    HoldingMessage: "Let me get a teammate to confirm this.",
})
```

`raise_vigil` mode requires a Vigil repository (`WithVigilRepository`). The result flags
`ReasoningResult.HandoffRaised` and carries the `Confidence` band.

---

## Per-Spirit overrides

Global policies are the default for every Spirit. Any Spirit can override any subset via
`ReasoningOverrides` — a `nil` field falls back to the global default, so you only specify what differs.

```go
spirit.ReasoningOverrides = &eywa.ReasoningOverrides{
    // This Spirit reflects twice even though the global default is off…
    Reflection: &eywa.ReflectionPolicy{Enabled: true, MaxRounds: 2},
    // …and disables grounding even if it's globally on.
    Grounding:  &eywa.GroundingPolicy{Enabled: false},
}
```

This makes meta-cognition a per-agent dial: a fast FAQ Spirit runs lean, while a high-stakes Spirit
enables plan + reflection + grounding + handoff.

---

## Putting it together

```go
weave, _ := eywa.NewWeaveBuilder(ctx).
    WithToolResultLimits(eywa.ToolResultLimits{MaxChars: 8000, Strategy: eywa.ToolShapeTruncate}).
    WithProgressPolicy(eywa.ProgressPolicy{Enabled: true, StallWindow: 3}).
    WithCompressionPolicy(eywa.CompressionPolicy{Enabled: true, MaxContextChars: 24000, KeepRecent: 3}).
    WithReflectionPolicy(eywa.ReflectionPolicy{Enabled: true, MaxRounds: 1}).
    WithGroundingPolicy(eywa.GroundingPolicy{Enabled: true, MinCitations: 1, OnViolation: eywa.GroundingReviseOnce}).
    WithPlanPolicy(eywa.PlanPolicy{Enabled: true, MaxItems: 8}).
    WithHandoffPolicy(eywa.HandoffPolicy{Enabled: true, MinConfidence: eywa.ConfidenceMedium, Mode: eywa.HandoffAnnotateOnly}).
    // ... repositories, oracle, etc.
    Build()
```

All seven are independent. Start with none (classic ReAct), then enable the ones your use case needs.
