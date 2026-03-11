package quickjs

import (
	"context"
	"fmt"
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

func TestQuickJSBasic(t *testing.T) {
	vm := NewQuickJSVM()
	var logs strings.Builder
	host := &sandbox.HostContext{
		Logger: &logs,
	}
	cfg := sandbox.VMConfig{
		Type:    "quickjs",
		Timeout: time.Second,
	}

	if err := vm.Init(cfg, host); err != nil {
		t.Fatal(err)
	}
	defer vm.Close()

	res, err := vm.Run(context.Background(), "1 + 1")
	if err != nil {
		t.Fatal(err)
	}

	// QuickJS might return float64 or int
	if fmt.Sprint(res) != "2" {
		t.Errorf("expected 2, got %v (%T)", res, res)
	}

	_, err = vm.Run(context.Background(), "log('hello from js')")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(logs.String(), "hello from js") {
		t.Errorf("logs missing expected output: %q", logs.String())
	}
}

func TestQuickJSTools(t *testing.T) {
	vm := NewQuickJSVM()
	mt := &mockTools{}
	host := &sandbox.HostContext{
		Tools: mt,
	}
	cfg := sandbox.VMConfig{
		Type:       "quickjs",
		AllowTools: []string{"test_tool"},
	}

	if err := vm.Init(cfg, host); err != nil {
		t.Fatal(err)
	}
	defer vm.Close()

	res, err := vm.Run(context.Background(), "test_tool({foo: 'bar'})")
	if err != nil {
		t.Fatal(err)
	}

	if !mt.called {
		t.Error("mock tool was not called")
	}

	// Result from mock tool was map[string]any{"result": "mocked"}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", res)
	}
	if m["result"] != "mocked" {
		t.Errorf("expected 'mocked', got %v", m["result"])
	}
}
