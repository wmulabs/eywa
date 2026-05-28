# Example 11: MCP Client (Conduit)

Demonstrates **Conduit** — Eywa's MCP (Model Context Protocol) client.

## What this shows

- Connecting to an external MCP server over HTTP (StreamableHTTP transport)
- Auto-discovering tools via MCP's `tools/list` endpoint at `Build()` time
- Tools registered as Eywa actions with the prefix `<conduit_name>__<tool_name>`
- Spirit using MCP tools via `AllowedActions`
- Multiple MCP servers on the same `Weave` instance

## Key concepts

### Conduit

```go
conduit := eywamcp.NewConduit(eywamcp.ConduitConfig{
    Name:      "tools_server",
    Transport: "http",
    URL:       "http://localhost:3001",
    Timeout:   15 * time.Second,
    // Optional auth:
    // Headers: map[string]string{"Authorization": "Bearer " + apiKey},
})
```

### Tool name convention

MCP tool names are prefixed at discovery time: `<conduit_name>__<mcp_tool_name>`

```go
// MCP server exposes: "echo", "add"
// Registered in Eywa as: "tools_server__echo", "tools_server__add"

spirit.AllowedActions = []eywa.AllowedAction{
    {Name: "tools_server__echo"},
    {Name: "tools_server__add"},
}
```

Use `{Name: "*"}` to allow all tools from all conduits (not recommended for production).

### WeaveBuilder integration

```go
weave, err := eywa.NewWeaveBuilder(ctx).
    // ...
    WithConduit(conduit).  // tools auto-registered on Build()
    Build()
```

If the MCP server is unreachable at `Build()`, a warning is logged but the build succeeds.

## Running

Start a sample MCP server first:

```bash
npx @modelcontextprotocol/server-everything
```

Then:

```bash
export OPENAI_API_KEY=sk-...
export MONGO_URL=mongodb://localhost:27017
export REDIS_URL=redis://localhost:6379
export MCP_SERVER_URL=http://localhost:3001

go run .
```
