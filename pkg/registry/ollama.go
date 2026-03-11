package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// OllamaModel implements the ADK model.LLM interface using the official OpenAI Go SDK
// with a custom base URL for Ollama compatibility.
type OllamaModel struct {
	client    openai.Client
	modelName string
}

// NewOllamaModel creates a new OllamaModel with the specified model name and base URL.
func NewOllamaModel(modelName string, baseURL string) *OllamaModel {
	client := openai.NewClient(
		option.WithBaseURL(baseURL),
		option.WithAPIKey("ollama"), // Ollama doesn't require an API key
	)
	return &OllamaModel{
		client:    client,
		modelName: modelName,
	}
}

// Name returns the model name.
func (m *OllamaModel) Name() string {
	return m.modelName
}

// GenerateContent implements the model.LLM interface.
func (m *OllamaModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		chatReq := m.buildChatRequest(req)

		if stream {
			m.handleStreaming(ctx, chatReq, yield)
		} else {
			m.handleNonStreaming(ctx, chatReq, yield)
		}
	}
}

func (m *OllamaModel) handleNonStreaming(ctx context.Context, req openai.ChatCompletionNewParams, yield func(*model.LLMResponse, error) bool) {
	resp, err := m.client.Chat.Completions.New(ctx, req)
	if err != nil {
		yield(nil, fmt.Errorf("ollama chat completion failed: %w", err))
		return
	}

	llmResp := m.convertResponse(resp)
	yield(llmResp, nil)
}

type aggregatedToolCall struct {
	ID   string
	Type string
	Name string
	Args string
}

func (m *OllamaModel) handleStreaming(ctx context.Context, req openai.ChatCompletionNewParams, yield func(*model.LLMResponse, error) bool) {
	stream := m.client.Chat.Completions.NewStreaming(ctx, req)

	var aggregatedContent strings.Builder
	var toolCalls []aggregatedToolCall
	var finishReason string

	for stream.Next() {
		chunk := stream.Current()

		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]

			if choice.Delta.Content != "" {
				aggregatedContent.WriteString(choice.Delta.Content)
			}

			// Aggregate tool calls
			for _, tc := range choice.Delta.ToolCalls {
				idx := int(tc.Index)
				for len(toolCalls) <= idx {
					toolCalls = append(toolCalls, aggregatedToolCall{})
				}
				if tc.ID != "" {
					toolCalls[idx].ID = tc.ID
				}
				if tc.Type != "" {
					toolCalls[idx].Type = string(tc.Type)
				}
				if tc.Function.Name != "" {
					toolCalls[idx].Name = tc.Function.Name
				}
				toolCalls[idx].Args += tc.Function.Arguments
			}

			if choice.FinishReason != "" {
				finishReason = string(choice.FinishReason)
			}

			// Yield partial response
			partialResp := &model.LLMResponse{
				Partial: true,
				Content: &genai.Content{
					Parts: []*genai.Part{genai.NewPartFromText(aggregatedContent.String())},
					Role:  "model",
				},
			}
			if !yield(partialResp, nil) {
				return
			}
		}
	}

	if err := stream.Err(); err != nil {
		yield(nil, fmt.Errorf("ollama streaming failed: %w", err))
		return
	}

	// Build final response
	finalResp := &model.LLMResponse{
		Partial: false,
		Content: &genai.Content{
			Parts: []*genai.Part{},
			Role:  "model",
		},
	}

	if aggregatedContent.Len() > 0 {
		finalResp.Content.Parts = append(finalResp.Content.Parts, genai.NewPartFromText(aggregatedContent.String()))
	}

	// Add tool calls
	for _, tc := range toolCalls {
		if tc.Name != "" {
			var args map[string]any
			if tc.Args != "" {
				json.Unmarshal([]byte(tc.Args), &args)
			}
			part := genai.NewPartFromFunctionCall(tc.Name, args)
			if tc.ID != "" {
				part.FunctionCall.ID = tc.ID
			}
			finalResp.Content.Parts = append(finalResp.Content.Parts, part)
		}
	}

	// Set finish reason
	switch finishReason {
	case "stop":
		finalResp.FinishReason = genai.FinishReasonStop
	case "tool_calls":
		finalResp.FinishReason = genai.FinishReasonStop
	case "length":
		finalResp.FinishReason = genai.FinishReasonMaxTokens
	}

	yield(finalResp, nil)
}

func (m *OllamaModel) buildChatRequest(req *model.LLMRequest) openai.ChatCompletionNewParams {
	params := openai.ChatCompletionNewParams{
		Model: m.modelName,
	}

	// Add system instruction from Config
	if req.Config != nil && req.Config.SystemInstruction != nil {
		for _, part := range req.Config.SystemInstruction.Parts {
			if part.Text != "" {
				params.Messages = append(params.Messages, openai.SystemMessage(part.Text))
			}
		}
	}

	// Convert messages
	for _, content := range req.Contents {
		msgs := m.convertContentToMessages(content)
		params.Messages = append(params.Messages, msgs...)
	}

	// Convert tools from Config
	if req.Config != nil && len(req.Config.Tools) > 0 {
		for _, tool := range req.Config.Tools {
			for _, decl := range tool.FunctionDeclarations {
				params.Tools = append(params.Tools, openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
					Name:        decl.Name,
					Description: openai.String(decl.Description),
					Parameters:  m.convertSchemaToParams(decl.Parameters),
				}))
			}
		}
	}

	// Apply generation config
	if req.Config != nil {
		if req.Config.Temperature != nil {
			params.Temperature = openai.Float(float64(*req.Config.Temperature))
		}
		if req.Config.MaxOutputTokens != 0 {
			params.MaxTokens = openai.Int(int64(req.Config.MaxOutputTokens))
		}
		if req.Config.TopP != nil {
			params.TopP = openai.Float(float64(*req.Config.TopP))
		}
		if len(req.Config.StopSequences) > 0 {
			params.Stop = openai.ChatCompletionNewParamsStopUnion{
				OfStringArray: req.Config.StopSequences,
			}
		}
	}

	return params
}

func (m *OllamaModel) convertSchemaToParams(schema *genai.Schema) shared.FunctionParameters {
	if schema == nil {
		return nil
	}
	// Convert genai.Schema to a map for OpenAI
	schemaMap := map[string]any{
		"type": strings.ToLower(string(schema.Type)),
	}
	if schema.Description != "" {
		schemaMap["description"] = schema.Description
	}
	if len(schema.Properties) > 0 {
		props := make(map[string]any)
		for name, prop := range schema.Properties {
			propMap := map[string]any{
				"type": strings.ToLower(string(prop.Type)),
			}
			if prop.Description != "" {
				propMap["description"] = prop.Description
			}
			props[name] = propMap
		}
		schemaMap["properties"] = props
	}
	if len(schema.Required) > 0 {
		schemaMap["required"] = schema.Required
	}
	return schemaMap
}

func (m *OllamaModel) convertContentToMessages(content *genai.Content) []openai.ChatCompletionMessageParamUnion {
	var messages []openai.ChatCompletionMessageParamUnion
	role := content.Role

	switch role {
	case "user":
		var parts []openai.ChatCompletionContentPartUnionParam
		for _, part := range content.Parts {
			if part.Text != "" {
				parts = append(parts, openai.TextContentPart(part.Text))
			} else if part.InlineData != nil {
				dataURL := fmt.Sprintf("data:%s;base64,%s", part.InlineData.MIMEType, base64.StdEncoding.EncodeToString(part.InlineData.Data))
				parts = append(parts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
					URL: dataURL,
				}))
			}
		}
		messages = append(messages, openai.UserMessage(parts))

	case "model":
		var textContent string
		var toolCalls []openai.ChatCompletionMessageToolCallUnionParam
		for _, part := range content.Parts {
			if part.Text != "" {
				textContent += part.Text
			} else if part.FunctionCall != nil {
				argsJSON, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID: part.FunctionCall.ID,
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name:      part.FunctionCall.Name,
							Arguments: string(argsJSON),
						},
					},
				})
			}
		}
		if len(toolCalls) > 0 {
			msg := openai.ChatCompletionAssistantMessageParam{
				ToolCalls: toolCalls,
			}
			if textContent != "" {
				msg.Content = openai.ChatCompletionAssistantMessageParamContentUnion{OfString: openai.String(textContent)}
			}
			messages = append(messages, openai.ChatCompletionMessageParamUnion{OfAssistant: &msg})
		} else {
			messages = append(messages, openai.AssistantMessage(textContent))
		}

	case "tool":
		for _, part := range content.Parts {
			if part.FunctionResponse != nil {
				respJSON, _ := json.Marshal(part.FunctionResponse.Response)
				messages = append(messages, openai.ToolMessage(string(respJSON), part.FunctionResponse.ID))
			}
		}
	}

	return messages
}

func (m *OllamaModel) convertResponse(resp *openai.ChatCompletion) *model.LLMResponse {
	llmResp := &model.LLMResponse{
		Partial: false,
		Content: &genai.Content{
			Parts: []*genai.Part{},
			Role:  "model",
		},
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]

		// Add text content
		if choice.Message.Content != "" {
			llmResp.Content.Parts = append(llmResp.Content.Parts, genai.NewPartFromText(choice.Message.Content))
		}

		// Add tool calls
		for _, tc := range choice.Message.ToolCalls {
			funcCall := tc.AsFunction()
			var args map[string]any
			if funcCall.Function.Arguments != "" {
				json.Unmarshal([]byte(funcCall.Function.Arguments), &args)
			}
			part := genai.NewPartFromFunctionCall(funcCall.Function.Name, args)
			part.FunctionCall.ID = tc.ID
			llmResp.Content.Parts = append(llmResp.Content.Parts, part)
		}

		// Set finish reason
		switch choice.FinishReason {
		case "stop":
			llmResp.FinishReason = genai.FinishReasonStop
		case "tool_calls":
			llmResp.FinishReason = genai.FinishReasonStop
		case "length":
			llmResp.FinishReason = genai.FinishReasonMaxTokens
		}
	}

	// Add usage metadata
	llmResp.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
		PromptTokenCount:     int32(resp.Usage.PromptTokens),
		CandidatesTokenCount: int32(resp.Usage.CompletionTokens),
		TotalTokenCount:      int32(resp.Usage.TotalTokens),
	}

	return llmResp
}
