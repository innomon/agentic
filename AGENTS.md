# MedAgent - ADK-Go Project

## Overview

MedAgent is a medical document transcription agent built with Google's ADK-Go framework. It converts PDF and image files of medical documents into FHIR R5 compliant JSON.

## Commands

```bash
# Build the project
go build -o med-agent .

# Run in console mode
./med-agent console

# Run in web UI mode
./med-agent web

# Run as API server
./med-agent api

# Run tests
go test ./...

# Tidy dependencies
go mod tidy
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
│   │   ├── agents.go           # Built-in agent types (llm, sequential, etc.)
│   │   └── tools.go            # Tools registry and built-in tool types
│   ├── config/
│   │   └── config.go           # Config loader with schema-based parsing
│   └── registry/
│       ├── model.go            # Model registry (lazy loading)
│       └── agent.go            # Agent registry (dependency resolution)
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
- Use `full.NewLauncher()` from `google.golang.org/adk/cmd/launcher/full`
- Agent functions should accept `(ctx context.Context, m model.LLM)` and return `(agent.Agent, error)`
- Use `SubAgents` field in `llmagent.Config` for routing to sub-agents
- ADK-Go auto-injects `transfer_to_agent` tool when SubAgents are declared

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
