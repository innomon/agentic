package componentreg

import (
	"context"
	"fmt"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/agent/workflowagents/loopagent"
	"google.golang.org/adk/agent/workflowagents/parallelagent"
	"google.golang.org/adk/agent/workflowagents/sequentialagent"
)

type AgentBase struct {
	Type        string   `yaml:"type"`
	Description string   `yaml:"description"`
	SubAgents   []string `yaml:"sub_agents"`
}

type LLMAgentConfig struct {
	AgentBase   `yaml:",inline"`
	Model       string `yaml:"model"`
	Instruction string `yaml:"instruction"`
}

func (c *LLMAgentConfig) Validate() error {
	if c.Model == "" {
		return fmt.Errorf("model is required for llm agent")
	}
	return nil
}

type SequentialAgentConfig struct {
	AgentBase `yaml:",inline"`
}

type ParallelAgentConfig struct {
	AgentBase `yaml:",inline"`
}

type LoopAgentConfig struct {
	AgentBase     `yaml:",inline"`
	MaxIterations uint `yaml:"max_iterations"`
}

func llmCreator(ctx context.Context, name string, cfg *LLMAgentConfig, models ModelRegistry, sub []agent.Agent) (agent.Agent, error) {
	m, err := models.Get(ctx, cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	return llmagent.New(llmagent.Config{
		Name:        name,
		Description: cfg.Description,
		Model:       m,
		Instruction: cfg.Instruction,
		SubAgents:   sub,
	})
}

func sequentialCreator(_ context.Context, name string, cfg *SequentialAgentConfig, _ ModelRegistry, sub []agent.Agent) (agent.Agent, error) {
	return sequentialagent.New(sequentialagent.Config{
		AgentConfig: agent.Config{
			Name:        name,
			Description: cfg.Description,
			SubAgents:   sub,
		},
	})
}

func parallelCreator(_ context.Context, name string, cfg *ParallelAgentConfig, _ ModelRegistry, sub []agent.Agent) (agent.Agent, error) {
	return parallelagent.New(parallelagent.Config{
		AgentConfig: agent.Config{
			Name:        name,
			Description: cfg.Description,
			SubAgents:   sub,
		},
	})
}

func loopCreator(_ context.Context, name string, cfg *LoopAgentConfig, _ ModelRegistry, sub []agent.Agent) (agent.Agent, error) {
	return loopagent.New(loopagent.Config{
		AgentConfig: agent.Config{
			Name:        name,
			Description: cfg.Description,
			SubAgents:   sub,
		},
		MaxIterations: cfg.MaxIterations,
	})
}

func init() {
	RegisterAgentType("llm", llmCreator)
	RegisterAgentType("sequential", sequentialCreator)
	RegisterAgentType("parallel", parallelCreator)
	RegisterAgentType("loop", loopCreator)
}
