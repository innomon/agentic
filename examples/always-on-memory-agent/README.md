# Always-On Memory Agent

An always-on AI memory agent built with Google ADK-Go & Gemini on the **Agentic** framework.

This project translates the [Google Cloud Always-On Memory Agent](https://github.com/GoogleCloudPlatform/generative-ai/tree/main/gemini/agents/always-on-memory-agent) reference implementation into the Go `agentic` framework.

Most AI agents have amnesia: they process information when asked, then forget everything after session exit. This agent gives your system persistent, evolving memory that runs continuously, actively ingesting content, consolidating facts like sleep cycles, and serving grounded answers with citations.

---

## Architecture & Agent Graph

```
                   ┌───────────────────────────────┐
                   │       MemoryOrchestrator      │
                   │       (Root LLM Agent)        │
                   └──────────────┬────────────────┘
                                  │
         ┌────────────────────────┼────────────────────────┐
         ▼                        ▼                        ▼
┌──────────────────┐    ┌───────────────────┐    ┌──────────────────┐
│   IngestAgent    │    │ ConsolidateAgent  │    │    QueryAgent    │
│  (Multimodal /   │    │  (Sleep Cycle     │    │  (Grounding &    │
│   Extraction)    │    │   Synthesis)      │    │   Citations)     │
└────────┬─────────┘    └─────────┬─────────┘    └────────┬─────────┘
         │                        │                       │
         ▼                        ▼                       ▼
┌──────────────────┐    ┌───────────────────┐    ┌──────────────────┐
│   store_memory   │    │store_consolidation│    │ read_all_memories│
│                  │    │read_unconsolidated│    │read_consolidations│
└────────┬─────────┘    └─────────┬─────────┘    └────────┬─────────┘
         │                        │                       │
         └────────────────────────┼───────────────────────┘
                                  ▼
                     ┌──────────────────────────┐
                     │   SQLite Memory Database │
                     │       (memory.db)        │
                     └──────────────────────────┘
```

The system implements both a **hierarchical multi-agent tree** (`MemoryOrchestrator` -> `IngestAgent`, `ConsolidateAgent`, `QueryAgent`) and a **DAG graph workflow** (`MemoryGraphWorkflow`) defined in `examples/always-on-memory-agent/config.yaml`.

### Workflow Graph (`MemoryGraphWorkflow`)

The `MemoryGraphWorkflow` agent (`type: workflow`) defines an explicit Directed Acyclic Graph (DAG) for structured memory processing using ADK 2.0's workflow engine:

- **Graph Nodes**:
  - `classify_request` (powered by `MemoryOrchestrator` to classify intent)
  - `ingest_node` (executes `IngestAgent`)
  - `consolidate_node` (executes `ConsolidateAgent`)
  - `query_node` (executes `QueryAgent`)
- **Graph Edges & Conditional Routes**:
  - `START` -> `classify_request`
  - `classify_request` -> `ingest_node` (route: `"ingest"`)
  - `classify_request` -> `consolidate_node` (route: `"consolidate"`)
  - `classify_request` -> `query_node` (route: `"query"`)
  - `classify_request` -> `query_node` (route: `"DEFAULT"`)

#### When `MemoryGraphWorkflow` is Triggered

1. **Explicit DAG Mode**: When `root_agent: MemoryGraphWorkflow` is configured in `examples/always-on-memory-agent/config.yaml`, incoming requests execute through the explicit DAG workflow engine rather than standard LLM sub-agent tool transfers.
2. **Mermaid Diagram Export**: When executing `./agentic --export-graph examples/always-on-memory-agent/config.yaml`, the framework parses `MemoryGraphWorkflow` and exports visual graph syntax.

---

## Key Components

### 1. Database Layer (`pkg/alwaysonmem`)
Uses GORM with SQLite (`memory.db`) to persist structured memory without needing vector databases or embeddings.
- **`memories`**: Stores raw input, extracted summary, entities (JSON), topics (JSON), connections (JSON), importance score, and consolidation status.
- **`consolidations`**: Stores synthesized cross-memory insights, linked memory IDs, and creation timestamps.
- **`processed_files`**: Tracks automatically ingested inbox files to prevent duplicate processing.

### 2. Builtin Tools
- **`store_memory`**: Ingests structured attributes (raw text, summary, entities, topics, importance, source).
- **`read_all_memories`**: Retrieves stored memories ordered by recency.
- **`read_unconsolidated_memories`**: Fetches memories pending consolidation.
- **`store_consolidation`**: Stores synthesized insights, updates entity link connections, and marks source memories as consolidated.
- **`read_consolidation_history`**: Accesses past higher-level consolidation records.
- **`get_memory_stats`**: Returns memory counts and consolidation statistics.
- **`delete_memory`**: Deletes a specific memory by ID.
- **`clear_all_memories`**: Full reset of memory database and inbox files.

### 3. Background Services
- **File Watcher (`inbox/`)**: Continuously monitors the `./inbox` directory for new text files, documents, and media (`.txt`, `.md`, `.json`, `.csv`, `.pdf`, `.png`, `.jpg`, `.mp3`, `.mp4`, etc.) and automatically invokes `IngestAgent`.
- **Consolidation Loop**: Runs on a periodic timer (default: every 30 minutes) to review unconsolidated memories and trigger `ConsolidateAgent` when 2+ memories exist.

---

## Quick Start

### 1. Build and Run via `agentic` CLI

```bash
# Build main binary
go build -o agentic .

# Run CLI console mode
./agentic examples/always-on-memory-agent/config.yaml console

# Run REST API and Web UI server
./agentic examples/always-on-memory-agent/config.yaml web -port 8080
```

### 2. Run via Standalone Binary

```bash
# Build standalone memory agent
go build -o memory-agent ./examples/always-on-memory-agent

# Run standalone agent watching inbox/ and consolidating every 15 minutes
./memory-agent -watch inbox -consolidate-every 15 console
```

---

## Usage Examples

### Console Ingestion & Querying

```bash
./agentic examples/always-on-memory-agent/config.yaml console

User -> Ingest this note: "Anthropic reports 62% of Claude usage is code-related. AI agents are the fastest growing category."
Agent -> 📥 Stored memory #1: Anthropic reports 62% of Claude usage is code-related.

User -> What do you know about AI agents and code generation?
Agent -> Based on stored memories: Anthropic reports that 62% of Claude usage is code-related, and AI agents represent the fastest growing category [Memory 1].
```

### Automatic Inbox Ingestion

Drop any supported file into the `inbox/` folder:

```bash
cp /path/to/meeting_notes.md ./inbox/
```

The background watcher will detect `meeting_notes.md`, parse its contents, extract entities/topics, and store the memory in `memory.db`.
