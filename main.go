package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/innomon/med-agent/internal/auth"
	"github.com/innomon/med-agent/internal/config"
	"github.com/innomon/med-agent/internal/console"
	"github.com/innomon/med-agent/internal/registry"
	_ "github.com/innomon/med-agent/internal/routing"
	_ "github.com/innomon/med-agent/internal/wasm"

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
		launcherConfig.Middleware = verifier.MiddlewareMux
		log.Printf("JWT authentication enabled (issuer=%s, audience=%s)", jwt.Issuer, jwt.Audience)
	}

	l := universal.NewLauncher(
		console.New(),
		web.NewLauncher(api.NewLauncher(), webui.NewLauncher()),
	)

	if err := l.Execute(ctx, launcherConfig, os.Args[largs:]); err != nil {
		log.Fatalf("Launcher error: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
