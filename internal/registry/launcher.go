package registry

import (
	"context"
	"fmt"

	"github.com/innomon/agentic/internal/gnogent/storage"
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

	if r.cfg.Session != nil {
		switch r.cfg.Session.Provider {
		case "database":
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
		case "gnogent":
			sessionSvc, err := openGnogentSessionService(r.cfg.Session.DSN, r.cfg.Session.AutoMigrate)
			if err != nil {
				return nil, fmt.Errorf("failed to create gnogent session service: %w", err)
			}
			cfg.SessionService = sessionSvc
		}
	}

	if r.cfg.Memory != nil {
		switch r.cfg.Memory.Provider {
		case "database":
			memorySvc, err := memory.OpenDatabaseMemoryService(r.cfg.Memory.Driver, r.cfg.Memory.DSN, r.cfg.Memory.AutoMigrate)
			if err != nil {
				return nil, fmt.Errorf("failed to create memory service: %w", err)
			}
			cfg.MemoryService = memorySvc
		case "gnogent":
			memorySvc, err := openGnogentMemoryService(r.cfg.Memory.DSN, r.cfg.Memory.AutoMigrate)
			if err != nil {
				return nil, fmt.Errorf("failed to create gnogent memory service: %w", err)
			}
			cfg.MemoryService = memorySvc
		}
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

func openGnogentSessionService(dsn string, autoMigrate bool) (*storage.GnogentSessionService, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	svc := storage.NewGnogentSessionService(db)
	if autoMigrate {
		if err := svc.AutoMigrate(); err != nil {
			return nil, fmt.Errorf("failed to auto-migrate: %w", err)
		}
	}
	return svc, nil
}

func openGnogentMemoryService(dsn string, autoMigrate bool) (*storage.GnogentMemoryService, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	svc := storage.NewGnogentMemoryService(db)
	if autoMigrate {
		if err := svc.AutoMigrate(); err != nil {
			return nil, fmt.Errorf("failed to auto-migrate: %w", err)
		}
	}
	return svc, nil
}
