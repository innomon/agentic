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
- [x] Implement tool-to-VM mapping logic for QuickJS, Starlark, Prolog
- [x] Implement resource limit enforcement (Timeout)
- [x] Implement resource limit enforcement (Memory)
- [ ] Implement network and storage allow-list enforcement
- [x] Integrate with `pkg/registry/tools.go` for seamless tool access

## Phase 4: Validation & Examples
- [x] Unit tests for `SandboxManager`
- [x] Unit tests for QuickJS, Starlark, Prolog
- [ ] Example configurations in `examples/sandbox`
- [ ] End-to-end test with an LLM agent using `sandbox_run`
