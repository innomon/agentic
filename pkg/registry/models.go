package registry

import (
	"context"
	"fmt"

	adkopenai "github.com/byebyebruce/adk-go-openai"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
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
	if cfg.APIKey != "" {
		clientCfg.APIKey = cfg.APIKey
	}
	if cfg.Backend != "" {
		clientCfg.Backend = genai.BackendGeminiAPI
		if cfg.Backend == "vertexai" {
			clientCfg.Backend = genai.BackendVertexAI
		}
	}
	if cfg.Project != "" {
		clientCfg.Project = cfg.Project
	}
	if cfg.Location != "" {
		clientCfg.Location = cfg.Location
	}
	return gemini.NewModel(ctx, cfg.ModelID, clientCfg)
}

func openaiCreator(_ context.Context, cfg *OpenAIConfig) (model.LLM, error) {
	return adkopenai.NewOpenAIModelWithAPIKey(cfg.ModelID, cfg.APIKey), nil
}

func ollamaCreator(_ context.Context, cfg *OllamaConfig) (model.LLM, error) {
	return NewOllamaModel(cfg.ModelID, cfg.BaseURL), nil
}

func init() {
	RegisterModelProvider("gemini", geminiCreator)
	RegisterModelProvider("openai", openaiCreator)
	RegisterModelProvider("ollama", ollamaCreator)
}
