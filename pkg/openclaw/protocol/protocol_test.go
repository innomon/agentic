package protocol_test

import (
	"crypto/ed25519"
	"encoding/json"
	"testing"

	"github.com/innomon/agentic/pkg/openclaw/protocol"
)

func TestParseRequestFrame(t *testing.T) {
	data := `{"type":"req","id":"abc-123","method":"connect","params":{"minProtocol":3}}`
	frame, err := protocol.ParseFrame([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	req, ok := frame.(*protocol.RequestFrame)
	if !ok {
		t.Fatalf("expected *RequestFrame, got %T", frame)
	}
	if req.ID != "abc-123" {
		t.Errorf("ID = %q, want %q", req.ID, "abc-123")
	}
	if req.Method != "connect" {
		t.Errorf("Method = %q, want %q", req.Method, "connect")
	}
}

func TestParseResponseFrame_OK(t *testing.T) {
	data := `{"type":"res","id":"abc-123","ok":true,"payload":{"hello":"world"}}`
	frame, err := protocol.ParseFrame([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	res, ok := frame.(*protocol.ResponseFrame)
	if !ok {
		t.Fatalf("expected *ResponseFrame, got %T", frame)
	}
	if !res.OK {
		t.Error("OK should be true")
	}
	if res.Payload == nil {
		t.Error("Payload should not be nil")
	}
}

func TestParseResponseFrame_Error(t *testing.T) {
	data := `{"type":"res","id":"abc-123","ok":false,"error":{"message":"bad","code":"ERR"}}`
	frame, err := protocol.ParseFrame([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	res, ok := frame.(*protocol.ResponseFrame)
	if !ok {
		t.Fatalf("expected *ResponseFrame, got %T", frame)
	}
	if res.OK {
		t.Error("OK should be false")
	}
	if res.Error == nil {
		t.Fatal("Error should not be nil")
	}
	if res.Error.Message != "bad" {
		t.Errorf("Error.Message = %q, want %q", res.Error.Message, "bad")
	}
}

func TestParseEventFrame(t *testing.T) {
	data := `{"type":"event","event":"tick","seq":5}`
	frame, err := protocol.ParseFrame([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	ev, ok := frame.(*protocol.EventFrame)
	if !ok {
		t.Fatalf("expected *EventFrame, got %T", frame)
	}
	if ev.Event != "tick" {
		t.Errorf("Event = %q, want %q", ev.Event, "tick")
	}
	if ev.Seq != 5 {
		t.Errorf("Seq = %d, want %d", ev.Seq, 5)
	}
}

func TestParseFrame_InvalidJSON(t *testing.T) {
	_, err := protocol.ParseFrame([]byte(`{not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseFrame_UnknownType(t *testing.T) {
	_, err := protocol.ParseFrame([]byte(`{"type":"unknown"}`))
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestParseFrame_MissingType(t *testing.T) {
	_, err := protocol.ParseFrame([]byte(`{"id":"abc"}`))
	if err == nil {
		t.Error("expected error for missing type")
	}
}

func TestValidateRequestFrame_MissingID(t *testing.T) {
	f := &protocol.RequestFrame{Type: "req", Method: "connect"}
	if err := protocol.ValidateRequestFrame(f); err == nil {
		t.Error("expected error for missing ID")
	}
}

func TestValidateRequestFrame_MissingMethod(t *testing.T) {
	f := &protocol.RequestFrame{Type: "req", ID: "abc"}
	if err := protocol.ValidateRequestFrame(f); err == nil {
		t.Error("expected error for missing method")
	}
}

func TestValidateResponseFrame_OKWithoutPayload(t *testing.T) {
	f := &protocol.ResponseFrame{Type: "res", ID: "abc", OK: true}
	if err := protocol.ValidateResponseFrame(f); err == nil {
		t.Error("expected error for ok=true without payload")
	}
}

func TestValidateResponseFrame_ErrorWithoutError(t *testing.T) {
	f := &protocol.ResponseFrame{Type: "res", ID: "abc", OK: false}
	if err := protocol.ValidateResponseFrame(f); err == nil {
		t.Error("expected error for ok=false without error")
	}
}

func TestNewResponse(t *testing.T) {
	res := protocol.NewResponse("id-1", true, map[string]string{"key": "val"}, nil)
	if res.Type != "res" {
		t.Errorf("Type = %q, want %q", res.Type, "res")
	}
	if res.ID != "id-1" {
		t.Errorf("ID = %q, want %q", res.ID, "id-1")
	}
	if !res.OK {
		t.Error("OK should be true")
	}
	var payload map[string]string
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["key"] != "val" {
		t.Errorf("payload[key] = %q, want %q", payload["key"], "val")
	}
}

func TestNewEvent(t *testing.T) {
	ev := protocol.NewEvent("tick", nil, 42)
	if ev.Type != "event" {
		t.Errorf("Type = %q, want %q", ev.Type, "event")
	}
	if ev.Event != "tick" {
		t.Errorf("Event = %q, want %q", ev.Event, "tick")
	}
	if ev.Seq != 42 {
		t.Errorf("Seq = %d, want %d", ev.Seq, 42)
	}
}

func TestCanonicalSignPayload_Deterministic(t *testing.T) {
	p1 := protocol.CanonicalSignPayload("dev1", "cli1", "web", "op", []string{"a", "b"}, 1000, "tok", "n1")
	p2 := protocol.CanonicalSignPayload("dev1", "cli1", "web", "op", []string{"a", "b"}, 1000, "tok", "n1")
	if string(p1) != string(p2) {
		t.Error("canonical payload not deterministic")
	}
	// Verify it's valid JSON
	var m map[string]any
	if err := json.Unmarshal(p1, &m); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
}

func TestVerifyDeviceSignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	payload := protocol.CanonicalSignPayload("dev1", "cli1", "web", "op", []string{"a"}, 1000, "", "nonce")
	sig := ed25519.Sign(priv, payload)

	pubB64 := protocol.Base64URLEncode(pub)
	sigB64 := protocol.Base64URLEncode(sig)

	if err := protocol.VerifyDeviceSignature(pubB64, sigB64, payload); err != nil {
		t.Errorf("valid signature failed: %v", err)
	}
}

func TestVerifyDeviceSignature_Invalid(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("test payload")
	badSig := make([]byte, ed25519.SignatureSize)

	pubB64 := protocol.Base64URLEncode(pub)
	sigB64 := protocol.Base64URLEncode(badSig)

	if err := protocol.VerifyDeviceSignature(pubB64, sigB64, payload); err == nil {
		t.Error("expected error for invalid signature")
	}
}

func TestVerifyDeviceSignature_BadKey(t *testing.T) {
	err := protocol.VerifyDeviceSignature("not-valid-key!!!", "sig", []byte("payload"))
	if err == nil {
		t.Error("expected error for bad key")
	}
}

func TestBase64URLRoundTrip(t *testing.T) {
	original := []byte("hello world, this is a test of base64url encoding")
	encoded := protocol.Base64URLEncode(original)
	decoded, err := protocol.Base64URLDecode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != string(original) {
		t.Errorf("round trip failed: got %q, want %q", decoded, original)
	}
}
