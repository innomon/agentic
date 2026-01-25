package componentreg

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"sync"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"gopkg.in/yaml.v3"
)

type Validatable interface {
	Validate() error
}

func strictDecode(node *yaml.Node, out any) error {
	b, err := yaml.Marshal(node)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	return dec.Decode(out)
}

type ModelCreator[T any] func(ctx context.Context, cfg *T) (model.LLM, error)

type modelProviderEntry struct {
	schema reflect.Type
	decode func(*yaml.Node) (any, error)
	create func(context.Context, any) (model.LLM, error)
}

var (
	modelProviders   = make(map[string]modelProviderEntry)
	modelProvidersMu sync.RWMutex
)

func RegisterModelProvider[T any](name string, creator ModelCreator[T]) {
	modelProvidersMu.Lock()
	defer modelProvidersMu.Unlock()

	t := reflect.TypeOf((*T)(nil)).Elem()
	modelProviders[name] = modelProviderEntry{
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
		create: func(ctx context.Context, a any) (model.LLM, error) {
			return creator(ctx, a.(*T))
		},
	}
}

func GetModelProvider(name string) (decode func(*yaml.Node) (any, error), create func(context.Context, any) (model.LLM, error), ok bool) {
	modelProvidersMu.RLock()
	defer modelProvidersMu.RUnlock()
	e, ok := modelProviders[name]
	if !ok {
		return nil, nil, false
	}
	return e.decode, e.create, true
}

type ModelRegistry interface {
	Get(ctx context.Context, name string) (model.LLM, error)
}

type ToolRegistry interface {
	GetMultiple(ctx context.Context, names []string) (any, error)
}

type AgentCreator[T any] func(ctx context.Context, name string, cfg *T, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error)

type agentTypeEntry struct {
	schema reflect.Type
	decode func(*yaml.Node) (any, error)
	create func(ctx context.Context, name string, cfg any, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error)
}

var (
	agentTypes   = make(map[string]agentTypeEntry)
	agentTypesMu sync.RWMutex
)

func RegisterAgentType[T any](typeName string, creator AgentCreator[T]) {
	agentTypesMu.Lock()
	defer agentTypesMu.Unlock()

	t := reflect.TypeOf((*T)(nil)).Elem()
	agentTypes[typeName] = agentTypeEntry{
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
		create: func(ctx context.Context, name string, a any, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
			return creator(ctx, name, a.(*T), models, tools, sub)
		},
	}
}

func GetAgentType(typeName string) (decode func(*yaml.Node) (any, error), create func(ctx context.Context, name string, cfg any, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error), ok bool) {
	agentTypesMu.RLock()
	defer agentTypesMu.RUnlock()
	e, ok := agentTypes[typeName]
	if !ok {
		return nil, nil, false
	}
	return e.decode, e.create, true
}

type ModelDiscriminator struct {
	Provider string `yaml:"provider"`
}

type AgentDiscriminator struct {
	Type string `yaml:"type"`
}

func DecodeModelConfig(name string, node *yaml.Node) (provider string, cfg any, err error) {
	var d ModelDiscriminator
	if err := node.Decode(&d); err != nil {
		return "", nil, fmt.Errorf("failed to read provider: %w", err)
	}
	if d.Provider == "" {
		return "", nil, fmt.Errorf("model %q missing provider", name)
	}

	decode, _, ok := GetModelProvider(d.Provider)
	if !ok {
		return "", nil, fmt.Errorf("model %q: unknown provider %q", name, d.Provider)
	}

	cfg, err = decode(node)
	if err != nil {
		return "", nil, fmt.Errorf("model %q: %w", name, err)
	}

	return d.Provider, cfg, nil
}

func DecodeAgentConfig(name string, node *yaml.Node) (typeName string, cfg any, err error) {
	var d AgentDiscriminator
	if err := node.Decode(&d); err != nil {
		return "", nil, fmt.Errorf("failed to read type: %w", err)
	}
	typeName = d.Type
	if typeName == "" {
		typeName = "llm"
	}

	decode, _, ok := GetAgentType(typeName)
	if !ok {
		return "", nil, fmt.Errorf("agent %q: unknown type %q", name, typeName)
	}

	cfg, err = decode(node)
	if err != nil {
		return "", nil, fmt.Errorf("agent %q: %w", name, err)
	}

	return typeName, cfg, nil
}

func CreateModel(ctx context.Context, provider string, cfg any) (model.LLM, error) {
	_, create, ok := GetModelProvider(provider)
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", provider)
	}
	return create(ctx, cfg)
}

func CreateAgent(ctx context.Context, typeName, name string, cfg any, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
	_, create, ok := GetAgentType(typeName)
	if !ok {
		return nil, fmt.Errorf("unknown agent type %q", typeName)
	}
	return create(ctx, name, cfg, models, tools, sub)
}
