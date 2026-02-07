package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/innomon/med-agent/internal/config"
	"github.com/innomon/med-agent/internal/console"
	"github.com/innomon/med-agent/internal/memory"
	"github.com/innomon/med-agent/internal/registry"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/universal"
	"google.golang.org/adk/cmd/launcher/web"
	"google.golang.org/adk/cmd/launcher/web/api"
	"google.golang.org/adk/cmd/launcher/web/webui"
	adkmemory "google.golang.org/adk/memory"
	"google.golang.org/adk/session"
	sessiondb "google.golang.org/adk/session/database"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	ctx := context.Background()
	var cfg *config.Config
	var err error
	var largs = 1

	// Check if first arg is a config file
	if len(os.Args) > 1 && (strings.HasSuffix(os.Args[1], ".yml") || strings.HasSuffix(os.Args[1], ".yaml")) {
		cfg, err = config.Load(os.Args[1])
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		largs = 2
	} else {
		cfg, err = config.LoadDefault()
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	}

	reg := registry.New(cfg)

	rootAgent, err := reg.GetRoot(ctx)
	if err != nil {
		log.Fatalf("Failed to create root agent: %v", err)
	}

	launcherConfig := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(rootAgent),
		SessionService: session.InMemoryService(),
	}

	if cfg.Session != nil && cfg.Session.Provider == "database" {
		dialector, err := openDialector(cfg.Session.Driver, cfg.Session.DSN)
		if err != nil {
			log.Fatalf("Failed to create session service: %v", err)
		}
		sessionSvc, err := sessiondb.NewSessionService(dialector)
		if err != nil {
			log.Fatalf("Failed to create session service: %v", err)
		}
		if cfg.Session.AutoMigrate {
			if err := sessiondb.AutoMigrate(sessionSvc); err != nil {
				log.Fatalf("Failed to auto-migrate session schema: %v", err)
			}
		}
		launcherConfig.SessionService = sessionSvc
	}

	if cfg.Memory != nil && cfg.Memory.Provider == "database" {
		memorySvc, err := memory.OpenDatabaseMemoryService(cfg.Memory.Driver, cfg.Memory.DSN, cfg.Memory.AutoMigrate)
		if err != nil {
			log.Fatalf("Failed to create memory service: %v", err)
		}
		launcherConfig.MemoryService = memorySvc
	} else {
		launcherConfig.MemoryService = adkmemory.InMemoryService()
	}

	// Create launcher with custom console (file attachment support) and web UI
	l := universal.NewLauncher(
		console.New(), // Custom console with @/path/to/file syntax
		web.NewLauncher(api.NewLauncher(), webui.NewLauncher()), // Web UI and API server
	)

	if err := l.Execute(ctx, launcherConfig, os.Args[largs:]); err != nil {
		log.Fatalf("Launcher error: %v\n\n%s", err, l.CommandLineSyntax())
	}
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
