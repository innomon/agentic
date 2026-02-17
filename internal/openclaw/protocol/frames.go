package protocol

import "encoding/json"

const (
	FrameTypeReq   = "req"
	FrameTypeRes   = "res"
	FrameTypeEvent = "event"
)

// RequestFrame is sent from client to server.
type RequestFrame struct {
	Type   string          `json:"type"`
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// ResponseFrame is sent from server to client.
type ResponseFrame struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	OK      bool            `json:"ok"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *ErrorObject    `json:"error,omitempty"`
}

// EventFrame is sent from server to client for push notifications.
type EventFrame struct {
	Type         string          `json:"type"`
	Event        string          `json:"event"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	Seq          uint64          `json:"seq"`
	StateVersion string          `json:"stateVersion,omitempty"`
}

// ErrorObject describes an error returned in a ResponseFrame.
type ErrorObject struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func (e *ErrorObject) Error() string {
	if e.Code != "" {
		return e.Code + ": " + e.Message
	}
	return e.Message
}

// ConnectParams are the parameters sent in the "connect" request.
type ConnectParams struct {
	MinProtocol int               `json:"minProtocol"`
	MaxProtocol int               `json:"maxProtocol"`
	Client      ClientInfo        `json:"client"`
	Caps        []string          `json:"caps,omitempty"`
	Commands    []string          `json:"commands,omitempty"`
	Permissions map[string]any    `json:"permissions,omitempty"`
	PathEnv     map[string]string `json:"pathEnv,omitempty"`
	Auth        *AuthObject       `json:"auth,omitempty"`
	Role        string            `json:"role"`
	Scopes      []string          `json:"scopes"`
	Device      *DevicePayload    `json:"device,omitempty"`
}

// ClientInfo identifies the connecting client.
type ClientInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName,omitempty"`
	Version     string `json:"version,omitempty"`
	Platform    string `json:"platform,omitempty"`
	Mode        string `json:"mode,omitempty"`
	InstanceID  string `json:"instanceId,omitempty"`
}

// DevicePayload carries a device signature for authentication.
type DevicePayload struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
	Signature string `json:"signature"`
	SignedAt  int64  `json:"signedAt"`
	Nonce     string `json:"nonce,omitempty"`
}

// AuthObject carries authentication credentials.
type AuthObject struct {
	Token    string `json:"token,omitempty"`
	Password string `json:"password,omitempty"`
}

// HelloPayload is the server response to a successful connect request.
type HelloPayload struct {
	Auth   *HelloAuthPayload `json:"auth,omitempty"`
	Policy *PolicyPayload    `json:"policy,omitempty"`
}

// HelloAuthPayload contains authentication details in the hello response.
type HelloAuthPayload struct {
	DeviceToken string   `json:"deviceToken,omitempty"`
	Role        string   `json:"role,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	IssuedAtMs  int64    `json:"issuedAtMs,omitempty"`
}

// PolicyPayload contains server policy in the hello response.
type PolicyPayload struct {
	TickIntervalMs int `json:"tickIntervalMs,omitempty"`
}

// NewResponse constructs a ResponseFrame.
func NewResponse(id string, ok bool, payload any, err *ErrorObject) *ResponseFrame {
	f := &ResponseFrame{
		Type:  FrameTypeRes,
		ID:    id,
		OK:    ok,
		Error: err,
	}
	if payload != nil {
		raw, _ := json.Marshal(payload)
		f.Payload = raw
	}
	return f
}

// NewEvent constructs an EventFrame.
func NewEvent(event string, payload any, seq uint64) *EventFrame {
	f := &EventFrame{
		Type:  FrameTypeEvent,
		Event: event,
		Seq:   seq,
	}
	if payload != nil {
		raw, _ := json.Marshal(payload)
		f.Payload = raw
	}
	return f
}
