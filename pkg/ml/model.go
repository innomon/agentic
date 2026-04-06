package ml

import (
	"context"
	"fmt"
	"iter"
	"strings"
	"sync"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// MLModel is a local LLM model backed by GGUF weights and a pure-Go transformer.
type MLModel struct {
	cfg       *MLConfig
	arch      Arch
	tokenizer Tokenizer
	info      *GGUFModelInfo
	weights   WeightMap
	initOnce  sync.Once
	initErr   error
}

// NewMLModel creates a new MLModel from the given config.
// The model is lazily initialized on the first call to GenerateContent.
func NewMLModel(cfg *MLConfig) (*MLModel, error) {
	return &MLModel{cfg: cfg}, nil
}

// Init loads the model weights, tokenizer, and architecture from the GGUF file.
func (m *MLModel) Init() error {
	m.initOnce.Do(func() {
		m.initErr = m.doInit()
	})
	return m.initErr
}

func (m *MLModel) doInit() error {
	path := m.cfg.ModelPath

	info, err := ParseGGUF(path)
	if err != nil {
		return fmt.Errorf("parse GGUF metadata: %w", err)
	}
	m.info = info

	weights, err := LoadWeights(path)
	if err != nil {
		return fmt.Errorf("load weights: %w", err)
	}
	m.weights = weights

	tok, err := NewTokenizerFromGGUF(path)
	if err != nil {
		return fmt.Errorf("load tokenizer: %w", err)
	}
	m.tokenizer = tok

	archName := strings.ToLower(info.Architecture)
	switch archName {
	case "llama":
		arch, err := NewLlamaArch(weights, info)
		if err != nil {
			return fmt.Errorf("create llama arch: %w", err)
		}
		m.arch = arch
	case "granite", "granitehybrid", "granitemoehybrid":
		arch, err := NewGraniteHybridArch(weights, info)
		if err != nil {
			return fmt.Errorf("create granite hybrid arch: %w", err)
		}
		m.arch = arch
	default:
		return fmt.Errorf("unsupported architecture: %s", archName)
	}

	return nil
}

func (m *MLModel) Name() string {
	return m.cfg.ModelID
}

func (m *MLModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		// Lazy initialization.
		if err := m.Init(); err != nil {
			yield(nil, fmt.Errorf("ml init: %w", err))
			return
		}

		// Format the request into a prompt string using the architecture-appropriate template.
		var prompt string
		switch m.arch.Name() {
		case "granitehybrid":
			prompt = FormatPromptGranite(req)
		default:
			prompt = FormatPrompt(req)
		}

		// Build sampling parameters from the request config.
		params := m.buildGenerateParams(req)

		// Count prompt tokens for usage metadata.
		promptTokens, _ := m.tokenizer.Encode(prompt)
		promptTokenCount := int32(len(promptTokens))

		// Determine if tools are declared.
		hasTools := req.Config != nil && len(req.Config.Tools) > 0

		// Generate text.
		var fullText strings.Builder
		var tokensGenerated int32

		for chunk, err := range Generate(ctx, m.arch, m.tokenizer, prompt, params) {
			if err != nil {
				yield(nil, fmt.Errorf("ml generate: %w", err))
				return
			}

			fullText.WriteString(chunk)
			tokensGenerated++

			// Yield partial responses when streaming.
			if stream {
				partial := &model.LLMResponse{
					Partial: true,
					Content: &genai.Content{
						Parts: []*genai.Part{genai.NewPartFromText(fullText.String())},
						Role:  "model",
					},
				}
				if !yield(partial, nil) {
					return
				}
			}
		}

		// Build the final response.
		output := fullText.String()
		finalResp := &model.LLMResponse{
			Partial: false,
			Content: &genai.Content{
				Parts: []*genai.Part{},
				Role:  "model",
			},
			FinishReason: genai.FinishReasonStop,
			UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
				PromptTokenCount:     promptTokenCount,
				CandidatesTokenCount: tokensGenerated,
				TotalTokenCount:      promptTokenCount + tokensGenerated,
			},
		}

		// Check for max tokens finish reason.
		if req.Config != nil && req.Config.MaxOutputTokens > 0 && tokensGenerated >= req.Config.MaxOutputTokens {
			finalResp.FinishReason = genai.FinishReasonMaxTokens
		}

		// Try to parse tool calls from the output if tools were declared.
		if hasTools {
			calls, remainingText := ParseToolCalls(output)
			if len(calls) > 0 {
				if remainingText != "" {
					finalResp.Content.Parts = append(finalResp.Content.Parts, genai.NewPartFromText(remainingText))
				}
				for _, fc := range calls {
					finalResp.Content.Parts = append(finalResp.Content.Parts, genai.NewPartFromFunctionCall(fc.Name, fc.Args))
				}
			} else {
				finalResp.Content.Parts = append(finalResp.Content.Parts, genai.NewPartFromText(output))
			}
		} else {
			finalResp.Content.Parts = append(finalResp.Content.Parts, genai.NewPartFromText(output))
		}

		yield(finalResp, nil)
	}
}

// buildGenerateParams extracts generation parameters from the LLM request config.
func (m *MLModel) buildGenerateParams(req *model.LLMRequest) GenerateParams {
	params := GenerateParams{
		MaxTokens: 512,
		Sampler:   DefaultSamplerConfig(),
	}

	if req.Config == nil {
		return params
	}

	if req.Config.MaxOutputTokens > 0 {
		params.MaxTokens = int(req.Config.MaxOutputTokens)
	}

	if req.Config.Temperature != nil {
		params.Sampler.Temperature = *req.Config.Temperature
	}

	if req.Config.TopP != nil {
		params.Sampler.TopP = *req.Config.TopP
	}

	if req.Config.TopK != nil {
		params.Sampler.TopK = int(*req.Config.TopK)
	}

	if len(req.Config.StopSequences) > 0 {
		params.StopSequences = req.Config.StopSequences
	}

	return params
}
