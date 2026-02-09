package registry

import (
	"context"
	"fmt"

	"github.com/innomon/agentic/internal/memory"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	adkmemory "google.golang.org/adk/memory"
	"google.golang.org/adk/session"
	sessiondb "google.golang.org/adk/session/database"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

	if r.cfg.Session != nil && r.cfg.Session.Provider == "database" {
		dialector, err := openDialector(r.cfg.Session.Driver, r.cfg.Session.DSN)
		if err != nil {
			return nil, fmt.Errorf("failed to create session service: %w", err)
		}
		sessionSvc, err := sessiondb.NewSessionService(dialector)
		if err != nil {
			return nil, fmt.Errorf("failed to create session service: %w", err)
		}
		if r.cfg.Session.AutoMigrate {
			if err := sessiondb.AutoMigrate(sessionSvc); err != nil {
				return nil, fmt.Errorf("failed to auto-migrate session schema: %w", err)
			}
		}
		cfg.SessionService = sessionSvc
	}

	if r.cfg.Memory != nil && r.cfg.Memory.Provider == "database" {
		memorySvc, err := memory.OpenDatabaseMemoryService(r.cfg.Memory.Driver, r.cfg.Memory.DSN, r.cfg.Memory.AutoMigrate)
		if err != nil {
			return nil, fmt.Errorf("failed to create memory service: %w", err)
		}
		cfg.MemoryService = memorySvc
	}

	return cfg, nil
}

func openDialector(driver, dsn string) (gorm.Dialector, error) {
	switch driver {
	case "postgres":
		return postgres.Open(dsn), nil
	case "sqlite":
		return sqlite.Open(dsn), nil
	default:
		return nil, fmt.Errorf("unsupported driver %q (supported: postgres, sqlite)", driver)
	}
}
