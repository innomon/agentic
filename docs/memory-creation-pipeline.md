This specification is designed to be ingested by a coding agent (like Gemini 1.5 Pro or 2.0 Flash) to build out the memory infrastructure using the **ADK Go SDK** and **Google MCP**.

---

#  Multi-Agent Memory Specification

## 1. Overview

Implement a three-tier memory system (Episodic, Semantic, Procedural) for an ADK-based agentic workflow. 
The system must capture session data, store it via a Google MCP Database Server, and retrieve/consolidate it for future sessions.

## 2. Core Architecture

The system follows a **"Write-on-Exit, Read-on-Entry, Consolidate-on-Schedule"** pattern.

### Memory Definitions

* **Episodic:** Chronological logs of specific interactions and tool outputs.
* **Semantic:** Abstracted facts, user preferences, and entity relationships.
* **Procedural:** Specialized "rules of engagement" or workflows derived from user feedback.

---

## 3. Database Schema (PostgreSQL/pgvector)

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE agent_memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    session_id TEXT,
    memory_type VARCHAR(20) CHECK (memory_type IN ('episodic', 'semantic', 'procedural')),
    content TEXT NOT NULL,
    content_embedding vector(768), -- Optimized for Gemini text-embedding-004/005
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_mem_lookup ON agent_memories (user_id, memory_type);
CREATE INDEX idx_vector_search ON agent_memories USING hnsw (content_embedding vector_cosine_ops);

```

---

## 4. Agent Pipeline Requirements

### A. The Ingestion Pipeline (SequentialAgent)

1. **TaskAgent:** Performs the primary user request.
2. **MemoryExtractor:** * **Instruction:** "Analyze the `TaskAgent` output and the `user_input`. Extract specific events (Episodic), new facts (Semantic), and refined workflows (Procedural). Call the `save_memory` tool for each."
* **Tools:** Must have access to MCP `insert` tools.



### B. The Retrieval Pipeline (SequentialAgent)

1. **ContextLoader:** * **Instruction:** "Query the database for the last 3 episodic memories and all relevant semantic/procedural memories for `user_id`. Format as a 'Context Block'."
* **OutputKey:** `retrieved_context`.


2. **PrimaryAssistant:** * **Instruction:** "Use the `retrieved_context` to personalize the response. If the context says the user prefers Go, do not provide Python code."

### C. The Consolidator (Background Worker)

* **Trigger:** Cron-based or Ticker loop.
* **Logic:** 1.  Read `episodic` entries where `metadata->'consolidated'` is false.
2.  LLM summarizes these into a single `semantic` update.
3.  Mark episodes as `consolidated: true` or delete them.

---

## 5. Implementation Instructions for the Coding Agent

### Step 1: MCP Configuration

Create a `tools.yaml` for the Google MCP Database Server:

* Define `search_memory`: Vector similarity search on `content_embedding`.
* Define `store_memory`: Standard insert with embedding generation.
* Define `purge_history`: Cleanup of old episodic logs.

### Step 2: ADK Go-SDK Setup

* Initialize `mcptool.New()` pointing to the MCP Toolbox.
* Use `sequentialagent.New()` to wrap the agents.
* Implement a custom `agent.State` to pass `retrieved_context` between sub-agents.

### Step 3: Prompt Engineering

* **System Prompt for Extractor:** "You are a cognitive psychologist for AI. Your job is to convert conversation noise into structured knowledge."
* **System Prompt for Primary:** "Always prioritize 'Procedural' memory rules over generic model training."

---

## 6. Verification Criteria

* [ ] Does the agent recall the user's name/preference in a brand new session?
* [ ] Does the DB show distinct entries for `episodic` vs `semantic`?
* [ ] Does the `Consolidator` successfully reduce 10 rows of episodes into 1 row of semantic fact?

---
