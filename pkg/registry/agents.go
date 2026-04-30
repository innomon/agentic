package registry

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/agent/workflowagents/loopagent"
	"google.golang.org/adk/agent/workflowagents/parallelagent"
	"google.golang.org/adk/agent/workflowagents/sequentialagent"
	"google.golang.org/adk/tool/mcptoolset"
)

type AgentBase struct {
	Type        string   `yaml:"type"`
	Description string   `yaml:"description"`
	SubAgents   []string `yaml:"sub_agents"`
}

type MCPToolsetConfig struct {
	Endpoint string `yaml:"endpoint"`
}

type LLMAgentConfig struct {
	AgentBase   `yaml:",inline"`
	Model       string            `yaml:"model"`
	Instruction string            `yaml:"instruction"`
	Tools       []string          `yaml:"tools"`
	MCPToolsets []MCPToolsetConfig `yaml:"mcp_toolsets"`
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

func llmCreator(ctx context.Context, name string, cfg *LLMAgentConfig, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
	m, err := models.Get(ctx, cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	agentCfg := llmagent.Config{
		Name:        name,
		Description: cfg.Description,
		Model:       m,
		Instruction: cfg.Instruction,
		SubAgents:   sub,
	}

	// Add tools if specified
	if len(cfg.Tools) > 0 && tools != nil {
		t, err := tools.GetMultiple(ctx, cfg.Tools)
		if err != nil {
			return nil, fmt.Errorf("failed to get tools: %w", err)
		}
		agentCfg.Tools = t
	}

	// Add MCP toolsets if specified
	for _, mcpCfg := range cfg.MCPToolsets {
		endpoint := ExpandEnvWithDefaults(mcpCfg.Endpoint)
		ts, err := mcptoolset.New(mcptoolset.Config{
			Transport: &mcp.StreamableClientTransport{
				Endpoint: endpoint,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create MCP toolset for endpoint %q: %w", endpoint, err)
		}
		agentCfg.Toolsets = append(agentCfg.Toolsets, ts)
		log.Printf("Agent %q: added MCP toolset from %s", name, endpoint)
	}

	return llmagent.New(agentCfg)
}

func sequentialCreator(_ context.Context, name string, cfg *SequentialAgentConfig, _ ModelRegistry, _ ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
	return sequentialagent.New(sequentialagent.Config{
		AgentConfig: agent.Config{
			Name:        name,
			Description: cfg.Description,
			SubAgents:   sub,
		},
	})
}

func parallelCreator(_ context.Context, name string, cfg *ParallelAgentConfig, _ ModelRegistry, _ ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
	return parallelagent.New(parallelagent.Config{
		AgentConfig: agent.Config{
			Name:        name,
			Description: cfg.Description,
			SubAgents:   sub,
		},
	})
}

func loopCreator(_ context.Context, name string, cfg *LoopAgentConfig, _ ModelRegistry, _ ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
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
