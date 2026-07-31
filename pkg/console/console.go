package console

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Launcher implements a custom console with file attachment syntax.
// Usage: ./agentic console
// Then type messages with @/path/to/file syntax to attach files.
type Launcher struct {
	flags  *flag.FlagSet
	config *config
}

type config struct {
	streamingMode       agent.StreamingMode
	streamingModeString string
}

// New creates a new custom console launcher with file attachment support.
func New() *Launcher {
	cfg := &config{}
	fs := flag.NewFlagSet("console", flag.ContinueOnError)
	fs.StringVar(&cfg.streamingModeString, "streaming_mode", string(agent.StreamingModeSSE),
		"streaming mode (none|sse)")
	return &Launcher{config: cfg, flags: fs}
}

// Keyword returns the command keyword for this launcher.
func (l *Launcher) Keyword() string {
	return "console"
}

var help = `Console mode with file attachment support.

Usage: ./agentic console [options]

Attach files using @/path/to/file syntax:
  User -> Process this document @./document.pdf
  User -> Analyze these files @./image.png @./notes.txt

Commands:
  /help                 Show this help message
  /save [filename]      Save session to JSON file (default: session_<timestamp>.json)
  /mcp [endpoint]       List and verify MCP tools (default: use configured endpoints)
  /exit, /quit          Exit the console

Supported file types:
  PDF:    .pdf
  Images: .png, .jpg, .jpeg, .gif, .webp, .tiff
  Text:   .txt, .json, .csv

Options:
  --streaming_mode=sse  Streaming mode (none|sse)
`

// CommandLineSyntax returns usage documentation.
func (l *Launcher) CommandLineSyntax() string {
	return help
}

// SimpleDescription returns a short description.
func (l *Launcher) SimpleDescription() string {
	return "interactive console with file attachment support (@/path/to/file)"
}

// Parse parses command-line arguments.
func (l *Launcher) Parse(args []string) ([]string, error) {
	if err := l.flags.Parse(args); err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}
	mode := l.config.streamingModeString
	if mode != string(agent.StreamingModeNone) && mode != string(agent.StreamingModeSSE) {
		return nil, fmt.Errorf("invalid streaming_mode: %s", mode)
	}
	l.config.streamingMode = agent.StreamingMode(mode)
	return l.flags.Args(), nil
}

// consoleContext holds session state for commands
type consoleContext struct {
	ctx            context.Context
	sessionService session.Service
	session        session.Session
	appName        string
	userID         string
}

// commandResult represents the result of a command
type commandResult struct {
	output string
	exit   bool
}

// command handler type
type commandHandler func(cc *consoleContext, args string) (*commandResult, error)

// Run executes the console REPL.
func (l *Launcher) Run(ctx context.Context, cfg *launcher.Config) error {
	const (
		userID  = "console_user"
		appName = "Agentic"
	)

	sessionService := cfg.SessionService
	if sessionService == nil {
		sessionService = session.InMemoryService()
	}

	resp, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName: appName,
		UserID:  userID,
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	rootAgent := cfg.AgentLoader.RootAgent()
	sess := resp.Session

	r, err := runner.New(runner.Config{
		AppName:         appName,
		Agent:           rootAgent,
		SessionService:  sessionService,
		ArtifactService: cfg.ArtifactService,
	})
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	// Console context for commands
	cc := &consoleContext{
		ctx:            ctx,
		sessionService: sessionService,
		session:        sess,
		appName:        appName,
		userID:         userID,
	}

	// Command registry
	commands := map[string]commandHandler{
		"help": cmdHelp,
		"save": cmdSave,
		"mcp":  cmdMCP,
		"exit": cmdExit,
		"quit": cmdExit,
	}

	reader := bufio.NewReader(os.Stdin)
	filePattern := regexp.MustCompile(`@([^\s]+)`)

	fmt.Println("Agentic Console (attach files with @/path/to/file syntax)")
	fmt.Println("Example: Analyze this document @./document.pdf")
	fmt.Println("Type '/help' for commands, '/exit' to quit.")

	for {
		fmt.Print("User -> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle commands
		if strings.HasPrefix(input, "/") {
			cmdLine := input[1:]
			parts := strings.SplitN(cmdLine, " ", 2)
			cmdName := parts[0]
			cmdArgs := ""
			if len(parts) > 1 {
				cmdArgs = strings.TrimSpace(parts[1])
			}

			handler, ok := commands[cmdName]
			if !ok {
				fmt.Printf("Unknown command: /%s (type /help for commands)\n", cmdName)
				continue
			}

			result, err := handler(cc, cmdArgs)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			if result.output != "" {
				fmt.Println(result.output)
			}
			if result.exit {
				return nil
			}
			continue
		}

		userMsg, err := parseInput(input, filePattern)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		fmt.Print("\nAgent -> ")
		streamingMode := l.config.streamingMode
		if streamingMode == "" {
			streamingMode = agent.StreamingModeSSE
		}

		prevText := ""
		for event, err := range r.Run(ctx, userID, sess.ID(), userMsg, agent.RunConfig{
			StreamingMode: streamingMode,
		}) {
			if err != nil {
				fmt.Printf("\nError: %v\n", err)
				break
			}
			if event.Content == nil {
				continue
			}

			text := ""
			for _, p := range event.Content.Parts {
				text += p.Text
			}

			if streamingMode != agent.StreamingModeSSE {
				fmt.Print(text)
				continue
			}

			if !event.IsFinalResponse() {
				fmt.Print(text)
				prevText += text
				continue
			}

			if text != prevText {
				fmt.Print(text)
			}
			prevText = ""
		}
		fmt.Println()
	}
}

// parseInput parses user input and extracts file attachments.
func parseInput(input string, filePattern *regexp.Regexp) (*genai.Content, error) {
	matches := filePattern.FindAllStringSubmatch(input, -1)
	textContent := strings.TrimSpace(filePattern.ReplaceAllString(input, ""))

	var parts []*genai.Part

	if textContent != "" {
		parts = append(parts, genai.NewPartFromText(textContent))
	}

	for _, match := range matches {
		filePath := match[1]

		// Expand ~ to home directory
		if strings.HasPrefix(filePath, "~") {
			home, err := os.UserHomeDir()
			if err == nil {
				filePath = filepath.Join(home, filePath[1:])
			}
		}

		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return nil, fmt.Errorf("invalid path %q: %w", filePath, err)
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("cannot read %q: %w", filePath, err)
		}

		mimeType := getMIMEType(filePath)
		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: mimeType,
				Data:     data,
			},
		})

		fmt.Printf("[Attached: %s (%s, %d bytes)]\n", filepath.Base(filePath), mimeType, len(data))
	}

	if len(parts) == 0 {
		return nil, fmt.Errorf("empty message")
	}

	return &genai.Content{
		Role:  genai.RoleUser,
		Parts: parts,
	}, nil
}

// getMIMEType returns the MIME type based on file extension.
func getMIMEType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	types := map[string]string{
		".pdf":  "application/pdf",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
		".tiff": "image/tiff",
		".tif":  "image/tiff",
		".txt":  "text/plain",
		".json": "application/json",
		".csv":  "text/csv",
		".xml":  "application/xml",
		".html": "text/html",
	}
	if t, ok := types[ext]; ok {
		return t
	}
	return "application/octet-stream"
}

// Command handlers

func cmdHelp(_ *consoleContext, _ string) (*commandResult, error) {
	return &commandResult{output: help}, nil
}

func cmdMCP(cc *consoleContext, args string) (*commandResult, error) {
	endpoint := strings.TrimSpace(args)
	if endpoint == "" {
		return &commandResult{output: "Usage: /mcp <endpoint_url>\nExample: /mcp http://localhost:8082/mcp"}, nil
	}

	fmt.Printf("Connecting to MCP endpoint: %s...\n", endpoint)

	transport := &mcp.StreamableClientTransport{
		Endpoint: endpoint,
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "agentic-console-debug",
		Version: "1.0.0",
	}, nil)

	ctx, cancel := context.WithTimeout(cc.ctx, 15*time.Second)
	defer cancel()

	fmt.Println("Connecting and initializing session...")
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to init MCP session: %w", err)
	}
	defer session.Close()

	fmt.Println("Listing tools...")
	resp, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Successfully connected to %s\n", endpoint))
	sb.WriteString(fmt.Sprintf("Found %d tools:\n", len(resp.Tools)))
	for _, t := range resp.Tools {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
	}

	return &commandResult{output: sb.String()}, nil
}

func cmdExit(_ *consoleContext, _ string) (*commandResult, error) {
	return &commandResult{output: "Goodbye!", exit: true}, nil
}

func cmdSave(cc *consoleContext, args string) (*commandResult, error) {
	// Get session with events
	resp, err := cc.sessionService.Get(cc.ctx, &session.GetRequest{
		AppName:   cc.appName,
		UserID:    cc.userID,
		SessionID: cc.session.ID(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Determine filename
	filename := args
	if filename == "" {
		filename = fmt.Sprintf("session_%s.json", time.Now().Format("20060102_150405"))
	}
	if !strings.HasSuffix(filename, ".json") {
		filename += ".json"
	}

	// Build session export structure
	export := buildSessionExport(resp.Session)

	// Marshal to JSON
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &commandResult{
		output: fmt.Sprintf("Session saved to %s (%d bytes)", filename, len(data)),
	}, nil
}

// SessionExport represents the exported session format
type SessionExport struct {
	ID        string         `json:"id"`
	AppName   string         `json:"appName"`
	UserID    string         `json:"userID"`
	CreatedAt time.Time      `json:"createdAt"`
	Events    []EventExport  `json:"events"`
	State     map[string]any `json:"state,omitempty"`
}

// EventExport represents an exported event
type EventExport struct {
	ID           string    `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	Author       string    `json:"author"`
	Role         string    `json:"role"`
	Content      []PartExport `json:"content,omitempty"`
	FunctionCall *FunctionCallExport `json:"functionCall,omitempty"`
	FunctionResponse *FunctionResponseExport `json:"functionResponse,omitempty"`
}

// PartExport represents an exported content part
type PartExport struct {
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	DataSize int    `json:"dataSize,omitempty"` // Size of binary data (not exported)
}

// FunctionCallExport represents an exported function call
type FunctionCallExport struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// FunctionResponseExport represents an exported function response
type FunctionResponseExport struct {
	Name   string `json:"name"`
	Result any    `json:"result,omitempty"`
}

func buildSessionExport(sess session.Session) *SessionExport {
	export := &SessionExport{
		ID:        sess.ID(),
		AppName:   sess.AppName(),
		UserID:    sess.UserID(),
		CreatedAt: time.Now(),
		Events:    []EventExport{},
		State:     make(map[string]any),
	}

	// Export state
	if sess.State() != nil {
		for k, v := range sess.State().All() {
			export.State[k] = v
		}
	}

	// Export events
	if sess.Events() != nil {
		for event := range sess.Events().All() {
			eventExport := EventExport{
				ID:        event.ID,
				Timestamp: event.Timestamp,
				Author:    event.Author,
			}

			if event.Content != nil {
				eventExport.Role = string(event.Content.Role)
				for _, part := range event.Content.Parts {
					pe := PartExport{}
					if part.Text != "" {
						pe.Text = part.Text
					}
					if part.InlineData != nil {
						pe.MimeType = part.InlineData.MIMEType
						pe.DataSize = len(part.InlineData.Data)
					}
					if part.FunctionCall != nil {
						eventExport.FunctionCall = &FunctionCallExport{
							Name: part.FunctionCall.Name,
							Args: part.FunctionCall.Args,
						}
					}
					if part.FunctionResponse != nil {
						eventExport.FunctionResponse = &FunctionResponseExport{
							Name:   part.FunctionResponse.Name,
							Result: part.FunctionResponse.Response,
						}
					}
					if pe.Text != "" || pe.MimeType != "" {
						eventExport.Content = append(eventExport.Content, pe)
					}
				}
			}

			export.Events = append(export.Events, eventExport)
		}
	}

	return export
}
