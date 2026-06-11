//go:build tinygo.wasm

package main

import (
	"encoding/json"
	"strings"
	"unsafe"
)

// Declare host logger import
//go:wasmimport env log_msg
func log_msg(ptr int32, len int32)

func logString(msg string) {
	buf := []byte(msg)
	log_msg(int32(uintptr(unsafe.Pointer(&buf[0]))), int32(len(buf)))
}

//export alloc
func alloc(size uint32) *byte {
	buf := make([]byte, size)
	return &buf[0]
}

// Struct matching JSON payloads passed from the host

type InvocationContextJSON struct {
	InvocationID string `json:"invocation_id"`
	SessionID    string `json:"session_id"`
	UserID       string `json:"user_id"`
	AppName      string `json:"app_name"`
	Branch       string `json:"branch"`
	Input        string `json:"input,omitempty"`
}

type OnUserMessageInput struct {
	Context     InvocationContextJSON `json:"context"`
	UserMessage Content               `json:"user_message"`
}

type ContentPart struct {
	Text string `json:"text,omitempty"`
}

type Content struct {
	Parts []ContentPart `json:"parts,omitempty"`
}

type ContentOutput struct {
	Content *Content `json:"content,omitempty"`
	Error   string   `json:"error,omitempty"`
}

//export on_user_message
func on_user_message(ptr *byte, size uint32) uint64 {
	inBytes := unsafe.Slice(ptr, size)
	var input OnUserMessageInput
	if err := json.Unmarshal(inBytes, &input); err != nil {
		logString("wasm-plugin: failed to unmarshal input")
		return 0
	}

	logString("wasm-plugin: intercepted user message from user " + input.Context.UserID)

	// Simple check: if message contains "secret", return a guardrail error
	hasSecret := false
	for _, part := range input.UserMessage.Parts {
		if strings.Contains(strings.ToLower(part.Text), "secret") {
			hasSecret = true
			break
		}
	}

	if hasSecret {
		logString("wasm-plugin: blocked message containing secret keyword")
		out := ContentOutput{
			Error: "Guardrail blocked: User message contains sensitive keyword 'secret'",
		}
		outBytes, _ := json.Marshal(out)
		outPtr := &outBytes[0]
		outLen := uint32(len(outBytes))
		return (uint64(uintptr(unsafe.Pointer(outPtr))) << 32) | uint64(outLen)
	}

	// Otherwise, modify the message to append a suffix
	var modifiedParts []ContentPart
	for _, part := range input.UserMessage.Parts {
		modifiedParts = append(modifiedParts, ContentPart{
			Text: part.Text + "\n\n[Secured by WasmPlugin]",
		})
	}

	out := ContentOutput{
		Content: &Content{
			Parts: modifiedParts,
		},
	}
	outBytes, _ := json.Marshal(out)
	outPtr := &outBytes[0]
	outLen := uint32(len(outBytes))
	return (uint64(uintptr(unsafe.Pointer(outPtr))) << 32) | uint64(outLen)
}

func main() {}
