package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/innomon/agentic/pkg/config"
	"github.com/innomon/agentic/pkg/registry"
	"google.golang.org/genai"
)

func main() {
	apiKeyFlag := flag.String("api-key", "", "Google API Key (overrides GOOGLE_API_KEY env var)")
	configPath := flag.String("config", "", "Path to config file")
	verbose := flag.Bool("v", false, "Verbose output")
	flag.Parse()

	apiKey := *apiKeyFlag
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}

	var cfg *config.Config
	var err error
	if *configPath != "" {
		cfg, err = config.Load(*configPath)
	} else {
		cfg, err = config.LoadDefault()
	}

	configuredModels := make(map[string][]string) // model_id -> []config_names
	if err == nil {
		for name, m := range cfg.Models {
			if id, ok := m.Config.(interface{ GetModelID() string }); ok {
				configuredModels[id.GetModelID()] = append(configuredModels[id.GetModelID()], name)
			}
		}
	}

	if apiKey == "" && len(configuredModels) > 0 {
		// Try to find an API key from configured models
		for _, m := range cfg.Models {
			if m.Provider == "gemini" {
				if gc, ok := m.Config.(*registry.GeminiConfig); ok && gc.APIKey != "" {
					apiKey = gc.APIKey
					break
				}
			}
		}
	}

	if apiKey == "" {
		log.Fatal("Google API Key not found. Set GOOGLE_API_KEY environment variable or provide it via -api-key flag, or configure it in config.yaml")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatalf("Failed to create Gemini client: %v", err)
	}

	page, err := client.Models.List(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to list models: %v", err)
	}

	fmt.Printf("%-35s %-40s %-15s %s\n", "NAME", "DISPLAY NAME", "CONFIGURED", "DESCRIPTION")
	fmt.Println(fmt.Sprintf("%-35s %-40s %-15s %s", strings.Repeat("-", 35), strings.Repeat("-", 40), strings.Repeat("-", 15), strings.Repeat("-", 30)))
	for _, m := range page.Items {
		name := strings.TrimPrefix(m.Name, "models/")

		configNames := ""
		if names, ok := configuredModels[name]; ok {
			configNames = strings.Join(names, ", ")
		} else if names, ok := configuredModels[m.Name]; ok { // Try with models/ prefix
			configNames = strings.Join(names, ", ")
		}

		fmt.Printf("%-35s %-40s %-15s %s\n", name, m.DisplayName, configNames, m.Description)
		if *verbose {
			fmt.Printf("  Actions: %v\n", m.SupportedActions)
			fmt.Printf("  Input Limit: %d, Output Limit: %d\n", m.InputTokenLimit, m.OutputTokenLimit)
			fmt.Println()
		}
	}
}
