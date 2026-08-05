# Specification: OKF Tools & Agentic Workflows

## Overview
Implement deterministic tools and workflow agents for the OKF (Open Knowledge Format) spec using the `agentic` ADK-Go framework.

## Functional Requirements
- **Deterministic Tools**:
  - Full-text indexing for OKF documents/corpus.
  - RAG chunking with metadata (tags, taxonomy hierarchy, source paths).
  - `get_taxonomy` tool to load canonical `taxonomy.md`.
  - File and directory operations for OKF catalogs.
- **Workflow Agents**:
  - `QueryToTaxonomyAgent` workflow agent: Extract relevant taxonomies and tags given a user query using RAG and in-context LLM tagging.

## Acceptance Criteria
- Unit tests for OKF tools and taxonomy workflow.
- Example YAML configuration in `examples/okf/config.yaml`.
- All code passing `go build ./...` and `go test ./...`.
