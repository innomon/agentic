package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/innomon/agentic/pkg/auth"
	"github.com/innomon/agentic/pkg/config"
	_ "github.com/innomon/agentic/pkg/gnogent"
	"github.com/innomon/agentic/pkg/openclaw/server"
	_ "github.com/innomon/agentic/pkg/prologmem"
	"github.com/innomon/agentic/pkg/registry"
	_ "github.com/innomon/agentic/pkg/routing"
	_ "github.com/innomon/agentic/pkg/wasm"
)

func main() {
	ctx := context.Background()
	var cfg *config.Config
	var err error

	if len(os.Args) > 1 && (strings.HasSuffix(os.Args[1], ".yml") || strings.HasSuffix(os.Args[1], ".yaml")) {
		cfg, err = config.Load(os.Args[1])
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	} else {
		cfg, err = config.LoadDefault()
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
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
		_ = verifier
		log.Printf("JWT authentication enabled (issuer=%s, audience=%s)", jwt.Issuer, jwt.Audience)
	}

	agentBridge, err := server.NewAgentBridge(launcherConfig)
	if err != nil {
		log.Fatalf("Failed to create agent bridge: %v", err)
	}

	srv := server.New(server.Config{})
	srv.SetAgentHandler(agentBridge.Handler())

	log.Printf("Starting OpenClaw gateway server on %s%s", srv.Cfg().Bind, srv.Cfg().Path)
	if err := srv.Start(ctx); err != nil {
		log.Fatalf("Gateway server error: %v", err)
	}
}
