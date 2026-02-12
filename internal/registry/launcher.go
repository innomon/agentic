package registry

import (
	"context"
	"fmt"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	adkmemory "google.golang.org/adk/memory"
	"google.golang.org/adk/session"
)

func (r *Registry) BuildLauncherConfig(ctx context.Context) (*launcher.Config, error) {
	rootAgent, err := r.GetRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create root agent: %w", err)
	}

	cfg := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(rootAgent),
		SessionService: session.InMemoryService(),
		MemoryService:  adkmemory.InMemoryService(),
	}

	if r.cfg.Session != nil {
		sessionSvc, err := CreateProvider[SessionConfig, session.Service](ctx, "session", r.cfg.Session.Provider, r.cfg.Session)
		if err != nil {
			return nil, fmt.Errorf("failed to create session service: %w", err)
		}
		cfg.SessionService = sessionSvc
	}

	if r.cfg.Memory != nil {
		memorySvc, err := CreateProvider[MemoryConfig, adkmemory.Service](ctx, "memory", r.cfg.Memory.Provider, r.cfg.Memory)
		if err != nil {
			return nil, fmt.Errorf("failed to create memory service: %w", err)
		}
		cfg.MemoryService = memorySvc
	}

	return cfg, nil
}
