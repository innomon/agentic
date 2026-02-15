# Multi-Agent Memory Pipeline

A three-tier memory system (Episodic, Semantic, Procedural) for ADK-based agentic workflows. The system captures session data, stores it in PostgreSQL with pgvector, and retrieves/consolidates it for future sessions.

## Usage

```bash
./agentic examples/mem-pipeline/config.yaml console
```

## Prerequisites

1. **PostgreSQL** with the [pgvector](https://github.com/pgvector/pgvector) extension installed
2. Create the database and tables:

```bash
createdb mem_pipeline
go run examples/mem-pipeline/initdb/main.go \
  "postgres://user:pass@localhost:5432/mem_pipeline?sslmode=disable"
```

3. Set `GOOGLE_API_KEY` for Gemini model access

## Architecture

The system follows a **Write-on-Exit, Read-on-Entry** pattern:

### Retrieval Pipeline (entry point)

1. **ContextLoader** — Queries the memory database for recent episodic events, semantic facts, and procedural rules, then assembles a Context Block.
2. **PrimaryAssistant** — Uses the Context Block to personalise responses, respect preferences, and follow procedural rules.

### Ingestion Pipeline (runs after each response)

1. **MemoryExtractor** — Analyses the completed conversation and calls `store_memory` for each extracted piece of knowledge.

### Memory Tiers

| Tier | Description | Examples |
|------|-------------|----------|
| **Episodic** | Chronological logs of interactions | "User asked about Go testing", "Tool returned 3 results" |
| **Semantic** | Abstracted facts and preferences | "User prefers Go over Python", "User's name is Alice" |
| **Procedural** | Rules of engagement and workflows | "Always respond in markdown", "Check tests before deploying" |

## Tools

| Tool | Description |
|------|-------------|
| `search_memory` | Vector similarity search on `content_embedding` |
| `store_memory` | Insert a memory entry with embedding generation |
| `purge_history` | Cleanup old or consolidated episodic logs |

## Database Schema

The `initdb` utility creates:

- `agent_memories` table with UUID primary key, user/session IDs, memory type, content, pgvector embedding (768d), JSONB metadata, and timestamp
- B-tree index on `(user_id, memory_type)` for lookups
- HNSW index on `content_embedding` for cosine similarity search

## Configuration

Update the PostgreSQL DSN in `config.yaml` before running:

```yaml
session:
  dsn: postgres://user:pass@localhost:5432/mem_pipeline?sslmode=disable
memory:
  dsn: postgres://user:pass@localhost:5432/mem_pipeline?sslmode=disable
```

## Verification

- [ ] Agent recalls user name/preferences in a new session
- [ ] Database shows distinct entries for episodic vs semantic vs procedural
- [ ] Consolidator reduces episodic rows into semantic facts
