// Package main implements the Agentic framework — a config-driven multi-agent
// system built on Google's ADK-Go (google.golang.org/adk).
//
// Agentic lets you define agents, models, tools, and workflows entirely in YAML
// configuration files and extend the system with WebAssembly plugins at runtime,
// without recompilation.
//
// # Quick Start
//
// Set an API key, build, and run:
//
//	export GOOGLE_API_KEY="your-key"
//	go build -o agentic .
//	./agentic -console                            # interactive terminal
//	./agentic -webui -a2a                          # web UI at :8080
//	./agentic -webui -host=192.168.1.10            # LAN access
//	./agentic -console examples/farmer/config.yaml # custom config
//
// # Command Line Flags
//
//   - -console          — Interactive terminal mode with @file attachments.
//   - -webui            — Browser-based user interface.
//   - -a2a              — REST API (A2A) sublauncher.
//   - -openclaw         — OpenClaw WebSocket gateway sublauncher.
//   - -host <string>    — Host to use for API and WebUI address (default: localhost).
//   - -port <int>       — Port to listen on (default: 8080).
//
// # Architecture Overview
//
// The entry point (main.go) loads a YAML config, creates a [Registry], builds a
// [launcher.Config], and hands off to ADK's universal launcher which supports
// console, web, and API modes.
//
//	┌────────────┐   YAML  ┌──────────┐  agents/models/tools    ┌──────────┐
//	│  config/   │ ──────▶ │ Registry │ ─────────────────────▶  │ ADK      │
//	│  *.yaml    │         └──────────┘                         │ Launcher │
//	└────────────┘              │                               └──────────┘
//	                            ▼
//	                    ┌───────────────┐
//	                    │  Component    │
//	                    │  Registry     │
//	                    │  (compreg)    │
//	                    └───────────────┘
//
// # Internal Packages
//
// The framework is organised into the following internal packages:
//
//   - [internal/registry]   — Unified registry: config parsing, component type
//     registration, instance caching, and launcher config building.
//   - [internal/config]     — Thin wrapper that loads YAML config files.
//   - [internal/compreg]    — Global concurrent-safe component map (Set / Lookup).
//   - [internal/console]    — Custom ADK console launcher with @file attachment
//     syntax and session export.
//   - [internal/auth]       — RS256 JWT token verification and HTTP middleware.
//   - [internal/memory]     — Database-backed memory.Service (GORM, PostgreSQL/SQLite).
//   - [internal/userdb]     — GORM-backed user profile database with JSONB columns.
//   - [internal/routing]    — Role-based routing agent type and UserDB tool type.
//   - [internal/wasm]       — WebAssembly agent and tool types via wazero, with
//     security policies, OCI registry support, and per-invocation isolation.
//   - [internal/gnogent]    — Deterministic GnoVM agent (no LLM) with Postgres-backed
//     session/memory, GnoVM wrapper, JWT auth, and health diagnostics.
//
// # Agent Types
//
// Agents are declared in config YAML under the "agents" key. The "type" field
// selects the agent implementation. Built-in types:
//
//   - llm (default)             — Standard LLM agent via llmagent.New().
//   - sequential                — Executes sub-agents once in declared order.
//   - parallel                  — Executes sub-agents concurrently.
//   - loop                      — Repeatedly executes sub-agents (max_iterations).
//   - routing                   — Role-based routing with user profile lookup,
//     admin config override, and contextual disambiguation.
//   - wasm                      — WebAssembly agent via wazero with sub-agent
//     host functions.
//   - gnogent     — Deterministic GnoVM agent (no LLM, state
//     persisted to Postgres).
//   - gnogent                   — LLM-backed agent with GnoVM state management,
//     freeze/thaw/brain_query tools.
//
// Custom agent types are registered with [registry.RegisterAgentType]:
//
//	type MyConfig struct {
//	    registry.AgentBase `yaml:",inline"`
//	    Endpoint string    `yaml:"endpoint"`
//	}
//	func init() {
//	    registry.RegisterAgentType("custom", func(ctx context.Context, name string,
//	        cfg *MyConfig, models registry.ModelRegistry, tools registry.ToolRegistry,
//	        sub []agent.Agent) (agent.Agent, error) {
//	        return myAgent, nil
//	    })
//	}
//
// # Model Providers
//
// Models are declared under the "models" key. Built-in providers:
//
//   - gemini   — Google Gemini / Vertex AI.
//   - openai   — OpenAI API (GPT-4o, etc.).
//   - ollama   — Local Ollama via the official OpenAI Go SDK with custom base URL.
//
// Custom providers are registered with [registry.RegisterModelProvider].
//
// # Tool System
//
// Tools are declared under the "tools" key. Built-in tool types:
//
//   - (default / builtin)  — Go handler registered with [registry.RegisterToolHandler].
//   - gemini               — Gemini built-in tools (e.g. google_search).
//   - userdb               — GORM-backed user profile CRUD operations.
//   - wasm                 — Sandboxed WebAssembly tool via wazero with security
//     policies and OCI registry support.
//
// Custom tool types are registered with [registry.RegisterToolType].
//
// # Session & Memory Providers
//
// Session (active conversation state) and memory (conversation history) services
// support pluggable backends via [registry.RegisterProvider]:
//
//   - inmemory (default)   — ADK's built-in in-memory service.
//   - database             — GORM-backed (PostgreSQL, SQLite).
//   - gnogent              — Postgres-backed with GnoVM-specific tables.
//   - vertexai             — Google Vertex AI Reasoning Engine (session only).
//
// # Authentication
//
// Optional RS256 JWT middleware protects API endpoints. Configure under "auth.jwt"
// in YAML. Set BYPASS_AUTH=true for local development. Access claims in callbacks
// via [auth.ClaimsFromContext].
//
// # WebAssembly Extensions
//
// WASM tools and agents run in sandboxed wazero runtimes with configurable
// security policies:
//
//   - Filesystem sandbox: mount read-only paths via allowed_paths.
//   - Network guard: domain allow-list via allowed_domains (supports wildcards).
//   - Memory limit: memory_max_pages (default 256 = 16 MB).
//   - OCI support: pull modules from OCI registries (e.g. ghcr.io).
//   - Compilation cache: disk-backed wazero cache for fast restarts.
//
// WASM tool ABI: export alloc(size) → ptr, run_tool(ptr, len) → i64, optionally
// free(ptr, size). Host functions: env.log_msg, env.http_fetch.
//
// WASM agent ABI: export execute() → i32. Host functions: env.subagent_count,
// env.subagent_name, env.run_subagent, env.log_msg.
//
// # OpenAI-Compatible Proxy
//
// The openai-proxy/ directory contains a standalone HTTP server that translates
// OpenAI chat completion requests (streaming and non-streaming) into ADK SSE
// calls. It exposes /v1/chat/completions and /v1/models endpoints, allowing any
// OpenAI SDK client to use Agentic as a drop-in backend.
//
// # Console
//
// The custom console launcher supports:
//
//   - File attachments via @/path/to/file syntax (auto-detects MIME type).
//   - Session export to JSON via /export command.
//   - /help and /exit commands.
//
// # Environment Variables
//
//   - GOOGLE_API_KEY  — API key for Gemini models.
//   - OPENAI_API_KEY  — API key for OpenAI models.
//   - BYPASS_AUTH     — Set to "true" to skip JWT verification (dev, localhost only).
//
// # Configuration Reference
//
// All configuration is in YAML. The default config path is config/config.yaml.
// A minimal configuration requires a model and a root agent:
//
//	root_agent: RootAgent
//
//	models:
//	  gemini-flash:
//	    provider: gemini
//	    model_id: gemini-2.0-flash
//	    default: true
//
//	agents:
//	  RootAgent:
//	    description: General-purpose assistant
//	    model: gemini-flash
//	    instruction: You are a helpful assistant.
//
// Top-level config sections: root_agent, models, agents, tools, session, memory, auth.
package main
