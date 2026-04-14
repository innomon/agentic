package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/innomon/agentic/pkg/compreg"
	"github.com/innomon/agentic/pkg/sandbox"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/adk/tool/geminitool"
	"gopkg.in/yaml.v3"
)

type ToolHandler func(ctx context.Context, args map[string]any) (any, error)

type ToolBase struct {
	Type        string           `yaml:"type"`
	Description string           `yaml:"description"`
	Parameters  map[string]Param `yaml:"parameters"`
}

type Param struct {
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

type ToolCreator[T any] func(ctx context.Context, name string, cfg *T, sandboxes SandboxRegistry) (tool.Tool, error)

type toolFactory struct {
	decode func(*yaml.Node) (any, error)
	create func(ctx context.Context, name string, cfg any, sandboxes SandboxRegistry) (tool.Tool, error)
}

func RegisterToolType[T any](typeName string, creator ToolCreator[T]) {
	compreg.Set("tool:"+typeName, toolFactory{
		decode: decodeCfg[T],
		create: func(ctx context.Context, name string, a any, sandboxes SandboxRegistry) (tool.Tool, error) {
			return creator(ctx, name, a.(*T), sandboxes)
		},
	})
}

func RegisterToolHandler(name string, handler ToolHandler) {
	compreg.Set("tool_handler:"+name, handler)
}

func GetToolHandler(name string) (ToolHandler, bool) {
	return compreg.Lookup[ToolHandler]("tool_handler:" + name)
}

type ToolDiscriminator struct {
	Type string `yaml:"type"`
}

func DecodeToolConfig(name string, node *yaml.Node) (typeName string, cfg any, err error) {
	var d ToolDiscriminator
	if err := node.Decode(&d); err != nil {
		return "", nil, fmt.Errorf("failed to read type: %w", err)
	}
	typeName = d.Type
	if typeName == "" {
		typeName = "builtin"
	}

	e, ok := compreg.Lookup[toolFactory]("tool:" + typeName)
	if !ok {
		return "", nil, fmt.Errorf("tool %q: unknown type %q", name, typeName)
	}

	cfg, err = e.decode(node)
	if err != nil {
		return "", nil, fmt.Errorf("tool %q: %w", name, err)
	}

	return typeName, cfg, nil
}

func DecodeSandboxConfig(name string, node *yaml.Node) (typeName string, cfg any, err error) {
	var d struct {
		Type string `yaml:"type"`
	}
	if err := node.Decode(&d); err != nil {
		return "", nil, fmt.Errorf("failed to read type: %w", err)
	}
	typeName = d.Type
	if typeName == "" {
		return "", nil, fmt.Errorf("sandbox %q: missing type", name)
	}

	var sandboxCfg SandboxToolConfig
	if err := node.Decode(&sandboxCfg); err != nil {
		return "", nil, fmt.Errorf("sandbox %q: %w", name, err)
	}

	return typeName, &sandboxCfg, nil
}

func createTool(ctx context.Context, typeName, name string, cfg any, sandboxes SandboxRegistry) (tool.Tool, error) {
	e, ok := compreg.Lookup[toolFactory]("tool:" + typeName)
	if !ok {
		return nil, fmt.Errorf("unknown tool type %q", typeName)
	}
	return e.create(ctx, name, cfg, sandboxes)
}

type BuiltinToolConfig struct {
	ToolBase `yaml:",inline"`
}

func builtinToolCreator(_ context.Context, name string, cfg *BuiltinToolConfig, _ SandboxRegistry) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        name,
		Description: cfg.Description,
	}, func(ctx tool.Context, args map[string]any) (any, error) {
		handler, ok := GetToolHandler(name)
		if !ok {
			return nil, fmt.Errorf("no handler registered for tool %q", name)
		}
		return handler(ctx, args)
	})
}

type GeminiToolConfig struct {
	ToolBase `yaml:",inline"`
	Tool     string `yaml:"tool"`
}

var geminiBuiltins = map[string]func() tool.Tool{
	"google_search": func() tool.Tool { return geminitool.GoogleSearch{} },
}

func geminiToolCreator(_ context.Context, name string, cfg *GeminiToolConfig, _ SandboxRegistry) (tool.Tool, error) {
	key := cfg.Tool
	if key == "" {
		key = name
	}
	factory, ok := geminiBuiltins[key]
	if !ok {
		return nil, fmt.Errorf("unknown gemini built-in tool %q", key)
	}
	return factory(), nil
}

func init() {
	RegisterToolType("builtin", builtinToolCreator)
	RegisterToolType("gemini", geminiToolCreator)
	RegisterToolType("sandbox", sandboxToolCreator)
}

type SandboxToolConfig struct {
	ToolBase `yaml:",inline"`
	Type     string            `yaml:"type"`
	Memory   int               `yaml:"memory_limit_mb"`
	Timeout  string            `yaml:"timeout"` // e.g., "5s"
	Tools    []string          `yaml:"allow_tools"`
	Net      []string          `yaml:"allow_net"`
	Env      map[string]string `yaml:"env"`
}

type SandboxRunArgs struct {
	Code    string `json:"code" jsonschema:"description=The code to execute in the sandbox"`
	Sandbox string `json:"sandbox" jsonschema:"description=The name of the sandbox to use"`
}

func sandboxToolCreator(ctx context.Context, name string, cfg *SandboxToolConfig, sandboxes SandboxRegistry) (tool.Tool, error) {
	timeout, _ := time.ParseDuration(cfg.Timeout)

	// Pre-create/warm-up the VM
	_, err := sandboxes.GetOrCreateVM(name, sandbox.VMConfig{
		Type:          cfg.Type,
		MemoryLimitMB: cfg.Memory,
		Timeout:       timeout,
		AllowTools:    cfg.Tools,
		AllowNet:      cfg.Net,
		Env:           cfg.Env,
	})
	if err != nil {
		return nil, err
	}

	return functiontool.New(functiontool.Config{
		Name:        name,
		Description: cfg.Description,
	}, func(ctx tool.Context, args SandboxRunArgs) (any, error) {
		sandboxName := args.Sandbox
		if sandboxName == "" {
			sandboxName = name
		}
		return sandboxes.Run(ctx, sandboxName, args.Code)
	})
}
