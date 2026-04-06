package ml

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/innomon/agentic/pkg/registry"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func TestFormatPrompt(t *testing.T) {
	req := &model.LLMRequest{
		Config: &genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{genai.NewPartFromText("You are helpful.")},
			},
		},
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{genai.NewPartFromText("Hi")}},
		},
	}
	prompt := FormatPrompt(req)

	for _, want := range []string{
		"You are helpful.",
		"Hi",
		"<|begin_of_text|>",
		"<|start_header_id|>system<|end_header_id|>",
		"<|start_header_id|>user<|end_header_id|>",
		"<|start_header_id|>assistant<|end_header_id|>",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestFormatPromptMultiTurn(t *testing.T) {
	req := &model.LLMRequest{
		Config: &genai.GenerateContentConfig{},
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{genai.NewPartFromText("Hello")}},
			{Role: "model", Parts: []*genai.Part{genai.NewPartFromText("Hi there!")}},
			{Role: "user", Parts: []*genai.Part{genai.NewPartFromText("How are you?")}},
		},
	}
	prompt := FormatPrompt(req)
	for _, want := range []string{"Hello", "Hi there!", "How are you?"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestFormatPromptWithTools(t *testing.T) {
	req := &model.LLMRequest{
		Config: &genai.GenerateContentConfig{
			Tools: []*genai.Tool{
				{
					FunctionDeclarations: []*genai.FunctionDeclaration{
						{
							Name:        "get_weather",
							Description: "Get the current weather",
							Parameters: &genai.Schema{
								Type: "OBJECT",
								Properties: map[string]*genai.Schema{
									"location": {Type: "STRING", Description: "City name"},
								},
							},
						},
					},
				},
			},
		},
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{genai.NewPartFromText("What's the weather?")}},
		},
	}
	prompt := FormatPrompt(req)
	for _, want := range []string{"get_weather", "Get the current weather", "What's the weather?"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestFormatPromptWithFunctionCall(t *testing.T) {
	req := &model.LLMRequest{
		Config: &genai.GenerateContentConfig{},
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{genai.NewPartFromText("Weather in Paris")}},
			{Role: "model", Parts: []*genai.Part{
				genai.NewPartFromFunctionCall("get_weather", map[string]any{"location": "Paris"}),
			}},
			{Role: "tool", Parts: []*genai.Part{
				genai.NewPartFromFunctionResponse("get_weather", map[string]any{"temp": 22}),
			}},
		},
	}
	prompt := FormatPrompt(req)
	for _, want := range []string{"get_weather", "Paris"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestFormatPromptGeneric(t *testing.T) {
	req := &model.LLMRequest{
		Config: &genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{genai.NewPartFromText("Be brief.")},
			},
		},
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{genai.NewPartFromText("Hi")}},
		},
	}
	prompt := FormatPromptGeneric(req)
	for _, want := range []string{"System: Be brief.", "User: Hi", "Assistant:"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestFormatPromptNoConfig(t *testing.T) {
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{genai.NewPartFromText("Hi")}},
		},
	}
	prompt := FormatPrompt(req)
	if !strings.Contains(prompt, "Hi") {
		t.Error("prompt missing user message")
	}
	if strings.Contains(prompt, "<|start_header_id|>system<|end_header_id|>") {
		t.Error("prompt should not contain system header when no system instruction")
	}
}

func TestParseToolCalls(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantCalls  int
		wantName   string
		wantArgs   map[string]any
		wantRemain string
	}{
		{
			name:      "simple tool call",
			input:     `{"name": "get_weather", "parameters": {"location": "Paris"}}`,
			wantCalls: 1,
			wantName:  "get_weather",
			wantArgs:  map[string]any{"location": "Paris"},
		},
		{
			name:      "with python tag",
			input:     "<|python_tag|>\n{\"name\": \"search\", \"parameters\": {\"q\": \"test\"}}",
			wantCalls: 1,
			wantName:  "search",
			wantArgs:  map[string]any{"q": "test"},
		},
		{
			name:       "text with tool call",
			input:      "Let me check that. {\"name\": \"lookup\", \"parameters\": {\"id\": 42}}",
			wantCalls:  1,
			wantName:   "lookup",
			wantRemain: "Let me check that.",
		},
		{
			name:       "no tool call",
			input:      "Just a normal response without any JSON.",
			wantCalls:  0,
			wantRemain: "Just a normal response without any JSON.",
		},
		{
			name:       "non-tool JSON",
			input:      `Here is some data: {"key": "value"}`,
			wantCalls:  0,
			wantRemain: `Here is some data: {"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls, remaining := ParseToolCalls(tt.input)
			if len(calls) != tt.wantCalls {
				t.Fatalf("got %d calls, want %d", len(calls), tt.wantCalls)
			}
			if tt.wantCalls > 0 {
				if calls[0].Name != tt.wantName {
					t.Errorf("got name %q, want %q", calls[0].Name, tt.wantName)
				}
			}
			if tt.wantRemain != "" && remaining != tt.wantRemain {
				t.Errorf("got remaining %q, want %q", remaining, tt.wantRemain)
			}
		})
	}
}

func TestModelLLM(t *testing.T) {
	modelPath := os.Getenv("ML_TEST_MODEL")
	if modelPath == "" {
		t.Skip("set ML_TEST_MODEL to a GGUF file path")
	}

	cfg := &MLConfig{
		ModelBase: registry.ModelBase{Provider: "ml", ModelID: "test"},
	}
	cfg.ModelPath = modelPath

	m, err := NewMLModel(cfg)
	if err != nil {
		t.Fatalf("NewMLModel: %v", err)
	}

	maxTokens := int32(50)
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{genai.NewPartFromText("Hello!")}},
		},
		Config: &genai.GenerateContentConfig{
			MaxOutputTokens: maxTokens,
		},
	}

	var lastResp *model.LLMResponse
	for resp, err := range m.GenerateContent(context.Background(), req, false) {
		if err != nil {
			t.Fatalf("GenerateContent: %v", err)
		}
		lastResp = resp
	}
	if lastResp == nil {
		t.Fatal("no response received")
	}
	if lastResp.Content == nil {
		t.Fatal("response content is nil")
	}
	if lastResp.Partial {
		t.Error("final response should not be partial")
	}
	if lastResp.UsageMetadata == nil {
		t.Fatal("usage metadata is nil")
	}
	if lastResp.UsageMetadata.PromptTokenCount <= 0 {
		t.Error("prompt token count should be positive")
	}
	t.Logf("Response: %+v", lastResp.Content)
}

func TestModelLLMStreaming(t *testing.T) {
	modelPath := os.Getenv("ML_TEST_MODEL")
	if modelPath == "" {
		t.Skip("set ML_TEST_MODEL to a GGUF file path")
	}

	cfg := &MLConfig{
		ModelBase: registry.ModelBase{Provider: "ml", ModelID: "test"},
	}
	cfg.ModelPath = modelPath

	m, err := NewMLModel(cfg)
	if err != nil {
		t.Fatalf("NewMLModel: %v", err)
	}

	maxTokens := int32(20)
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{genai.NewPartFromText("Count to 5.")}},
		},
		Config: &genai.GenerateContentConfig{
			MaxOutputTokens: maxTokens,
		},
	}

	var responses []*model.LLMResponse
	for resp, err := range m.GenerateContent(context.Background(), req, true) {
		if err != nil {
			t.Fatalf("GenerateContent streaming: %v", err)
		}
		responses = append(responses, resp)
	}

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses (partial + final), got %d", len(responses))
	}

	// All but last should be partial.
	for i, r := range responses[:len(responses)-1] {
		if !r.Partial {
			t.Errorf("response %d should be partial", i)
		}
	}

	// Last should be final.
	last := responses[len(responses)-1]
	if last.Partial {
		t.Error("final response should not be partial")
	}
	if last.UsageMetadata == nil {
		t.Error("final response should have usage metadata")
	}
}
