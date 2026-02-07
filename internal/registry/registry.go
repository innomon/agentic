package registry

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/innomon/med-agent/internal/componentreg"
	"github.com/innomon/med-agent/internal/config"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

var (
	_ componentreg.ModelRegistry = (*modelAdapter)(nil)
	_ componentreg.ToolRegistry  = (*toolAdapter)(nil)
)

type loader func(ctx context.Context, r *Registry, name string) (any, error)

type Registry struct {
	cfg      *config.Config
	mu       sync.RWMutex
	items    map[string]any
	loaders  map[reflect.Type]loader
	building map[string]bool
}

func New(cfg *config.Config) *Registry {
	r := &Registry{
		cfg:      cfg,
		items:    make(map[string]any),
		building: make(map[string]bool),
	}
	r.loaders = map[reflect.Type]loader{
		typeOf[model.LLM]():   loadModel,
		typeOf[tool.Tool]():   loadTool,
		typeOf[agent.Agent](): loadAgent,
	}
	return r
}

func typeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func itemKey[T any](name string) string {
	return typeOf[T]().String() + ":" + name
}

func Get[T any](ctx context.Context, r *Registry, name string) (T, error) {
	k := itemKey[T](name)

	r.mu.RLock()
	if v, ok := r.items[k]; ok {
		r.mu.RUnlock()
		return v.(T), nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	return getOrLoadLocked[T](ctx, r, name)
}

func getOrLoadLocked[T any](ctx context.Context, r *Registry, name string) (T, error) {
	k := itemKey[T](name)
	if v, ok := r.items[k]; ok {
		return v.(T), nil
	}

	ldr, ok := r.loaders[typeOf[T]()]
	if !ok {
		var zero T
		return zero, fmt.Errorf("no loader registered for type %s", typeOf[T]())
	}

	v, err := ldr(ctx, r, name)
	if err != nil {
		var zero T
		return zero, err
	}

	r.items[k] = v
	return v.(T), nil
}

func (r *Registry) GetDefaultModel(ctx context.Context) (model.LLM, error) {
	name, _, err := r.cfg.GetDefaultModel()
	if err != nil {
		return nil, err
	}
	return Get[model.LLM](ctx, r, name)
}

func (r *Registry) GetTools(ctx context.Context, names []string) ([]tool.Tool, error) {
	if len(names) == 0 {
		return nil, nil
	}
	var tools []tool.Tool
	for _, name := range names {
		t, err := Get[tool.Tool](ctx, r, name)
		if err != nil {
			return nil, err
		}
		if t != nil {
			tools = append(tools, t)
		}
	}
	return tools, nil
}

func (r *Registry) GetRoot(ctx context.Context) (agent.Agent, error) {
	return Get[agent.Agent](ctx, r, "MedAgent")
}

func loadModel(ctx context.Context, r *Registry, name string) (any, error) {
	entry, err := r.cfg.GetModel(name)
	if err != nil {
		return nil, err
	}
	m, err := componentreg.CreateModel(ctx, entry.Provider, entry.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create model %q: %w", name, err)
	}
	return m, nil
}

func loadTool(ctx context.Context, r *Registry, name string) (any, error) {
	entry, err := r.cfg.GetTool(name)
	if err != nil {
		return nil, err
	}
	return componentreg.CreateTool(ctx, entry.Type, name, entry.Config)
}

func loadAgent(ctx context.Context, r *Registry, name string) (any, error) {
	if r.building[name] {
		return nil, fmt.Errorf("circular dependency detected for agent %q", name)
	}
	r.building[name] = true
	defer delete(r.building, name)

	entry, err := r.cfg.GetAgent(name)
	if err != nil {
		return nil, err
	}

	var subAgents []agent.Agent
	for _, subName := range entry.SubAgents {
		sub, err := getOrLoadLocked[agent.Agent](ctx, r, subName)
		if err != nil {
			return nil, fmt.Errorf("failed to build sub-agent %q for %q: %w", subName, name, err)
		}
		subAgents = append(subAgents, sub)
	}

	a, err := componentreg.CreateAgent(ctx, entry.Type, name, entry.Config, &modelAdapter{r}, &toolAdapter{r}, subAgents)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent %q: %w", name, err)
	}
	return a, nil
}

type modelAdapter struct{ r *Registry }

func (a *modelAdapter) Get(ctx context.Context, name string) (model.LLM, error) {
	return Get[model.LLM](ctx, a.r, name)
}

type toolAdapter struct{ r *Registry }

func (a *toolAdapter) GetMultiple(ctx context.Context, names []string) (any, error) {
	return a.r.GetTools(ctx, names)
}
