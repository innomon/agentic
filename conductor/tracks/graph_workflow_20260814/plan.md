# Implementation Plan: ADK 2.0 Graph Workflow Architecture Upgrade

Upgrade the `agentic` framework workflow engine to support full ADK 2.0 Graph Workflow features including cyclic graphs, payload state mapping, sub-graphs, WASM nodes, visualization, retries, and persistence.

## Phase 1: Core Graph Workflow Architecture Extensions
- [x] Task: Extend Workflow Configuration Data Structures in `pkg/registry/agents.go`
    - [x] Add `InputMap`, `OutputMap`, `Retries`, `FallbackNode` to `WorkflowNodeEntry`
    - [x] Add `SubWorkflow` node type support in YAML schema
    - [x] Add Mermaid visual exporter function `ExportMermaid(cfg *WorkflowAgentConfig) string`
- [x] Task: Enhance Node Factory & Cyclic Edge Resolution
    - [x] Update `workflowCreator` in `pkg/registry/agents.go` to handle cycles, payload state transformers, and WASM/tool nodes
    - [x] Add node-level retry wrapper and state logging middleware for graph nodes
- [x] Task: Conductor - User Manual Verification 'Core Graph Workflow Architecture Extensions' (Protocol in workflow.md)

## Phase 2: Example Configuration & Observability Tools
- [x] Task: Implement Visual Serialization Exporter
    - [x] Add CLI flag or package utility method to render ADK workflow graphs to Mermaid / DOT markdown
- [x] Task: Create Comprehensive Graph Workflow Example
    - [x] Create `examples/graph-workflow/config.yaml` demonstrating cycle loops, payload mappings, sub-graphs, and retries
    - [x] Create `examples/graph-workflow/README.md` with visual diagram and usage instructions
- [x] Task: Conductor - User Manual Verification 'Example Configuration & Observability Tools' (Protocol in workflow.md)

## Phase 3: Testing & Verification
- [x] Task: Add Unit & Integration Tests for Graph Workflows
    - [x] Write unit tests in `pkg/registry/workflow_graph_test.go` verifying cycles, state mappings, and retries
    - [x] Run full test suite `go test ./...` and `go build ./...`
- [x] Task: Conductor - User Manual Verification 'Testing & Verification' (Protocol in workflow.md)
