# MedAgent

A medical document transcription agent built with Google's [ADK-Go](https://github.com/google/adk-go) framework. MedAgent converts medical documents (PDFs and images) into FHIR R5 compliant JSON.

## Features

- **Multi-format Input**: Accepts PDF documents and image files (PNG, JPG, JPEG, TIFF)
- **OCR Capabilities**: Extracts text from scanned documents and images using Gemini's multimodal capabilities
- **Document Classification**: Automatically identifies document types (Prescription, Discharge Summary, Lab Report, Diagnostic Report)
- **FHIR R5 Output**: Generates compliant FHIR resources with proper coding systems (LOINC, SNOMED CT, RxNorm, UCUM)
- **Config-Driven**: All agents, models, and tools defined in YAML configuration
- **Persistent Memory**: Database-backed conversation memory with configurable providers (PostgreSQL, SQLite)
- **JWT Authentication**: Optional RS256 JWT verification middleware for securing API endpoints
- **Role-Based Routing**: Route users to agents based on database-stored roles with admin config override and contextual disambiguation
- **User Profile Database**: GORM-backed user management with JSONB profile/metadata, status tracking, and admin role protection
- **Extensible Tools**: Define custom tools in YAML with Go handlers for agent capabilities
- **WebAssembly Extensions**: Sandboxed WASM tool and agent types via wazero with security policy engine, OCI registry support, and per-invocation isolation

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                MedAgent (Router)                    │
│         Detects input type and routes               │
└─────────────┬───────────────────┬───────────────────┘
              │                   │
    ┌─────────▼─────────┐   ┌─────▼─────────┐
    │ PDFExtractorAgent │   │   OCRAgent    │
    │  (PDF documents)  │   │   (Images)    │
    └─────────┬─────────┘   └───────┬───────┘
              │                     │
    ┌─────────▼─────────┐   ┌───────▼───────┐
    │ PDFTxt2FhirAgent  │   │OCRTxt2FhirAgent│
    │   (Classifier)    │   │  (Classifier)  │
    └─────────┬─────────┘   └───────┬───────┘
              │                     │
    ┌────┬────┼────┬────┐   ┌────┬──┼──┬────┐
    ▼    ▼    ▼    ▼    ▼   ▼    ▼  ▼  ▼    ▼
 ┌─────────────────────────────────────────────┐
 │           Specialist Agents (per branch)    │
 │  Prescription │ DischargeSummary │ LabReport│
 │  DiagnosticImaging │ OtherDocument          │
 └─────────────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────┐
│               FHIR R5 JSON Output               │
│  MedicationRequest │ Composition │ DiagnosticReport │
│                DocumentReference                │
└─────────────────────────────────────────────────┘
```

> **Note:** Each branch (PDF/OCR) has its own set of specialist agents to comply with ADK's single-parent constraint.

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

Interactive terminal session with file attachment support:

```bash
./med-agent console
```

**Attach files using `@/path/to/file` syntax:**

```
MedAgent Console (attach files with @/path/to/file syntax)
Example: Create FHIR from this @./labtest.pdf
Type 'exit' or 'quit' to exit.

User -> Create a FHIR record from this lab report @./document.pdf
[Attached: document.pdf (application/pdf, 125432 bytes)]

Agent -> ...
```

**Examples:**
```bash
# Single file
User -> Extract prescription from this image @./prescription.png

# Multiple files
User -> Compare these two reports @./report1.pdf @./report2.pdf

# With home directory expansion
User -> Process this @~/Documents/labtest.pdf
```

**Supported file types:**
- PDF: `.pdf`
- Images: `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`, `.tiff`
- Text: `.txt`, `.json`, `.csv`, `.xml`, `.html`

### Web UI Mode

Browser-based interface with file upload support:

```bash
./med-agent web
```

Open http://localhost:8080/ui/ in your browser. You can drag-and-drop PDF and image files directly.

Options:
- `--port PORT` - Custom port (default: 8080)

### API Server Mode

REST API for integration with file attachments via base64 encoding:

```bash
./med-agent web
```

#### Sending Files via API

1. **Create a session:**
   ```bash
   SESSION=$(curl -s -X POST "http://localhost:8080/api/apps/MedAgent/users/user/sessions" \
     -H "Content-Type: application/json" -d '{}' | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
   echo "Session: $SESSION"
   ```

2. **Send a PDF file with a prompt:**
   ```bash
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
           {"text": "Create a FHIR record from this lab report"},
           {"inlineData": {"mimeType": "application/pdf", "data": "'"$(base64 -w0 /path/to/document.pdf)"'"}}
         ]
       }
     }'
   ```

3. **Send an image file:**
   ```bash
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
           {"text": "Extract prescription from this image"},
           {"inlineData": {"mimeType": "image/png", "data": "'"$(base64 -w0 /path/to/image.png)"'"}}
         ]
       }
     }'
   ```

#### Supported MIME Types

| Format | MIME Type |
|--------|-----------|
| PDF | `application/pdf` |
| PNG | `image/png` |
| JPEG | `image/jpeg` |
| GIF | `image/gif` |
| WebP | `image/webp` |
| TIFF | `image/tiff` |

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

### OpenAI-Compatible API (Proxy)

MedAgent includes an OpenAI-compatible proxy server that allows integration with tools expecting the OpenAI API format.

#### Build and Run

```bash
cd openai-proxy
go build -o openai-proxy .
./openai-proxy -config config.yaml
```

#### Configuration

Edit `openai-proxy/config.yaml`:

```yaml
proxy:
  listen: ":9080"           # Proxy listen address
  adk:
    endpoint: http://localhost:8080  # ADK server URL
    app_name: MedAgent              # Agent app name
  defaults:
    user_id: openai-proxy-user      # Default user ID
```

#### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/chat/completions` | POST | Chat completions (streaming & non-streaming) |
| `/v1/models` | GET | List available models |
| `/health` | GET | Health check |

#### Example Usage

**Chat completion (streaming):**
```bash
curl -X POST http://localhost:9080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "MedAgent",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'
```

**Chat completion (non-streaming):**
```bash
curl -X POST http://localhost:9080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "MedAgent",
    "messages": [{"role": "user", "content": "What is FHIR?"}],
    "stream": false
  }'
```

**With image attachment (base64 data URL):**
```bash
curl -X POST http://localhost:9080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "MedAgent",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "Extract prescription from this image"},
        {"type": "image_url", "image_url": {"url": "data:image/png;base64,'"$(base64 -w0 image.png)"'"}}
      ]
    }],
    "stream": true
  }'
```

**Use with OpenAI SDK:**
```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:9080/v1",
    api_key="not-required"  # API key not validated
)

response = client.chat.completions.create(
    model="MedAgent",
    messages=[{"role": "user", "content": "Create FHIR from this lab report"}],
    stream=True
)

for chunk in response:
    print(chunk.choices[0].delta.content, end="")
```

## Configuration

Agents, models, tools, and memory are configured in `config/config.yaml`:

```yaml
root_agent: MedAgent  # optional, defaults to "RootAgent"

models:
  gemini-flash:
    provider: gemini
    model_id: gemini-2.0-flash
    default: true

  gemini-pro:
    provider: gemini
    model_id: gemini-2.5-pro-preview-06-05

memory:
  provider: database
  driver: postgres
  dsn: postgres://user:pass@localhost/medagent
  auto_migrate: true

session:
  provider: database
  driver: postgres
  dsn: postgres://user:pass@localhost/medagent
  auto_migrate: true

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

### Root Agent

The `root_agent` top-level field specifies which agent in the `agents` map is the entry point. If omitted, it defaults to `RootAgent`.

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
| `tools` | List of tool names from tools config | No |
| `instruction` | System prompt/instructions | Yes (for `llm`) |
| `max_iterations` | Loop iterations (0 = until escalation) | No (for `loop`) |

#### Built-in Agent Types

| Type | Description |
|------|-------------|
| `llm` | Standard LLM agent (default) |
| `sequential` | Executes sub-agents once in order |
| `parallel` | Executes sub-agents concurrently |
| `loop` | Repeatedly executes sub-agents |
| `routing` | Role-based user routing with disambiguation |
| `wasm` | WebAssembly agent via wazero with sub-agent host functions |

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

#### Routing Agent

The `routing` agent type routes users to sub-agents based on their database roles. It supports admin users from config, anonymous fallback, status/channel checks, and LLM-based disambiguation when multiple roles match.

```yaml
agents:
  RoutingAgent:
    type: routing
    description: Routes users based on roles
    model: gemini-flash
    admin_users: [admin1, admin2]     # admin users (config-only, never set via DB)
    role_routes:
      admin: AdminAgent
      farmer: FarmerAgent
      seller: SellerAgent
      anonymous: PublicInfoAgent      # optional anonymous fallback
    tools: [get_user_profile]
    sub_agents:
      - AdminAgent
      - FarmerAgent
      - SellerAgent
      - PublicInfoAgent
```

| Field | Description | Required |
|-------|-------------|----------|
| `admin_users` | User IDs granted admin role (config-only) | No |
| `role_routes` | Map of role name → sub-agent name | Yes |
| `model` | Model for disambiguation decisions | Yes |
| `tools` | Tools available to the routing agent | No |

Routing logic:
1. Retrieves user profile via `get_user_profile` tool
2. Admin users (from config) route directly to the admin agent
3. Users with status `Pending` or `Suspended` are denied
4. Single matching role → route to mapped agent
5. Multiple matching roles → LLM disambiguates based on query context
6. Unknown users → route to `anonymous` agent or reject

See `config/routing-sample.yaml` for a complete working example.

### Custom Agent Types

Register custom agent types with their own config schema using Go generics:

```go
package myagent

import (
    "context"
    "github.com/innomon/med-agent/internal/registry"
    "google.golang.org/adk/agent"
)

// Define your config struct with custom fields
type MyAgentConfig struct {
    registry.AgentBase `yaml:",inline"`
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
    registry.RegisterAgentType("myType", createMyAgent)
}

func createMyAgent(ctx context.Context, name string, cfg *MyAgentConfig, models registry.ModelRegistry, tools registry.ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
    // cfg is fully typed - access cfg.CustomField, cfg.Threshold directly
    // tools.GetMultiple(ctx, names) returns ([]tool.Tool, error)
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

### Tools Registry

Tools can be defined in YAML and reused across agents. Register tool handlers in Go, then reference tools by name in agent configs.

#### Defining Tools in Config

```yaml
tools:
  get_weather:
    description: Get current weather for a location
    parameters:
      location: {type: string, required: true, description: City name}
      unit: {type: string, description: celsius or fahrenheit}

  search_documents:
    description: Search medical documents
    parameters:
      query: {type: string, required: true}
      limit: {type: integer}

agents:
  WeatherAgent:
    model: gemini-flash
    tools: [get_weather]  # reference tools by name
    instruction: |
      You help users check weather...
```

#### Registering Tool Handlers

```go
package main

import (
    "context"
    "github.com/innomon/med-agent/internal/registry"
)

func init() {
    // Register handler for the "get_weather" tool
    registry.RegisterToolHandler("get_weather", func(ctx context.Context, args map[string]any) (any, error) {
        location := args["location"].(string)
        // Fetch weather data...
        return map[string]any{
            "temperature": 22,
            "condition":   "sunny",
        }, nil
    })
}
```

#### Tool Configuration Fields

| Field | Description | Required |
|-------|-------------|----------|
| `type` | Tool type (default: `builtin`) | No |
| `description` | Tool description for the LLM | Yes |
| `parameters` | Map of parameter definitions | No |

#### Parameter Fields

| Field | Description |
|-------|-------------|
| `type` | `string`, `number`, `integer`, `boolean`, `array`, `object` |
| `description` | Parameter description |
| `required` | Whether the parameter is required |

#### Custom Tool Types

Register custom tool types with their own config schema:

```go
import (
    "context"
    "github.com/innomon/med-agent/internal/registry"
    "google.golang.org/adk/tool"
    "google.golang.org/adk/tool/functiontool"
)

type APIToolConfig struct {
    registry.ToolBase `yaml:",inline"`
    Endpoint string `yaml:"endpoint"`
    Method   string `yaml:"method"`
}

func init() {
    registry.RegisterToolType("api", func(ctx context.Context, name string, cfg *APIToolConfig) (tool.Tool, error) {
        return functiontool.New(functiontool.Config{
            Name:        name,
            Description: cfg.Description,
        }, func(tctx tool.Context, args map[string]any) (any, error) {
            // Make API call to cfg.Endpoint...
            return result, nil
        })
    })
}
```

Then use in config:

```yaml
tools:
  fetch_patient:
    type: api
    description: Fetch patient data from EHR
    endpoint: https://ehr.example.com/api/patients
    method: GET
```

#### UserDB Tool Type

The `userdb` tool type provides GORM-backed user profile CRUD operations. All tools sharing the same DSN use a singleton database connection.

```yaml
tools:
  get_user_profile:
    type: userdb
    op: get_profile                # Operation: get_profile, create_user, update_status, update_roles, update_channels, delete_user
    description: Retrieve user profile
    parameters:
      user_id: {type: string, required: true}
    db:
      driver: postgres             # postgres or sqlite
      dsn: postgres://user:pass@localhost/medagent
      auto_migrate: true
    admin_users: [admin1, admin2]  # Users with config-level admin role
```

| Field | Description | Required |
|-------|-------------|----------|
| `op` | Operation to perform | Yes |
| `db.driver` | Database driver (`postgres`, `sqlite`) | Yes |
| `db.dsn` | Database connection string | Yes |
| `db.auto_migrate` | Auto-create/update schema | No |
| `admin_users` | Admin user IDs (merged into role results) | No |

The user profile schema stores:
- **status**: `Active`, `Pending`, or `Suspended`
- **profile** (JSONB): `user_id`, `roles[]`, `channels[]`
- **metadata** (JSONB): `update_timestamp`, `updated_by`, `channel`

Admin role protection: the `update_roles` operation rejects any attempt to set the `admin` role; admin access is exclusively controlled via the `admin_users` config list.

#### Wasm Tool Type

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

agents:
  MyAgent:
    model: gemini-flash
    tools: [my_wasm_tool]
```

| Field | Description | Required |
|-------|-------------|----------|
| `module_path` | Path to local `.wasm` file | Yes (or `oci_ref`) |
| `oci_ref` | OCI registry reference | Yes (or `module_path`) |
| `cache_dir` | Directory for OCI blob cache | No |
| `security.allowed_paths` | Absolute paths mounted read-only into guest | No |
| `security.allowed_domains` | Domains allowed for `http_fetch` host function | No |
| `security.memory_max_pages` | Max wasm memory pages (default: 256 = 16MB) | No |

**Security model:**
- Filesystem access is deny-by-default; only paths in `allowed_paths` are mounted read-only via wazero's `FSConfig`
- Network access is guarded by host function wrappers that check URLs against `allowed_domains` before making HTTP requests
- Memory is capped at `memory_max_pages` (default 256 = 16MB) via `WithMemoryLimitPages`
- Private/loopback addresses are always blocked

**Wasm module ABI:** Tool modules must export `alloc(size) -> ptr`, `run_tool(ptr, len) -> i64` (packed `out_ptr << 32 | out_len`), and optionally `free(ptr, size)`. Host functions `env.log_msg` and `env.http_fetch` are available.

#### Wasm Agent Type

The `wasm` agent type runs a WebAssembly module as an ADK agent with access to sub-agents via host functions:

```yaml
agents:
  MyWasmAgent:
    type: wasm
    description: "Run a WebAssembly module as an agent"
    module_path: ./plugins/my_agent.wasm
    sub_agents:
      - SubAgent1
      - SubAgent2
```

The module must export an `execute() -> i32` function. Host functions available: `env.subagent_count`, `env.subagent_name`, `env.run_subagent`, `env.log_msg`.

### Custom Model Providers

Register custom model providers with their own config schema:

```go
package myprovider

import (
    "context"
    "github.com/innomon/med-agent/internal/registry"
    "google.golang.org/adk/model"
)

type MyProviderConfig struct {
    registry.ModelBase `yaml:",inline"`
    Endpoint string `yaml:"endpoint"`
    Timeout  int    `yaml:"timeout"`
}

func init() {
    registry.RegisterModelProvider("myprovider", createMyModel)
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

### Session Configuration

Session stores active conversation state. If omitted or set to `provider: inmemory`, in-memory storage is used.

```yaml
# Database-backed sessions
session:
  provider: database
  driver: postgres
  dsn: postgres://user:pass@localhost/medagent
  auto_migrate: true

# Vertex AI sessions
session:
  provider: vertexai
  project: my-gcp-project
  location: us-central1
  reasoning_engine: projects/my-project/locations/us-central1/reasoningEngines/12345
```

#### Session Configuration Fields

| Field | Description | Required |
|-------|-------------|----------|
| `provider` | `inmemory` (default), `database`, or `vertexai` | No |
| `driver` | Database driver (`postgres`, `sqlite`) | Yes (for `database`) |
| `dsn` | Database connection string | Yes (for `database`) |
| `auto_migrate` | Auto-create/update schema on startup | No |
| `project` | GCP project ID | Yes (for `vertexai`) |
| `location` | GCP region | Yes (for `vertexai`) |
| `reasoning_engine` | Reasoning Engine resource name | Yes (for `vertexai`) |

### Memory Configuration

Memory stores agent conversation history. If omitted or set to `provider: inmemory`, in-memory storage is used. Set `provider: database` for persistent storage via GORM.

```yaml
# PostgreSQL
memory:
  provider: database
  driver: postgres
  dsn: postgres://user:pass@localhost/medagent
  auto_migrate: true

# SQLite (for development)
memory:
  provider: database
  driver: sqlite
  dsn: file:memory.db
  auto_migrate: true
```

#### Memory Configuration Fields

| Field | Description | Required |
|-------|-------------|----------|
| `provider` | `inmemory` (default) or `database` | No |
| `driver` | Database driver (`postgres`, `sqlite`) | Yes (for `database`) |
| `dsn` | Database connection string | Yes (for `database`) |
| `auto_migrate` | Auto-create/update schema on startup | No |

### Authentication

Optional RS256 JWT authentication for API endpoints. When configured, all web/API requests require a valid `Authorization: Bearer <token>` header.

```yaml
auth:
  jwt:
    public_key_path: secrets/jwt_public.pem
    issuer: whatsadk-gateway
    audience: adk-agent
```

#### Key Setup

```bash
openssl genrsa -out secrets/jwt_private.pem 2048
openssl rsa -in secrets/jwt_private.pem -pubout -out secrets/jwt_public.pem
```

The gateway signs tokens with the private key. The ADK server verifies using the public key.

#### Auth Configuration Fields

| Field | Description | Required |
|-------|-------------|----------|
| `public_key_path` | Path to RSA public key PEM file | Yes |
| `issuer` | Expected JWT issuer claim | No |
| `audience` | Expected JWT audience claim | No |

#### JWT Claims

| Claim | Type | Description |
|-------|------|-------------|
| `user_id` | `string` | Sender identifier (e.g., WhatsApp phone number) |
| `channel` | `string` | Channel identifier (e.g., `"whatsapp"`) |
| `iss` | `string` | Token issuer |
| `aud` | `string` | Token audience |
| `iat` | `number` | Issued at (Unix timestamp) |
| `exp` | `number` | Expiry (Unix timestamp) |

#### Development Bypass

Set `BYPASS_AUTH=true` to skip JWT verification for localhost requests. When active, requests from `localhost`, `127.0.0.1`, or `::1` are automatically authenticated with default claims (`user_id=local-dev`, `channel=local`).

```bash
BYPASS_AUTH=true ./med-agent web
```

> **Warning**: Never enable `BYPASS_AUTH` in production. The bypass only applies to localhost origins.

#### Testing with curl

```bash
# Authenticated request
curl -X POST http://localhost:8080/api/run_sse \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{ ... }'

# Missing auth — returns 401
curl -X POST http://localhost:8080/api/run_sse \
  -H "Content-Type: application/json" \
  -d '{ ... }'

# Local dev with auth bypass
BYPASS_AUTH=true ./med-agent web
curl -X POST http://localhost:8080/api/run_sse \
  -H "Content-Type: application/json" \
  -d '{ ... }'  # no Authorization header needed
```

## Project Structure

```
med-agent/
├── main.go                      # Entry point
├── config/
│   ├── config.yaml             # Agent, model, and tool configuration
│   └── routing-sample.yaml     # Sample routing agent configuration
├── internal/
│   ├── compreg/
│   │   └── compreg.go          # Global component register (shared map)
│   ├── config/
│   │   └── config.go           # Config file loader (thin wrapper)
│   ├── console/
│   │   └── console.go          # Custom console with @file attachment syntax
│   ├── auth/
│   │   └── verifier.go         # JWT RS256 token verification and middleware
│   ├── memory/
│   │   └── mem2db.go           # Database-backed memory service (GORM)
│   ├── routing/                # Role-based routing agent
│   │   ├── routing.go          # Routing agent type (role→agent mapping)
│   │   └── tools.go            # UserDB tool type (GORM user profiles)
│   ├── userdb/
│   │   └── userdb.go           # User profile database (GORM, JSONB)
│   ├── wasm/                   # WASM extension (Wassette)
│   │   ├── wasm.go             # Wasm agent type (wazero runtime, sub-agent host fns)
│   │   ├── tool.go             # Wasm tool type (registry integration, per-invocation isolation)
│   │   ├── policy.go           # Security policy engine (FS sandbox, domain allow-list, memory limits)
│   │   ├── abi.go              # Component bridge ABI (alloc/run_tool/free linear memory protocol)
│   │   ├── cache.go            # Compilation cache (wazero disk-backed) + bytecode cache
│   │   ├── oci.go              # OCI registry puller (regclient, digest-based disk cache)
│   │   └── host_net.go         # Guarded HTTP host functions (domain-checked http_fetch)
│   └── registry/               # Unified registry (config, components, instances)
│       ├── registry.go         # Instance cache with generic Get[T]
│       ├── config.go           # Config types and YAML parsing
│       ├── compreg.go          # Component type registration and factories
│       ├── launcher.go         # Launcher config builder (session, memory)
│       ├── models.go           # Built-in model providers (Gemini, OpenAI)
│       ├── ollama.go           # Ollama provider (official OpenAI SDK)
│       ├── agents.go           # Built-in agent types (llm, sequential, etc.)
│       └── tools.go            # Tool type registration and built-in tools
├── openai-proxy/               # OpenAI-compatible API proxy
│   ├── main.go                 # Proxy server (Ollama-style design)
│   └── config.yaml             # Proxy configuration
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

2. **PDFExtractorAgent** (PDF Branch)
   - Extracts text from PDF documents
   - Handles multi-page documents
   - Transfers to PDFTxt2FhirAgent

3. **OCRAgent** (Image Branch)
   - Extracts text from images
   - Handles handwritten prescriptions
   - Transfers to OCRTxt2FhirAgent

4. **PDFTxt2FhirAgent / OCRTxt2FhirAgent** (Classifiers)
   - Analyzes extracted text
   - Classifies document type
   - Routes to branch-specific specialist agent

5. **Specialist Agents** (Separate sets for PDF and OCR branches)
   - **PDFPrescriptionAgent / OCRPrescriptionAgent** → MedicationRequest
   - **PDFDischargeSummaryAgent / OCRDischargeSummaryAgent** → Composition
   - **PDFLabReportAgent / OCRLabReportAgent** → DiagnosticReport (LAB)
   - **PDFDiagnosticImagingAgent / OCRDiagnosticImagingAgent** → DiagnosticReport (RAD)
   - **PDFOtherDocumentAgent / OCROtherDocumentAgent** → DocumentReference

> **Why separate branches?** ADK-Go requires each agent to have only one parent. Since both PDFExtractor and OCRAgent need to route to the same classifier/specialist flow, we duplicate the downstream agents for each branch.

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
