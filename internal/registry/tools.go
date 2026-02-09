package registry

import (
	"context"
	"fmt"

	"github.com/innomon/agentic/internal/compreg"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/adk/tool/geminitool"
	"gopkg.in/yaml.v3"
)

type ToolHandler func(ctx context.Context, args map[string]any) (any, error)

type ToolBase struct {
	Description string           `yaml:"description"`
	Parameters  map[string]Param `yaml:"parameters"`
}

type Param struct {
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

type ToolCreator[T any] func(ctx context.Context, name string, cfg *T) (tool.Tool, error)

type toolFactory struct {
	decode func(*yaml.Node) (any, error)
	create func(ctx context.Context, name string, cfg any) (tool.Tool, error)
}

func RegisterToolType[T any](typeName string, creator ToolCreator[T]) {
	compreg.Set("tool:"+typeName, toolFactory{
		decode: decodeCfg[T],
		create: func(ctx context.Context, name string, a any) (tool.Tool, error) {
			return creator(ctx, name, a.(*T))
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

func createTool(ctx context.Context, typeName, name string, cfg any) (tool.Tool, error) {
	e, ok := compreg.Lookup[toolFactory]("tool:" + typeName)
	if !ok {
		return nil, fmt.Errorf("unknown tool type %q", typeName)
	}
	return e.create(ctx, name, cfg)
}

type BuiltinToolConfig struct {
	ToolBase `yaml:",inline"`
}

func builtinToolCreator(_ context.Context, name string, cfg *BuiltinToolConfig) (tool.Tool, error) {
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

func geminiToolCreator(_ context.Context, name string, cfg *GeminiToolConfig) (tool.Tool, error) {
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
}
