package registry

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	adkmemory "google.golang.org/adk/memory"
	"google.golang.org/adk/plugin"
	"google.golang.org/adk/plugin/loggingplugin"
	"google.golang.org/adk/plugin/retryandreflect"
	"google.golang.org/adk/runner"
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
		if c, ok := sessionSvc.(io.Closer); ok {
			r.closers = append(r.closers, c)
		}
	}

	if r.cfg.Memory != nil {
		memorySvc, err := CreateProvider[MemoryConfig, adkmemory.Service](ctx, "memory", r.cfg.Memory.Provider, r.cfg.Memory)
		if err != nil {
			return nil, fmt.Errorf("failed to create memory service: %w", err)
		}
		cfg.MemoryService = memorySvc
		if c, ok := memorySvc.(io.Closer); ok {
			r.closers = append(r.closers, c)
		}
	}

	var plugins []*plugin.Plugin
	for _, p := range r.cfg.Plugins {
		var pl *plugin.Plugin
		var err error
		if creator, ok := GetPluginCreator(p.Type); ok {
			pl, err = creator(ctx, p.Name, p)
		} else {
			switch p.Type {
			case "logging", "logging_plugin":
				pl, err = loggingplugin.New(p.Name)
			case "retry", "retry_and_reflect":
				var opts []retryandreflect.PluginOption
				if maxRetries, ok := p.Config["max_retries"].(int); ok {
					opts = append(opts, retryandreflect.WithMaxRetries(maxRetries))
				} else if maxRetriesFloat, ok := p.Config["max_retries"].(float64); ok {
					opts = append(opts, retryandreflect.WithMaxRetries(int(maxRetriesFloat)))
				}
				if errIfExceeded, ok := p.Config["error_if_retry_exceeded"].(bool); ok {
					opts = append(opts, retryandreflect.WithErrorIfRetryExceeded(errIfExceeded))
				}
				if scopeStr, ok := p.Config["scope"].(string); ok {
					scope := retryandreflect.Invocation
					if scopeStr == "global" {
						scope = retryandreflect.Global
					}
					opts = append(opts, retryandreflect.WithTrackingScope(scope))
				}
				pl, err = retryandreflect.New(opts...)
			default:
				return nil, fmt.Errorf("unknown plugin type %q", p.Type)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("failed to create plugin %q (type %q): %w", p.Name, p.Type, err)
		}
		plugins = append(plugins, pl)
		if c, ok := interface{}(pl).(io.Closer); ok {
			r.closers = append(r.closers, c)
		}
	}
	cfg.PluginConfig = runner.PluginConfig{
		Plugins: plugins,
	}

	return cfg, nil
}
