# VM Sandbox Framework Progress Checklist

## Phase 1: Core Framework
- [x] Define `SandboxVM` interface and `HostContext` in `pkg/sandbox/types.go`
- [x] Implement `SandboxManager` in `pkg/sandbox/manager.go`
- [x] Create base `sandbox` tool type in `pkg/registry`
- [x] Define configuration structures for sandboxes in `pkg/registry/config.go`

## Phase 2: Engine Implementations
- [x] **QuickJS** engine wrapper (`pkg/sandbox/engines/quickjs`)
- [x] **Starlark** engine wrapper (`pkg/sandbox/engines/starlark`)
- [x] **GnoVM** engine wrapper (`pkg/sandbox/engines/gnovm`)
- [x] **Prolog** engine wrapper (`pkg/sandbox/engines/prolog`)

## Phase 3: Tool Injection & Security
- [ ] Implement tool-to-VM mapping logic for each engine
- [ ] Implement resource limit enforcement (Memory, Timeout)
- [ ] Implement network and storage allow-list enforcement
- [ ] Integrate with `pkg/registry/tools.go` for seamless tool access

## Phase 4: Validation & Examples
- [ ] Unit tests for `SandboxManager`
- [ ] Unit tests for each VM engine
- [ ] Example configurations in `examples/sandbox`
- [ ] End-to-end test with an LLM agent using `sandbox_run`
