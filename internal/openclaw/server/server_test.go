package server_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/innomon/agentic/internal/openclaw/protocol"
	"github.com/innomon/agentic/internal/openclaw/server"
)

func setupTestServer(t *testing.T, cfg server.Config) (*server.Server, *websocket.Conn, func()) {
	t.Helper()
	srv := server.New(cfg)

	ts := httptest.NewServer(srv)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		ts.Close()
		t.Fatalf("dial: %v", err)
	}

	cleanup := func() {
		ws.Close()
		ts.Close()
	}
	return srv, ws, cleanup
}

func sendRequest(t *testing.T, ws *websocket.Conn, method string, params any) *protocol.ResponseFrame {
	t.Helper()
	id := uuid.New().String()
	req := protocol.RequestFrame{
		Type:   "req",
		ID:     id,
		Method: method,
	}
	if params != nil {
		raw, _ := json.Marshal(params)
		req.Params = raw
	}
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := ws.WriteJSON(req); err != nil {
		t.Fatalf("write: %v", err)
	}

	var res protocol.ResponseFrame
	if err := ws.ReadJSON(&res); err != nil {
		t.Fatalf("read: %v", err)
	}
	if res.ID != id {
		t.Fatalf("response ID mismatch: got %s, want %s", res.ID, id)
	}
	return &res
}

func connectClient(t *testing.T, ws *websocket.Conn, token string) {
	t.Helper()
	params := protocol.ConnectParams{
		MinProtocol: 3,
		MaxProtocol: 3,
		Client:      protocol.ClientInfo{ID: "test-client"},
		Role:        "operator",
		Scopes:      []string{"operator.admin"},
	}
	if token != "" {
		params.Auth = &protocol.AuthObject{Token: token}
	}
	res := sendRequest(t, ws, "connect", params)
	if !res.OK {
		t.Fatalf("connect failed: %v", res.Error)
	}
}

func TestConnect_TokenAuth(t *testing.T) {
	_, ws, cleanup := setupTestServer(t, server.Config{Tokens: []string{"test-token"}})
	defer cleanup()

	params := protocol.ConnectParams{
		MinProtocol: 3,
		MaxProtocol: 3,
		Client:      protocol.ClientInfo{ID: "test-client", Version: "1.0"},
		Role:        "operator",
		Scopes:      []string{"operator.admin"},
		Auth:        &protocol.AuthObject{Token: "test-token"},
	}

	res := sendRequest(t, ws, "connect", params)
	if !res.OK {
		t.Fatalf("connect should succeed, got error: %v", res.Error)
	}

	var hello protocol.HelloPayload
	if err := json.Unmarshal(res.Payload, &hello); err != nil {
		t.Fatal(err)
	}
	if hello.Policy == nil {
		t.Fatal("expected policy in hello")
	}
	if hello.Policy.TickIntervalMs == 0 {
		t.Error("expected tickIntervalMs > 0")
	}
	if hello.Auth == nil {
		t.Fatal("expected auth in hello")
	}
	if hello.Auth.Role != "operator" {
		t.Errorf("role = %q, want %q", hello.Auth.Role, "operator")
	}
}

func TestConnect_InvalidToken(t *testing.T) {
	_, ws, cleanup := setupTestServer(t, server.Config{Tokens: []string{"valid-token"}})
	defer cleanup()

	params := protocol.ConnectParams{
		MinProtocol: 3,
		MaxProtocol: 3,
		Client:      protocol.ClientInfo{ID: "test-client"},
		Role:        "operator",
		Scopes:      []string{},
		Auth:        &protocol.AuthObject{Token: "wrong-token"},
	}

	res := sendRequest(t, ws, "connect", params)
	if res.OK {
		t.Fatal("connect with invalid token should fail")
	}
	if res.Error.Code != "ERR_AUTH_FAILED" {
		t.Errorf("error code = %q, want ERR_AUTH_FAILED", res.Error.Code)
	}
}

func TestConnect_NoAuth(t *testing.T) {
	_, ws, cleanup := setupTestServer(t, server.Config{Tokens: []string{"some-token"}})
	defer cleanup()

	params := protocol.ConnectParams{
		MinProtocol: 3,
		MaxProtocol: 3,
		Client:      protocol.ClientInfo{ID: "test-client"},
		Role:        "user",
		Scopes:      []string{},
	}

	res := sendRequest(t, ws, "connect", params)
	if res.OK {
		t.Fatal("connect without auth should fail when tokens configured")
	}
	if res.Error.Code != "ERR_AUTH_REQUIRED" {
		t.Errorf("error code = %q, want ERR_AUTH_REQUIRED", res.Error.Code)
	}
}

func TestConnect_NoAuthRequired(t *testing.T) {
	_, ws, cleanup := setupTestServer(t, server.Config{})
	defer cleanup()

	params := protocol.ConnectParams{
		MinProtocol: 3,
		MaxProtocol: 3,
		Client:      protocol.ClientInfo{ID: "test-client"},
		Role:        "user",
		Scopes:      []string{},
	}

	res := sendRequest(t, ws, "connect", params)
	if !res.OK {
		t.Fatalf("connect without auth should succeed when no tokens configured: %v", res.Error)
	}
}

func TestConnect_RequireFirstMessage(t *testing.T) {
	_, ws, cleanup := setupTestServer(t, server.Config{})
	defer cleanup()

	// Send a non-connect request as first message.
	id := uuid.New().String()
	req := protocol.RequestFrame{
		Type:   "req",
		ID:     id,
		Method: "config.get",
	}
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := ws.WriteJSON(req); err != nil {
		t.Fatalf("write: %v", err)
	}

	var res protocol.ResponseFrame
	if err := ws.ReadJSON(&res); err != nil {
		t.Fatalf("read: %v", err)
	}
	if res.OK {
		t.Fatal("non-connect first message should fail")
	}
	if res.Error.Code != "ERR_NOT_AUTHENTICATED" {
		t.Errorf("error code = %q, want ERR_NOT_AUTHENTICATED", res.Error.Code)
	}
}

func TestConfigGet(t *testing.T) {
	_, ws, cleanup := setupTestServer(t, server.Config{})
	defer cleanup()
	connectClient(t, ws, "")

	res := sendRequest(t, ws, "config.get", nil)
	if !res.OK {
		t.Fatalf("config.get failed: %v", res.Error)
	}

	var cfg map[string]any
	if err := json.Unmarshal(res.Payload, &cfg); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg["bind"]; !ok {
		t.Error("expected bind in config response")
	}
	if _, ok := cfg["tick_interval_ms"]; !ok {
		t.Error("expected tick_interval_ms in config response")
	}
}

func TestUnknownMethod(t *testing.T) {
	_, ws, cleanup := setupTestServer(t, server.Config{})
	defer cleanup()
	connectClient(t, ws, "")

	res := sendRequest(t, ws, "nonexistent.method", nil)
	if res.OK {
		t.Fatal("unknown method should fail")
	}
	if res.Error.Code != "ERR_UNKNOWN_METHOD" {
		t.Errorf("error code = %q, want ERR_UNKNOWN_METHOD", res.Error.Code)
	}
}

func TestTickEmission(t *testing.T) {
	_, ws, cleanup := setupTestServer(t, server.Config{TickIntervalMs: 100})
	defer cleanup()
	connectClient(t, ws, "")

	// Read events and wait for a tick.
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	for i := 0; i < 10; i++ {
		_, message, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		parsed, err := protocol.ParseFrame(message)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		ev, ok := parsed.(*protocol.EventFrame)
		if !ok {
			continue
		}
		if ev.Event == "tick" {
			if ev.Seq == 0 {
				t.Error("tick seq should be > 0")
			}
			return // Success
		}
	}
	t.Error("did not receive tick event")
}

func TestSend_Broadcast(t *testing.T) {
	srv := server.New(server.Config{})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	// Connect client 1.
	ws1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws1.Close()
	connectClient(t, ws1, "")

	// Connect client 2.
	ws2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws2.Close()
	connectClient(t, ws2, "")

	// Client 1 sends a message.
	sendReq := map[string]any{
		"event":   "test.message",
		"payload": map[string]string{"text": "hello"},
	}
	res := sendRequest(t, ws1, "send", sendReq)
	if !res.OK {
		t.Fatalf("send failed: %v", res.Error)
	}

	// Client 2 should receive the event.
	ws2.SetReadDeadline(time.Now().Add(2 * time.Second))
	for i := 0; i < 10; i++ {
		_, message, err := ws2.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		parsed, err := protocol.ParseFrame(message)
		if err != nil {
			continue
		}
		ev, ok := parsed.(*protocol.EventFrame)
		if !ok {
			continue
		}
		if ev.Event == "test.message" {
			return // Success
		}
	}
	t.Error("client 2 did not receive broadcast event")
}
