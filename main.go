package main

import (
	"context"
	"flag"
	"log"
	"strings"

	"github.com/innomon/agentic/pkg/auth"
	"github.com/innomon/agentic/pkg/config"
	"github.com/innomon/agentic/pkg/console"
	_ "github.com/innomon/agentic/pkg/gnogent"
	_ "github.com/innomon/agentic/pkg/ml"
	openclawlauncher "github.com/innomon/agentic/pkg/openclaw/launcher"
	_ "github.com/innomon/agentic/pkg/prologmem"
	"github.com/innomon/agentic/pkg/registry"
	_ "github.com/innomon/agentic/pkg/routing"
	_ "github.com/innomon/agentic/pkg/wasm"

	"github.com/a2aproject/a2a-go/a2asrv"
	adklauncher "google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/universal"
	"google.golang.org/adk/cmd/launcher/web"
	"google.golang.org/adk/cmd/launcher/web/a2a"
	"google.golang.org/adk/cmd/launcher/web/api"
	"google.golang.org/adk/cmd/launcher/web/webui"
)

func main() {
	ctx := context.Background()

	openClaw := flag.Bool("openclaw", false, "add openclaw launcher")
	webUI := flag.Bool("webui", false, "add webui launcher")
	a2aFlag := flag.Bool("a2a", false, "add a2a launcher")
	consoleFlag := flag.Bool("console", false, "add console launcher")
	flag.Parse()

	var cfg *config.Config
	var err error
	var largs = 0
	args := flag.Args()

	if len(args) > 0 && (strings.HasSuffix(args[0], ".yml") || strings.HasSuffix(args[0], ".yaml")) {
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
		launcherConfig.A2AOptions = append(launcherConfig.A2AOptions, a2asrv.WithCallInterceptor(&auth.JWTInterceptor{Verifier: verifier}))
		log.Printf("JWT authentication enabled (issuer=%s, audience=%s)", jwt.Issuer, jwt.Audience)
	}

	webSublaunchers := []web.Sublauncher{api.NewLauncher()}
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

	if err := l.Execute(ctx, launcherConfig, args[largs:]); err != nil {
		log.Fatalf("Launcher error: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
