package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the proxy configuration
type Config struct {
	Proxy ProxyConfig `yaml:"proxy"`
}

type ProxyConfig struct {
	Listen   string       `yaml:"listen"`
	ADK      ADKConfig    `yaml:"adk"`
	Defaults DefaultsConf `yaml:"defaults"`
}

type ADKConfig struct {
	Endpoint string `yaml:"endpoint"`
	AppName  string `yaml:"app_name"`
}

type DefaultsConf struct {
	UserID string `yaml:"user_id"`
}

// OpenAI Request/Response Types (following Ollama patterns)

type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type ResponseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
}

type JSONSchema struct {
	Schema json.RawMessage `json:"schema"`
}

type ChatCompletionRequest struct {
	Model            string          `json:"model"`
	Messages         []Message       `json:"messages"`
	Stream           bool            `json:"stream"`
	StreamOptions    *StreamOptions  `json:"stream_options"`
	MaxTokens        *int            `json:"max_tokens"`
	Temperature      *float64        `json:"temperature"`
	TopP             *float64        `json:"top_p"`
	Stop             any             `json:"stop"`
	FrequencyPenalty *float64        `json:"frequency_penalty"`
	PresencePenalty  *float64        `json:"presence_penalty"`
	ResponseFormat   *ResponseFormat `json:"response_format"`
}

type ChatCompletion struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	SystemFingerprint string   `json:"system_fingerprint"`
	Choices           []Choice `json:"choices"`
	Usage             Usage    `json:"usage,omitempty"`
}

type ChatCompletionChunk struct {
	ID                string        `json:"id"`
	Object            string        `json:"object"`
	Created           int64         `json:"created"`
	Model             string        `json:"model"`
	SystemFingerprint string        `json:"system_fingerprint"`
	Choices           []ChunkChoice `json:"choices"`
	Usage             *Usage        `json:"usage,omitempty"`
}

type Choice struct {
	Index        int      `json:"index"`
	Message      Message  `json:"message"`
	FinishReason *string  `json:"finish_reason"`
	Logprobs     *any     `json:"logprobs"`
}

type ChunkChoice struct {
	Index        int     `json:"index"`
	Delta        Message `json:"delta"`
	FinishReason *string `json:"finish_reason"`
	Logprobs     *any    `json:"logprobs"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// ADK Types

type ADKContent struct {
	Role  string    `json:"role"`
	Parts []ADKPart `json:"parts"`
}

type ADKPart struct {
	Text       string         `json:"text,omitempty"`
	InlineData *ADKInlineData `json:"inlineData,omitempty"`
}

type ADKInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type ADKRunRequest struct {
	AppName    string     `json:"appName"`
	UserID     string     `json:"userId"`
	SessionID  string     `json:"sessionId"`
	Streaming  bool       `json:"streaming"`
	NewMessage ADKContent `json:"newMessage"`
}

type ADKSessionResponse struct {
	ID string `json:"id"`
}

type ADKEvent struct {
	ID           string          `json:"id"`
	Time         int64           `json:"time"`
	Author       string          `json:"author"`
	Content      *ADKEventContent `json:"content,omitempty"`
	ErrorCode    string          `json:"errorCode,omitempty"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	Partial      bool            `json:"partial,omitempty"`
	TurnComplete bool            `json:"turnComplete,omitempty"`
}

type ADKEventContent struct {
	Role  string    `json:"role"`
	Parts []ADKPart `json:"parts"`
}

// Server

type Server struct {
	config *Config
	client *http.Client
}

func NewServer(config *Config) *Server {
	return &Server{
		config: config,
		client: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (s *Server) createSession(ctx context.Context, userID string) (string, error) {
	url := fmt.Sprintf("%s/api/apps/%s/users/%s/sessions",
		s.config.Proxy.ADK.Endpoint,
		s.config.Proxy.ADK.AppName,
		userID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return "", fmt.Errorf("create session request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create session failed: %s", string(body))
	}

	var session ADKSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return "", fmt.Errorf("decode session response: %w", err)
	}

	return session.ID, nil
}

func (s *Server) convertToADKContent(messages []Message) (ADKContent, error) {
	var parts []ADKPart

	for _, msg := range messages {
		var prefix string
		switch msg.Role {
		case "system":
			prefix = "[System]: "
		case "user":
			prefix = ""
		case "assistant":
			prefix = "[Assistant]: "
		case "tool":
			prefix = fmt.Sprintf("[Tool %s]: ", msg.Name)
		default:
			prefix = fmt.Sprintf("[%s]: ", msg.Role)
		}

		switch content := msg.Content.(type) {
		case string:
			parts = append(parts, ADKPart{Text: prefix + content})

		case []any:
			for _, c := range content {
				data, ok := c.(map[string]any)
				if !ok {
					continue
				}
				switch data["type"] {
				case "text":
					if text, ok := data["text"].(string); ok {
						parts = append(parts, ADKPart{Text: prefix + text})
					}
				case "image_url":
					if urlData, ok := data["image_url"].(map[string]any); ok {
						if url, ok := urlData["url"].(string); ok {
							if part := s.convertImageURL(url); part != nil {
								parts = append(parts, *part)
							}
						}
					}
				}
			}
		}
	}

	return ADKContent{
		Role:  "user",
		Parts: parts,
	}, nil
}

func (s *Server) convertImageURL(url string) *ADKPart {
	if strings.HasPrefix(url, "data:") {
		parts := strings.SplitN(url, ",", 2)
		if len(parts) != 2 {
			return nil
		}
		mimeType := "image/png"
		if strings.Contains(parts[0], "image/jpeg") {
			mimeType = "image/jpeg"
		} else if strings.Contains(parts[0], "image/gif") {
			mimeType = "image/gif"
		} else if strings.Contains(parts[0], "image/webp") {
			mimeType = "image/webp"
		} else if strings.Contains(parts[0], "application/pdf") {
			mimeType = "application/pdf"
		}
		return &ADKPart{
			InlineData: &ADKInlineData{
				MimeType: mimeType,
				Data:     parts[1],
			},
		}
	}
	return nil
}

func (s *Server) runADK(ctx context.Context, userID, sessionID string, content ADKContent, streaming bool) (*http.Response, error) {
	adkReq := ADKRunRequest{
		AppName:    s.config.Proxy.ADK.AppName,
		UserID:     userID,
		SessionID:  sessionID,
		Streaming:  streaming,
		NewMessage: content,
	}

	body, err := json.Marshal(adkReq)
	if err != nil {
		return nil, fmt.Errorf("marshal adk request: %w", err)
	}

	url := fmt.Sprintf("%s/api/run_sse", s.config.Proxy.ADK.Endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create run request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return s.client.Do(req)
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		s.handleCORS(w)
		return
	}

	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed")
		return
	}

	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("Invalid request: %v", err))
		return
	}

	userID := s.config.Proxy.Defaults.UserID
	if userID == "" {
		userID = "openai-proxy-user"
	}

	sessionID, err := s.createSession(r.Context(), userID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "session_error", fmt.Sprintf("Failed to create session: %v", err))
		return
	}

	content, err := s.convertToADKContent(req.Messages)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "conversion_error", fmt.Sprintf("Failed to convert messages: %v", err))
		return
	}

	resp, err := s.runADK(r.Context(), userID, sessionID, content, req.Stream)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "adk_error", fmt.Sprintf("Failed to run ADK: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.writeError(w, resp.StatusCode, "adk_error", fmt.Sprintf("ADK error: %s", string(body)))
		return
	}

	id := fmt.Sprintf("chatcmpl-%d", rand.Intn(999999999))

	if req.Stream {
		s.handleStreamingResponse(w, resp.Body, id, req.Model, req.StreamOptions)
	} else {
		s.handleNonStreamingResponse(w, resp.Body, id, req.Model)
	}
}

func (s *Server) handleStreamingResponse(w http.ResponseWriter, body io.Reader, id, model string, streamOpts *StreamOptions) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "streaming_error", "Streaming not supported")
		return
	}

	scanner := bufio.NewScanner(body)
	var fullContent strings.Builder
	sentRole := false

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "" {
			continue
		}

		var event ADKEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if event.ErrorCode != "" {
			chunk := s.createErrorChunk(id, model, event.ErrorMessage)
			s.writeSSEChunk(w, flusher, chunk)
			continue
		}

		if event.Content == nil || len(event.Content.Parts) == 0 {
			if event.TurnComplete {
				finishReason := "stop"
				chunk := ChatCompletionChunk{
					ID:                id,
					Object:            "chat.completion.chunk",
					Created:           time.Now().Unix(),
					Model:             model,
					SystemFingerprint: "fp_adk",
					Choices: []ChunkChoice{{
						Index:        0,
						Delta:        Message{},
						FinishReason: &finishReason,
					}},
				}
				s.writeSSEChunk(w, flusher, chunk)
			}
			continue
		}

		if event.Author == "user" {
			continue
		}

		for _, part := range event.Content.Parts {
			if part.Text == "" {
				continue
			}

			delta := Message{Content: part.Text}
			if !sentRole {
				delta.Role = "assistant"
				sentRole = true
			}

			fullContent.WriteString(part.Text)

			chunk := ChatCompletionChunk{
				ID:                id,
				Object:            "chat.completion.chunk",
				Created:           time.Now().Unix(),
				Model:             model,
				SystemFingerprint: "fp_adk",
				Choices: []ChunkChoice{{
					Index: 0,
					Delta: delta,
				}},
			}

			s.writeSSEChunk(w, flusher, chunk)
		}
	}

	if streamOpts != nil && streamOpts.IncludeUsage {
		usage := s.estimateUsage(fullContent.String())
		chunk := ChatCompletionChunk{
			ID:                id,
			Object:            "chat.completion.chunk",
			Created:           time.Now().Unix(),
			Model:             model,
			SystemFingerprint: "fp_adk",
			Choices:           []ChunkChoice{},
			Usage:             &usage,
		}
		s.writeSSEChunk(w, flusher, chunk)
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (s *Server) handleNonStreamingResponse(w http.ResponseWriter, body io.Reader, id, model string) {
	scanner := bufio.NewScanner(body)
	var fullContent strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "" {
			continue
		}

		var event ADKEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if event.ErrorCode != "" {
			s.writeError(w, http.StatusInternalServerError, event.ErrorCode, event.ErrorMessage)
			return
		}

		if event.Author == "user" {
			continue
		}

		if event.Content != nil {
			for _, part := range event.Content.Parts {
				fullContent.WriteString(part.Text)
			}
		}
	}

	finishReason := "stop"
	response := ChatCompletion{
		ID:                id,
		Object:            "chat.completion",
		Created:           time.Now().Unix(),
		Model:             model,
		SystemFingerprint: "fp_adk",
		Choices: []Choice{{
			Index: 0,
			Message: Message{
				Role:    "assistant",
				Content: fullContent.String(),
			},
			FinishReason: &finishReason,
		}},
		Usage: s.estimateUsage(fullContent.String()),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) createErrorChunk(id, model, message string) ChatCompletionChunk {
	return ChatCompletionChunk{
		ID:                id,
		Object:            "chat.completion.chunk",
		Created:           time.Now().Unix(),
		Model:             model,
		SystemFingerprint: "fp_adk",
		Choices: []ChunkChoice{{
			Index: 0,
			Delta: Message{Content: fmt.Sprintf("[Error: %s]", message)},
		}},
	}
}

func (s *Server) writeSSEChunk(w http.ResponseWriter, flusher http.Flusher, chunk ChatCompletionChunk) {
	data, _ := json.Marshal(chunk)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func (s *Server) estimateUsage(content string) Usage {
	tokens := len(content) / 4
	return Usage{
		PromptTokens:     100,
		CompletionTokens: tokens,
		TotalTokens:      100 + tokens,
	}
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		s.handleCORS(w)
		return
	}

	response := map[string]any{
		"object": "list",
		"data": []map[string]any{
			{
				"id":       s.config.Proxy.ADK.AppName,
				"object":   "model",
				"created":  time.Now().Unix(),
				"owned_by": "adk",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    "error",
			Code:    code,
		},
	})
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if config.Proxy.Listen == "" {
		config.Proxy.Listen = ":9080"
	}
	if config.Proxy.ADK.Endpoint == "" {
		config.Proxy.ADK.Endpoint = "http://localhost:8080"
	}
	if config.Proxy.ADK.AppName == "" {
		config.Proxy.ADK.AppName = "Agentic"
	}
	if config.Proxy.Defaults.UserID == "" {
		config.Proxy.Defaults.UserID = "openai-proxy-user"
	}

	return &config, nil
}

func main() {
	configPath := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	config, err := loadConfig(*configPath)
	if err != nil {
		if os.IsNotExist(err) {
			config = &Config{
				Proxy: ProxyConfig{
					Listen: ":9080",
					ADK: ADKConfig{
						Endpoint: "http://localhost:8080",
						AppName:  "Agentic",
					},
					Defaults: DefaultsConf{
						UserID: "openai-proxy-user",
					},
				},
			}
			log.Printf("Using default config (no config file found)")
		} else {
			log.Fatalf("Failed to load config: %v", err)
		}
	}

	server := NewServer(config)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", server.handleChatCompletions)
	mux.HandleFunc("/v1/models", server.handleModels)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Starting OpenAI Proxy on %s", config.Proxy.Listen)
	log.Printf("  ADK Endpoint: %s", config.Proxy.ADK.Endpoint)
	log.Printf("  App Name: %s", config.Proxy.ADK.AppName)
	log.Printf("  OpenAI API: http://localhost%s/v1/", config.Proxy.Listen)

	if err := http.ListenAndServe(config.Proxy.Listen, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
