# Product Definition

# Initial Concept
Build tools and agentic workflows for OKF (Open Knowledge Format / Knowledge Catalog) using the `agentic` ADK-Go config-driven multi-agent framework.

## Vision & Overview
The project aims to integrate OKF specification features into `agentic` through specialized deterministic tools and workflow agents:
1. **Deterministic Tools**: Full text indexing, RAG chunking with metadata, taxonomy file access, and file/directory operations.
2. **Workflow Agents**: Query-to-taxonomy extraction via RAG and in-context LLM tagging.

## Core Features & Requirements
- **Deterministic Tools**:
  - Full-text indexing for OKF documents/corpus.
  - RAG chunking parser with metadata retention (tags, taxonomies, sources).
  - Taxonomy retriever tool (`get_taxonomy`).
  - Native filesystem/directory operations for OKF directory structures.
- **Workflow Agents**:
  - `QueryToTaxonomy` workflow agent executing RAG-based taxonomy extraction.
  - In-context LLM agent for identifying applicable taxonomies and tags given a user query and `taxonomy.md`.

## OKF Tools & Agentic Workflows
- Built-in OKF tools: `okf_get_taxonomy`, `okf_file_ops`, `okf_rag_chunker`, `okf_fulltext_index`.
- Workflow agents for user query to taxonomy extraction and RAG tagging.
