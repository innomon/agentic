package main

import (
	"context"
	"log"
	"os"

	"github.com/innomon/med-agent/internal/agents"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

func main() {
	ctx := context.Background()

	rootAgent, err := createRootAgent(ctx)
	if err != nil {
		log.Fatalf("Failed to create root agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(rootAgent),
	}

	l := full.NewLauncher()
	if err := l.Execute(ctx, config, os.Args[1:]); err != nil {
		log.Fatalf("Launcher error: %v\n\n%s", err, l.CommandLineSyntax())
	}
}

func createRootAgent(ctx context.Context) (agent.Agent, error) {
	model, err := gemini.NewModel(ctx, "gemini-2.0-flash", &genai.ClientConfig{})
	if err != nil {
		return nil, err
	}

	return agents.NewMedAgent(ctx, model)
}
