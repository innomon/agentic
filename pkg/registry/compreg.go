package registry

import (
	"bytes"
	"context"
	"fmt"

	"github.com/innomon/agentic/pkg/compreg"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/plugin"
	"google.golang.org/adk/tool"
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

func decodeCfg[T any](n *yaml.Node) (any, error) {
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
}

type ModelRegistry interface {
	Get(ctx context.Context, name string) (model.LLM, error)
}

type ToolRegistry interface {
	GetMultiple(ctx context.Context, names []string) ([]tool.Tool, error)
}

type modelFactory struct {
	decode func(*yaml.Node) (any, error)
	create func(context.Context, any) (model.LLM, error)
}

type ModelCreator[T any] func(ctx context.Context, cfg *T) (model.LLM, error)

func RegisterModelProvider[T any](name string, creator ModelCreator[T]) {
	compreg.Set("model:"+name, modelFactory{
		decode: decodeCfg[T],
		create: func(ctx context.Context, a any) (model.LLM, error) {
			return creator(ctx, a.(*T))
		},
	})
}

type agentFactory struct {
	decode func(*yaml.Node) (any, error)
	create func(ctx context.Context, name string, cfg any, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error)
}

type AgentCreator[T any] func(ctx context.Context, name string, cfg *T, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error)

func RegisterAgentType[T any](typeName string, creator AgentCreator[T]) {
	compreg.Set("agent:"+typeName, agentFactory{
		decode: decodeCfg[T],
		create: func(ctx context.Context, name string, a any, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
			return creator(ctx, name, a.(*T), models, tools, sub)
		},
	})
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

	e, ok := compreg.Lookup[modelFactory]("model:" + d.Provider)
	if !ok {
		return "", nil, fmt.Errorf("model %q: unknown provider %q", name, d.Provider)
	}

	cfg, err = e.decode(node)
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

	e, ok := compreg.Lookup[agentFactory]("agent:" + typeName)
	if !ok {
		return "", nil, fmt.Errorf("agent %q: unknown type %q", name, typeName)
	}

	cfg, err = e.decode(node)
	if err != nil {
		return "", nil, fmt.Errorf("agent %q: %w", name, err)
	}

	return typeName, cfg, nil
}

func createModel(ctx context.Context, provider string, cfg any) (model.LLM, error) {
	e, ok := compreg.Lookup[modelFactory]("model:" + provider)
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", provider)
	}
	return e.create(ctx, cfg)
}

func createAgent(ctx context.Context, typeName, name string, cfg any, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
	e, ok := compreg.Lookup[agentFactory]("agent:" + typeName)
	if !ok {
		return nil, fmt.Errorf("unknown agent type %q", typeName)
	}
	return e.create(ctx, name, cfg, models, tools, sub)
}

// ProviderCreator defines the function signature for creating a provider service.
type ProviderCreator[C any, S any] func(ctx context.Context, cfg *C) (S, error)

// RegisterProvider registers a new provider creator for a specific service type.
// serviceType should be "session", "memory", etc.
func RegisterProvider[C any, S any](serviceType, providerName string, creator ProviderCreator[C, S]) {
	key := fmt.Sprintf("%s:%s", serviceType, providerName)
	compreg.Set(key, creator)
}

// CreateProvider instantiates a provider service from the registry.
func CreateProvider[C any, S any](ctx context.Context, serviceType, providerName string, cfg *C) (S, error) {
	key := fmt.Sprintf("%s:%s", serviceType, providerName)

	var zero S
	creatorAny, ok := compreg.Lookup[any](key)
	if !ok {
		return zero, fmt.Errorf("%s provider %q not found", serviceType, providerName)
	}

	creator, ok := creatorAny.(ProviderCreator[C, S])

	if !ok {

		return zero, fmt.Errorf("internal error: invalid creator type for %s provider %q", serviceType, providerName)

	}

	return creator(ctx, cfg)
}

// PluginCreator defines the function signature for creating custom ADK plugins.
type PluginCreator func(ctx context.Context, name string, entry PluginEntry) (*plugin.Plugin, error)

// RegisterPluginCreator registers a new plugin creator for a specific plugin type.
func RegisterPluginCreator(typeName string, creator PluginCreator) {
	compreg.Set("plugin_creator:"+typeName, creator)
}

// GetPluginCreator retrieves a registered plugin creator by type name.
func GetPluginCreator(typeName string) (PluginCreator, bool) {
	return compreg.Lookup[PluginCreator]("plugin_creator:" + typeName)
}

