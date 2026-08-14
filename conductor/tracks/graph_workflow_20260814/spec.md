# Specification: ADK 2.0 Graph Workflow Architecture Upgrade

## Overview
This track upgrades the `agentic` ADK 2.0 workflow architecture (`type: workflow`) to support advanced graph workflow capabilities as described in ADK 2.0 (https://adk.dev/graphs/). It enhances the existing DAG/workflow engine in `pkg/registry/agents.go` to support cycles, node payload/state mappings, sub-graph execution, WASM node integrations, graph visualization, node-level retries, and execution state persistence.

## Functional Requirements
1. **Cyclic Graph & Loop Node Support**:
   - Extend graph builder to support backward edges/cycles safely.
   - Support loop nodes with iteration limits and termination route evaluation.

2. **State Passing & Payload Key Mapping**:
   - Allow graph nodes to specify explicit input/output key mappings across node context states.

3. **Sub-Graph & Nested Workflow Nodes**:
   - Support invoking nested workflows or sub-graphs as nodes within a parent graph.

4. **WASM & Tool Execution Nodes**:
   - Support WebAssembly binaries and custom tools as native graph nodes (`node.wasm`, `node.tool`).

5. **Visual Graph Serialization**:
   - Generate Mermaid / DOT format graph representations for visualization and debugging.

6. **Reliability & Observability**:
   - Configurable per-node retries and error fallback edges.
   - Session-backed execution state persistence and real-time execution tracing/logging.

## Non-Functional Requirements
- 100% backward compatibility with existing `type: workflow` YAML configs (e.g. `examples/a2ui-workflow/config.yaml`).
- Clean compilation (`go build ./...`) and tests (`go test ./...`).

## Acceptance Criteria
- Unit tests covering cyclic execution, sub-graphs, payload mapping, and retries.
- Example config `examples/graph-workflow/config.yaml` validating new features.
- CLI/package capability to output Mermaid diagram strings for workflows.
