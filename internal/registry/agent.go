package registry

import (
	"context"
	"fmt"
	"sync"

	"github.com/innomon/med-agent/internal/componentreg"
	"github.com/innomon/med-agent/internal/config"
	"google.golang.org/adk/agent"
)

type AgentRegistry struct {
	cfg      *config.Config
	models   *ModelRegistry
	tools    *ToolRegistry
	agents   map[string]agent.Agent
	building map[string]bool
	mu       sync.Mutex
}

func NewAgentRegistry(cfg *config.Config, models *ModelRegistry, tools *ToolRegistry) *AgentRegistry {
	return &AgentRegistry{
		cfg:      cfg,
		models:   models,
		tools:    tools,
		agents:   make(map[string]agent.Agent),
		building: make(map[string]bool),
	}
}

func (r *AgentRegistry) Get(ctx context.Context, name string) (agent.Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.getOrBuild(ctx, name)
}

func (r *AgentRegistry) getOrBuild(ctx context.Context, name string) (agent.Agent, error) {
	if a, ok := r.agents[name]; ok {
		return a, nil
	}

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
		sub, err := r.getOrBuild(ctx, subName)
		if err != nil {
			return nil, fmt.Errorf("failed to build sub-agent %q for %q: %w", subName, name, err)
		}
		subAgents = append(subAgents, sub)
	}

	a, err := componentreg.CreateAgent(ctx, entry.Type, name, entry.Config, r.models, r.tools, subAgents)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent %q: %w", name, err)
	}

	r.agents[name] = a
	return a, nil
}

func (r *AgentRegistry) GetRoot(ctx context.Context) (agent.Agent, error) {
	return r.Get(ctx, "MedAgent")
}
