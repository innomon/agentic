package componentreg

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"gopkg.in/yaml.v3"
)

// ToolHandler is a function that executes a tool and returns the result.
type ToolHandler func(ctx context.Context, args map[string]any) (any, error)

// ToolBase contains common fields for all tool configs.
type ToolBase struct {
	Description string            `yaml:"description"`
	Parameters  map[string]Param  `yaml:"parameters"`
}

// Param defines a tool parameter schema.
type Param struct {
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

// ToolCreator creates an ADK tool from a typed config.
type ToolCreator[T any] func(ctx context.Context, name string, cfg *T) (tool.Tool, error)

type toolTypeEntry struct {
	schema  reflect.Type
	decode  func(*yaml.Node) (any, error)
	create  func(ctx context.Context, name string, cfg any) (tool.Tool, error)
}

var (
	toolTypes   = make(map[string]toolTypeEntry)
	toolTypesMu sync.RWMutex
)

// RegisterToolType registers a custom tool type with its config schema.
func RegisterToolType[T any](typeName string, creator ToolCreator[T]) {
	toolTypesMu.Lock()
	defer toolTypesMu.Unlock()

	t := reflect.TypeOf((*T)(nil)).Elem()
	toolTypes[typeName] = toolTypeEntry{
		schema: t,
		decode: func(n *yaml.Node) (any, error) {
			cfg := new(T)
			if err := strictDecode(n, cfg); err != nil {
				return nil, err
			}
			if v, ok := any(cfg).(Validatable); ok {
				if err := v.Validate(); err != nil {
					return nil, err
				}
			}
			return cfg, nil
		},
		create: func(ctx context.Context, name string, a any) (tool.Tool, error) {
			return creator(ctx, name, a.(*T))
		},
	}
}

// GetToolType returns the decoder and creator for a tool type.
func GetToolType(typeName string) (decode func(*yaml.Node) (any, error), create func(ctx context.Context, name string, cfg any) (tool.Tool, error), ok bool) {
	toolTypesMu.RLock()
	defer toolTypesMu.RUnlock()
	e, ok := toolTypes[typeName]
	if !ok {
		return nil, nil, false
	}
	return e.decode, e.create, true
}

// ToolDiscriminator extracts the type field for polymorphic decoding.
type ToolDiscriminator struct {
	Type string `yaml:"type"`
}

// DecodeToolConfig decodes a tool config using the registered type.
func DecodeToolConfig(name string, node *yaml.Node) (typeName string, cfg any, err error) {
	var d ToolDiscriminator
	if err := node.Decode(&d); err != nil {
		return "", nil, fmt.Errorf("failed to read type: %w", err)
	}
	typeName = d.Type
	if typeName == "" {
		typeName = "builtin"
	}

	decode, _, ok := GetToolType(typeName)
	if !ok {
		return "", nil, fmt.Errorf("tool %q: unknown type %q", name, typeName)
	}

	cfg, err = decode(node)
	if err != nil {
		return "", nil, fmt.Errorf("tool %q: %w", name, err)
	}

	return typeName, cfg, nil
}

// CreateTool creates an ADK tool using the registered type.
func CreateTool(ctx context.Context, typeName, name string, cfg any) (tool.Tool, error) {
	_, create, ok := GetToolType(typeName)
	if !ok {
		return nil, fmt.Errorf("unknown tool type %q", typeName)
	}
	return create(ctx, name, cfg)
}

// BuiltinToolConfig is the default config for simple tools defined in YAML.
type BuiltinToolConfig struct {
	ToolBase `yaml:",inline"`
}

// Tool handler registry for runtime execution
var (
	toolHandlers   = make(map[string]ToolHandler)
	toolHandlersMu sync.RWMutex
)

// RegisterToolHandler registers a handler function for a tool by name.
func RegisterToolHandler(name string, handler ToolHandler) {
	toolHandlersMu.Lock()
	defer toolHandlersMu.Unlock()
	toolHandlers[name] = handler
}

// GetToolHandler returns the registered handler for a tool.
func GetToolHandler(name string) (ToolHandler, bool) {
	toolHandlersMu.RLock()
	defer toolHandlersMu.RUnlock()
	h, ok := toolHandlers[name]
	return h, ok
}

// builtinToolCreator creates an ADK tool that delegates to registered handlers.
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

func init() {
	RegisterToolType("builtin", builtinToolCreator)
}
