package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"strings"

	"github.com/innomon/agentic/pkg/auth"
	"github.com/innomon/agentic/pkg/config"
	"github.com/innomon/agentic/pkg/console"
	"github.com/innomon/agentic/pkg/logger"
	_ "github.com/innomon/agentic/pkg/alwaysonmem"
	_ "github.com/innomon/agentic/pkg/fsread"
	_ "github.com/innomon/agentic/pkg/gnogent"
	_ "github.com/innomon/agentic/pkg/ml"
	_ "github.com/innomon/agentic/pkg/okf"
	openclawlauncher "github.com/innomon/agentic/pkg/openclaw/launcher"
	_ "github.com/innomon/agentic/pkg/prologmem"
	"github.com/innomon/agentic/pkg/registry"
	_ "github.com/innomon/agentic/pkg/routing"
	_ "github.com/innomon/agentic/pkg/wasm"

	"github.com/a2aproject/a2a-go/v2/a2asrv"
	adklauncher "google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/universal"
	"google.golang.org/adk/v2/cmd/launcher/web"
	"google.golang.org/adk/v2/cmd/launcher/web/a2a"
	"google.golang.org/adk/v2/cmd/launcher/web/api"
	"google.golang.org/adk/v2/cmd/launcher/web/webui"
)

func main() {
	ctx := context.Background()

	openClaw := flag.Bool("openclaw", false, "add openclaw launcher")
	webUI := flag.Bool("webui", false, "add webui launcher")
	a2aFlag := flag.Bool("a2a", false, "add a2a launcher")
	consoleFlag := flag.Bool("console", false, "add console launcher")
	apiFlag := flag.Bool("api", true, "add api launcher")
	exportGraph := flag.Bool("export-graph", false, "export workflow graph to Mermaid format and exit")
	runMsg := flag.String("run", "", "run a single message and exit")
	host := flag.String("host", "localhost", "host to use for api_server_address and webui_address (e.g. your local IP)")
	port := flag.Int("port", 8080, "port to listen on")
	flag.Parse()

	var cfg *config.Config
	var err error
	var largs = 0
	args := flag.Args()

	if len(args) > 1 && (strings.HasSuffix(args[1], ".yml") || strings.HasSuffix(args[1], ".yaml")) {
		cfg, err = config.Load(args[1])
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		args = append(args[:1], args[2:]...)
	} else if len(args) > 0 && (strings.HasSuffix(args[0], ".yml") || strings.HasSuffix(args[0], ".yaml")) {
		cfg, err = config.Load(args[0])
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		largs = 1
	} else {
		cfg, err = config.LoadDefault()
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	}

	// Initialize structured logger
	logCfg := logger.Config{
		Level:          "INFO",
		ConsoleEnabled: true,
		FileEnabled:    false,
	}
	if cfg.Logging != nil {
		logCfg.Level = cfg.Logging.Level
		logCfg.ConsoleEnabled = cfg.Logging.Console
		logCfg.FileEnabled = cfg.Logging.File
		logCfg.Dir = cfg.Logging.Dir
		logCfg.FileName = cfg.Logging.Filename
		logCfg.MaxSizeMB = cfg.Logging.MaxSizeMB
		logCfg.MaxBackups = cfg.Logging.MaxBackups
	}
	if _, err := logger.Init(logCfg); err != nil {
		log.Fatalf("Failed to initialize structured logger: %v", err)
	}

	// Redirect standard log library to our structured logger
	log.SetFlags(0)
	log.SetOutput(slog.NewLogLogger(logger.Log.Handler(), slog.LevelInfo).Writer())

	if *exportGraph {
		for name, agentEntry := range cfg.Agents {
			if agentEntry.Type == "workflow" {
				if wfCfg, ok := agentEntry.Config.(*registry.WorkflowAgentConfig); ok {
					fmt.Printf("Workflow %q:\n%s\n", name, wfCfg.ExportMermaid(name))
				}
			}
		}
		return
	}

	if *runMsg != "" {
		reg := registry.New(cfg).WithInput(*runMsg)
		root, err := reg.GetRoot(ctx)
		if err != nil {
			log.Fatalf("Failed to get root agent: %v", err)
		}
		for ev, err := range root.Run(reg) {
			if err != nil {
				log.Fatalf("Run error: %v", err)
			}
			if ev.Content != nil {
				for _, p := range ev.Content.Parts {
					fmt.Print(p.Text)
				}
			}
		}
		fmt.Println()
		return
	}

	if *openClaw {
		cfg.OpenClaw = true
	}
	if *webUI {
		cfg.WebUI = true
	}
	if *a2aFlag {
		cfg.A2A = true
	}
	if *consoleFlag {
		cfg.Console = true
	}

	reg := registry.New(cfg)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic: %v", r)
		}
		if err := reg.Close(); err != nil {
			log.Printf("Error closing registry: %v", err)
		}
	}()

	launcherConfig, err := reg.BuildLauncherConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to build launcher config: %v", err)
	}

	if authCfg := reg.Config().Auth; authCfg != nil && authCfg.JWT != nil {
		jwt := authCfg.JWT
		verifier, err := auth.NewJWTVerifier(jwt.PublicKeyPath, jwt.Issuer, jwt.Audience)
		if err != nil {
			log.Fatalf("Failed to create JWT verifier: %v", err)
		}
		launcherConfig.A2AOptions = append(launcherConfig.A2AOptions, a2asrv.WithCallInterceptors(&auth.JWTInterceptor{Verifier: verifier}))
		log.Printf("JWT authentication enabled (issuer=%s, audience=%s)", jwt.Issuer, jwt.Audience)
	}

	var webSublaunchers []web.Sublauncher

	if *apiFlag {
		webSublaunchers = append(webSublaunchers, api.NewLauncher())
	}
	if cfg.OpenClaw {
		webSublaunchers = append(webSublaunchers, openclawlauncher.NewLauncher())
	}
	if cfg.WebUI {
		webSublaunchers = append(webSublaunchers, webui.NewLauncher())
	}
	if cfg.A2A {
		webSublaunchers = append(webSublaunchers, a2a.NewLauncher())
	}

	var launchers []adklauncher.SubLauncher
	if cfg.Console {
		launchers = append(launchers, console.New())
	}
	launchers = append(launchers, web.NewLauncher(webSublaunchers...))

	l := universal.NewLauncher(launchers...)

	// Build the command list with flags associated with their respective launchers
	hostVal := *host
	portVal := *port

	var remainingArgs []string

	if len(args[largs:]) == 0 {
		// Default launch: web [flags] api [flags] webui [flags]
		remainingArgs = append(remainingArgs, "web", fmt.Sprintf("-port=%d", portVal))

		if *apiFlag {
			remainingArgs = append(remainingArgs, "api")
			if cfg.WebUI {
				remainingArgs = append(remainingArgs, fmt.Sprintf("-webui_address=http://%s:%d", hostVal, portVal))
			}
		}
		if cfg.OpenClaw {
			remainingArgs = append(remainingArgs, "openclaw")
		}
		if cfg.WebUI {
			remainingArgs = append(remainingArgs, "webui")
			if *apiFlag {
				remainingArgs = append(remainingArgs, fmt.Sprintf("-api_server_address=http://%s:%d/api", hostVal, portVal))
			}
		}
		if cfg.A2A {
			remainingArgs = append(remainingArgs, "a2a")
		}
	} else {
		// User provided custom arguments
		remainingArgs = args[largs:]

		// If the user didn't provide host/port flags, we might still want to inject them
		// but it's trickier here. For now, let's keep it simple and only inject if no args.
		// However, the user specifically asked for -host to work with -webui.
	}

	if err := l.Execute(ctx, launcherConfig, remainingArgs); err != nil {
		log.Fatalf("Launcher error: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
