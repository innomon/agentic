package registry

import (
	"context"
	"fmt"
	"log"

	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/agent/workflowagent"
	"google.golang.org/adk/agent/workflowagents/loopagent"
	"google.golang.org/adk/agent/workflowagents/parallelagent"
	"google.golang.org/adk/agent/workflowagents/sequentialagent"
	"google.golang.org/adk/tool/mcptoolset"
	"google.golang.org/adk/workflow"
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

type WorkflowNodeEntry struct {
	Name  string `yaml:"name"`
	Agent string `yaml:"agent"`
	Tool  string `yaml:"tool"`
}

type WorkflowEdgeEntry struct {
	From  string `yaml:"from"`
	To    string `yaml:"to"`
	Route string `yaml:"route"`
}

type WorkflowAgentConfig struct {
	AgentBase `yaml:",inline"`
	Nodes     []WorkflowNodeEntry `yaml:"nodes"`
	Edges     []WorkflowEdgeEntry `yaml:"edges"`
}

func (c *WorkflowAgentConfig) GetSubAgents() []string {
	var subs []string
	for _, n := range c.Nodes {
		if n.Agent != "" {
			subs = append(subs, n.Agent)
		}
	}
	return subs
}

func parseRoute(r string) workflow.Route {
	if r == "DEFAULT" || r == "default" || r == "" {
		return workflow.Default
	}
	if b, err := strconv.ParseBool(r); err == nil {
		return workflow.BoolRoute(b)
	}
	if i, err := strconv.Atoi(r); err == nil {
		return workflow.IntRoute(i)
	}
	return workflow.StringRoute(r)
}

func workflowCreator(ctx context.Context, name string, cfg *WorkflowAgentConfig, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
	agentMap := make(map[string]agent.Agent)
	for i, subName := range cfg.GetSubAgents() {
		agentMap[subName] = sub[i]
	}

	nodesMap := make(map[string]workflow.Node)
	nodesMap["START"] = workflow.Start
	nodesMap["start"] = workflow.Start

	for _, n := range cfg.Nodes {
		var wfNode workflow.Node
		if n.Agent != "" {
			ag, ok := agentMap[n.Agent]
			if !ok {
				return nil, fmt.Errorf("sub-agent %q not found in resolved sub-agents", n.Agent)
			}
			var err error
			wfNode, err = workflow.NewAgentNode(ag, workflow.NodeConfig{})
			if err != nil {
				return nil, fmt.Errorf("failed to create agent node %q: %w", n.Name, err)
			}
		} else if n.Tool != "" {
			tList, err := tools.GetMultiple(ctx, []string{n.Tool})
			if err != nil || len(tList) == 0 {
				return nil, fmt.Errorf("failed to get tool %q for node %q: %w", n.Tool, n.Name, err)
			}
			var errNode error
			wfNode, errNode = workflow.NewToolNode(tList[0], workflow.NodeConfig{})
			if errNode != nil {
				return nil, fmt.Errorf("failed to create tool node %q: %w", n.Name, errNode)
			}
		} else {
			return nil, fmt.Errorf("node %q must specify either agent or tool", n.Name)
		}
		nodesMap[n.Name] = wfNode
	}

	var edges []workflow.Edge
	for _, e := range cfg.Edges {
		fromNode, ok := nodesMap[e.From]
		if !ok {
			return nil, fmt.Errorf("edge from %q: node not found", e.From)
		}
		toNode, ok := nodesMap[e.To]
		if !ok {
			return nil, fmt.Errorf("edge to %q: node not found", e.To)
		}
		edges = append(edges, workflow.Edge{
			From:  fromNode,
			To:    toNode,
			Route: parseRoute(e.Route),
		})
	}

	return workflowagent.New(workflowagent.Config{
		Name:        name,
		Description: cfg.Description,
		SubAgents:   sub,
		Edges:       edges,
	})
}

func init() {
	RegisterAgentType("llm", llmCreator)
	RegisterAgentType("sequential", sequentialCreator)
	RegisterAgentType("parallel", parallelCreator)
	RegisterAgentType("loop", loopCreator)
	RegisterAgentType("workflow", workflowCreator)
}
