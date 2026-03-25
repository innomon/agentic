# GnoVM Native Package Injection Guide

This document explains how native Go packages are injected into the GnoVM in the Agentic framework, enabling deterministic Gno logic to interact with the host environment safely.

## 1. Architecture Overview

Native packages allow Gno source code to "import" and call Go functions. This is essential for features that Gno cannot perform natively or deterministically, such as:
- Logging to the host console/logger.
- Calling external ADK tools.
- Invoking other sub-agents.
- Accessing host-provided context (e.g., `InvocationContext`).

### The Registry Pattern

The framework maintains two global registries in `pkg/gnovm/machine.go`:
- `sandboxNativePkgs`: Packages available to all Gno sandboxes.
- `agentNativePkgs`: Packages available to all Gno agents (`gnogent`).

## 2. How Injection Works

The injection happens during the initialization of a `MachineWrapper`:

1. **Resolver Setup**: `NewMachineWrapper` calls `store.SetNativeResolver`.
2. **Lookup**: When Gno code imports a path like `gno.land/p/log`, the resolver iterates through the registered `NativePkg` list.
3. **Dispatch**: If a matching `pkgPath` and `functionName` are found, the resolver returns a Go function with the signature `func(m *gnolang.Machine)`.
4. **Context Access**: Native functions can access the host environment via `m.Context`.
   - **Sandboxes** receive a `*sandbox.HostContext`.
   - **Agents** receive a `*gnovm.AgentContext`.

## 3. Built-in Native Packages

### Sandbox Packages
- **`gno.land/p/log`**:
  - `Println(msg string)`: Outputs to the sandbox's configured `Logger`.

### Agent Packages
- **`gno.land/p/log`**:
  - `Println(msg string)`: Outputs to the agent's standard output.
- **`gno.land/p/agent`**:
  - `CallSubAgent(name string, input string) string`: Synchronously invokes a sub-agent and returns its final response.
  - `CallTool(name string) string`: Synchronously invokes a tool (currently with empty arguments).

## 4. How to Add Custom Native Packages

### Step 1: Define the Implementation
Create a new file (e.g., `pkg/gnovm/my_native.go`) and use an `init` function to register your package.

```go
package gnovm

import (
	"reflect"
	"github.com/gnolang/gno/gnovm/pkg/gnolang"
)

func init() {
	RegisterAgentNativePkg(&NativePkg{
		Name: "myutils",
		Path: "gno.land/p/myutils",
		Funcs: map[string]func(m *gnolang.Machine){
			"GetVersion": func(m *gnolang.Machine) {
				m.PushValue(gnolang.Go2GnoValue(m.Alloc, m.Store, reflect.ValueOf("1.0.0")))
			},
		},
	})
}
```

### Step 2: Declare in Gno
In your `agent.gno` or sandbox script, import the package and use its functions.

```go
package main

import "gno.land/p/myutils"

func main() {
    version := myutils.GetVersion()
}
```

## 5. Configuration Updates

### GnoAgent (`gnogent`)
Agents can now declare a list of `tools` they have access to:

```yaml
type: gnogent
name: MyAgent
tools:
  - my_custom_tool
gnovm:
  source_file: agent.gno
```

These tools are injected into the `AgentContext` and made available to the `agent.CallTool` native function.
