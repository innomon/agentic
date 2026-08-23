package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/innomon/agentic/pkg/alwaysonmem"
	"github.com/innomon/agentic/pkg/alwaysonmem"
	"github.com/innomon/agentic/pkg/auth"
	"github.com/innomon/agentic/pkg/config"
	"github.com/innomon/agentic/pkg/console"
	_ "github.com/innomon/agentic/pkg/fsread"
	_ "github.com/innomon/agentic/pkg/gnogent"
	_ "github.com/innomon/agentic/pkg/ml"
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

var supportedExtensions = map[string]bool{
	".txt": true, ".md": true, ".json": true, ".csv": true,
	".log": true, ".xml": true, ".yaml": true, ".yml": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".bmp": true, ".svg": true, ".mp3": true,
	".wav": true, ".ogg": true, ".flac": true, ".m4a": true,
	".aac": true, ".mp4": true, ".webm": true, ".mov": true,
	".avi": true, ".mkv": true, ".pdf": true,
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	openClaw := flag.Bool("openclaw", false, "add openclaw launcher")
	webUI := flag.Bool("webui", false, "add webui launcher")
	a2aFlag := flag.Bool("a2a", false, "add a2a launcher")
	consoleFlag := flag.Bool("console", false, "add console launcher")
	apiFlag := flag.Bool("api", true, "add api launcher")
	runMsg := flag.String("run", "", "run a single message and exit")
	host := flag.String("host", "localhost", "host to use for api_server_address and webui_address")
	port := flag.Int("port", 8080, "port to listen on")
	watchFolder := flag.String("watch", "inbox", "folder to watch for new files (default: inbox)")
	consolidateMin := flag.Int("consolidate-every", 30, "consolidation interval in minutes (default: 30)")
	flag.Parse()

	defaultConfigPath := "config.yaml"
	if _, err := os.Stat(defaultConfigPath); err != nil {
		defaultConfigPath = filepath.Join("examples", "always-on-memory-agent", "config.yaml")
	}

	args := flag.Args()
	var cfg *config.Config
	var err error
	var largs = 0

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
		cfg, err = config.Load(defaultConfigPath)
		if err != nil {
			log.Fatalf("Failed to load config from %s: %v", defaultConfigPath, err)
		}
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

	// Start background file watcher and background consolidation loop
	go startFileWatcher(ctx, reg, *watchFolder)
	go startConsolidationLoop(ctx, reg, time.Duration(*consolidateMin)*time.Minute)

	if *runMsg != "" {
		regWithInput := reg.WithInput(*runMsg)
		root, err := regWithInput.GetRoot(ctx)
		if err != nil {
			log.Fatalf("Failed to get root agent: %v", err)
		}
		for ev, err := range root.Run(regWithInput) {
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
	hostVal := *host
	portVal := *port

	var remainingArgs []string
	if len(args[largs:]) == 0 {
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
		remainingArgs = args[largs:]
	}

	log.Printf("🧠 Always-On Memory Agent starting...")
	log.Printf("   Watch folder: %s/", *watchFolder)
	log.Printf("   Consolidation: every %dm", *consolidateMin)
	if err := l.Execute(ctx, launcherConfig, remainingArgs); err != nil {
		log.Fatalf("Launcher execution failed: %v", err)
	}
}

func startFileWatcher(ctx context.Context, reg *registry.Registry, folder string) {
	if err := os.MkdirAll(folder, 0755); err != nil {
		log.Printf("Failed to create watch folder %s: %v", folder, err)
		return
	}

	log.Printf("👁️ Watching inbox folder: %s/", folder)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processInboxFiles(ctx, reg, folder)
		}
	}
}

func processInboxFiles(ctx context.Context, reg *registry.Registry, folder string) {
	db, err := alwaysonmem.GetDB()
	if err != nil {
		return
	}

	entries, err := os.ReadDir(folder)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !supportedExtensions[ext] {
			continue
		}

		fullPath := filepath.Join(folder, entry.Name())

		var count int64
		db.Model(&alwaysonmem.ProcessedFile{}).Where("path = ?", fullPath).Count(&count)
		if count > 0 {
			continue
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			log.Printf("Error reading file %s: %v", fullPath, err)
			continue
		}

		log.Printf("📄 Ingesting new inbox file: %s", entry.Name())
		msg := fmt.Sprintf("Ingest content from file %s:\n\n%s", entry.Name(), string(content))
		regWithInput := reg.WithInput(msg)
		root, err := regWithInput.GetRoot(ctx)
		if err != nil {
			log.Printf("Error getting root agent for ingestion: %v", err)
			continue
		}

		for _, err := range root.Run(regWithInput) {
			if err != nil {
				log.Printf("Ingestion error for %s: %v", entry.Name(), err)
			}
		}

		now := time.Now().UTC().Format(time.RFC3339)
		db.Create(&alwaysonmem.ProcessedFile{
			Path:        fullPath,
			ProcessedAt: now,
		})
	}
}

func startConsolidationLoop(ctx context.Context, reg *registry.Registry, interval time.Duration) {
	log.Printf("🔄 Consolidation timer started: interval %v", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			db, err := alwaysonmem.GetDB()
			if err != nil {
				continue
			}

			var count int64
			db.Model(&alwaysonmem.Memory{}).Where("consolidated = ?", 0).Count(&count)
			if count >= 2 {
				log.Printf("🔄 Triggering memory consolidation (%d unconsolidated memories)...", count)
				regWithInput := reg.WithInput("Consolidate unconsolidated memories and store insights.")
				root, err := regWithInput.GetRoot(ctx)
				if err != nil {
					log.Printf("Error getting root agent for consolidation: %v", err)
					continue
				}

				for _, err := range root.Run(regWithInput) {
					if err != nil {
						log.Printf("Consolidation run error: %v", err)
					}
				}
			} else {
				log.Printf("🔄 Skipping consolidation (%d unconsolidated memories)", count)
			}
		}
	}
}
