package starlark

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

func TestStarlarkBasic(t *testing.T) {
	vm := NewStarlarkVM()
	var logs strings.Builder
	host := &sandbox.HostContext{
		Logger: &logs,
	}
	cfg := sandbox.VMConfig{
		Type:    "starlark",
		Timeout: time.Second,
	}

	if err := vm.Init(cfg, host); err != nil {
		t.Fatal(err)
	}
	defer vm.Close()

	res, err := vm.Run(context.Background(), "result = 1 + 1")
	if err != nil {
		t.Fatal(err)
	}

	if res != int64(2) {
		t.Errorf("expected 2, got %v (%T)", res, res)
	}

	_, err = vm.Run(context.Background(), "log('hello from starlark')")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(logs.String(), "hello from starlark") {
		t.Errorf("logs missing expected output: %q", logs.String())
	}
}

func TestStarlarkTools(t *testing.T) {
	vm := NewStarlarkVM()
	mt := &mockTools{}
	host := &sandbox.HostContext{
		Tools: mt,
	}
	cfg := sandbox.VMConfig{
		Type:       "starlark",
		AllowTools: []string{"test_tool"},
	}

	if err := vm.Init(cfg, host); err != nil {
		t.Fatal(err)
	}
	defer vm.Close()

	res, err := vm.Run(context.Background(), "result = test_tool({'foo': 'bar'})")
	if err != nil {
		t.Fatal(err)
	}

	if !mt.called {
		t.Error("mock tool was not called")
	}

	t.Logf("res type: %T, value: %+v", res, res)
	if m, ok := res.(map[string]any); ok {
		for k, v := range m {
			t.Logf("key: %q (len %d), val: %v", k, len(k), v)
		}
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", res)
	}
	if m["result"] != "mocked" {
		t.Errorf("expected 'mocked', got %v", m["result"])
	}
}
