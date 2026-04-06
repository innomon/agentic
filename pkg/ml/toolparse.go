package ml

import (
	"encoding/json"
	"strings"

	"google.golang.org/genai"
)

// ParseToolCalls attempts to detect and parse tool calls from model output.
// Returns any found function calls and the remaining non-tool-call text.
func ParseToolCalls(output string) ([]*genai.FunctionCall, string) {
	// Strip the <|python_tag|> marker if present.
	cleaned := strings.TrimSpace(output)
	cleaned = strings.TrimPrefix(cleaned, "<|python_tag|>")
	cleaned = strings.TrimSpace(cleaned)

	var calls []*genai.FunctionCall
	var remaining strings.Builder

	// Try to find JSON objects with "name" and "parameters" keys.
	// Walk through the string looking for top-level '{' ... '}' blocks.
	i := 0
	for i < len(cleaned) {
		idx := strings.Index(cleaned[i:], "{")
		if idx < 0 {
			remaining.WriteString(cleaned[i:])
			break
		}

		// Append text before the brace.
		remaining.WriteString(cleaned[i : i+idx])
		braceStart := i + idx

		// Find the matching closing brace.
		end := findMatchingBrace(cleaned, braceStart)
		if end < 0 {
			remaining.WriteString(cleaned[braceStart:])
			break
		}

		candidate := cleaned[braceStart : end+1]
		if fc := tryParseToolCall(candidate); fc != nil {
			calls = append(calls, fc)
		} else {
			remaining.WriteString(candidate)
		}

		i = end + 1
	}

	return calls, strings.TrimSpace(remaining.String())
}

// findMatchingBrace returns the index of the closing '}' that matches the
// opening '{' at position start, accounting for nesting and quoted strings.
// Returns -1 if no match is found.
func findMatchingBrace(s string, start int) int {
	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(s); i++ {
		ch := s[i]

		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' && inString {
			escaped = true
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// tryParseToolCall attempts to parse a JSON string as a tool call with "name"
// and "parameters" keys. Returns nil if the JSON doesn't match the expected
// structure.
func tryParseToolCall(s string) *genai.FunctionCall {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil
	}

	nameRaw, hasName := raw["name"]
	paramsRaw, hasParams := raw["parameters"]
	if !hasName || !hasParams {
		return nil
	}

	var name string
	if err := json.Unmarshal(nameRaw, &name); err != nil {
		return nil
	}

	var params map[string]any
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return nil
	}

	return &genai.FunctionCall{
		Name: name,
		Args: params,
	}
}
