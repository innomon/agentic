package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/innomon/agentic/internal/openclaw/protocol"
)

// Config holds client configuration.
type Config struct {
	URL            string        // WebSocket URL (default "ws://127.0.0.1:18789/ws")
	MaxPayload     int64         // Max message size (default 25*1024*1024)
	RequestTimeout time.Duration // Default request timeout (default 30s)

	BackoffMin    time.Duration // Initial backoff (default 1s)
	BackoffMax    time.Duration // Max backoff (default 30s)
	AutoReconnect bool          // Whether to auto-reconnect (default false)
}

func (c *Config) setDefaults() {
	if c.URL == "" {
		c.URL = "ws://127.0.0.1:18789/ws"
	}
	if c.MaxPayload == 0 {
		c.MaxPayload = 25 * 1024 * 1024
	}
	if c.RequestTimeout == 0 {
		c.RequestTimeout = 30 * time.Second
	}
	if c.BackoffMin == 0 {
		c.BackoffMin = time.Second
	}
	if c.BackoffMax == 0 {
		c.BackoffMax = 30 * time.Second
	}
}

// EventHandler is called when a server event is received.
type EventHandler func(event *protocol.EventFrame)

// GapHandler is called when a sequence gap is detected.
type GapHandler func(expected, received uint64)

type pendingRequest struct {
	ch          chan *protocol.ResponseFrame
	expectFinal bool
}

// Client is an OpenClaw gateway client.
type Client struct {
	cfg Config
	ws  *websocket.Conn

	mu      sync.Mutex
	pending map[string]*pendingRequest

	writeMu sync.Mutex

	eventMu  sync.RWMutex
	handlers map[string]EventHandler
	onClose  func(error)
	onGap    GapHandler

	lastSeq      uint64
	lastTick     time.Time
	tickInterval time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// New creates a new client with the given config.
func New(cfg Config) *Client {
	cfg.setDefaults()
	return &Client{
		cfg:      cfg,
		pending:  make(map[string]*pendingRequest),
		handlers: make(map[string]EventHandler),
		done:     make(chan struct{}),
	}
}

// Connect establishes the WebSocket connection and starts the read pump.
func (c *Client) Connect(ctx context.Context) error {
	ws, _, err := websocket.DefaultDialer.DialContext(ctx, c.cfg.URL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	ws.SetReadLimit(c.cfg.MaxPayload)
	c.ws = ws
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.done = make(chan struct{})
	go c.readPump()
	return nil
}

// SendConnect sends the connect handshake and waits for the response.
func (c *Client) SendConnect(params *protocol.ConnectParams) (*protocol.HelloPayload, error) {
	res, err := c.Request(c.ctx, "connect", params, false)
	if err != nil {
		return nil, err
	}
	if !res.OK {
		if res.Error != nil {
			return nil, res.Error
		}
		return nil, errors.New("connect failed")
	}
	var hello protocol.HelloPayload
	if err := json.Unmarshal(res.Payload, &hello); err != nil {
		return nil, fmt.Errorf("parse hello: %w", err)
	}
	if hello.Policy != nil && hello.Policy.TickIntervalMs > 0 {
		c.tickInterval = time.Duration(hello.Policy.TickIntervalMs) * time.Millisecond
		c.lastTick = time.Now()
	}
	return &hello, nil
}

// Request sends an RPC request and waits for the response.
func (c *Client) Request(ctx context.Context, method string, params any, expectFinal bool) (*protocol.ResponseFrame, error) {
	id := uuid.New().String()
	req := protocol.RequestFrame{
		Type:   protocol.FrameTypeReq,
		ID:     id,
		Method: method,
	}
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		req.Params = raw
	}

	pr := &pendingRequest{
		ch:          make(chan *protocol.ResponseFrame, 1),
		expectFinal: expectFinal,
	}

	c.mu.Lock()
	c.pending[id] = pr
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	c.writeMu.Lock()
	err := c.ws.WriteJSON(req)
	c.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	timeout := c.cfg.RequestTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if d := time.Until(deadline); d < timeout {
			timeout = d
		}
	}

	select {
	case res := <-pr.ch:
		return res, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("request %s timed out after %v", method, timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// OnEvent registers an event handler. Use "*" for all events.
func (c *Client) OnEvent(event string, handler EventHandler) {
	c.eventMu.Lock()
	defer c.eventMu.Unlock()
	c.handlers[event] = handler
}

// OnClose registers a handler called when the connection closes.
func (c *Client) OnClose(handler func(error)) {
	c.eventMu.Lock()
	defer c.eventMu.Unlock()
	c.onClose = handler
}

// OnGap registers a handler called when a sequence gap is detected.
func (c *Client) OnGap(handler GapHandler) {
	c.eventMu.Lock()
	defer c.eventMu.Unlock()
	c.onGap = handler
}

// Close closes the connection.
func (c *Client) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	if c.ws != nil {
		return c.ws.Close()
	}
	return nil
}

// ConnectWithBackoff retries Connect with exponential backoff.
func (c *Client) ConnectWithBackoff(ctx context.Context) error {
	attempt := 0
	for {
		err := c.Connect(ctx)
		if err == nil {
			return nil
		}
		delay := c.cfg.BackoffMin * time.Duration(math.Pow(2, float64(attempt)))
		if delay > c.cfg.BackoffMax {
			delay = c.cfg.BackoffMax
		}
		attempt++
		log.Printf("openclaw client: connect failed (%v), retrying in %v", err, delay)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		c.cancel()
		close(c.done)
	}()

	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("openclaw client: read error: %v", err)
			}
			c.rejectAllPending(err)
			c.eventMu.RLock()
			onClose := c.onClose
			c.eventMu.RUnlock()
			if onClose != nil {
				onClose(err)
			}
			return
		}

		parsed, err := protocol.ParseFrame(message)
		if err != nil {
			log.Printf("openclaw client: invalid frame: %v", err)
			continue
		}

		switch f := parsed.(type) {
		case *protocol.ResponseFrame:
			c.handleResponse(f)
		case *protocol.EventFrame:
			c.handleEvent(f)
		}
	}
}

func (c *Client) handleResponse(res *protocol.ResponseFrame) {
	c.mu.Lock()
	pr, ok := c.pending[res.ID]
	c.mu.Unlock()
	if !ok {
		return
	}

	if pr.expectFinal && res.OK {
		var status struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(res.Payload, &status) == nil && status.Status == "accepted" {
			return // Interim ACK; wait for final.
		}
	}

	select {
	case pr.ch <- res:
	default:
	}
}

func (c *Client) handleEvent(ev *protocol.EventFrame) {
	// Gap detection
	if ev.Seq > 0 && c.lastSeq > 0 && ev.Seq > c.lastSeq+1 {
		c.eventMu.RLock()
		onGap := c.onGap
		c.eventMu.RUnlock()
		if onGap != nil {
			onGap(c.lastSeq+1, ev.Seq)
		}
	}
	if ev.Seq > 0 {
		c.lastSeq = ev.Seq
	}

	if ev.Event == "tick" {
		c.lastTick = time.Now()
	}

	c.eventMu.RLock()
	specific := c.handlers[ev.Event]
	wildcard := c.handlers["*"]
	c.eventMu.RUnlock()

	if specific != nil {
		specific(ev)
	}
	if wildcard != nil {
		wildcard(ev)
	}
}

func (c *Client) rejectAllPending(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, pr := range c.pending {
		errRes := &protocol.ResponseFrame{
			Type: protocol.FrameTypeRes,
			ID:   id,
			OK:   false,
			Error: &protocol.ErrorObject{
				Message: "connection closed: " + err.Error(),
				Code:    "ERR_CONNECTION_CLOSED",
			},
		}
		select {
		case pr.ch <- errRes:
		default:
		}
	}
}
