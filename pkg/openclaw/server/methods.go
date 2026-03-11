package server

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/innomon/agentic/pkg/openclaw/protocol"
)

// RegisterCoreMethods registers all built-in method handlers on the server.
// Call this after New() to set up all standard methods.
func (s *Server) RegisterCoreMethods() {
	s.RegisterMethod("config.get", s.handleConfigGet)
	s.RegisterMethod("config.schema", s.handleConfigSchema)
	s.RegisterMethod("send", s.handleSend)
	s.RegisterMethod("agent.", s.handleAgent)
}

// SetAgentHandler sets a custom handler for all agent.* methods.
func (s *Server) SetAgentHandler(handler MethodHandler) {
	s.agentHandler = handler
}

func (s *Server) handleConfigGet(ctx context.Context, conn *Conn, req *protocol.RequestFrame) (*protocol.ResponseFrame, error) {
	safeConfig := map[string]any{
		"bind":             s.cfg.Bind,
		"path":             s.cfg.Path,
		"tick_interval_ms": s.cfg.TickIntervalMs,
		"max_payload":      s.cfg.MaxPayload,
		"require_device":   s.cfg.RequireDevice,
	}
	return protocol.NewResponse(req.ID, true, safeConfig, nil), nil
}

func (s *Server) handleConfigSchema(ctx context.Context, conn *Conn, req *protocol.RequestFrame) (*protocol.ResponseFrame, error) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"bind":             map[string]string{"type": "string", "description": "Listen address"},
			"path":             map[string]string{"type": "string", "description": "WebSocket path"},
			"tick_interval_ms": map[string]string{"type": "integer", "description": "Tick interval in ms"},
			"max_payload":      map[string]string{"type": "integer", "description": "Max message size in bytes"},
			"tokens":           map[string]string{"type": "array", "description": "Allowed auth tokens"},
			"password":         map[string]string{"type": "string", "description": "Optional password"},
			"allow_password":   map[string]string{"type": "boolean", "description": "Whether password auth is enabled"},
			"require_device":   map[string]string{"type": "boolean", "description": "Whether device auth is required"},
		},
	}
	return protocol.NewResponse(req.ID, true, schema, nil), nil
}

func (s *Server) handleSend(ctx context.Context, conn *Conn, req *protocol.RequestFrame) (*protocol.ResponseFrame, error) {
	var params struct {
		IdempotencyKey string          `json:"idempotencyKey"`
		Event          string          `json:"event"`
		Payload        json.RawMessage `json:"payload"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	if params.IdempotencyKey != "" {
		if s.dedup.Check(params.IdempotencyKey) {
			return protocol.NewResponse(req.ID, true, map[string]string{"status": "deduplicated"}, nil), nil
		}
		s.dedup.Record(params.IdempotencyKey)
	}

	eventName := params.Event
	if eventName == "" {
		eventName = "message"
	}

	s.mu.Lock()
	targets := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		if c != conn && c.authed {
			targets = append(targets, c)
		}
	}
	s.mu.Unlock()

	for _, target := range targets {
		target.SendEvent(eventName, params.Payload)
	}

	return protocol.NewResponse(req.ID, true, map[string]string{"status": "sent"}, nil), nil
}

func (s *Server) handleAgent(ctx context.Context, conn *Conn, req *protocol.RequestFrame) (*protocol.ResponseFrame, error) {
	subMethod := strings.TrimPrefix(req.Method, "agent.")

	if s.agentHandler != nil {
		return s.agentHandler(ctx, conn, req)
	}

	return protocol.NewResponse(req.ID, false, nil, &protocol.ErrorObject{
		Message: "agent method not implemented: " + subMethod,
		Code:    "ERR_NOT_IMPLEMENTED",
	}), nil
}
