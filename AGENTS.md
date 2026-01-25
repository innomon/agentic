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
│   ├── config/
│   │   └── config.go           # Config types and loader
│   ├── registry/
│   │   ├── model.go            # Model registry
│   │   ├── regmod.go           # Model creators (Gemini, OpenAI)
│   │   └── agent.go            # Agent registry

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

## Custom Agent Types

The agent registry supports custom agent types via the factory pattern. Register custom types before agent creation:

```go
import "github.com/innomon/med-agent/internal/registry"

func init() {
    registry.RegisterAgentType("myType", func(ctx context.Context, cfg *config.AgentConfig, models *registry.ModelRegistry, subAgents []agent.Agent) (agent.Agent, error) {
        // Custom agent creation logic
        return myCustomAgent, nil
    })
}
```

Built-in types:
- `llm` (default) - Standard LLM agent via `llmagent.New()`

Specify type in config:
```yaml
agents:
  MyAgent:
    type: myType  # omit for default "llm"
    description: "..."
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
    provider: gemini          # Required: gemini or openai
    model_id: gemini-2.0-flash # Required: provider-specific model ID
    default: true             # Optional: set as default model
    api_key: ${API_KEY}       # Optional: API key (uses env var if omitted)
    backend: vertexai         # Optional (Gemini): gemini or vertexai
    project: my-gcp-project   # Optional (Vertex AI): GCP project ID
    location: us-central1     # Optional (Vertex AI): GCP region
```
