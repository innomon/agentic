package console

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// Launcher implements a custom console with file attachment syntax.
// Usage: ./med-agent console
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

// CommandLineSyntax returns usage documentation.
func (l *Launcher) CommandLineSyntax() string {
	return `Console mode with file attachment support.

Usage: ./med-agent console [options]

Attach files using @/path/to/file syntax:
  User -> Create FHIR from this lab report @./document.pdf
  User -> Extract prescription @./prescription.png @./notes.txt

Supported file types:
  PDF:    .pdf
  Images: .png, .jpg, .jpeg, .gif, .webp, .tiff
  Text:   .txt, .json, .csv

Options:
  --streaming_mode=sse  Streaming mode (none|sse)
`
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

// Run executes the console REPL.
func (l *Launcher) Run(ctx context.Context, cfg *launcher.Config) error {
	const (
		userID  = "console_user"
		appName = "MedAgent"
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

	reader := bufio.NewReader(os.Stdin)
	filePattern := regexp.MustCompile(`@([^\s]+)`)

	fmt.Println("MedAgent Console (attach files with @/path/to/file syntax)")
	fmt.Println("Example: Create FHIR from this @./labtest.pdf")
	fmt.Println("Type 'exit' or 'quit' to exit.\n")

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
		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return nil
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
