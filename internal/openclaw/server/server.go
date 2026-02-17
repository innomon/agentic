package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/innomon/agentic/internal/openclaw/auth"
	"github.com/innomon/agentic/internal/openclaw/protocol"
)

// MethodHandler is a function that handles a gateway RPC method.
type MethodHandler func(ctx context.Context, conn *Conn, req *protocol.RequestFrame) (*protocol.ResponseFrame, error)

// Server is the OpenClaw gateway WebSocket server.
type Server struct {
	cfg      Config
	upgrader websocket.Upgrader
	verifier *auth.Verifier
	tokens   auth.DeviceTokenStore
	methods  map[string]MethodHandler

	mu    sync.Mutex
	conns map[*Conn]struct{}

	dedup        *DedupCache
	agentHandler MethodHandler

	httpSrv *http.Server
}

// New creates a new gateway server.
func New(cfg Config) *Server {
	cfg.SetDefaults()
	s := &Server{
		cfg: cfg,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
		verifier: auth.NewVerifier(cfg.Tokens, cfg.Password, cfg.AllowPassword),
		tokens:   auth.NewMemoryDeviceTokenStore(),
		methods:  make(map[string]MethodHandler),
		conns:    make(map[*Conn]struct{}),
		dedup:    NewDedupCache(5 * time.Minute),
	}
	s.dedup.StartCleanupLoop(context.Background(), time.Minute)
	s.RegisterMethod("connect", s.HandleConnect)
	s.RegisterCoreMethods()
	return s
}

// RegisterMethod registers a handler for an RPC method name.
// Supports exact match (e.g., "config.get") — for prefix matching
// (e.g., "agent.*"), register "agent." as the key.
func (s *Server) RegisterMethod(method string, handler MethodHandler) {
	s.methods[method] = handler
}

// HandleConnect is the built-in connect method handler.
func (s *Server) HandleConnect(ctx context.Context, conn *Conn, req *protocol.RequestFrame) (*protocol.ResponseFrame, error) {
	var params protocol.ConnectParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return protocol.NewResponse(req.ID, false, nil, &protocol.ErrorObject{
				Message: "invalid connect params: " + err.Error(),
				Code:    "ERR_INVALID_PARAMS",
			}), nil
		}
	}

	// Verify auth: token or password.
	authenticated := false
	authToken := ""
	if params.Auth != nil {
		if params.Auth.Token != "" {
			if s.verifier.VerifyToken(params.Auth.Token) {
				authenticated = true
				authToken = params.Auth.Token
			} else {
				return protocol.NewResponse(req.ID, false, nil, &protocol.ErrorObject{
					Message: "invalid token",
					Code:    "ERR_AUTH_FAILED",
				}), nil
			}
		}
		if !authenticated && params.Auth.Password != "" {
			if s.verifier.VerifyPassword(params.Auth.Password) {
				authenticated = true
			} else {
				return protocol.NewResponse(req.ID, false, nil, &protocol.ErrorObject{
					Message: "invalid password",
					Code:    "ERR_AUTH_FAILED",
				}), nil
			}
		}
	}

	// If no auth provided and tokens are configured, reject.
	if !authenticated && (len(s.cfg.Tokens) > 0 || (s.cfg.AllowPassword && s.cfg.Password != "")) {
		return protocol.NewResponse(req.ID, false, nil, &protocol.ErrorObject{
			Message: "authentication required",
			Code:    "ERR_AUTH_REQUIRED",
		}), nil
	}

	// Device signature verification.
	if params.Device != nil {
		payload := protocol.CanonicalSignPayload(
			params.Device.ID,
			params.Client.ID,
			params.Client.Mode,
			params.Role,
			params.Scopes,
			params.Device.SignedAt,
			authToken,
			params.Device.Nonce,
		)
		if err := protocol.VerifyDeviceSignature(params.Device.PublicKey, params.Device.Signature, payload); err != nil {
			return protocol.NewResponse(req.ID, false, nil, &protocol.ErrorObject{
				Message: "device signature failed: " + err.Error(),
				Code:    "ERR_DEVICE_AUTH",
			}), nil
		}
	} else if s.cfg.RequireDevice {
		// Device required but not provided — issue challenge.
		nonce := uuid.New().String()
		if err := conn.SendEvent("connect.challenge", map[string]string{"nonce": nonce}); err != nil {
			return nil, fmt.Errorf("sending challenge: %w", err)
		}
		return nil, nil
	}

	// Issue device token.
	var deviceToken string
	if params.Device != nil {
		var err error
		deviceToken, err = auth.GenerateDeviceToken()
		if err != nil {
			return nil, fmt.Errorf("generating device token: %w", err)
		}
		if err := s.tokens.Store(auth.DeviceToken{
			DeviceID:   params.Device.ID,
			Role:       params.Role,
			Token:      deviceToken,
			Scopes:     params.Scopes,
			IssuedAtMs: time.Now().UnixMilli(),
		}); err != nil {
			return nil, fmt.Errorf("storing device token: %w", err)
		}
	}

	// Set connection auth state.
	conn.authed = true
	conn.clientID = params.Client.ID
	conn.role = params.Role
	conn.scopes = params.Scopes
	if params.Device != nil {
		conn.deviceID = params.Device.ID
	}

	// Start tick loop.
	go conn.StartTickLoop()

	// Build hello payload.
	hello := protocol.HelloPayload{
		Auth: &protocol.HelloAuthPayload{
			DeviceToken: deviceToken,
			Role:        params.Role,
			Scopes:      params.Scopes,
			IssuedAtMs:  time.Now().UnixMilli(),
		},
		Policy: &protocol.PolicyPayload{
			TickIntervalMs: s.cfg.TickIntervalMs,
		},
	}

	return protocol.NewResponse(req.ID, true, hello, nil), nil
}

// Cfg returns the server configuration.
func (s *Server) Cfg() Config { return s.cfg }

// ServeHTTP handles the WebSocket upgrade.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("openclaw: upgrade error: %v", err)
		return
	}

	conn := NewConn(ws, s)
	s.addConn(conn)
	go conn.ReadPump()
}

// Start starts the HTTP server. Blocks until context is cancelled or error.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle(s.cfg.Path, s)

	s.httpSrv = &http.Server{
		Addr:    s.cfg.Bind,
		Handler: mux,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("openclaw: gateway listening on %s%s", s.cfg.Bind, s.cfg.Path)
		errCh <- s.httpSrv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	for conn := range s.conns {
		_ = conn.Close(websocket.CloseGoingAway, "server shutting down")
	}
	s.mu.Unlock()

	if s.httpSrv != nil {
		return s.httpSrv.Shutdown(ctx)
	}
	return nil
}

// dispatch routes a request to the appropriate method handler.
func (s *Server) dispatch(ctx context.Context, conn *Conn, req *protocol.RequestFrame) {
	handler, ok := s.methods[req.Method]
	if !ok {
		// Try prefix match.
		for prefix, h := range s.methods {
			if strings.HasSuffix(prefix, ".") && strings.HasPrefix(req.Method, prefix) {
				handler = h
				ok = true
				break
			}
		}
	}

	if !ok {
		errRes := protocol.NewResponse(req.ID, false, nil, &protocol.ErrorObject{
			Message: "unknown method: " + req.Method,
			Code:    "ERR_UNKNOWN_METHOD",
		})
		if err := conn.SendResponse(errRes); err != nil {
			log.Printf("openclaw: send error response failed: %v", err)
		}
		return
	}

	res, err := handler(ctx, conn, req)
	if err != nil {
		log.Printf("openclaw: handler error for %s: %v", req.Method, err)
		errRes := protocol.NewResponse(req.ID, false, nil, &protocol.ErrorObject{
			Message: err.Error(),
			Code:    "ERR_INTERNAL",
		})
		if sendErr := conn.SendResponse(errRes); sendErr != nil {
			log.Printf("openclaw: send error response failed: %v", sendErr)
		}
		return
	}
	if res != nil {
		if err := conn.SendResponse(res); err != nil {
			log.Printf("openclaw: send response failed: %v", err)
		}
	}
}

func (s *Server) addConn(conn *Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conns[conn] = struct{}{}
}

func (s *Server) removeConn(conn *Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, conn)
}
