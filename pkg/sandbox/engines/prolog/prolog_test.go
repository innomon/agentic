package prolog

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/innomon/agentic/pkg/sandbox"
)

type mockTools struct {
	sandbox.ToolRegistry
	called bool
}

func (m *mockTools) CallTool(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	m.called = true
	return map[string]any{"result": "mocked"}, nil
}

func TestPrologBasic(t *testing.T) {
	vm := NewPrologVM()
	var logs strings.Builder
	host := &sandbox.HostContext{
		Logger: &logs,
	}
	cfg := sandbox.VMConfig{
		Type:    "prolog",
		Timeout: time.Second,
	}

	if err := vm.Init(cfg, host); err != nil {
		t.Fatal(err)
	}
	defer vm.Close()

	res, err := vm.Run(context.Background(), "log('hello from prolog').")
	if err != nil {
		t.Fatal(err)
	}

	if res != "solution found" {
		t.Errorf("expected 'solution found', got %v", res)
	}

	if !strings.Contains(logs.String(), "hello from prolog") {
		t.Errorf("logs missing expected output: %q", logs.String())
	}
}

func TestPrologTools(t *testing.T) {
	vm := NewPrologVM()
	mt := &mockTools{}
	host := &sandbox.HostContext{
		Tools: mt,
	}
	cfg := sandbox.VMConfig{
		Type:       "prolog",
		AllowTools: []string{"test_tool"},
	}

	if err := vm.Init(cfg, host); err != nil {
		t.Fatal(err)
	}
	defer vm.Close()

	// In Prolog we query: test_tool(Args, Result)
	// We use an atom 'args' for now as placeholder for map
	res, err := vm.Run(context.Background(), "test_tool(args, R).")
	if err != nil {
		t.Fatal(err)
	}

	if !mt.called {
		t.Error("mock tool was not called")
	}

	if res != "solution found" {
		t.Errorf("expected 'solution found', got %v", res)
	}
}
