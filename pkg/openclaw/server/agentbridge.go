package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/innomon/agentic/pkg/openclaw/protocol"
)

const agentAppName = "Agentic"

// AgentBridge bridges OpenClaw agent.* RPC methods to an ADK runner.
type AgentBridge struct {
	runner         *runner.Runner
	sessionService session.Service

	mu       sync.Mutex
	sessions map[string]string // connClientID -> sessionID
}

// NewAgentBridge creates an AgentBridge from a launcher.Config.
func NewAgentBridge(cfg *launcher.Config) (*AgentBridge, error) {
	sessionSvc := cfg.SessionService
	if sessionSvc == nil {
		sessionSvc = session.InMemoryService()
	}

	r, err := runner.New(runner.Config{
		AppName:         agentAppName,
		Agent:           cfg.AgentLoader.RootAgent(),
		SessionService:  sessionSvc,
		ArtifactService: cfg.ArtifactService,
		MemoryService:   cfg.MemoryService,
	})
	if err != nil {
		return nil, fmt.Errorf("creating runner: %w", err)
	}

	return &AgentBridge{
		runner:         r,
		sessionService: sessionSvc,
		sessions:       make(map[string]string),
	}, nil
}

// Handler returns a MethodHandler that routes agent.* methods.
func (ab *AgentBridge) Handler() MethodHandler {
	return func(ctx context.Context, conn *Conn, req *protocol.RequestFrame) (*protocol.ResponseFrame, error) {
		subMethod := strings.TrimPrefix(req.Method, "agent.")
		switch subMethod {
		case "session.create":
			return ab.handleSessionCreate(ctx, conn, req)
		case "send":
			return ab.handleSend(ctx, conn, req)
		default:
			return protocol.NewResponse(req.ID, false, nil, &protocol.ErrorObject{
				Message: "unknown agent method: " + subMethod,
				Code:    "ERR_UNKNOWN_METHOD",
			}), nil
		}
	}
}

// agentSessionCreateParams are the parameters for agent.session.create.
type agentSessionCreateParams struct {
	UserID string `json:"userId"`
}

func (ab *AgentBridge) handleSessionCreate(ctx context.Context, conn *Conn, req *protocol.RequestFrame) (*protocol.ResponseFrame, error) {
	var params agentSessionCreateParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return protocol.NewResponse(req.ID, false, nil, &protocol.ErrorObject{
				Message: "invalid params: " + err.Error(),
				Code:    "ERR_INVALID_PARAMS",
			}), nil
		}
	}
	userID := params.UserID
	if userID == "" {
		userID = conn.clientID
	}
	if userID == "" {
		userID = "anonymous"
	}

	resp, err := ab.sessionService.Create(ctx, &session.CreateRequest{
		AppName: agentAppName,
		UserID:  userID,
	})
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}

	sessionID := resp.Session.ID()

	ab.mu.Lock()
	ab.sessions[conn.clientID] = sessionID
	ab.mu.Unlock()

	return protocol.NewResponse(req.ID, true, map[string]string{
		"sessionId": sessionID,
	}, nil), nil
}

// agentSendParams are the parameters for agent.send.
type agentSendParams struct {
	SessionID string `json:"sessionId"`
	UserID    string `json:"userId"`
	Text      string `json:"text"`
}

func (ab *AgentBridge) handleSend(ctx context.Context, conn *Conn, req *protocol.RequestFrame) (*protocol.ResponseFrame, error) {
	var params agentSendParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return protocol.NewResponse(req.ID, false, nil, &protocol.ErrorObject{
				Message: "invalid params: " + err.Error(),
				Code:    "ERR_INVALID_PARAMS",
			}), nil
		}
	}

	if params.Text == "" {
		return protocol.NewResponse(req.ID, false, nil, &protocol.ErrorObject{
			Message: "text is required",
			Code:    "ERR_INVALID_PARAMS",
		}), nil
	}

	sessionID := params.SessionID
	if sessionID == "" {
		ab.mu.Lock()
		sessionID = ab.sessions[conn.clientID]
		ab.mu.Unlock()
	}
	if sessionID == "" {
		return protocol.NewResponse(req.ID, false, nil, &protocol.ErrorObject{
			Message: "no session; call agent.session.create first",
			Code:    "ERR_NO_SESSION",
		}), nil
	}

	userID := params.UserID
	if userID == "" {
		userID = conn.clientID
	}
	if userID == "" {
		userID = "anonymous"
	}

	userMsg := genai.NewContentFromText(params.Text, genai.RoleUser)

	// Run the agent asynchronously, streaming events back to the client.
	go ab.runAgent(conn, req.ID, userID, sessionID, userMsg)

	// Return an accepted ACK immediately; the final response comes via events + a final res frame.
	return protocol.NewResponse(req.ID, true, map[string]string{"status": "accepted"}, nil), nil
}

func (ab *AgentBridge) runAgent(conn *Conn, reqID, userID, sessionID string, msg *genai.Content) {
	ctx := conn.ctx

	var finalText string
	for event, err := range ab.runner.Run(ctx, userID, sessionID, msg, agent.RunConfig{
		StreamingMode: agent.StreamingModeSSE,
	}) {
		if err != nil {
			log.Printf("openclaw: agent run error: %v", err)
			_ = conn.SendEvent("agent.error", map[string]string{
				"requestId": reqID,
				"error":     err.Error(),
			})
			return
		}

		if event.Content == nil {
			continue
		}

		var text string
		for _, p := range event.Content.Parts {
			text += p.Text
		}

		if text == "" {
			continue
		}

		payload := map[string]any{
			"requestId": reqID,
			"text":      text,
			"author":    event.Author,
			"partial":   event.Partial,
			"final":     event.IsFinalResponse(),
		}

		_ = conn.SendEvent("agent.text", payload)

		if event.IsFinalResponse() {
			finalText = text
		}
	}

	// Send a final response frame to complete the request.
	res := protocol.NewResponse(reqID, true, map[string]any{
		"text": finalText,
	}, nil)
	if err := conn.SendResponse(res); err != nil {
		log.Printf("openclaw: send final agent response failed: %v", err)
	}
}
