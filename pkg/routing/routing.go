// Package routing provides role-based user routing agent implementations.
package routing

import (
	"context"
	"fmt"
	"strings"

	"github.com/innomon/agentic/pkg/registry"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
)

// RoutingAgentConfig defines the YAML configuration parameters for a routing agent.
type RoutingAgentConfig struct {
	registry.AgentBase `yaml:",inline"`
	Model              string            `yaml:"model"`
	Instruction        string            `yaml:"instruction"`
	Tools              []string          `yaml:"tools"`
	AdminUsers         []string          `yaml:"admin_users"`
	RoleRoutes         map[string]string `yaml:"role_routes"`
}

// Validate checks whether the RoutingAgentConfig contains all required fields.
func (c *RoutingAgentConfig) Validate() error {
	if c.Model == "" {
		return fmt.Errorf("model is required for routing agent")
	}
	if len(c.RoleRoutes) == 0 {
		return fmt.Errorf("role_routes is required for routing agent")
	}
	return nil
}

// routingCreator instantiates a new routing LLM agent based on the provided configuration.
func routingCreator(ctx context.Context, name string, cfg *RoutingAgentConfig, models registry.ModelRegistry, tools registry.ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
	m, err := models.Get(ctx, cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	instruction := buildRoutingInstruction(cfg)

	agentCfg := llmagent.Config{
		Name:        name,
		Description: cfg.Description,
		Model:       m,
		Instruction: instruction,
		SubAgents:   sub,
	}

	if len(cfg.Tools) > 0 && tools != nil {
		t, err := tools.GetMultiple(ctx, cfg.Tools)
		if err != nil {
			return nil, fmt.Errorf("failed to get tools: %w", err)
		}
		agentCfg.Tools = t
	}

	return llmagent.New(agentCfg)
}

// buildRoutingInstruction constructs the system instructions for the routing agent LLM.
func buildRoutingInstruction(cfg *RoutingAgentConfig) string {
	var sb strings.Builder

	if cfg.Instruction != "" {
		sb.WriteString(cfg.Instruction)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Routing Rules\n\n")
	sb.WriteString("You are a routing agent. Follow these steps for EVERY user request:\n\n")

	sb.WriteString("### Step 1: Retrieve User Profile\n")
	sb.WriteString("Call the `get_user_profile` tool with the user's ID to retrieve their profile.\n")
	sb.WriteString("The user ID is available from the conversation context.\n\n")

	if len(cfg.AdminUsers) > 0 {
		sb.WriteString("### Admin Users\n")
		sb.WriteString("The following users are administrators (configured at system level, cannot be changed via tools):\n")
		for _, u := range cfg.AdminUsers {
			sb.WriteString(fmt.Sprintf("- `%s`\n", u))
		}
		if route, ok := cfg.RoleRoutes["admin"]; ok {
			sb.WriteString(fmt.Sprintf("\nAdmin users MUST be routed directly to **%s** regardless of query context.\n\n", route))
		}
	}

	sb.WriteString("### Role-to-Agent Routing Table\n")
	sb.WriteString("| Role | Route To |\n|------|----------|\n")
	for role, agentName := range cfg.RoleRoutes {
		sb.WriteString(fmt.Sprintf("| %s | %s |\n", role, agentName))
	}
	sb.WriteString("\n")

	if _, ok := cfg.RoleRoutes["anonymous"]; ok {
		sb.WriteString("### Anonymous Users\n")
		sb.WriteString("If a user is not found in the database, treat them as `anonymous` and route according to the table above.\n\n")
	} else {
		sb.WriteString("### Anonymous Users\n")
		sb.WriteString("If a user is not found in the database, politely inform them that they do not have access and cannot proceed.\n\n")
	}

	sb.WriteString("### Multiple Roles (Disambiguation)\n")
	sb.WriteString("If a user has multiple roles that map to different agents:\n")
	sb.WriteString("1. Analyze the user's query/context to determine the most appropriate role.\n")
	sb.WriteString("2. Route to the agent matching the most contextually relevant role.\n")
	sb.WriteString("3. If the context is ambiguous, ask the user to clarify their intent.\n\n")

	sb.WriteString("### Status Checks\n")
	sb.WriteString("- Only route users with `Active` status.\n")
	sb.WriteString("- If a user's status is `Pending`, inform them their account is pending approval.\n")
	sb.WriteString("- If a user's status is `Suspended`, inform them their account has been suspended.\n\n")

	sb.WriteString("### Channel Checks\n")
	sb.WriteString("- Verify the user has access to the current channel.\n")
	sb.WriteString("- Users with `ALL` in their channels array have access to every channel.\n")
	sb.WriteString("- If the user does not have channel access, politely deny the request.\n")

	return sb.String()
}

func init() {
	registry.RegisterAgentType("routing", routingCreator)
}
