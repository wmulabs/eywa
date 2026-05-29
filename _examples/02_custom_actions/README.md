# ⚡ Example 02: Custom Actions

**Complexity:** ⭐⭐ Intermediate  
**Time:** ~20 minutes

How to implement and register custom Actions that a Spirit can call during reasoning.

---

## 📖 What You'll Learn

- Implement the `Action` interface
- Register Actions in an `ActionRegistry`
- Use `Spirit.AllowedActions` to control which Actions the Oracle can invoke
- Classify errors: `BusinessError` vs `InfrastructureError`
- Handle multi-action turns (Oracle calls multiple tools in one iteration)

---

## ⚡ Actions in This Example

### `get_weather` — Retrieval Action

Simulates fetching weather data for a city.

```
User: "What's the weather in São Paulo?"
Oracle: [calls get_weather with city="são paulo"]
Action: "Weather in São Paulo: Sunny 28°C, humidity 65%"
Oracle: "The weather in São Paulo is sunny at 28°C..."
```

### `calculator` — General Action

Performs basic math operations.

```
User: "What is 156 * 789?"
Oracle: [calls calculator with operation="multiply", a=156, b=789]
Action: "156.00 multiply 789.00 = 123084.00"
Oracle: "156 × 789 equals 123,084."
```

---

## 🔧 The Action Interface

```go
type Action interface {
    GetName() string                                        // unique snake_case name
    GetDescription() string                                 // tells Oracle when to use it
    GetParameters() map[string]interface{}                  // JSON Schema
    GetCategory() ActionCategory                            // Delivery | Retrieval | Modification | General
    IsCritical() bool                                       // true = response delivered, skip Voice
    Validate(args map[string]interface{}) error             // called before Execute
    Execute(ctx context.Context, args map[string]interface{}) (string, error)
}
```

> [!TIP]
> The description is what the Oracle reads to decide when to call your Action. Make it specific and clear about the input it expects.

---

## ⚠️ Error Classification

```go
// User error — not retryable, message shown to user via BusinessErrorInstructions
return "", eywa.NewBusinessError("cannot divide by zero")

// Infrastructure error — retryable (if WithActionRetry configured), hidden from user
return "", eywa.NewInfrastructureError("payment API unavailable", err)
```

> [!IMPORTANT]
> Always classify errors correctly. `BusinessError` is shown to the user (transformed by the Spirit's `BusinessErrorInstructions`). `InfrastructureError` is hidden and triggers retry logic.

---

## ⚙️ Prerequisites

- Go 1.25+
- MongoDB and Redis running locally
- OpenAI API key

```bash
docker run -d -p 27017:27017 --name mongodb mongo:latest
docker run -d -p 6379:6379 --name redis redis:latest
```

---

## 🔑 Environment Variables

```bash
export OPENAI_API_KEY="sk-..."

# Optional — defaults shown
export MONGO_URL="mongodb://localhost:27017"
export MONGO_DATABASE="eywa_example"
export REDIS_URL="redis://localhost:6379"
export SERVICE_NAME="eywa"
export ENVIRONMENT="lcl"
```

---

## 🚀 Run

```bash
cd _examples
go run ./02_custom_actions/main.go
```

---

## 📊 Expected Output

```
=== Eywa — Custom Actions ===
Actions registered: get_weather, calculator

--- Test 1: Weather ---
Status:  success
Actions: [get_weather]
Reply:   The weather in São Paulo is currently sunny at 28°C with 65% humidity.

--- Test 2: Calculator ---
Status:  success
Actions: [calculator]
Reply:   156 multiplied by 789 equals 123,084.

--- Test 3: Multi-action ---
Status:  success
Actions: [get_weather calculator]
Reply:   In Tokyo it's rainy at 18°C with 85% humidity. And 25 + 75 = 100.
```

---

## 🏗️ Real-World Action Ideas

| Action | Category | `IsCritical` |
|--------|----------|-------------|
| `send_whatsapp_message` | Delivery | `true` — response already sent |
| `track_order` | Retrieval | `false` |
| `cancel_order` | Modification | `false` |
| `create_ticket` | Modification | `false` |
| `generate_invoice` | General | `false` |

---

## 🔍 Troubleshooting

**Action not being called:**
- Check `Spirit.AllowedActions` includes the action name
- Make the description more explicit about when to use it
- Try asking more directly: "use the weather tool for São Paulo"

**Wrong arguments error:**
- Verify `GetParameters()` returns a valid JSON Schema
- Test `Validate()` independently before integrating

---

## ➡️ Next Steps

- [03 — Advanced Routing](../03_advanced_routing/) — Scouts + Pathfinders + multi-Spirit routing
- [04 — Sync vs Async](../04_async_concept/) — production-scale event processing
