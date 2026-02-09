# Agentic - ADK-Go Framework

## Overview

Agentic is a config-driven agentic framework built on Google's ADK-Go. It enables building multi-agent systems entirely through YAML configuration and WebAssembly plugins — no recompilation needed to add new use-cases.

## Commands

```bash
# Build the project
go build -o agentic .

# Run with default config (config/config.yaml)
./agentic console

# Run with custom config
./agentic examples/farmer/config.yaml console

# Run in web UI mode (http://localhost:8080/ui/)
./agentic web

# Run tests
go test ./...

# Tidy dependencies
go mod tidy
```

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
├── main.go                      # Entry point
├── config/
│   └── config.yaml              # Default configuration
├── examples/
│   ├── med-fhir/                # Medical FHIR transcription use-case
│   │   ├── config.yaml
│   │   ├── pkg/fhir/types.go    # FHIR R5 Go type definitions
│   │   └── README.md
│   ├── farmer/                  # Organic farming advisor use-case
│   │   ├── config.yaml
│   │   └── README.md
│   ├── routing/                 # Role-based routing example
│   │   ├── config.yaml
│   │   └── README.md
│   ├── search/                  # Web search agent example
│   │   ├── config.yaml
│   │   └── README.md
│   └── wasm-sequential/         # WASM orchestrator example
│       ├── config.yaml
│       ├── main.go
│       ├── Makefile
│       └── README.md
├── internal/
│   ├── compreg/
│   │   └── compreg.go           # Global component register (shared map)
│   ├── config/
│   │   └── config.go            # Config file loader (thin wrapper)
│   ├── console/
│   │   └── console.go           # Custom console with @file attachment syntax
│   ├── auth/
│   │   └── verifier.go          # JWT RS256 token verification and middleware
│   ├── memory/
│   │   └── mem2db.go            # Database-backed memory service (GORM)
│   ├── routing/                 # Role-based routing agent
│   │   ├── routing.go           # Routing agent type (role→agent mapping)
│   │   └── tools.go             # UserDB tool type (GORM user profiles)
│   ├── userdb/
│   │   └── userdb.go            # User profile database (GORM, JSONB)
│   ├── wasm/                    # WASM extension (wazero runtime)
│   │   ├── wasm.go              # Wasm agent type (sub-agent host fns)
│   │   ├── tool.go              # Wasm tool type (per-invocation isolation)
│   │   ├── policy.go            # Security policy engine (FS sandbox, domain allow-list)
│   │   ├── abi.go               # Component bridge ABI (alloc/run_tool/free)
│   │   ├── cache.go             # Compilation cache (wazero disk-backed)
│   │   ├── oci.go               # OCI registry puller (regclient, digest cache)
│   │   └── host_net.go          # Guarded HTTP host functions
│   └── registry/                # Unified registry (config, components, instances)
│       ├── registry.go          # Instance cache with generic Get[T]
│       ├── config.go            # Config types and YAML parsing
│       ├── compreg.go           # Component type registration and factories
│       ├── launcher.go          # Launcher config builder (session, memory)
│       ├── models.go            # Built-in model providers (Gemini, OpenAI)
│       ├── ollama.go            # Ollama provider (official OpenAI SDK)
│       ├── agents.go            # Built-in agent types (llm, sequential, etc.)
│       └── tools.go             # Tool type registration and built-in tools
├── openai-proxy/                # OpenAI-compatible API proxy
│   ├── main.go
│   └── config.yaml
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
- Use `universal.NewLauncher()` from `google.golang.org/adk/cmd/launcher/universal` with custom sub-launchers
- Agent functions should accept `(ctx context.Context, m model.LLM)` and return `(agent.Agent, error)`
- Use `SubAgents` field in `llmagent.Config` for routing to sub-agents
- ADK-Go auto-injects `transfer_to_agent` tool when SubAgents are declared
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

## UserDB Tools

The `userdb` tool type provides GORM-backed user profile management. Tools share a singleton DB connection per DSN.

```yaml
tools:
  get_user_profile:
    type: userdb
    op: get_profile
    description: Retrieve user profile
    parameters:
      user_id: {type: string, required: true}
    db:
      driver: postgres
      dsn: postgres://user:pass@localhost/mydb
      auto_migrate: true
    admin_users: [admin1, admin2]
```

Operations: `get_profile`, `create_user`, `update_status`, `update_roles`, `update_channels`, `delete_user`.

- Admin role cannot be set via `update_roles`; it is config-only via `admin_users`.
- `get_profile` returns `{"found": false}` for unknown users (enables anonymous routing).
- Caller ID for audit is extracted from JWT claims (`auth.ClaimsFromContext`).

## Custom Agent Types

The component registry uses Go generics for type-safe registration. Each component defines its own config struct:

```go
import "github.com/innomon/agentic/internal/registry"

type MyAgentConfig struct {
    registry.AgentBase `yaml:",inline"`
    CustomField string `yaml:"custom_field"`
}

func (c *MyAgentConfig) Validate() error { return nil }

func init() {
    registry.RegisterAgentType("myType", func(ctx context.Context, name string, cfg *MyAgentConfig, models registry.ModelRegistry, tools registry.ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
        return myCustomAgent, nil
    })
}
```

Built-in types:
- `llm` (default) - Standard LLM agent via `llmagent.New()`
- `sequential` - Executes sub-agents once in order via `sequentialagent.New()`
- `parallel` - Executes sub-agents concurrently via `parallelagent.New()`
- `loop` - Repeatedly executes sub-agents via `loopagent.New()` (use `max_iterations` config)
- `routing` - Role-based routing agent with user profile lookup and disambiguation
- `wasm` - WebAssembly agent via wazero runtime with sub-agent host functions

Specify type in config:
```yaml
agents:
  MyAgent:
    type: myType
    description: "..."
    custom_field: "value"

  MyWorkflow:
    type: sequential
    description: "Run agents in order"
    sub_agents: [Agent1, Agent2]

  MyLoop:
    type: loop
    description: "Iterative refinement"
    max_iterations: 3
    sub_agents: [RefineAgent]

  MyRouter:
    type: routing
    model: gemini-flash
    admin_users: [admin1]
    role_routes:
      admin: AdminAgent
      user: UserAgent
      anonymous: PublicAgent
    tools: [get_user_profile]
    sub_agents: [AdminAgent, UserAgent, PublicAgent]

  MyWasmAgent:
    type: wasm
    description: "Run a WebAssembly module as an agent"
    module_path: ./plugins/my_agent.wasm
    sub_agents: [SubAgent1]
```

## Wasm Tools

The `wasm` tool type executes sandboxed WebAssembly modules as ADK tools. Modules are loaded from local files or OCI registries. Each invocation creates a fresh wazero runtime for complete state isolation.

```yaml
tools:
  my_wasm_tool:
    type: wasm
    description: Run a sandboxed WASM tool
    module_path: ./plugins/tool.wasm
    security:
      allowed_paths: [/data/input]
      allowed_domains: [api.example.com, "*.internal.com"]
      memory_max_pages: 256

  oci_wasm_tool:
    type: wasm
    description: WASM tool from OCI registry
    oci_ref: ghcr.io/myorg/my-tool:latest
    cache_dir: /tmp/wasm-cache
    security:
      allowed_domains: [api.example.com]
```

| Field | Description | Required |
|-------|-------------|----------|
| `module_path` | Path to local `.wasm` file | Yes (or `oci_ref`) |
| `oci_ref` | OCI registry reference | Yes (or `module_path`) |
| `cache_dir` | Directory for OCI blob cache | No |
| `security.allowed_paths` | Absolute paths mounted read-only into guest | No |
| `security.allowed_domains` | Domains allowed for `http_fetch` host function | No |
| `security.memory_max_pages` | Max wasm memory pages (default: 256 = 16MB) | No |

### Wasm Module ABI

Wasm tool modules must export:
- `alloc(size i32) -> i32` — allocate a buffer in guest memory
- `run_tool(input_ptr i32, input_len i32) -> i64` — run the tool; returns packed `(out_ptr << 32 | out_len)`
- `free(ptr i32, size i32)` (optional) — free guest memory

Host functions available to modules:
- `env.log_msg(ptr i32, len i32)` — log a message
- `env.http_fetch(req_ptr i32, req_len i32) -> i64` — guarded HTTP request (JSON in/out, domain-checked)

### Wasm Agent Host Functions

Wasm agents additionally have access to sub-agent host functions:
- `env.subagent_count() -> i32` — number of sub-agents
- `env.subagent_name(index i32, buf_ptr i32, buf_cap i32) -> i32` — get sub-agent name
- `env.run_subagent(index i32) -> i32` — execute a sub-agent

## Custom Model Providers

Register model providers with custom config schemas:

```go
import "github.com/innomon/agentic/internal/registry"

type MyProviderConfig struct {
    registry.ModelBase `yaml:",inline"`
    Endpoint string `yaml:"endpoint"`
}

func init() {
    registry.RegisterModelProvider("myprovider", func(ctx context.Context, cfg *MyProviderConfig) (model.LLM, error) {
        return createModel(cfg.Endpoint, cfg.ModelID), nil
    })
}
```

## Session Configuration

```yaml
session:
  provider: database
  driver: postgres
  dsn: postgres://user:pass@localhost/mydb
  auto_migrate: true
```

| Field | Description | Required |
|-------|-------------|----------|
| `provider` | `inmemory` (default), `database`, or `vertexai` | No |
| `driver` | Database driver (`postgres`, `sqlite`) | Yes (for `database`) |
| `dsn` | Database connection string | Yes (for `database`) |
| `auto_migrate` | Auto-create/update schema on startup | No |
| `project` | GCP project ID | Yes (for `vertexai`) |
| `location` | GCP region | Yes (for `vertexai`) |
| `reasoning_engine` | Reasoning Engine resource name | Yes (for `vertexai`) |

## Memory Configuration

```yaml
memory:
  provider: database
  driver: postgres
  dsn: postgres://user:pass@localhost/mydb
  auto_migrate: true
```

| Field | Description | Required |
|-------|-------------|----------|
| `provider` | `inmemory` (default) or `database` | No |
| `driver` | Database driver (`postgres`, `sqlite`) | Yes (for `database`) |
| `dsn` | Database connection string | Yes (for `database`) |
| `auto_migrate` | Auto-create/update schema on startup | No |

## Auth Configuration

JWT authentication protects API endpoints when configured. Tokens use RS256 signing.

```yaml
auth:
  jwt:
    public_key_path: secrets/jwt_public.pem
    issuer: my-gateway
    audience: agentic
```

| Field | Description | Required |
|-------|-------------|----------|
| `public_key_path` | Path to RSA public key PEM file | Yes |
| `issuer` | Expected JWT issuer claim | No |
| `audience` | Expected JWT audience claim | No |

Generate the key pair:

```bash
openssl genrsa -out secrets/jwt_private.pem 2048
openssl rsa -in secrets/jwt_private.pem -pubout -out secrets/jwt_public.pem
```

Access claims in ADK callbacks:

```go
claims := auth.ClaimsFromContext(ctx)
if claims != nil {
    log.Printf("user_id=%s, channel=%s", claims.UserID, claims.Channel)
}
```

## Environment Variables

- `GOOGLE_API_KEY` - Required for Gemini model access (if not set in config)
- `OPENAI_API_KEY` - Required for OpenAI model access (if not set in config)
- `BYPASS_AUTH` - Set to `true` to skip JWT verification for localhost requests (dev only)

## Model Configuration

```yaml
models:
  my-model:
    provider: gemini
    model_id: gemini-2.0-flash
    default: true
    api_key: ${API_KEY}
    backend: vertexai
    project: my-gcp-project
    location: us-central1

  ollama-llama:
    provider: ollama
    model_id: llama3.2
    base_url: http://localhost:11434/v1
```

### Ollama Provider

The `ollama` provider uses the official OpenAI Go SDK (`github.com/openai/openai-go/v3`) with a custom base URL to connect to Ollama's OpenAI-compatible API.

Required fields:
- `provider: ollama`
- `model_id`: The model name as shown in `ollama list`
- `base_url`: The Ollama server URL with `/v1` suffix (e.g., `http://localhost:11434/v1`)
