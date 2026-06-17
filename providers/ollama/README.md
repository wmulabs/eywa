# eywa — Ollama provider

Native [Ollama](https://ollama.com) Oracle for Eywa. Talks to Ollama's `/api/chat` endpoint, needs
no API key, and defaults to `http://localhost:11434` — so agents run against local models out of the
box.

```go
import "github.com/wmulabs/eywa/providers/ollama"

oracle := ollama.NewOracle(ollama.Config{
    Host:      "http://localhost:11434", // optional; this is the default
    KeepAlive: "5m",                     // optional: keep the model loaded between requests
})
// Register it on the Weave; set Spirit.ModelConfig.Provider = "ollama" and Model = "llama3.1".
```

## Capabilities

- Chat completion with system prompt, sampling options (`temperature`, `num_predict`, `top_p`,
  `top_k`, `stop`) and `keep_alive`.
- Tool calling for models that support it (mapped to Ollama's `tools` / `tool_calls`).
- Token usage from `prompt_eval_count` / `eval_count`.

Media (images/audio/documents) is handled by Eywa's transcription fallback in this version; native
vision for vision-capable models is a follow-up. Streaming and structured output land in subsequent
releases.

## Notes

- `Host` is operator-provided configuration, not user input. Response bodies are read with a bounded
  `io.LimitReader`.
- `IsAvailable()` reflects whether a host is configured; a connection error surfaces from
  `GenerateResponse` when the server is not running.
