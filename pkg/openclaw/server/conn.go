package server

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/innomon/agentic/pkg/openclaw/protocol"
)

// Conn represents a single WebSocket connection to the gateway.
type Conn struct {
	ws  *websocket.Conn
	srv *Server

	// Auth state
	authed   bool
	deviceID string
	clientID string
	role     string
	scopes   []string

	// Sequence tracking
	seq atomic.Uint64

	// Write serialization
	writeMu sync.Mutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewConn wraps a websocket connection.
func NewConn(ws *websocket.Conn, srv *Server) *Conn {
	ctx, cancel := context.WithCancel(context.Background())
	ws.SetReadLimit(srv.cfg.MaxPayload)
	return &Conn{
		ws:     ws,
		srv:    srv,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
}

// ReadPump reads frames from the WebSocket and dispatches them.
func (c *Conn) ReadPump() {
	defer func() {
		c.cancel()
		c.srv.removeConn(c)
		c.ws.Close()
		close(c.done)
	}()

	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("openclaw: read error: %v", err)
			}
			return
		}

		parsed, err := protocol.ParseFrame(message)
		if err != nil {
			log.Printf("openclaw: invalid frame: %v", err)
			errRes := protocol.NewResponse("", false, nil, &protocol.ErrorObject{
				Message: err.Error(),
				Code:    "ERR_INVALID_FRAME",
			})
			if sendErr := c.SendResponse(errRes); sendErr != nil {
				log.Printf("openclaw: send error response failed: %v", sendErr)
			}
			continue
		}

		req, ok := parsed.(*protocol.RequestFrame)
		if !ok {
			log.Printf("openclaw: expected request frame, got %T", parsed)
			continue
		}

		// The first message must be a connect request.
		if !c.authed && req.Method != "connect" {
			errRes := protocol.NewResponse(req.ID, false, nil, &protocol.ErrorObject{
				Message: "first message must be connect",
				Code:    "ERR_NOT_AUTHENTICATED",
			})
			_ = c.SendResponse(errRes)
			_ = c.Close(websocket.ClosePolicyViolation, "not authenticated")
			return
		}

		c.srv.dispatch(c.ctx, c, req)
	}
}

// SendResponse sends a response frame.
func (c *Conn) SendResponse(res *protocol.ResponseFrame) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	data, err := json.Marshal(res)
	if err != nil {
		return err
	}
	if err := c.ws.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("openclaw: write error: %v", err)
		c.cancel()
		return err
	}
	return nil
}

// SendEvent sends an event frame, incrementing the sequence number.
func (c *Conn) SendEvent(event string, payload any) error {
	frame := protocol.NewEvent(event, payload, c.NextSeq())
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	if err := c.ws.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("openclaw: write event error: %v", err)
		c.cancel()
		return err
	}
	return nil
}

// Close closes the connection with the given WebSocket close code and message.
func (c *Conn) Close(code int, message string) error {
	c.cancel()
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	msg := websocket.FormatCloseMessage(code, message)
	return c.ws.WriteControl(websocket.CloseMessage, msg, time.Now().Add(5*time.Second))
}

// NextSeq returns the next sequence number (atomic increment).
func (c *Conn) NextSeq() uint64 {
	return c.seq.Add(1)
}

// StartTickLoop starts sending tick events at the configured interval.
// Stop by cancelling the conn's context.
func (c *Conn) StartTickLoop() {
	interval := time.Duration(c.srv.cfg.TickIntervalMs) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.SendEvent("tick", nil); err != nil {
				return
			}
		}
	}
}
