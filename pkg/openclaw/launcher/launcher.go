package launcher

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/innomon/agentic/pkg/openclaw/server"

	adklauncher "google.golang.org/adk/v2/cmd/launcher"
	weblauncher "google.golang.org/adk/v2/cmd/launcher/web"
)

type openclawConfig struct {
	wsPath string
}

type openclawLauncher struct {
	flags  *flag.FlagSet
	config *openclawConfig
}

// NewLauncher creates a new OpenClaw web sublauncher that adds the
// WebSocket gateway to the shared web server.
func NewLauncher() weblauncher.Sublauncher {
	cfg := &openclawConfig{}
	fs := flag.NewFlagSet("openclaw", flag.ContinueOnError)
	fs.StringVar(&cfg.wsPath, "ws-path", "/ws", "WebSocket endpoint path")
	return &openclawLauncher{flags: fs, config: cfg}
}

func (o *openclawLauncher) Keyword() string { return "openclaw" }

func (o *openclawLauncher) SimpleDescription() string {
	return "adds OpenClaw WebSocket gateway for agent communication"
}

func (o *openclawLauncher) CommandLineSyntax() string {
	return "  -ws-path string  WebSocket endpoint path (default \"/ws\")\n"
}

func (o *openclawLauncher) Parse(args []string) ([]string, error) {
	if err := o.flags.Parse(args); err != nil {
		return nil, fmt.Errorf("failed to parse openclaw flags: %w", err)
	}
	return o.flags.Args(), nil
}

func (o *openclawLauncher) SetupSubrouters(router *mux.Router, config *adklauncher.Config) error {
	bridge, err := server.NewAgentBridge(config)
	if err != nil {
		return fmt.Errorf("creating agent bridge: %w", err)
	}

	srv := server.New(server.Config{
		Path: o.config.wsPath,
	})
	srv.SetAgentHandler(bridge.Handler())

	router.Handle(o.config.wsPath, http.HandlerFunc(srv.ServeHTTP))
	return nil
}

func (o *openclawLauncher) UserMessage(webURL string, printer func(v ...any)) {
	printer(fmt.Sprintf("  openclaw:  WebSocket gateway at %s%s", webURL, o.config.wsPath))
	log.Printf("openclaw: gateway ready")
}
