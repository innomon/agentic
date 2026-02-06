# MedAgent - ADK-Go Project

## Overview

MedAgent is a medical document transcription agent built with Google's ADK-Go framework. It converts PDF and image files of medical documents into FHIR R5 compliant JSON.

## Commands

```bash
# Build the project
go build -o med-agent .

# Run in console mode (with @file attachment syntax)
./med-agent console

# Run in web UI mode (http://localhost:8080/ui/)
./med-agent web

# Run tests
go test ./...

# Tidy dependencies
go mod tidy
```

### Console Mode File Attachments

The console supports attaching files using `@/path/to/file` syntax:

```bash
./med-agent console
User -> Create FHIR from this lab report @./document.pdf
User -> Extract prescription @./image.png @./notes.txt
```

### API File Attachments

Send files via the REST API using base64-encoded inline data:

```bash
# Create session
SESSION=$(curl -s -X POST "http://localhost:8080/api/apps/MedAgent/users/user/sessions" \
  -H "Content-Type: application/json" -d '{}' | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

# Send PDF
curl -N -X POST "http://localhost:8080/api/run_sse" \
  -H "Content-Type: application/json" \
  -d '{
    "appName": "MedAgent",
    "userId": "user",
    "sessionId": "'"$SESSION"'",
    "streaming": true,
    "newMessage": {
      "role": "user",
      "parts": [
        {"text": "Create FHIR from this"},
        {"inlineData": {"mimeType": "application/pdf", "data": "'"$(base64 -w0 file.pdf)"'"}}
      ]
    }
  }'
```

## Project Structure

```
med-agent/
├── main.go                      # Entry point with launcher setup
├── config/
│   └── config.yaml             # Agent and model configuration
├── internal/
│   ├── componentreg/           # Generic component registry (Go generics)
│   │   ├── registry.go         # Core registration with generics
│   │   ├── models.go           # Built-in model providers (Gemini, OpenAI)
│   │   ├── ollama.go           # Ollama provider (official OpenAI SDK)
│   │   ├── agents.go           # Built-in agent types (llm, sequential, etc.)
│   │   └── tools.go            # Tools registry and built-in tool types
│   ├── config/
│   │   └── config.go           # Config loader with schema-based parsing
│   ├── console/
│   │   └── console.go          # Custom console with @file attachment syntax
│   ├── memory/
│   │   └── mem2db.go           # Database-backed memory service (GORM)
│   └── registry/
│       ├── model.go            # Model registry (lazy loading)
│       ├── agent.go            # Agent registry (dependency resolution)
│       └── tool.go             # Tool registry (lazy loading)
├── pkg/
│   └── fhir/
│       └── types.go            # FHIR R5 Go type definitions
├── go.mod
├── go.sum
├── AGENTS.md                   # This file
└── README.md                   # Project documentation
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
    tools: [my_tool]  # attach tools to agent
```

```go
componentreg.RegisterToolHandler("my_tool", func(ctx context.Context, args map[string]any) (any, error) {
    return result, nil
})
```

## Custom Agent Types

The component registry uses Go generics for type-safe registration. Each component defines its own config struct:

```go
import "github.com/innomon/med-agent/internal/componentreg"

// Define config struct with custom fields
type MyAgentConfig struct {
    componentreg.AgentBase `yaml:",inline"`
    CustomField string `yaml:"custom_field"`
}

// Optional: implement Validate() for validation
func (c *MyAgentConfig) Validate() error { return nil }

func init() {
    componentreg.RegisterAgentType("myType", func(ctx context.Context, name string, cfg *MyAgentConfig, models componentreg.ModelRegistry, sub []agent.Agent) (agent.Agent, error) {
        // cfg is fully typed - access cfg.CustomField directly
        return myCustomAgent, nil
    })
}
```

Built-in types:
- `llm` (default) - Standard LLM agent via `llmagent.New()`
- `sequential` - Executes sub-agents once in order via `sequentialagent.New()`
- `parallel` - Executes sub-agents concurrently via `parallelagent.New()`
- `loop` - Repeatedly executes sub-agents via `loopagent.New()` (use `max_iterations` config)

Specify type in config:
```yaml
agents:
  MyAgent:
    type: myType  # omit for default "llm"
    description: "..."
    custom_field: "value"  # custom fields defined by component

  MyWorkflow:
    type: sequential
    description: "Run agents in order"
    sub_agents:
      - Agent1
      - Agent2

  MyLoop:
    type: loop
    description: "Iterative refinement"
    max_iterations: 3  # 0 = run until escalation
    sub_agents:
      - RefineAgent
```

## Custom Model Providers

Register model providers with custom config schemas:

```go
import "github.com/innomon/med-agent/internal/componentreg"

type MyProviderConfig struct {
    componentreg.ModelBase `yaml:",inline"`
    Endpoint string `yaml:"endpoint"`
}

func init() {
    componentreg.RegisterModelProvider("myprovider", func(ctx context.Context, cfg *MyProviderConfig) (model.LLM, error) {
        return createModel(cfg.Endpoint, cfg.ModelID), nil
    })
}
```

## Session Configuration

Session stores active conversation state. If omitted or set to `inmemory`, in-memory storage is used. Set `provider: database` for persistent storage via GORM, or `provider: vertexai` for Vertex AI.

Configure in `config/config.yaml`:

```yaml
session:
  provider: database
  driver: postgres          # Required: postgres or sqlite
  dsn: postgres://user:pass@localhost/medagent  # Required: connection string
  auto_migrate: true        # Optional: auto-create/update tables
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

Memory stores agent conversation history. If omitted or set to `inmemory`, in-memory storage is used. Set `provider: database` for persistent storage via GORM.

Configure in `config/config.yaml`:

```yaml
memory:
  provider: database
  driver: postgres          # Required: postgres or sqlite
  dsn: postgres://user:pass@localhost/medagent  # Required: connection string
  auto_migrate: true        # Optional: auto-create/update tables
```

| Field | Description | Required |
|-------|-------------|----------|
| `provider` | `inmemory` (default) or `database` | No |
| `driver` | Database driver (`postgres`, `sqlite`) | Yes (for `database`) |
| `dsn` | Database connection string | Yes (for `database`) |
| `auto_migrate` | Auto-create/update schema on startup | No |

## FHIR Coding Systems

- **RxNorm**: Medications (`http://www.nlm.nih.gov/research/umls/rxnorm`)
- **LOINC**: Lab tests, document types (`http://loinc.org`)
- **SNOMED CT**: Clinical findings, body sites (`http://snomed.info/sct`)
- **UCUM**: Units of measure (`http://unitsofmeasure.org`)

## Environment Variables

- `GOOGLE_API_KEY` - Required for Gemini model access (if not set in config)
- `OPENAI_API_KEY` - Required for OpenAI model access (if not set in config)

## Model Configuration

Models support the following configuration fields in `config/config.yaml`:

```yaml
models:
  my-model:
    provider: gemini          # Required: gemini, openai, or ollama
    model_id: gemini-2.0-flash # Required: provider-specific model ID
    default: true             # Optional: set as default model
    api_key: ${API_KEY}       # Optional: API key (uses env var if omitted)
    backend: vertexai         # Optional (Gemini): gemini or vertexai
    project: my-gcp-project   # Optional (Vertex AI): GCP project ID
    location: us-central1     # Optional (Vertex AI): GCP region

  # Ollama local model example
  ollama-llama:
    provider: ollama
    model_id: llama3.2        # Model name in Ollama
    base_url: http://localhost:11434/v1  # Required for Ollama
```

### Ollama Provider

The `ollama` provider uses the official OpenAI Go SDK (`github.com/openai/openai-go/v3`) with a custom base URL to connect to Ollama's OpenAI-compatible API.

Required fields:
- `provider: ollama`
- `model_id`: The model name as shown in `ollama list`
- `base_url`: The Ollama server URL with `/v1` suffix (e.g., `http://localhost:11434/v1`)
