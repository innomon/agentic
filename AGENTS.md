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

## FHIR Coding Systems

- **RxNorm**: Medications (`http://www.nlm.nih.gov/research/umls/rxnorm`)
- **LOINC**: Lab tests, document types (`http://loinc.org`)
- **SNOMED CT**: Clinical findings, body sites (`http://snomed.info/sct`)
- **UCUM**: Units of measure (`http://unitsofmeasure.org`)

## Environment Variables

- `GOOGLE_API_KEY` - Required for Gemini model access
