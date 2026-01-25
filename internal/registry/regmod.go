package registry

import (
	"context"

	adkopenai "github.com/byebyebruce/adk-go-openai"
	"github.com/innomon/med-agent/internal/config"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

// ModelCreator is a function type for creating LLM models.
type ModelCreator func(ctx context.Context, cfg config.ModelConfig) (model.LLM, error)

// GeminiModelCreator creates Gemini models using the Google GenAI SDK.
func GeminiModelCreator(ctx context.Context, cfg config.ModelConfig) (model.LLM, error) {
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

// OpenAIModelCreator creates OpenAI-compatible models.
func OpenAIModelCreator(_ context.Context, cfg config.ModelConfig) (model.LLM, error) {
	return adkopenai.NewOpenAIModelWithAPIKey(cfg.ModelID, cfg.APIKey), nil
}

var modelCreators = map[string]ModelCreator{
	"gemini": GeminiModelCreator,
	"openai": OpenAIModelCreator,
}

func RegisterModel(name string, creator ModelCreator) {
	modelCreators[name] = creator
}

func GetModelCreator(name string) (ModelCreator, bool) {
	creator, ok := modelCreators[name]
	return creator, ok
}

func init() {
	RegisterModel("gemini", GeminiModelCreator)
	RegisterModel("openai", OpenAIModelCreator)
}
