# Agentic Documentation Index

Welcome to the Agentic documentation. This page provides a logical overview and navigation for the system's specifications, implementation plans, and architectural guides.

## 🚀 Core Framework & Extensions
- **[Extending Agentic](extend_agentic.md)**: A technical guide for adding custom tools and extending the framework's capabilities.
- **[Tool Calling & Built-in Tools](tools.md)**: Documentation on ADK tool calling and built-in tools like `fs_read` and `fhir_get_schema`.
- **[Skill-Based Agent System](skills-implementation-plan.md)**: Implementation plan for building a production-grade, skill-aware supervisor and worker system.
- **[Router Auth Agent](router-auth-agent.md)**: Specification for role-based routing of user queries to specialized sub-agents.

## 🧠 Local ML Inference (`pkg/ml`)
- **[ML Provider Specifications](ML_SPECS.md)**: Overview of the pure-Go GGUF inference engine for zero-network, local LLM execution.
- **[ML Package Architecture](ml_arch.md)**: A deep dive into the internal design, performance optimizations, and supported architectures (LLaMA, Granite).
- **[ML Implementation Plan](ML_IMPLEMENTATION_PLAN.md)**: Detailed roadmap for building the embedded model provider from scaffolding to integration.

## ⛓️ GnoVM & Stateful Agents (Gnogent)
- **[Deterministic Agents with GnoVM](agentic-gno.md)**: How to combine GnoVM, ADK, and Postgres for deterministic "inner monologue" and persistent memory.
- **[GnoVM Native Packages](gnovm-native-packages.md)**: Guide to injecting host-provided Go functionality into the GnoVM sandbox.
- **[Gnogent Overview](gnogent/README.md)**: The flagship framework for secure, stateful AI agents using a "Freeze/Thaw" architecture.
- **[Gnogent Project Map](gnogent/PROJECT_MAP.md)**: Visualizing the "Life of a Message" and end-to-end data flow in Gnogent.
- **[Gno VM Redesign Notes](gnogent/ab-gno-notes.md)**: Status and architecture of the unified Gno VM package.
- **[Deployment & Scaling Guide](gnogent/README_DEPLOY.md)**: Operational manual for scaling Gnogent in production environments.

## 🛡️ Sandboxing & Execution Engines
- **[VM Sandbox Specifications](VM_SANDBOX_SPECS.md)**: Framework for secure, isolated execution of untrusted code (QuickJS, Starlark, Gno, Prolog).
- **[Sandbox Progress Checklist](VM_SANDBOX_PROGRESS.md)**: Current status of the core framework and engine implementations.
- **[WASM Agent Wrapper](wasm-agent.md)**: Specification for adding and managing WASM-based agents at runtime without recompilation.
- **[Go-Wassette](go-wassette.md)**: Architectural blueprint for re-implementing the OCI-based Wassette MCP server in Go.

## 🔌 Integration & Gateways
- **[OpenClaw Gateway Specs](CLAW_GATE_SPECS.md)**: Protocol specification for the JSON-over-WebSocket gateway and Agent Bridge.
- **[JWT Authentication Guide](adk-jwt-auth-server.md)**: How to implement RS256 JWT verification for authenticating requests from gateways.

## 🛠️ Infrastructure & Maintenance
- **[Provider Registry Refactor](PROVIDER_REFACTOR_PLAN.md)**: Dynamic registration system for Session and Memory service providers.
- **[Prolog Memory Provider](PROLOG_MEM_SPECS.md)**: Specification for logic-based fact storage and reasoning using Prolog.
- **[ADK v1.0.0 Upgrade Guide](adk-v1-upgrade.md)**: Migration steps and breaking changes when moving to the latest ADK version.
- **[ADK-Go Patches](adk-patches-25jan26.md)**: Documentation of upstream-bound patches for the ADK library regarding SSE and REST API fields.
