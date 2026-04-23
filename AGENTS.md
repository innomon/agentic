# Agentic - ADK-Go Framework

## Overview

Agentic is a config-driven agentic framework built on Google's ADK-Go. It enables building multi-agent systems entirely through YAML configuration and WebAssembly plugins — no recompilation needed to add new use-cases.

## Commands

```bash
# Build the project
go build -o agentic .

# Run with default config (config/config.yaml)
./agentic console

# Run with custom config (positional config path)
./agentic examples/farmer/config.yaml console

# Run in web UI mode (defaults to api sublauncher)
./agentic web

# Run with flags to enable sublaunchers and set options
./agentic -webui -openclaw -port 8080 -host localhost

# List available Gemini models and their configuration status
go build -o gemini-ls ./cmd/gemini-ls/main.go
./gemini-ls [-v] [-api-key=KEY] [-config=path/to/config.yaml]

# Run tests
go test ./...

# Tidy dependencies
go mod tidy
```

### Command Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-console` | Enable console launcher | `false` |
| `-webui` | Enable Web UI launcher | `false` |
| `-openclaw` | Enable OpenClaw WebSocket gateway | `false` |
| `-a2a` | Enable A2A launcher | `false` |
| `-api` | Enable REST API launcher | `true` |
| `-port` | Port to listen on | `8080` |
| `-host` | Host address for address calculation | `localhost` |

### Console Mode File Attachments

The console supports attaching files using `@/path/to/file` syntax:

```bash
./agentic console
User -> Analyze this document @./document.pdf
User -> Process these files @./image.png @./notes.txt
```

### API File Attachments

Send files via the REST API using base64-encoded inline data:

```bash
# Create session
SESSION=$(curl -s -X POST "http://localhost:8080/api/apps/Agentic/users/user/sessions" \
  -H "Content-Type: application/json" -d '{}' | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

# Send PDF
curl -N -X POST "http://localhost:8080/api/run_sse" \
  -H "Content-Type: application/json" \
  -d '{
    "appName": "Agentic",
    "userId": "user",
    "sessionId": "'"$SESSION"'",
    "streaming": true,
    "newMessage": {
      "role": "user",
      "parts": [
        {"text": "Analyze this document"},
        {"inlineData": {"mimeType": "application/pdf", "data": "'"$(base64 -w0 file.pdf)"'"}}
      ]
    }
  }'
```

## Root Agent

The `root_agent` top-level config field specifies which agent is the entry point. Defaults to `RootAgent` if omitted.

```yaml
root_agent: RootAgent
```

## Project Structure

```
agentic/
├── main.go                      # Entry point (universal launcher)
├── config/
│   └── config.yaml              # Default configuration
├── examples/
│   ├── med-fhir/                # Medical FHIR transcription use-case
│   ├── farmer/                  # Organic farming advisor use-case
│   ├── routing/                 # Role-based routing example
│   ├── search/                  # Web search agent example
│   ├── wasm-sequential/         # WASM orchestrator example
│   ├── prolog-memory/           # Logic-based Prolog knowledge example
│   └── ml/                      # Local embedded LLM example
├── pkg/
│   ├── fsread/                  # Filesystem tool (fs_read)
│   ├── compreg/
│   │   └── compreg.go           # Global component register (shared map)
│   ├── config/
│   │   └── config.go            # Config file loader
│   ├── console/
│   │   └── console.go           # Custom console with @file attachment syntax
│   ├── gnogent/                 # Deterministic GnoVM agent (no LLM)
│   ├── gnovm/                   # GnoVM engine and machine wrappers
│   ├── sandbox/                 # Generic VM sandbox manager
│   ├── auth/
│   │   └── verifier.go          # JWT RS256 token verification
│   ├── memory/
│   │   └── mem2db.go            # Database-backed memory service (GORM)
│   ├── prologmem/               # Prolog logic-based memory
│   ├── routing/                 # Role-based routing agent
│   ├── userdb/
│   │   └── userdb.go            # User profile database (GORM)
│   ├── ml/                      # Local embedded LLM (pure Go, GGUF)
│   ├── wasm/                    # WASM extension (wazero runtime)
│   ├── openclaw/
│   │   ├── launcher/
│   │   │   └── launcher.go      # OpenClaw sublauncher
│   └── registry/                # Unified registry (config, components, instances)
├── cmd/
│   ├── clawgate/
│   │   └── main.go              # Standalone OpenClaw gateway binary
│   └── gemini-ls/
│       └── main.go              # Model listing utility
├── go.mod
├── go.sum
├── AGENTS.md                    # This file
└── README.md
```

## Code Conventions

- Use ADK-Go patterns for agent creation
- Models are created via `google.golang.org/adk/model/gemini`
- Agents use `google.golang.org/adk/agent/llmagent`
- Launcher from `google.golang.org/adk/cmd/launcher`
- Use `universal.NewLauncher()` with custom sub-launchers
- Agent functions should accept `(ctx context.Context, m model.LLM)` and return `(agent.Agent, error)`
- Use `SubAgents` field in `llmagent.Config` for routing to sub-agents
- ADK-Go auto-injects `transfer_to_agent` tool when SubAgents are declared
- **Atomic Transfers**: When an agent performs a task and then transfers to a sub-agent, instruct the agent to call `transfer_to_agent` in the **same response** as its task output.
- **ADK limitation**: Each agent can only have one parent. Duplicate agent trees if multiple parents need the same sub-agent.

## Tools Registry

Tools are defined in YAML and referenced by agents. Register Go handlers for execution:

```yaml
tools:
  my_tool:
    description: Tool description
    parameters:
      param1: {type: string, required: true}

agents:
  MyAgent:
    tools: [my_tool]
```

```go
registry.RegisterToolHandler("my_tool", func(ctx context.Context, args map[string]any) (any, error) {
    return result, nil
})
```

#### Filesystem Tool (`fs_read`)

The `fs_read` built-in tool allows agents to read local files.

```yaml
tools:
  schema_reader:
    type: builtin
    description: Read the FHIR schema
    parameters:
      path: {type: string, required: true}
```

#### Gemini Built-in Tools

Agents using Gemini models can use built-in Google services.

```yaml
tools:
  web_search:
    type: gemini
    tool: google_search
    description: Search the web using Google
```

#### Sandbox Tool

Execute code in isolated VM environments (e.g., GnoVM).

```yaml
tools:
  gno_sandbox:
    type: sandbox
    description: Execute Gno code in a sandbox
    type: gno
    timeout: 5s
    memory_limit_mb: 128
```

## UserDB Tools

The `userdb` tool type provides GORM-backed user profile management.

```yaml
tools:
  get_user_profile:
    type: userdb
    op: get_profile
    db:
      driver: postgres
      dsn: postgres://user:pass@localhost/mydb
```

Operations: `get_profile`, `create_user`, `update_status`, `update_roles`, `update_channels`, `delete_user`.

## MCP Toolsets

Agents can connect to external [Model Context Protocol](https://modelcontextprotocol.io/) servers.

```yaml
agents:
  MyAgent:
    model: gemini-flash
    mcp_toolsets:
      - endpoint: "${MCP_SERVER_URL:-http://localhost:8082}/mcp"
```

## Custom Agent Types

The component registry uses Go generics for type-safe registration.

```go
registry.RegisterAgentType("myType", func(ctx context.Context, name string, cfg *MyAgentConfig, models registry.ModelRegistry, tools registry.ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
    return myCustomAgent, nil
})
```

Built-in types:
- `llm` (default) - Standard LLM agent
- `sequential`, `parallel`, `loop` - Workflow orchestrators
- `routing` - Role-based routing agent
- `wasm` - WebAssembly agent
- `gnogent` - Deterministic GnoVM agent

## Wasm Tools

The `wasm` tool type executes sandboxed WebAssembly modules.

```yaml
tools:
  my_wasm_tool:
    type: wasm
    module_path: ./plugins/tool.wasm
    security:
      allowed_paths: [/data/input]
      allowed_domains: ["*.api.com"]
```

## Logic Query Tool

Exposes an embedded Prolog interpreter as an ADK tool.

```yaml
tools:
  logic_query:
    type: logic_query
    kb_path: ./knowledge.pl
```

Actions: `query`, `assert`, `retract`, `check`, `save`.

## Session Configuration

```yaml
session:
  provider: database # inmemory, database, gnogent, vertexai
  driver: postgres
  dsn: postgres://user:pass@localhost/mydb
```

## Memory Configuration

```yaml
memory:
  provider: database # inmemory, database, gnogent, prolog
  driver: postgres
  dsn: postgres://user:pass@localhost/mydb
```

## Auth Configuration

JWT authentication protects API endpoints. Tokens use RS256 signing.

```yaml
auth:
  jwt:
    public_key_path: secrets/jwt_public.pem
    issuer: my-gateway
    audience: agentic
```

## Environment Variables

- `GOOGLE_API_KEY` - Required for Gemini
- `OPENAI_API_KEY` - Required for OpenAI
- `BYPASS_AUTH` - Set to `true` to skip JWT verification on localhost

## Model Configuration

```yaml
models:
  my-model:
    provider: gemini # gemini, openai, ollama, ml
    model_id: gemini-2.0-flash
    api_key: ${API_KEY}
```

### ML Provider (Local GGUF)

Runs GGUF-quantized models locally in pure Go.

```yaml
models:
  local-llm:
    provider: ml
    model_id: smollm2-135m
    model_path: ./models/SmolLM2-135M-Instruct-Q4_K_M.gguf
    threads: 4
```

Supported architectures: **LLaMA** family and **Granite** family (dense and hybrid).

## OpenClaw Gateway (clawgate)

OpenClaw WebSocket gateway routes client conversations to ADK agents. It can be run as a standalone binary or as a sublauncher within the main `agentic` process.

```bash
# Standalone
go build -o clawgate ./cmd/clawgate/
./clawgate

# Integrated
./agentic -openclaw web
```
