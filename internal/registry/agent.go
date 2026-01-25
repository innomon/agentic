package registry

import (
	"context"
	"fmt"
	"sync"

	"github.com/innomon/med-agent/internal/config"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/agent/workflowagents/loopagent"
	"google.golang.org/adk/agent/workflowagents/parallelagent"
	"google.golang.org/adk/agent/workflowagents/sequentialagent"
)

type AgentCreator func(ctx context.Context, cfg *config.AgentConfig, models *ModelRegistry, subAgents []agent.Agent) (agent.Agent, error)

var (
	agentCreators   = make(map[string]AgentCreator)
	agentCreatorsMu sync.RWMutex
)

func RegisterAgentType(typeName string, creator AgentCreator) {
	agentCreatorsMu.Lock()
	defer agentCreatorsMu.Unlock()
	agentCreators[typeName] = creator
}

func init() {
	RegisterAgentType("llm", createLLMAgent)
	RegisterAgentType("sequential", createSequentialAgent)
	RegisterAgentType("parallel", createParallelAgent)
	RegisterAgentType("loop", createLoopAgent)
}

func createLLMAgent(ctx context.Context, cfg *config.AgentConfig, models *ModelRegistry, subAgents []agent.Agent) (agent.Agent, error) {
	m, err := models.Get(ctx, cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	return llmagent.New(llmagent.Config{
		Name:        cfg.Name,
		Description: cfg.Description,
		Model:       m,
		Instruction: cfg.Instruction,
		SubAgents:   subAgents,
	})
}

func createSequentialAgent(_ context.Context, cfg *config.AgentConfig, _ *ModelRegistry, subAgents []agent.Agent) (agent.Agent, error) {
	return sequentialagent.New(sequentialagent.Config{
		AgentConfig: agent.Config{
			Name:        cfg.Name,
			Description: cfg.Description,
			SubAgents:   subAgents,
		},
	})
}

func createParallelAgent(_ context.Context, cfg *config.AgentConfig, _ *ModelRegistry, subAgents []agent.Agent) (agent.Agent, error) {
	return parallelagent.New(parallelagent.Config{
		AgentConfig: agent.Config{
			Name:        cfg.Name,
			Description: cfg.Description,
			SubAgents:   subAgents,
		},
	})
}

func createLoopAgent(_ context.Context, cfg *config.AgentConfig, _ *ModelRegistry, subAgents []agent.Agent) (agent.Agent, error) {
	return loopagent.New(loopagent.Config{
		AgentConfig: agent.Config{
			Name:        cfg.Name,
			Description: cfg.Description,
			SubAgents:   subAgents,
		},
		MaxIterations: cfg.MaxIterations,
	})
}

type AgentRegistry struct {
	cfg      *config.Config
	models   *ModelRegistry
	agents   map[string]agent.Agent
	building map[string]bool
	mu       sync.Mutex
}

func NewAgentRegistry(cfg *config.Config, models *ModelRegistry) *AgentRegistry {
	return &AgentRegistry{
		cfg:      cfg,
		models:   models,
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

	agentCfg, err := r.cfg.GetAgent(name)
	if err != nil {
		return nil, err
	}

	var subAgents []agent.Agent
	for _, subName := range agentCfg.SubAgents {
		sub, err := r.getOrBuild(ctx, subName)
		if err != nil {
			return nil, fmt.Errorf("failed to build sub-agent %q for %q: %w", subName, name, err)
		}
		subAgents = append(subAgents, sub)
	}

	agentType := agentCfg.Type
	if agentType == "" {
		agentType = "llm"
	}

	agentCreatorsMu.RLock()
	creator, ok := agentCreators[agentType]
	agentCreatorsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown agent type %q for agent %q", agentType, name)
	}

	agentCfg.Name = name
	a, err := creator(ctx, agentCfg, r.models, subAgents)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent %q: %w", name, err)
	}

	r.agents[name] = a
	return a, nil
}

func (r *AgentRegistry) GetRoot(ctx context.Context) (agent.Agent, error) {
	return r.Get(ctx, "MedAgent")
}
