package wasm

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

func TestWasmPlugin_BeforeRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wasm-plugin-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	guestCode := `
package main

import (
	"encoding/json"
	"unsafe"
)

//export alloc
func alloc(size uint32) *byte {
	buf := make([]byte, size)
	return &buf[0]
}

type InvocationContextJSON struct {
	InvocationID string ` + "`json:\"invocation_id\"`" + `
	SessionID    string ` + "`json:\"session_id\"`" + `
	UserID       string ` + "`json:\"user_id\"`" + `
	AppName      string ` + "`json:\"app_name\"`" + `
	Branch       string ` + "`json:\"branch\"`" + `
	Input        string ` + "`json:\"input,omitempty\"`" + `
}

type BeforeRunInput struct {
	Context InvocationContextJSON ` + "`json:\"context\"`" + `
}

type ContentPart struct {
	Text string ` + "`json:\"text,omitempty\"`" + `
}

type Content struct {
	Parts []ContentPart ` + "`json:\"parts,omitempty\"`" + `
}

type ContentOutput struct {
	Content *Content ` + "`json:\"content,omitempty\"`" + `
	Error   string   ` + "`json:\"error,omitempty\"`" + `
}

//export before_run
func before_run(ptr *byte, size uint32) uint64 {
	inBytes := unsafe.Slice(ptr, size)
	var input BeforeRunInput
	if err := json.Unmarshal(inBytes, &input); err != nil {
		return 0
	}

	out := ContentOutput{
		Content: &Content{
			Parts: []ContentPart{
				{Text: "Modified: " + input.Context.Input},
			},
		},
	}
	outBytes, _ := json.Marshal(out)
	
	outPtr := &outBytes[0]
	outLen := uint32(len(outBytes))
	return (uint64(uintptr(unsafe.Pointer(outPtr))) << 32) | uint64(outLen)
}

func main() {}
`

	srcPath := filepath.Join(tmpDir, "plugin.go")
	if err := os.WriteFile(srcPath, []byte(guestCode), 0644); err != nil {
		t.Fatalf("failed to write guest src: %v", err)
	}

	wasmPath := filepath.Join(tmpDir, "plugin.wasm")
	cmd := exec.Command("tinygo", "build", "-o", wasmPath, "-target", "wasi", "-buildmode", "c-shared", srcPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile wasm plugin: %v\nOutput: %s", err, string(out))
	}

	// Load the plugin
	ctx := context.Background()
	pl, err := NewWasmPlugin(ctx, "test-wasm-plugin", wasmPath, map[string]any{"some_config": "value"})
	if err != nil {
		t.Fatalf("failed to load wasm plugin: %v", err)
	}
	defer pl.Close()

	if pl.BeforeRunCallback() == nil {
		t.Fatalf("expected BeforeRunCallback to be bound")
	}

	// Mock invocation context
	mockSvc := session.InMemoryService()
	resp, err := mockSvc.Create(ctx, &session.CreateRequest{
		AppName: "app",
		UserID:  "user",
	})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	sess := resp.Session

	mockCtx := &mockInvocationContext{
		ctx:     ctx,
		session: sess,
		input:   "Hello WASM",
	}

	content, err := pl.BeforeRunCallback()(mockCtx)
	if err != nil {
		t.Fatalf("BeforeRunCallback failed: %v", err)
	}

	if content == nil {
		t.Fatalf("expected non-nil content")
	}

	if len(content.Parts) != 1 || content.Parts[0].Text != "Modified: Hello WASM" {
		t.Errorf("expected modified text, got: %v", content)
	}
}

type mockInvocationContext struct {
	ctx     context.Context
	session session.Session
	input   string
}

func (c *mockInvocationContext) Deadline() (time.Time, bool) { return c.ctx.Deadline() }
func (c *mockInvocationContext) Done() <-chan struct{}       { return c.ctx.Done() }
func (c *mockInvocationContext) Err() error                  { return c.ctx.Err() }
func (c *mockInvocationContext) Value(key any) any           { return c.ctx.Value(key) }
func (c *mockInvocationContext) Agent() agent.Agent          { return nil }
func (c *mockInvocationContext) Artifacts() agent.Artifacts  { return nil }
func (c *mockInvocationContext) Memory() agent.Memory        { return nil }
func (c *mockInvocationContext) Session() session.Session    { return c.session }
func (c *mockInvocationContext) Input() string               { return c.input }
func (c *mockInvocationContext) InvocationID() string        { return "test-id" }
func (c *mockInvocationContext) Branch() string              { return "test-branch" }
func (c *mockInvocationContext) IsolationScope() string      { return "" }
func (c *mockInvocationContext) UserContent() *genai.Content { return nil }
func (c *mockInvocationContext) RunConfig() *agent.RunConfig { return nil }
func (c *mockInvocationContext) EndInvocation()              {}
func (c *mockInvocationContext) Ended() bool                 { return false }
func (c *mockInvocationContext) ResumedInput(interruptID string) (any, bool) { return nil, false }
func (c *mockInvocationContext) WithContext(ctx context.Context) agent.InvocationContext {
	return &mockInvocationContext{ctx: ctx, session: c.session, input: c.input}
}
