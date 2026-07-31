package registry

import (
	"context"
	"iter"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
)

type RouteGeneratorAgentConfig struct {
	AgentBase `yaml:",inline"`
}

func routeGeneratorCreator(_ context.Context, name string, cfg *RouteGeneratorAgentConfig, _ ModelRegistry, _ ToolRegistry, _ []agent.Agent) (agent.Agent, error) {
	return agent.New(agent.Config{
		Name:        name,
		Description: cfg.Description,
		Run: func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {
				uc := ctx.UserContent()
				var inputVal string
				if uc != nil && len(uc.Parts) > 0 {
					inputVal = strings.TrimSpace(uc.Parts[0].Text)
				}
				category := strings.ToLower(inputVal)
				category = strings.TrimRight(category, ".")

				ev := session.NewEvent(ctx, ctx.InvocationID())
				ev.Routes = []string{category}
				ev.Output = category

				yield(ev, nil)
			}
		},
	})
}

func init() {
	RegisterAgentType("route_generator", routeGeneratorCreator)
}
