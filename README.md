# Agentic

A config-driven agentic framework built on Google's [ADK-Go](https://github.com/google/adk-go). Define agents, models, tools, and workflows entirely in YAML — extend with WebAssembly plugins, no recompilation needed.

**Work In Progress: Do not use yet**

For more detailed technical documentation and architectural guides, see [docs/README.md](docs/README.md).

## Features

- **Config-Driven**: All agents, models, tools, and workflows defined in YAML
- **Multiple Agent Types**: LLM, sequential, parallel, loop, routing, and WASM agents
- **Multi-Model Support**: Gemini, OpenAI, Ollama (local), and custom providers
- **WebAssembly Extensions**: Sandboxed WASM tools and agents via wazero with security policies, OCI registry support, and per-invocation isolation
- **MCP Integration**: Connect agents to external [Model Context Protocol](https://modelcontextprotocol.io/) servers for dynamic tool discovery
- **Role-Based Routing**: Route users to agents based on database-stored roles with admin config override and contextual disambiguation
- **Persistent Memory**: Database-backed conversation memory (PostgreSQL, SQLite)
- **Logic-Based Memory**: Prolog-powered knowledge base with fact assertion, logical queries, and inference via [ichiban/prolog](https://github.com/ichiban/prolog)
- **JWT Authentication**: Optional RS256 JWT verification middleware
- **User Profile Database**: GORM-backed user management with JSONB profile/metadata
- **OpenAI-Compatible Proxy**: Drop-in proxy for OpenAI API consumers
- **Console & Web UI**: Interactive terminal with file attachments, browser UI, REST API, and OpenClaw WebSocket gateway

## Quick Start

1. Set your API key:
   ```bash
   export GOOGLE_API_KEY="your-api-key"
   ```

2. Build:
   ```bash
   go build -o agentic .
   ```

3. Run:
   ```bash
   ./agentic console
   ```

## Usage

### Console Mode

Interactive terminal with `@/path/to/file` attachment syntax:

```bash
./agentic console
```

```
Agentic Console (attach files with @/path/to/file syntax)
Example: Analyze this document @./document.pdf
Type '/help' for commands, '/exit' to quit.

User -> Summarize this report @./report.pdf
[Attached: report.pdf (application/pdf, 125432 bytes)]

Agent -> ...
```

### Web UI Mode

```bash
./agentic -webui -a2a
```

Open http://localhost:8080/ui/ in your browser. Supports drag-and-drop file uploads.

#### LAN Access

To access the Web UI from another machine on your local network, specify your local IP address using the `-host` flag:

```bash
# Example: replace 192.168.1.10 with your actual local IP
./agentic -webui -a2a -host=192.168.1.10
```

Then open `http://192.168.1.10:8080/ui/` on any device in the same network.

### OpenClaw WebSocket Gateway

Run the OpenClaw WebSocket gateway as part of the web server:

```bash
# Web server with REST API + OpenClaw gateway
./agentic -a2a -openclaw

# All sublaunchers (API + Web UI + OpenClaw)
./agentic -a2a -webui -openclaw
```

The gateway listens on `/ws` by default. Use `-ws-path` to change it.

The standalone `cmd/clawgate` binary is also available for dedicated gateway deployments.

### Gemini Model Listing

List all available Gemini models and check which ones are configured in your YAML:

```bash
# Build the tool
go build -o gemini-ls ./cmd/gemini-ls/main.go

# Run (requires GOOGLE_API_KEY)
./gemini-ls

# Run with verbose output (shows token limits and actions)
./gemini-ls -v
```

### Custom Configuration

Load any YAML config file. You can also enable launchers directly in the YAML file.

```bash
./agentic -console examples/farmer/config.yaml
./agentic -webui -a2a examples/med-fhir/config.yaml
./agentic examples/flags/config.yaml
```

### Command Line

```bash
./agentic [flags] [config.yaml] [options]

Flags:
  -console          Interactive terminal mode
  -webui            Browser-based UI
  -a2a              REST API (A2A)
  -openclaw         OpenClaw WebSocket gateway
  -host             Host for API/WebUI (e.g. local IP)
  -port             Port to listen on (default: 8080)

Options:
  --help            Show help message
```

## Examples

Pre-built use-case configurations in `examples/`:

| Example | Description | Run Command |
|---------|-------------|-------------|
| [flags](examples/flags/) | All launchers enabled in config | `./agentic examples/flags/config.yaml` |
| [med-fhir](examples/med-fhir/) | Medical document → FHIR R5 JSON | `./agentic -console examples/med-fhir/config.yaml` |
| [farmer](examples/farmer/) | Organic farming advisor (India) | `./agentic -console examples/farmer/config.yaml` |
| [routing](examples/routing/) | Role-based user routing | `./agentic -console examples/routing/config.yaml` |
| [search](examples/search/) | Web search via Google Search | `./agentic -console examples/search/config.yaml` |
| [hedge](examples/hedge/) | Multi-agent trading committee (MCP) | `./agentic -console examples/hedge/config.yaml` |
| [wasm-sequential](examples/wasm-sequential/) | WASM orchestrator agent | `./agentic -console examples/wasm-sequential/config.yaml` |
| [wasm-loop](examples/wasm-loop/) | WASM iterative refinement loop | `./agentic -console examples/wasm-loop/config.yaml` |
| [prolog-memory](examples/prolog-memory/) | Logic-based Prolog knowledge agent | `./agentic -console examples/prolog-memory/config.yaml` |
| [openclaw](examples/openclaw/) | OpenClaw WebSocket gateway | `./agentic -console examples/openclaw/config.yaml` |

## Configuration

All configuration is in YAML. The default config is `config/config.yaml`.

### Minimal Configuration

```yaml
root_agent: RootAgent

models:
  gemini-flash:
    provider: gemini
    model_id: gemini-2.0-flash
    default: true

agents:
  RootAgent:
    description: General-purpose assistant
    model: gemini-flash
    instruction: |
      You are a helpful assistant.
```

### Root Agent

The `root_agent` field specifies the entry-point agent. Defaults to `RootAgent` if omitted.

### Models

```yaml
models:
  gemini-flash:
    provider: gemini
    model_id: gemini-2.0-flash
    default: true

  gpt4o:
    provider: openai
    model_id: gpt-4o

  local-llama:
    provider: ollama
    model_id: llama3.2
    base_url: http://localhost:11434/v1

  vertex-model:
    provider: gemini
    model_id: gemini-2.0-flash
    backend: vertexai
    project: my-gcp-project
    location: us-central1
```

| Field | Description | Required |
|-------|-------------|----------|
| `provider` | `gemini`, `openai`, or `ollama` | Yes |
| `model_id` | Provider-specific model identifier | Yes |
| `default` | Set to `true` for default model | No |
| `api_key` | API key (uses env var if omitted) | No |
| `backend` | For Gemini: `gemini` (default) or `vertexai` | No |
| `project` | GCP project ID (Vertex AI) | No |
| `location` | GCP region (Vertex AI) | No |
| `base_url` | Server URL (required for Ollama) | No |

### Agents

```yaml
agents:
  MyAgent:
    type: llm            # optional, defaults to "llm"
    description: Agent description
    model: gemini-flash
    tools: [tool1, tool2]
    sub_agents:
      - SubAgent1
    instruction: |
      System prompt...
```

| Field | Description | Required |
|-------|-------------|----------|
| `type` | Agent type (default: `llm`) | No |
| `description` | Agent description for routing | Yes |
| `model` | Model name from models config | Yes (for `llm`) |
| `sub_agents` | List of sub-agent names | No |
| `tools` | List of tool names | No |
| `mcp_toolsets` | List of MCP server endpoints (see [MCP Toolsets](#mcp-toolsets)) | No |
| `instruction` | System prompt | Yes (for `llm`) |
| `max_iterations` | Loop iterations (0 = until escalation) | No (for `loop`) |

#### Built-in Agent Types

| Type | Description |
|------|-------------|
| `llm` | Standard LLM agent (default) |
| `sequential` | Executes sub-agents once in order |
| `parallel` | Executes sub-agents concurrently |
| `loop` | Repeatedly executes sub-agents |
| `routing` | Role-based user routing with disambiguation |
| `wasm` | WebAssembly agent via wazero |
| `gnogent` | Deterministic GnoVM agent (no LLM) |

#### Workflow Examples

```yaml
agents:
  Pipeline:
    type: sequential
    description: Process documents in stages
    sub_agents: [ExtractAgent, TransformAgent, ValidateAgent]

  MultiAnalysis:
    type: parallel
    description: Run analyses simultaneously
    sub_agents: [SentimentAgent, EntityAgent, SummaryAgent]

  RefineLoop:
    type: loop
    description: Iterative refinement
    max_iterations: 3
    sub_agents: [DraftAgent, ReviewAgent]
```

#### Routing Agent

Routes users to sub-agents based on database roles:

```yaml
agents:
  Router:
    type: routing
    model: gemini-flash
    admin_users: [admin1, admin2]
    role_routes:
      admin: AdminAgent
      user: UserAgent
      anonymous: PublicAgent
    tools: [get_user_profile]
    sub_agents: [AdminAgent, UserAgent, PublicAgent]
```

| Field | Description | Required |
|-------|-------------|----------|
| `admin_users` | User IDs granted admin role (config-only) | No |
| `role_routes` | Map of role → sub-agent name | Yes |
| `model` | Model for disambiguation | Yes |

Routing logic:
1. Retrieves user profile via `get_user_profile` tool
2. Admin users (from config) route to admin agent
3. Pending/Suspended users are denied
4. Single role → direct route; multiple roles → LLM disambiguation
5. Unknown users → `anonymous` agent or reject

See [examples/routing/](examples/routing/) for a complete example.

#### Deterministic GnoVM Agent

Runs agents entirely through GnoVM logic — no LLM required. User input is processed deterministically through the GnoVM brain, which manages mood, friendship, and personality state persisted to Postgres.

```yaml
agents:
  Gnogent:
    type: gnogent
    description: Stateful deterministic agent powered by GnoVM
    database:
      dsn: postgres://user:pass@localhost/mydb
      auto_migrate: true
    gnovm:
      source_file: ./gno/agent.gno
      pkg_path: gno.land/p/agent
```

| Field | Description | Required |
|-------|-------------|----------|
| `database.dsn` | PostgreSQL connection string | Yes |
| `database.auto_migrate` | Auto-create schema on startup | No |
| `gnovm.source_file` | Path to `.gno` source file | Yes |
| `gnovm.pkg_path` | GnoVM package path (default: `gno.land/p/agent`) | No |

Each turn executes: thaw (restore VM state from DB) → SyncState (evolve mood/personality) → GetSystemContext (produce response) → AddTurn (archive conversation) → freeze (persist VM state to DB).

### Tools

Define tools in YAML, attach to agents by name:

```yaml
tools:
  get_weather:
    description: Get current weather for a location
    parameters:
      location: {type: string, required: true}
      unit: {type: string, description: celsius or fahrenheit}

agents:
  WeatherAgent:
    model: gemini-flash
    tools: [get_weather]
```

Register Go handlers:

```go
registry.RegisterToolHandler("get_weather", func(ctx context.Context, args map[string]any) (any, error) {
    location := args["location"].(string)
    return map[string]any{"temperature": 22, "condition": "sunny"}, nil
})
```

#### Tool Types

| Type | Description |
|------|-------------|
| `builtin` | Default — Go-registered handler (e.g., `fs_read`) |
| `gemini` | Gemini built-in tools (e.g., `google_search`) |
| `userdb` | GORM-backed user profile CRUD |
| `wasm` | Sandboxed WebAssembly module |
| `mcp` | Remote tools via MCP server (Streamable HTTP transport) |

#### UserDB Tool Type

GORM-backed user profile management:

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
    admin_users: [admin1]
```

Operations: `get_profile`, `create_user`, `update_status`, `update_roles`, `update_channels`, `delete_user`.

#### Wasm Tool Type

Sandboxed WebAssembly tools with security policies:

```yaml
tools:
  my_wasm_tool:
    type: wasm
    description: Run a sandboxed WASM tool
    module_path: ./plugins/tool.wasm
    security:
      allowed_paths: [/data/input]
      allowed_domains: [api.example.com]
      memory_max_pages: 256

  oci_tool:
    type: wasm
    description: WASM tool from OCI registry
    oci_ref: ghcr.io/myorg/my-tool:latest
```

| Field | Description | Required |
|-------|-------------|----------|
| `module_path` | Path to local `.wasm` file | Yes (or `oci_ref`) |
| `oci_ref` | OCI registry reference | Yes (or `module_path`) |
| `cache_dir` | OCI blob cache directory | No |
| `security.allowed_paths` | Read-only filesystem mounts | No |
| `security.allowed_domains` | Network allow-list | No |
| `security.memory_max_pages` | Memory limit (default: 256 = 16MB) | No |

Wasm module ABI: export `alloc(size) -> ptr`, `run_tool(ptr, len) -> i64`, optionally `free(ptr, size)`. Host functions: `env.log_msg`, `env.http_fetch`.

#### Wasm Agent Type

Run WebAssembly modules as agents with sub-agent access:

```yaml
agents:
  Orchestrator:
    type: wasm
    description: WASM orchestrator
    module_path: ./plugins/orchestrator.wasm
    sub_agents: [Agent1, Agent2]
```

The module exports `execute() -> i32`. Host functions: `env.subagent_count`, `env.subagent_name`, `env.run_subagent`, `env.subagent_output_len`, `env.subagent_output_get`, `env.set_input`, `env.get_input_len`, `env.get_input`, `env.log_msg`.

#### MCP Toolsets

Connect agents to external [Model Context Protocol](https://modelcontextprotocol.io/) servers. Unlike individual tool definitions, MCP toolsets dynamically discover all tools exposed by a remote MCP server at startup.

```yaml
agents:
  MyAgent:
    model: gemini-flash
    mcp_toolsets:
      - endpoint: "${MCP_SERVER_URL:-http://localhost:8082}/mcp"
    instruction: |
      You are an assistant with access to external tools.
```

| Field | Description | Required |
|-------|-------------|----------|
| `endpoint` | MCP server URL (Streamable HTTP transport) | Yes |

Endpoints support environment variable expansion with defaults using `${VAR:-default}` syntax.

MCP toolsets can be combined with regular `tools` on the same agent:

```yaml
agents:
  MyAgent:
    model: gemini-flash
    tools: [get_weather]
    mcp_toolsets:
      - endpoint: http://localhost:8082/mcp
      - endpoint: http://localhost:8083/mcp
```

### Logic Query Tool

Expose a Prolog knowledge base to agents for logical reasoning:

```yaml
tools:
  logic_query:
    type: logic_query
    description: Run Prolog logic queries
    kb_path: ./knowledge.pl
    timeout_seconds: 5
    parameters:
      action:
        type: string
        description: "Action: query, assert, retract, check, or save"
        required: true
      query:
        type: string
        description: "Prolog term or goal"
        required: true
```

| Field | Description | Required |
|-------|-------------|----------|
| `kb_path` | Path to `.pl` knowledge base file | Yes |
| `timeout_seconds` | Query timeout (default: 5) | No |

Actions: `assert` (add fact), `retract` (remove fact), `query` (find solutions), `check` (true/false), `save` (persist to disk).

Standard predicates: `mem_fact/3`, `mem_rel/3`, `mem_context/3`, `agent_rule/3`.

### Extensibility

#### Custom Agent Types

```go
import "github.com/innomon/agentic/pkg/registry"

type MyAgentConfig struct {
    registry.AgentBase `yaml:",inline"`
    CustomField string `yaml:"custom_field"`
}

func init() {
    registry.RegisterAgentType("myType", func(ctx context.Context, name string, cfg *MyAgentConfig, models registry.ModelRegistry, tools registry.ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
        return myAgent, nil
    })
}
```

#### Custom Tool Types

```go
import "github.com/innomon/agentic/pkg/registry"

type APIToolConfig struct {
    registry.ToolBase `yaml:",inline"`
    Endpoint string `yaml:"endpoint"`
}

func init() {
    registry.RegisterToolType("api", func(ctx context.Context, name string, cfg *APIToolConfig) (tool.Tool, error) {
        return apiTool, nil
    })
}
```

#### Custom Model Providers

```go
import "github.com/innomon/agentic/pkg/registry"

type MyProviderConfig struct {
    registry.ModelBase `yaml:",inline"`
    Endpoint string `yaml:"endpoint"`
}

func init() {
    registry.RegisterModelProvider("myprovider", func(ctx context.Context, cfg *MyProviderConfig) (model.LLM, error) {
        return myModel, nil
    })
}
```

### Session Configuration

Session stores active conversation state. Defaults to in-memory.

```yaml
session:
  provider: database
  driver: postgres
  dsn: postgres://user:pass@localhost/mydb
  auto_migrate: true
```

| Field | Description | Required |
|-------|-------------|----------|
| `provider` | `inmemory` (default), `database`, `gnogent`, or `vertexai` | No |
| `driver` | `postgres` or `sqlite` | Yes (for `database`) |
| `dsn` | Connection string | Yes (for `database`) |
| `auto_migrate` | Auto-create schema | No |
| `project` | GCP project ID | Yes (for `vertexai`) |
| `location` | GCP region | Yes (for `vertexai`) |
| `reasoning_engine` | Reasoning Engine resource | Yes (for `vertexai`) |

```yaml
# Gnogent provider (Postgres-backed with GnoVM session tables):
session:
  provider: gnogent
  dsn: postgres://user:pass@localhost/mydb
  auto_migrate: true
```

### Memory Configuration

Memory stores agent conversation history. Defaults to in-memory.

```yaml
memory:
  provider: database
  driver: postgres
  dsn: postgres://user:pass@localhost/mydb
  auto_migrate: true
```

| Field | Description | Required |
|-------|-------------|----------|
| `provider` | `inmemory` (default), `database`, `gnogent`, or `prolog` | No |
| `driver` | `postgres` or `sqlite` | Yes (for `database`) |
| `dsn` | Connection string | Yes (for `database`) |
| `auto_migrate` | Auto-create schema | No |

```yaml
# Gnogent provider (Postgres-backed with GnoVM memory tables):
memory:
  provider: gnogent
  dsn: postgres://user:pass@localhost/mydb
  auto_migrate: true
```

```yaml
# Prolog provider (logic-based memory with .pl file persistence):
memory:
  provider: prolog
  kb_path: ./knowledge.pl
```

### Authentication

Optional RS256 JWT authentication for API endpoints:

```yaml
auth:
  jwt:
    public_key_path: secrets/jwt_public.pem
    issuer: my-gateway
    audience: agentic
```

Generate keys:

```bash
openssl genrsa -out secrets/jwt_private.pem 2048
openssl rsa -in secrets/jwt_private.pem -pubout -out secrets/jwt_public.pem
```

Set `BYPASS_AUTH=true` for local development (localhost only).

### REST API

Files can be sent via base64-encoded inline data:

```bash
# Create session
SESSION=$(curl -s -X POST "http://localhost:8080/api/apps/Agentic/users/user/sessions" \
  -H "Content-Type: application/json" -d '{}' | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

# Send message with file
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

### OpenAI-Compatible Proxy

```bash
cd openai-proxy
go build -o openai-proxy .
./openai-proxy -config config.yaml
```

```yaml
proxy:
  listen: ":9080"
  adk:
    endpoint: http://localhost:8080
    app_name: Agentic
```

```python
from openai import OpenAI

client = OpenAI(base_url="http://localhost:9080/v1", api_key="not-required")
response = client.chat.completions.create(
    model="Agentic",
    messages=[{"role": "user", "content": "Hello"}],
    stream=True
)
```

## Project Structure

```
agentic/
├── main.go                          # Entry point
├── config/
│   └── config.yaml                  # Default configuration
├── examples/
│   ├── med-fhir/                    # Medical FHIR transcription
│   ├── farmer/                      # Organic farming advisor
│   ├── hedge/                       # Multi-agent trading committee (MCP)
│   ├── openclaw/                    # OpenClaw WebSocket gateway
│   ├── routing/                     # Role-based routing
│   ├── search/                      # Web search agent
│   ├── prolog-memory/                # Logic-based Prolog knowledge agent
│   ├── wasm-sequential/             # WASM orchestrator
│   └── wasm-loop/                   # WASM refinement loop
├── pkg/
│   ├── auth/                        # JWT verification middleware
│   ├── fsread/                      # Filesystem tool (fs_read)
│   ├── compreg/                     # Component register
│   ├── config/                      # Config file loader
│   ├── console/                     # Console with @file attachments
│   ├── gnogent/                     # Deterministic GnoVM agent (no LLM)
│   ├── memory/                      # Database-backed memory (GORM)
│   ├── prologmem/                    # Prolog logic-based memory (ichiban/prolog)
│   ├── registry/                    # Unified registry (agents, models, tools)
│   ├── openclaw/                    # OpenClaw gateway (protocol, auth, server, client, launcher)
│   ├── routing/                     # Role-based routing agent
│   ├── userdb/                      # User profile database
│   └── wasm/                        # WASM runtime (wazero)
├── openai-proxy/                    # OpenAI-compatible proxy
├── docs/                            # Design documents
├── patches/                         # ADK-Go fork patches
├── go.mod
└── go.sum
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GOOGLE_API_KEY` | Gemini model access |
| `OPENAI_API_KEY` | OpenAI model access |
| `BYPASS_AUTH` | Skip JWT for localhost (dev only) |

## Prerequisites

- Go 1.24+
- API key for your chosen model provider (Gemini, OpenAI), or Ollama for local models
