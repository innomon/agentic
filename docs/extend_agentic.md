# Instruction: Extending the Agentic Framework with Custom Tools

This document provides a technical guide for coding agents on how to extend the [Agentic](https://github.com/innomon/agentic) framework by adding custom tools. The Agentic framework acts as a bridge between high-level LLM agents and low-level Go operations.

## The Agentic Pattern

Extending the framework requires a two-step approach:
1.  **Imperative (Go):** Registering a `ToolHandler` that executes your logic.
2.  **Declarative (YAML):** Defining the tool's metadata and schema in a configuration file.

---

## 1. Registering the Tool Handler (Go)

Tools are registered globally using the `github.com/innomon/agentic/pkg/registry` package. This must be done **before** initializing the registry instance.

### Handler Signature
The handler function must match the `ToolHandler` signature:
```go
func(ctx context.Context, toolArgs map[string]any) (any, error)
```

### Registration Example
Map a unique tool name to its implementation:

```go
import (
    "context"
    "github.com/innomon/agentic/pkg/registry"
)

func RegisterTools(ops []MyOperation) {
    for _, op := range ops {
        currentOp := op
        registry.RegisterToolHandler(currentOp.Name(), func(ctx context.Context, toolArgs map[string]any) (any, error) {
            // 1. Extract and cast arguments from the LLM
            args, _ := toolArgs["args"].(string)
            
            // 2. Execute the underlying logic
            if err := currentOp.Run([]string{args}); err != nil {
                return nil, err
            }
            
            return "Execution successful", nil
        })
    }
}
```

---

## 2. Defining the Tool (YAML)

For an agent to "see" and use the registered handler, it must be defined in the `tools` section of your `config.yaml` with `type: builtin`.

### Configuration Structure
The `parameters` map follows a simplified JSON Schema structure handled by the `Param` struct in the framework.

```yaml
tools:
  my_custom_tool:
    type: builtin
    description: "Detailed description for the LLM to understand when to use this tool."
    parameters:
      args:
        type: "string"
        description: "Description of the arguments needed."
        required: true
```

### Assigning Tools to Agents
Tools are assigned to specific agents via the `tools` list:

```yaml
agents:
  my_agent:
    model: my-model-id
    tools:
      - "my_custom_tool"
```

---

## 3. Launching the Framework

In your application's entry point, initialize the registry with the configuration and start the launcher.

```go
import (
    "github.com/innomon/agentic/pkg/config"
    agenticRegistry "github.com/innomon/agentic/pkg/registry"
    "google.golang.org/adk/cmd/launcher/universal"
)

// 1. Load Config
cfg, _ := config.Load("config.yaml")

// 2. Initialize Registry (this binds the handlers registered in Step 1)
reg := agenticRegistry.New(cfg)
defer reg.Close()

// 3. Build Launcher Config
launcherConfig, _ := reg.BuildLauncherConfig(ctx)

// 4. Execute
l := universal.NewLauncher(...)
l.Execute(ctx, launcherConfig, args)
```

---

## Key References

- **Tool Logic (`pkg/registry/tools.go`):** [github.com/innomon/agentic/blob/main/pkg/registry/tools.go](https://github.com/innomon/agentic/blob/main/pkg/registry/tools.go) - Defines `ToolHandler`, `RegisterToolHandler`, and the `builtin` tool creator.
- **Config Schema (`pkg/registry/config.go`):** [github.com/innomon/agentic/blob/main/pkg/registry/config.go](https://github.com/innomon/agentic/blob/main/pkg/registry/config.go) - Defines the `Config` and `AgentEntry` structures.
- **Example Implementation:** [github.com/innomon/agentic/blob/main/examples/farmer/config.yaml](https://github.com/innomon/agentic/blob/main/examples/farmer/config.yaml) - A reference for complex agent hierarchies and tool usage.

## Best Practices for Agents
- **Atomic Tools:** Keep tool logic focused on a single responsibility.
- **Clear Descriptions:** The LLM's ability to use a tool depends entirely on the `description` fields in the YAML.
- **Error Handling:** Return meaningful errors from the `ToolHandler`; these are often passed back to the LLM for self-correction.
