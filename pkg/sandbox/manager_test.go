package sandbox

import (
	"context"
	"io"
	"testing"
	"time"
)

type mockVM struct {
	initCalled bool
	runCalled  bool
}

func (m *mockVM) Init(cfg VMConfig, host *HostContext) error {
	m.initCalled = true
	return nil
}

func (m *mockVM) Run(ctx context.Context, code string) (any, error) {
	m.runCalled = true
	if code == "error" {
		return nil, io.EOF
	}
	if code == "log" {
		if _, err := io.WriteString(ctx.Value("logger").(io.Writer), "test log"); err != nil {
			return nil, err
		}
	}
	return "ok", nil
}

func (m *mockVM) Reset() error { return nil }
func (m *mockVM) Close() error { return nil }

func TestManager(t *testing.T) {
	RegisterVMEngine("mock", func() SandboxVM { return &mockVM{} })

	host := &HostContext{Logger: io.Discard}
	mgr := NewManager(host)

	cfg := VMConfig{Type: "mock", Timeout: time.Second}
	vm, err := mgr.GetOrCreateVM("test", cfg)
	if err != nil {
		t.Fatal(err)
	}

	if !vm.(*mockVM).initCalled {
		t.Error("Init was not called")
	}

	ctx := context.WithValue(context.Background(), "logger", mgr.hostCtx.Logger)
	res, err := mgr.Run(ctx, "test", "hello")
	if err != nil {
		t.Fatal(err)
	}

	if res.Value != "ok" {
		t.Errorf("expected ok, got %v", res.Value)
	}

	if !vm.(*mockVM).runCalled {
		t.Error("Run was not called")
	}
}

func TestManagerLogCapture(t *testing.T) {
	RegisterVMEngine("mock", func() SandboxVM { return &mockVM{} })

	var out io.Writer = io.Discard
	mgr := NewManager(&HostContext{Logger: out})

	cfg := VMConfig{Type: "mock"}
	_, _ = mgr.GetOrCreateVM("log-test", cfg)

	ctx := context.WithValue(context.Background(), "logger", mgr.hostCtx.Logger)
	res, err := mgr.Run(ctx, "log-test", "log")
	if err != nil {
		t.Fatal(err)
	}

	if res.Logs != "test log" {
		t.Errorf("expected 'test log', got %q", res.Logs)
	}
}
