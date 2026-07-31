package registry

import (
	"context"
	"fmt"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/model/openaimodel"
	"google.golang.org/genai"
)

type ModelBase struct {
	Provider string `yaml:"provider"`
	ModelID  string `yaml:"model_id"`
	Default  bool   `yaml:"default"`
}

func (b *ModelBase) IsDefault() bool {
	return b.Default
}

func (b *ModelBase) GetModelID() string {
	return b.ModelID
}

func (b *ModelBase) Validate() error {
	if b.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if b.ModelID == "" {
		return fmt.Errorf("model_id is required")
	}
	return nil
}

type GeminiConfig struct {
	ModelBase `yaml:",inline"`
	APIKey    string `yaml:"api_key"`
	Backend   string `yaml:"backend"`
	Project   string `yaml:"project"`
	Location  string `yaml:"location"`
}

func (c *GeminiConfig) Validate() error {
	return c.ModelBase.Validate()
}

type OpenAIConfig struct {
	ModelBase `yaml:",inline"`
	APIKey    string `yaml:"api_key"`
	BaseURL   string `yaml:"base_url"`
}

func (c *OpenAIConfig) Validate() error {
	return c.ModelBase.Validate()
}

type OllamaConfig struct {
	ModelBase `yaml:",inline"`
	BaseURL   string `yaml:"base_url"`
}

func (c *OllamaConfig) Validate() error {
	if err := c.ModelBase.Validate(); err != nil {
		return err
	}
	if c.BaseURL == "" {
		return fmt.Errorf("base_url is required for ollama provider")
	}
	return nil
}

func geminiCreator(ctx context.Context, cfg *GeminiConfig) (model.LLM, error) {
	clientCfg := &genai.ClientConfig{}
	apiKey := ExpandEnvWithDefaults(cfg.APIKey)
	if apiKey != "" {
		clientCfg.APIKey = apiKey
	}
	backend := ExpandEnvWithDefaults(cfg.Backend)
	if backend != "" {
		clientCfg.Backend = genai.BackendGeminiAPI
		if backend == "vertexai" {
			clientCfg.Backend = genai.BackendVertexAI
		}
	}
	project := ExpandEnvWithDefaults(cfg.Project)
	if project != "" {
		clientCfg.Project = project
	}
	location := ExpandEnvWithDefaults(cfg.Location)
	if location != "" {
		clientCfg.Location = location
	}
	return gemini.NewModel(ctx, ExpandEnvWithDefaults(cfg.ModelID), clientCfg)
}

func openaiCreator(ctx context.Context, cfg *OpenAIConfig) (model.LLM, error) {
	apiKey := ExpandEnvWithDefaults(cfg.APIKey)
	baseURL := ExpandEnvWithDefaults(cfg.BaseURL)
	return openaimodel.NewModel(ctx, ExpandEnvWithDefaults(cfg.ModelID), &openaimodel.ClientConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
	})
}

func ollamaCreator(_ context.Context, cfg *OllamaConfig) (model.LLM, error) {
	return NewOllamaModel(ExpandEnvWithDefaults(cfg.ModelID), ExpandEnvWithDefaults(cfg.BaseURL)), nil
}

func init() {
	RegisterModelProvider("gemini", geminiCreator)
	RegisterModelProvider("openai", openaiCreator)
	RegisterModelProvider("ollama", ollamaCreator)
}
