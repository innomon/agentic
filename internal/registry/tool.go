package registry

import (
	"context"
	"sync"

	"github.com/innomon/med-agent/internal/componentreg"
	"github.com/innomon/med-agent/internal/config"
	"google.golang.org/adk/tool"
)

// ToolRegistry manages tool instances.
type ToolRegistry struct {
	cfg   *config.Config
	tools map[string]tool.Tool
	mu    sync.Mutex
}

// NewToolRegistry creates a new tool registry.
func NewToolRegistry(cfg *config.Config) *ToolRegistry {
	return &ToolRegistry{
		cfg:   cfg,
		tools: make(map[string]tool.Tool),
	}
}

// Get returns an ADK tool by name.
func (r *ToolRegistry) Get(ctx context.Context, name string) (tool.Tool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if t, ok := r.tools[name]; ok {
		return t, nil
	}

	entry, err := r.cfg.GetTool(name)
	if err != nil {
		return nil, err
	}

	t, err := componentreg.CreateTool(ctx, entry.Type, name, entry.Config)
	if err != nil {
		return nil, err
	}

	r.tools[name] = t
	return t, nil
}

// GetMultiple returns multiple tools as a slice.
func (r *ToolRegistry) GetMultiple(ctx context.Context, names []string) (any, error) {
	if len(names) == 0 {
		return nil, nil
	}

	var tools []tool.Tool
	for _, name := range names {
		t, err := r.Get(ctx, name)
		if err != nil {
			return nil, err
		}
		if t != nil {
			tools = append(tools, t)
		}
	}

	return tools, nil
}
