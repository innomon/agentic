# MedAgent

A medical document transcription agent built with Google's [ADK-Go](https://github.com/google/adk-go) framework. MedAgent converts medical documents (PDFs and images) into FHIR R5 compliant JSON.

## Features

- **Multi-format Input**: Accepts PDF documents and image files (PNG, JPG, JPEG, TIFF)
- **OCR Capabilities**: Extracts text from scanned documents and images using Gemini's multimodal capabilities
- **Document Classification**: Automatically identifies document types (Prescription, Discharge Summary, Lab Report, Diagnostic Report)
- **FHIR R5 Output**: Generates compliant FHIR resources with proper coding systems (LOINC, SNOMED CT, RxNorm, UCUM)
- **Config-Driven**: All agents and models defined in YAML configuration

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                MedAgent (Router)                    │
│         Detects input type and routes               │
└─────────────┬───────────────┬───────────────────────┘
              │               │
    ┌─────────▼─────┐   ┌─────▼─────┐
    │ PDF Extractor │   │ OCR Agent │
    │    Agent      │   │           │
    └───────┬───────┘   └─────┬─────┘
            │                 │
            └────────┬────────┘
                     │
           ┌─────────▼─────────┐
           │   Txt2Fhir Agent  │
           │  (Classifier)     │
           └─────────┬─────────┘
                     │
    ┌────────┬───────┼───────┬────────┐
    ▼        ▼       ▼       ▼        ▼
┌────────┐┌─────────┐┌────────┐┌──────────┐┌────────┐
│Prescrip││Discharge││  Lab   ││Diagnost  ││ Others │
│  tion  ││ Summary ││ Report ││ic Imag.  ││        │
└────────┘└─────────┘└────────┘└──────────┘└────────┘
    │        │          │          │          │
    ▼        ▼          ▼          ▼          ▼
┌────────────────────────────────────────────────┐
│              FHIR R5 JSON Output               │
│ MedicationRequest│Composition│DiagnosticReport │
│               DocumentReference                │
└────────────────────────────────────────────────┘
```

## FHIR R5 Resources Generated

| Document Type | FHIR Resource | Coding Systems |
|---------------|---------------|----------------|
| Prescription | MedicationRequest | RxNorm, SNOMED CT, UCUM |
| Discharge Summary | Composition | LOINC (section codes) |
| Lab Report | DiagnosticReport | LOINC, UCUM |
| Diagnostic/Imaging | DiagnosticReport | LOINC, SNOMED CT |
| Others | DocumentReference | LOINC |

## Prerequisites

- Go 1.24+
- Google API Key for Gemini access, OpenAI API Key for OpenAI models, or Ollama for local models

## Setup

1. Set your Google API key:
   ```bash
   export GOOGLE_API_KEY="your-api-key"
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Build the agent:
   ```bash
   go build -o med-agent .
   ```

## Usage

MedAgent supports multiple run modes via the ADK launcher:

### Console Mode (Interactive)

Interactive terminal session for testing:

```bash
./med-agent console
```

### Web UI Mode

Browser-based interface with file upload support:

```bash
./med-agent web
```

Open http://localhost:8080 in your browser.

Options:
- `--port PORT` - Custom port (default: 8080)

### API Server Mode

REST API for integration:

```bash
./med-agent api
```

Options:
- `--port PORT` - Custom port (default: 8080)

### Command Line Options

```bash
./med-agent [mode] [options]

Modes:
  console   Interactive terminal mode
  web       Web UI mode
  api       REST API server mode

Common Options:
  --help    Show help message
```

## Configuration

Agents and models are configured in `config/config.yaml`:

```yaml
models:
  gemini-flash:
    provider: gemini
    model_id: gemini-2.0-flash
    default: true

  gemini-pro:
    provider: gemini
    model_id: gemini-2.5-pro-preview-06-05

agents:
  MedAgent:
    description: Root medical agent
    model: gemini-flash
    sub_agents:
      - PDFExtractorAgent
      - OCRAgent
    instruction: |
      Your system prompt here...
```

### Adding New Models

Add entries under `models:` with provider and model ID:

```yaml
models:
  my-model:
    provider: gemini
    model_id: gemini-2.0-flash-lite
    api_key: ${GOOGLE_API_KEY}  # optional, uses env var if omitted
    
  my-openai-model:
    provider: openai
    model_id: gpt-4o
    api_key: ${OPENAI_API_KEY}
    
  my-vertexai-model:
    provider: gemini
    model_id: gemini-2.0-flash
    backend: vertexai
    project: my-gcp-project
    location: us-central1

  my-ollama-model:
    provider: ollama
    model_id: llama3.2
    base_url: http://localhost:11434/v1
```

#### Model Configuration Fields

| Field | Description | Required |
|-------|-------------|----------|
| `provider` | Model provider (`gemini`, `openai`, or `ollama`) | Yes |
| `model_id` | Provider-specific model identifier | Yes |
| `default` | Set to `true` for default model | No |
| `api_key` | API key for authentication | No (uses env var) |
| `backend` | For Gemini: `gemini` (default) or `vertexai` | No |
| `project` | GCP project ID (required for Vertex AI) | No |
| `location` | GCP region (required for Vertex AI) | No |
| `base_url` | Ollama server URL (required for Ollama) | No |

#### Ollama Provider

The `ollama` provider uses the official OpenAI Go SDK with a custom base URL to connect to Ollama's OpenAI-compatible API. This allows running models locally without an API key.

**Required fields for Ollama:**
- `provider: ollama`
- `model_id`: The model name as shown in `ollama list` (e.g., `llama3.2`, `mistral`, `codellama`)
- `base_url`: The Ollama server URL with `/v1` suffix (e.g., `http://localhost:11434/v1`)

**Example:**
```yaml
models:
  local-llama:
    provider: ollama
    model_id: llama3.2
    base_url: http://localhost:11434/v1
    default: true
```

### Adding New Agents

Add entries under `agents:` with model reference and instruction:

```yaml
agents:
  MyAgent:
    type: llm            # optional, defaults to "llm"
    description: Agent description
    model: gemini-flash
    sub_agents: []       # optional
    instruction: |
      System prompt...
```

#### Agent Configuration Fields

| Field | Description | Required |
|-------|-------------|----------|
| `type` | Agent type (default: `llm`) | No |
| `description` | Agent description for routing | Yes |
| `model` | Model name from models config | Yes (for `llm`) |
| `sub_agents` | List of sub-agent names | No |
| `instruction` | System prompt/instructions | Yes (for `llm`) |
| `max_iterations` | Loop iterations (0 = until escalation) | No (for `loop`) |

#### Built-in Agent Types

| Type | Description |
|------|-------------|
| `llm` | Standard LLM agent (default) |
| `sequential` | Executes sub-agents once in order |
| `parallel` | Executes sub-agents concurrently |
| `loop` | Repeatedly executes sub-agents |

#### Workflow Agent Examples

```yaml
agents:
  # Sequential workflow - runs agents in strict order
  ProcessingPipeline:
    type: sequential
    description: Process documents in stages
    sub_agents:
      - ExtractAgent
      - TransformAgent
      - ValidateAgent

  # Parallel workflow - runs agents concurrently
  MultiAnalysis:
    type: parallel
    description: Run multiple analyses simultaneously
    sub_agents:
      - SentimentAgent
      - EntityAgent
      - SummaryAgent

  # Loop workflow - iterative refinement
  RefineLoop:
    type: loop
    description: Iteratively refine output
    max_iterations: 3  # 0 = run until escalation
    sub_agents:
      - DraftAgent
      - ReviewAgent
```

### Custom Agent Types

Register custom agent types with their own config schema using Go generics:

```go
package myagent

import (
    "context"
    "github.com/innomon/med-agent/internal/componentreg"
    "google.golang.org/adk/agent"
)

// Define your config struct with custom fields
type MyAgentConfig struct {
    componentreg.AgentBase `yaml:",inline"`
    CustomField string `yaml:"custom_field"`
    Threshold   int    `yaml:"threshold"`
}

// Optional: implement Validate() for custom validation
func (c *MyAgentConfig) Validate() error {
    if c.Threshold < 0 {
        return fmt.Errorf("threshold must be non-negative")
    }
    return nil
}

func init() {
    componentreg.RegisterAgentType("myType", createMyAgent)
}

func createMyAgent(ctx context.Context, name string, cfg *MyAgentConfig, models componentreg.ModelRegistry, sub []agent.Agent) (agent.Agent, error) {
    // cfg is fully typed - access cfg.CustomField, cfg.Threshold directly
    return myCustomAgent, nil
}
```

Then reference in config:

```yaml
agents:
  MyCustomAgent:
    type: myType
    description: Custom agent
    custom_field: some-value
    threshold: 10
```

### Custom Model Providers

Register custom model providers with their own config schema:

```go
package myprovider

import (
    "context"
    "github.com/innomon/med-agent/internal/componentreg"
    "google.golang.org/adk/model"
)

type MyProviderConfig struct {
    componentreg.ModelBase `yaml:",inline"`
    Endpoint string `yaml:"endpoint"`
    Timeout  int    `yaml:"timeout"`
}

func init() {
    componentreg.RegisterModelProvider("myprovider", createMyModel)
}

func createMyModel(ctx context.Context, cfg *MyProviderConfig) (model.LLM, error) {
    // cfg is fully typed - access cfg.Endpoint, cfg.Timeout directly
    return myModel, nil
}
```

Then reference in config:

```yaml
models:
  my-custom-model:
    provider: myprovider
    model_id: custom-v1
    endpoint: https://api.example.com
    timeout: 30
```

## Project Structure

```
med-agent/
├── main.go                      # Entry point with launcher
├── config/
│   └── config.yaml             # Agent and model configuration
├── internal/
│   ├── componentreg/           # Generic component registry (Go generics)
│   │   ├── registry.go         # Core registration with generics
│   │   ├── models.go           # Built-in model providers (Gemini, OpenAI)
│   │   ├── ollama.go           # Ollama provider (official OpenAI SDK)
│   │   └── agents.go           # Built-in agent types (llm, sequential, etc.)
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
├── AGENTS.md
└── README.md
```

## Agent Hierarchy

1. **MedAgent** (Router)
   - Detects input type (PDF vs Image)
   - Routes to appropriate extraction agent

2. **PDFExtractorAgent**
   - Extracts text from PDF documents
   - Handles multi-page documents
   - Transfers to Txt2FhirAgent

3. **OCRAgent**
   - Extracts text from images
   - Handles handwritten prescriptions
   - Transfers to Txt2FhirAgent

4. **Txt2FhirAgent** (Classifier)
   - Analyzes extracted text
   - Classifies document type
   - Routes to specialist agent

5. **Specialist Agents**
   - **PrescriptionAgent** → MedicationRequest
   - **DischargeSummaryAgent** → Composition
   - **LabReportAgent** → DiagnosticReport (LAB)
   - **DiagnosticImagingAgent** → DiagnosticReport (RAD)
   - **OtherDocumentAgent** → DocumentReference

## Example Interaction

```
> I have a prescription image. [attach image]

MedAgent: Routing to OCR Agent for text extraction...
OCR Agent: Extracted prescription text...
Txt2Fhir Agent: Detected Prescription document, routing to Prescription Agent...
Prescription Agent: Generated FHIR MedicationRequest:

{
  "resourceType": "MedicationRequest",
  "status": "active",
  "intent": "order",
  "medication": {
    "concept": {
      "coding": [{
        "system": "http://www.nlm.nih.gov/research/umls/rxnorm",
        "code": "860975",
        "display": "Metformin 500 MG Oral Tablet"
      }]
    }
  },
  ...
}
```

## Disclaimer

This agent is for demonstration and development purposes. Medical document processing should be validated by healthcare professionals before clinical use. Always ensure compliance with HIPAA and other healthcare data regulations.
