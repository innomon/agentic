Re-implement the **Wassette** in Go with **Wazero**  for creating a highly portable, zero-dependency MCP (Model Context Protocol) server. 

The Go implementation using Wazero must navigate the current lack of native Component Model support in Wazero by using a **Capability-Based Module** approach.


## 1. Architectural Blueprint

**Design Pattern**: Use the project's framework and design pattern, refer [README](../README.md) and [AGENTS](../AGENETS.md). 

The core of your agent will be a Go-based "Lifecycle Manager" that bridges the MCP protocol and the Wazero runtime.

| Component | Wassette (Rust/Wasmtime) | Re-implementation (Go/Wazero) |
| --- | --- | --- |
| **Runtime** | Wasmtime | **Wazero** (Compiler Engine) |
| **Wasm Target** | WASI Preview 2 (Components) | **WASI Preview 1** (Modules) |
| **Registry** | OCI Artifacts | **Regclient** or **Oras-go** |
| **Security** | Wasmtime Capability System | **Custom Host Wrappers** + `FSConfig` |
| **Protocol** | Rust MCP SDK | `mcp-go` or `mcp-golang` |

---

## 2. Implementation Phases

### Phase I: The Security Framework (Policy Engine)

Wassette’s "deny-by-default" logic must be manually enforced in Go. You will create a `Policy` struct that defines what a module is allowed to touch.

```go
type SecurityPolicy struct {
    AllowedPaths    []string `yaml:"fs_allow"`
    AllowedDomains  []string `yaml:"network_allow"`
    MemoryMaxPages  uint32   `yaml:"memory_limit"`
}

```

Algin the security with the current ADK security used in this project.

* **Filesystem:** Use Wazero’s `FSConfig`. Instead of granting full host access, only mount specific subdirectories as `PreopenedDir`.
* **Network:** Since WASI Preview 1 (Wazero) doesn't have a standard socket API, you must provide **Host Functions** (e.g., `http_get`) that check the `AllowedDomains` allow-list before making the actual Go `http.Get` call.

### Phase II: Wasm Container Lifecycle

This layer manages the "pull-to-run" flow.

1. **Pull:** Use `github.com/regclient/regclient` to pull `.wasm` binaries from OCI registries.
2. **Compile:** Compile the binary once and cache the `wazero.CompiledModule` in memory.
3. **Instantiate:** Create a fresh `api.Module` for every MCP tool call to ensure total state isolation.
4. **Cleanup:** Use `defer mod.Close(ctx)` to ensure resources are freed after the tool returns.

### Phase III: The Component Bridge (ABI)

Because Wazero doesn't yet support the WIT-based Component Model, you need a convention for how tools export themselves.

* **Wassette Style:** Define a standard export function like `run_tool(input_json_ptr, len)`.
* **Go Binding:** The agent will read the config, Use the project's framework and design pattern, refer [README](../README.md) and [AGENTS](../AGENETS.md), input from MCP, write it into the Wasm module's linear memory, and call the exported function.

---

## 3. High-Level Implementation Steps for the Coding Agent

### Step 1: Initialize the MCP Server

Use the `mcp-go` library to handle the JSON-RPC communication.

```go
server := mcp.NewServer("wassette-go", "1.0.0")
server.AddTool(mcp.Tool{
    Name: "wasm_executor",
    Description: "Executes a sandboxed wasm tool",
    // ... schema ...
}, handleWasmExecution)

```

we **MUST** reuse the registry framework as in @internal/registry/registry.go 

### Step 2: Configure Wazero with Security Guards

Implement a function that builds the `ModuleConfig` based on the security policy.

Use the existing config @internal/config/config.go framework



### Step 3: Tool Execution Loop

1. **Fetch:** Resolve the Wasm binary from the OCI reference.
2. **Auth:** Verify the signature (if using Wassette’s security framework for trusted sources).
3. **Exec:** * Load parameters into linear memory.
* Execute.
* Read response from memory.



---

## 4. Security Framework Comparison

Unlike standard Wasm runtimes, your "Wassette-Go" must act as a **Reference Monitor**. Every import the Wasm module uses (like `sock_connect` or `fd_open`) must be intercepted by your Go code to verify it against the `SecurityPolicy`.

### Recommended Libraries

* **Runtime:** `github.com/tetratelabs/wazero`
* **MCP Protocol:** `github.com/mark3labs/mcp-go`
* **OCI Registry:** `github.com/regclient/regclient`
* **Config:** `gopkg.in/yaml.v3`

Reuse the existing ADK secuirty framework used in this project.