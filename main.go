package main

import (
	"context"
	"log"
	"os"
	"strings"

	_ "github.com/innomon/med-agent/internal/componentreg" // register model providers and agent types
	"github.com/innomon/med-agent/internal/config"
	"github.com/innomon/med-agent/internal/console"
	"github.com/innomon/med-agent/internal/registry"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/universal"
	"google.golang.org/adk/cmd/launcher/web"
	"google.golang.org/adk/cmd/launcher/web/api"
	"google.golang.org/adk/cmd/launcher/web/webui"
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

	modelRegistry := registry.NewModelRegistry(cfg)
	toolRegistry := registry.NewToolRegistry(cfg)
	agentRegistry := registry.NewAgentRegistry(cfg, modelRegistry, toolRegistry)

	rootAgent, err := agentRegistry.GetRoot(ctx)
	if err != nil {
		log.Fatalf("Failed to create root agent: %v", err)
	}

	launcherConfig := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(rootAgent),
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
