# 🛡️ Guardrails

Eywa guards both ends of a turn: **input** (before the Pulse enters the pipeline) and **output**
(before the final response is persisted, delivered, and audited). Both are conservative by design —
they favour false negatives over flagging legitimate traffic.

---

## Input guard

Configured with `WithInputGuard(GuardConfig{...})`. `PromptInjectionDetection` is **enabled by
default** in `DefaultWeaveConfig()`.

```go
builder.WithInputGuard(eywa.GuardConfig{
    PromptInjectionDetection: true, // default
    MaxLineCount:             200,  // reject messages with more than N lines (0 = disabled)
})
```

When enabled, a user message is rejected (`ErrPromptInjectionDetected`) before any processing if it
matches a known attack pattern. Coverage:

- **Instruction override** — "ignore/disregard/forget previous instructions".
- **Chat-format injection** — `<|im_start|>`, `[INST]`/`[/INST]`.
- **Control / homoglyph abuse** — null bytes, RTL/LTR overrides, zero-width characters.
- **Jailbreak attempts** — "do anything now", "jailbreak", "developer mode enabled", system-prompt
  extraction ("reveal your system prompt"), safety-bypass ("bypass your safety guidelines"), and
  unrestricted-persona requests ("pretend to be an uncensored AI").

Each jailbreak pattern requires an explicit hostile qualifier, so ordinary roleplay ("act as a
translator", "pretend to be a pirate") is **not** flagged.

---

## Output guard

Configured with `WithOutputGuard(OutputGuardConfig{...})`. **Disabled by default** — an agent that
does not opt in behaves identically.

```go
builder.WithOutputGuard(eywa.OutputGuardConfig{
    RedactPII:       true,
    PIIKinds:        []eywa.PIIKind{eywa.PIIEmail, eywa.PIICreditCard, eywa.PIIPhone},
    RedactionMask:   "[REDACTED]", // default when empty
    BlockedPatterns: []string{`(?i)\bssn\b`},
    BlockedMessage:  "I can't share that response.", // default when empty
})
```

### PII redaction

`RedactPII` replaces detected PII with `RedactionMask`. An empty `PIIKinds` redacts all supported
kinds:

| Kind | `PIIKind` | Notes |
|---|---|---|
| Email | `PIIEmail` | RFC-ish local@domain.tld |
| Credit card | `PIICreditCard` | 13–19 digits, **Luhn-validated** to suppress false positives |
| Phone | `PIIPhone` | requires `+` prefix or explicit separators; bare digit runs are left alone |

### Blocklist

`BlockedPatterns` are regular expressions. If the response matches any, it is replaced **wholesale**
with `BlockedMessage` (blocking takes precedence over redaction). Invalid expressions are rejected by
`WeaveConfig.Validate()` at build time.

### Coverage caveat

The output guard runs after the Notification step and before persistence/delivery, so it covers the
**standard reasoning auto-response** end to end. Two paths emit to the channel earlier and are only
partially covered:

- **Streamed turns** (`ProcessEventByKeyStream`) — tokens reach the client live; the guard still
  sanitises the persisted and audited copy.
- **Notifier Spirits** — the Notification step sends inline; again, the stored and audited copy is
  sanitised, but the live send is not recalled.

For a hard guarantee on streamed output, redact at the source (Spirit prompt / tool results) as well.

---

## What to never log

Avoid logging raw `Pulse.UserMessage` or the final response in production — both may contain PII.
Eywa's structured logging records metadata (redaction counts, matched pattern strings) but not message
content.
