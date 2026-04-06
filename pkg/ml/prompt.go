package ml

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// FormatPrompt converts an LLMRequest into a prompt string using the LLaMA 3 chat template.
func FormatPrompt(req *model.LLMRequest) string {
	var b strings.Builder

	b.WriteString("<|begin_of_text|>")

	// System instruction.
	systemText := extractSystemInstruction(req)
	if systemText != "" {
		b.WriteString("<|start_header_id|>system<|end_header_id|>\n\n")
		b.WriteString(systemText)
		b.WriteString("<|eot_id|>")
	}

	// Conversation turns.
	for _, content := range req.Contents {
		switch content.Role {
		case "user":
			b.WriteString("<|start_header_id|>user<|end_header_id|>\n\n")
			b.WriteString(extractText(content.Parts))
			b.WriteString("<|eot_id|>")

		case "model":
			b.WriteString("<|start_header_id|>assistant<|end_header_id|>\n\n")
			b.WriteString(formatModelParts(content.Parts))
			b.WriteString("<|eot_id|>")

		case "tool":
			b.WriteString("<|start_header_id|>tool<|end_header_id|>\n\n")
			b.WriteString(formatToolParts(content.Parts))
			b.WriteString("<|eot_id|>")
		}
	}

	// Open assistant turn for generation.
	b.WriteString("<|start_header_id|>assistant<|end_header_id|>\n\n")

	return b.String()
}

// FormatPromptGranite converts an LLMRequest into a Granite-style chat template prompt.
// Uses <|start_of_role|>role<|end_of_role|> delimiters with <|end_of_text|> turn endings.
func FormatPromptGranite(req *model.LLMRequest) string {
	var b strings.Builder

	// System instruction.
	systemText := extractSystemInstruction(req)
	if systemText != "" {
		b.WriteString("<|start_of_role|>system<|end_of_role|>")
		b.WriteString(systemText)
		b.WriteString("<|end_of_text|>\n")
	}

	// Conversation turns.
	for _, content := range req.Contents {
		switch content.Role {
		case "user":
			b.WriteString("<|start_of_role|>user<|end_of_role|>")
			b.WriteString(extractText(content.Parts))
			b.WriteString("<|end_of_text|>\n")

		case "model":
			b.WriteString("<|start_of_role|>assistant<|end_of_role|>")
			b.WriteString(formatModelParts(content.Parts))
			b.WriteString("<|end_of_text|>\n")

		case "tool":
			b.WriteString("<|start_of_role|>tool<|end_of_role|>")
			b.WriteString(formatToolParts(content.Parts))
			b.WriteString("<|end_of_text|>\n")
		}
	}

	// Open assistant turn for generation.
	b.WriteString("<|start_of_role|>assistant<|end_of_role|>")

	return b.String()
}

// FormatPromptGeneric converts an LLMRequest into a simple generic prompt format
// suitable for models that don't use the LLaMA 3 chat template.
func FormatPromptGeneric(req *model.LLMRequest) string {
	var b strings.Builder

	systemText := extractSystemInstruction(req)
	if systemText != "" {
		b.WriteString("System: ")
		b.WriteString(systemText)
		b.WriteString("\n\n")
	}

	for _, content := range req.Contents {
		switch content.Role {
		case "user":
			b.WriteString("User: ")
			b.WriteString(extractText(content.Parts))
			b.WriteString("\n")
		case "model":
			b.WriteString("Assistant: ")
			b.WriteString(formatModelParts(content.Parts))
			b.WriteString("\n")
		case "tool":
			b.WriteString("Tool: ")
			b.WriteString(formatToolParts(content.Parts))
			b.WriteString("\n")
		}
	}

	b.WriteString("Assistant:")
	return b.String()
}

// extractSystemInstruction builds the system message from config, including tool
// descriptions if tools are declared.
func extractSystemInstruction(req *model.LLMRequest) string {
	var parts []string

	if req.Config != nil && req.Config.SystemInstruction != nil {
		if text := extractText(req.Config.SystemInstruction.Parts); text != "" {
			parts = append(parts, text)
		}
	}

	if req.Config != nil && len(req.Config.Tools) > 0 {
		toolDesc := formatToolDeclarations(req.Config.Tools)
		if toolDesc != "" {
			parts = append(parts, toolDesc)
		}
	}

	return strings.Join(parts, "\n\n")
}

// extractText concatenates all text parts from a slice of genai.Part.
func extractText(parts []*genai.Part) string {
	var texts []string
	for _, p := range parts {
		if p.Text != "" {
			texts = append(texts, p.Text)
		}
	}
	return strings.Join(texts, "")
}

// formatModelParts formats model output parts, including function calls.
func formatModelParts(parts []*genai.Part) string {
	var texts []string
	for _, p := range parts {
		if p.Text != "" {
			texts = append(texts, p.Text)
		}
		if p.FunctionCall != nil {
			call := map[string]any{
				"name":       p.FunctionCall.Name,
				"parameters": p.FunctionCall.Args,
			}
			data, _ := json.Marshal(call)
			texts = append(texts, string(data))
		}
	}
	return strings.Join(texts, "")
}

// formatToolParts formats tool response parts.
func formatToolParts(parts []*genai.Part) string {
	var texts []string
	for _, p := range parts {
		if p.Text != "" {
			texts = append(texts, p.Text)
		}
		if p.FunctionResponse != nil {
			resp := map[string]any{
				"name":     p.FunctionResponse.Name,
				"response": p.FunctionResponse.Response,
			}
			data, _ := json.Marshal(resp)
			texts = append(texts, string(data))
		}
	}
	return strings.Join(texts, "")
}

// formatToolDeclarations builds a text description of available tools for the
// system prompt.
func formatToolDeclarations(tools []*genai.Tool) string {
	var b strings.Builder
	b.WriteString("You have access to the following tools. To use a tool, respond with a JSON object containing \"name\" and \"parameters\" keys.\n\nAvailable tools:")

	for _, tool := range tools {
		for _, decl := range tool.FunctionDeclarations {
			b.WriteString(fmt.Sprintf("\n- %s: %s", decl.Name, decl.Description))
			if decl.Parameters != nil && len(decl.Parameters.Properties) > 0 {
				b.WriteString("\n  Parameters:")
				for name, prop := range decl.Parameters.Properties {
					b.WriteString(fmt.Sprintf("\n    %s (%s): %s", name, prop.Type, prop.Description))
				}
			}
		}
	}

	return b.String()
}
