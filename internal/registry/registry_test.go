package registry

import (
	"context"
	"io"
	"iter"
	"testing"

	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

// --- mock types ---

type mockModelConfig struct {
	Default bool `yaml:"default"`
}

func (m *mockModelConfig) IsDefault() bool {
	return m.Default
}

type mockCloser struct {
	closed bool
}

func (m *mockCloser) Close() error {
	m.closed = true
	return nil
}

type mockLLM struct{}

func (m *mockLLM) Name() string { return "mock" }
func (m *mockLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {}
}

type mockTool struct {
	name string
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return "mock tool" }
func (m *mockTool) IsLongRunning() bool { return false }

type testToolConfig struct {
	ToolBase `yaml:",inline"`
}

// --- tests ---

func TestNew(t *testing.T) {
	cfg := &Config{
		Models: map[string]ModelEntry{
			"test-model": {
				Name:     "test-model",
				Provider: "test-provider",
				Config:   &mockModelConfig{Default: true},
			},
		},
	}
	reg := New(cfg)
	if reg == nil {
		t.Fatal("New() returned nil")
	}
	if reg.cfg != cfg {
		t.Errorf("New() did not set the config correctly")
	}
}

func TestGetDefaultModel(t *testing.T) {
	cfg := &Config{
		Models: map[string]ModelEntry{
			"test-model": {
				Name:     "test-model",
				Provider: "test-provider",
				Config:   &mockModelConfig{Default: true},
			},
		},
	}
	reg := New(cfg)
	if reg == nil {
		t.Fatal("New() returned nil")
	}
	RegisterModelProvider("test-provider", func(ctx context.Context, cfg *mockModelConfig) (model.LLM, error) {
		return &mockLLM{}, nil
	})
	name, err := reg.GetDefaultModel(context.Background())
	if err != nil {
		t.Fatalf("GetDefaultModel() returned an error: %v", err)
	}
	if name.Name() != "mock" {
		t.Errorf("GetDefaultModel() returned the wrong model: got %q, want %q", name.Name(), "mock")
	}
}

func TestGet(t *testing.T) {
	cfg := &Config{
		Models: map[string]ModelEntry{
			"test-model": {
				Name:     "test-model",
				Provider: "test-provider",
				Config:   &mockModelConfig{Default: true},
			},
		},
	}
	reg := New(cfg)
	if reg == nil {
		t.Fatal("New() returned nil")
	}

	// Pre-register a dummy creator function for the test provider.
	RegisterModelProvider("test-provider", func(ctx context.Context, cfg *mockModelConfig) (model.LLM, error) {
		return &mockLLM{}, nil
	})

	m, err := Get[model.LLM](context.Background(), reg, "test-model")
	if err != nil {
		t.Fatalf("Get() returned an error: %v", err)
	}
	if m == nil {
		t.Fatal("Get() returned a nil model")
	}
}

func TestClose(t *testing.T) {
	cfg := &Config{}
	reg := New(cfg)
	if reg == nil {
		t.Fatal("New() returned nil")
	}
	closer := &mockCloser{}
	reg.closers = []io.Closer{closer}
	reg.Close()
	if !closer.closed {
		t.Errorf("Close() did not close the closer")
	}
}

func TestGetTools(t *testing.T) {
	cfg := &Config{
		Tools: map[string]ToolEntry{
			"test-tool-1": {
				Name:   "test-tool-1",
				Type:   "test-type",
				Config: &testToolConfig{},
			},
			"test-tool-2": {
				Name:   "test-tool-2",
				Type:   "test-type",
				Config: &testToolConfig{},
			},
		},
	}
	reg := New(cfg)
	if reg == nil {
		t.Fatal("New() returned nil")
	}

	RegisterToolType("test-type", func(ctx context.Context, name string, cfg *testToolConfig) (tool.Tool, error) {
		return &mockTool{name: name}, nil
	})

	tools, err := reg.GetTools(context.Background(), []string{"test-tool-1", "test-tool-2"})
	if err != nil {
		t.Fatalf("GetTools() returned an error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name() != "test-tool-1" {
		t.Errorf("expected tool name %q, got %q", "test-tool-1", tools[0].Name())
	}
	if tools[1].Name() != "test-tool-2" {
		t.Errorf("expected tool name %q, got %q", "test-tool-2", tools[1].Name())
	}
}
