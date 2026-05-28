# Example 05 — Multi-Provider

Shows how a single Weave can run spirits backed by different LLM providers simultaneously.

## What you'll learn

- `AddOracle(...)` registers providers by name (`openai`, `groq`, `ollama`, …)
- `Spirit.ModelConfig.Provider` selects the Oracle at runtime — no engine changes
- OpenAI-compatible providers (Groq, Ollama, Mistral, Together) require no extra dependencies

## Prerequisites

- MongoDB and Redis running locally (or set env vars)
- `OPENAI_API_KEY` (required)
- One of: `GROQ_API_KEY` or `OLLAMA_URL` (optional — shows second provider)

## Environment variables

| Variable       | Default                    | Description                         |
|----------------|----------------------------|-------------------------------------|
| `OPENAI_API_KEY` | —                        | Required. OpenAI API key            |
| `GROQ_API_KEY`   | —                        | Optional. Groq API key              |
| `OLLAMA_URL`     | —                        | Optional. Ollama base URL (e.g. `http://localhost:11434`) |
| `OLLAMA_MODEL`   | `llama3.2`               | Model to use when Ollama is active  |
| `MONGO_URL`      | `mongodb://localhost:27017` | MongoDB connection string          |
| `MONGO_DATABASE` | `eywa_example`           | MongoDB database name               |
| `REDIS_URL`      | `redis://localhost:6379` | Redis connection string             |

## Run

```bash
# OpenAI only
OPENAI_API_KEY=sk-... go run .

# OpenAI + Groq
OPENAI_API_KEY=sk-... GROQ_API_KEY=gsk_... go run .

# OpenAI + local Ollama
OPENAI_API_KEY=sk-... OLLAMA_URL=http://localhost:11434 go run .
```

## Expected output

```
=== Eywa — Multi-Provider ===
Groq oracle registered

--- OpenAI ---
Spirit:   openai_assistant
Time:     843ms
Reply:    The capital of France is Paris.

--- Alternative provider ---
Spirit:   groq_assistant
Time:     312ms
Reply:    Paris is the capital of France.
```

## Adding more providers

Same pattern for Mistral, Together AI, or any OpenAI-compatible endpoint:

```go
builder = builder.AddOracle(eywaopenai.NewMistralOracle(os.Getenv("MISTRAL_API_KEY")))
// Spirit.ModelConfig.Provider = "mistral"

builder = builder.AddOracle(eywaopenai.NewTogetherOracle(os.Getenv("TOGETHER_API_KEY")))
// Spirit.ModelConfig.Provider = "together"

// Any custom OpenAI-compatible endpoint:
builder = builder.AddOracle(eywaopenai.NewOracleWithConfig(eywaopenai.Config{
    Name:    "my-provider",
    APIKey:  "...",
    BaseURL: "https://my-llm-server/v1",
}))
// Spirit.ModelConfig.Provider = "my-provider"
```
