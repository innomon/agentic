package registry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"
	"testing"

	"github.com/innomon/agentic/pkg/compreg"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"gopkg.in/yaml.v3"
)

// --- mock types ---

type mockModelConfig struct {
	Provider string `yaml:"provider"`
	Default  bool   `yaml:"default"`
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

type errCloser struct {
	err error
}

func (e *errCloser) Close() error { return e.err }

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

// newMockAgent creates a real agent.Agent via agent.New for testing.
func newMockAgent(name string) agent.Agent {
	a, err := agent.New(agent.Config{
		Name:        name,
		Description: "mock agent",
		Run: func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {}
		},
	})
	if err != nil {
		panic(fmt.Sprintf("failed to create mock agent: %v", err))
	}
	return a
}

// helper to build yaml.Node from YAML string
func yamlNode(t *testing.T, s string) *yaml.Node {
	t.Helper()
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(s), &node); err != nil {
		t.Fatalf("failed to unmarshal YAML: %v", err)
	}
	// yaml.Unmarshal wraps in a document node
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return &node
}

// uniqueName generates a test-unique key to avoid compreg collisions.
func uniqueName(t *testing.T, prefix string) string {
	return prefix + "-" + t.Name()
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
	provName := uniqueName(t, "provider")
	cfg := &Config{
		Models: map[string]ModelEntry{
			"test-model": {
				Name:     "test-model",
				Provider: provName,
				Config:   &mockModelConfig{Default: true},
			},
		},
	}
	reg := New(cfg)
	RegisterModelProvider(provName, func(ctx context.Context, cfg *mockModelConfig) (model.LLM, error) {
		return &mockLLM{}, nil
	})
	m, err := reg.GetDefaultModel(context.Background())
	if err != nil {
		t.Fatalf("GetDefaultModel() returned an error: %v", err)
	}
	if m.Name() != "mock" {
		t.Errorf("GetDefaultModel() returned wrong model: got %q, want %q", m.Name(), "mock")
	}
}

func TestGetDefaultModel_NoModels(t *testing.T) {
	cfg := &Config{Models: map[string]ModelEntry{}}
	reg := New(cfg)
	_, err := reg.GetDefaultModel(context.Background())
	if err == nil {
		t.Fatal("expected error for no models configured")
	}
	if !strings.Contains(err.Error(), "no models configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetDefaultModel_FallbackToFirst(t *testing.T) {
	provName := uniqueName(t, "provider")
	cfg := &Config{
		Models: map[string]ModelEntry{
			"only-model": {
				Name:     "only-model",
				Provider: provName,
				Config:   &mockModelConfig{Default: false},
			},
		},
	}
	reg := New(cfg)
	RegisterModelProvider(provName, func(ctx context.Context, cfg *mockModelConfig) (model.LLM, error) {
		return &mockLLM{}, nil
	})
	m, err := reg.GetDefaultModel(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected a model when fallback is used")
	}
}

func TestGet(t *testing.T) {
	provName := uniqueName(t, "provider")
	cfg := &Config{
		Models: map[string]ModelEntry{
			"test-model": {
				Name:     "test-model",
				Provider: provName,
				Config:   &mockModelConfig{Default: true},
			},
		},
	}
	reg := New(cfg)
	RegisterModelProvider(provName, func(ctx context.Context, cfg *mockModelConfig) (model.LLM, error) {
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

func TestGet_CacheHit(t *testing.T) {
	provName := uniqueName(t, "provider")
	callCount := 0
	cfg := &Config{
		Models: map[string]ModelEntry{
			"cached-model": {
				Name:     "cached-model",
				Provider: provName,
				Config:   &mockModelConfig{Default: true},
			},
		},
	}
	reg := New(cfg)
	RegisterModelProvider(provName, func(ctx context.Context, cfg *mockModelConfig) (model.LLM, error) {
		callCount++
		return &mockLLM{}, nil
	})

	_, _ = Get[model.LLM](context.Background(), reg, "cached-model")
	_, _ = Get[model.LLM](context.Background(), reg, "cached-model")

	if callCount != 1 {
		t.Errorf("creator called %d times, expected 1 (cache should hit)", callCount)
	}
}

func TestGet_ModelNotFound(t *testing.T) {
	cfg := &Config{Models: map[string]ModelEntry{}}
	reg := New(cfg)
	_, err := Get[model.LLM](context.Background(), reg, "missing")
	if err == nil {
		t.Fatal("expected error for missing model")
	}
	if !strings.Contains(err.Error(), `model "missing" not found`) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGet_ToolNotFound(t *testing.T) {
	cfg := &Config{Tools: map[string]ToolEntry{}}
	reg := New(cfg)
	_, err := Get[tool.Tool](context.Background(), reg, "missing")
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
	if !strings.Contains(err.Error(), `tool "missing" not found`) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGet_AgentNotFound(t *testing.T) {
	cfg := &Config{Agents: map[string]AgentEntry{}}
	reg := New(cfg)
	_, err := Get[agent.Agent](context.Background(), reg, "missing")
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
	if !strings.Contains(err.Error(), `agent "missing" not found`) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClose(t *testing.T) {
	cfg := &Config{}
	reg := New(cfg)
	closer := &mockCloser{}
	reg.closers = []io.Closer{closer}
	if err := reg.Close(); err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}
	if !closer.closed {
		t.Errorf("Close() did not close the closer")
	}
}

func TestClose_ErrorAggregation(t *testing.T) {
	cfg := &Config{}
	reg := New(cfg)
	reg.closers = []io.Closer{
		&mockCloser{},
		&errCloser{err: errors.New("close error 1")},
		&errCloser{err: errors.New("close error 2")},
	}
	err := reg.Close()
	if err == nil {
		t.Fatal("expected error from Close()")
	}
	if !strings.Contains(err.Error(), "close error 1") || !strings.Contains(err.Error(), "close error 2") {
		t.Errorf("expected both errors aggregated, got: %v", err)
	}
}

func TestClose_NoClosers(t *testing.T) {
	cfg := &Config{}
	reg := New(cfg)
	if err := reg.Close(); err != nil {
		t.Errorf("Close() with no closers should return nil, got: %v", err)
	}
}

func TestGetTools(t *testing.T) {
	typeName := uniqueName(t, "tool-type")
	cfg := &Config{
		Tools: map[string]ToolEntry{
			"test-tool-1": {Name: "test-tool-1", Type: typeName, Config: &testToolConfig{}},
			"test-tool-2": {Name: "test-tool-2", Type: typeName, Config: &testToolConfig{}},
		},
	}
	reg := New(cfg)
	RegisterToolType(typeName, func(ctx context.Context, name string, cfg *testToolConfig, _ SandboxRegistry) (tool.Tool, error) {
		return &mockTool{name: name}, nil
	})

	tools, err := reg.GetTools(context.Background(), []string{"test-tool-1", "test-tool-2"})
	if err != nil {
		t.Fatalf("GetTools() returned an error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
}

func TestGetTools_EmptyNames(t *testing.T) {
	reg := New(&Config{})
	tools, err := reg.GetTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tools != nil {
		t.Errorf("expected nil, got %v", tools)
	}

	tools, err = reg.GetTools(context.Background(), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tools != nil {
		t.Errorf("expected nil, got %v", tools)
	}
}

func TestGetTools_MissingTool(t *testing.T) {
	cfg := &Config{Tools: map[string]ToolEntry{}}
	reg := New(cfg)
	_, err := reg.GetTools(context.Background(), []string{"missing"})
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}

func TestGetRoot_Default(t *testing.T) {
	agentType := uniqueName(t, "agent-type")
	cfg := &Config{
		RootAgent: "",
		Agents: map[string]AgentEntry{
			"RootAgent": {Name: "RootAgent", Type: agentType, Config: &SequentialAgentConfig{}},
		},
	}
	reg := New(cfg)
	RegisterAgentType(agentType, func(ctx context.Context, name string, cfg *SequentialAgentConfig, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
		return newMockAgent(name), nil
	})

	a, err := reg.GetRoot(context.Background())
	if err != nil {
		t.Fatalf("GetRoot() error: %v", err)
	}
	if a.Name() != "RootAgent" {
		t.Errorf("expected RootAgent, got %q", a.Name())
	}
}

func TestGetRoot_CustomName(t *testing.T) {
	agentType := uniqueName(t, "agent-type")
	cfg := &Config{
		RootAgent: "MyRoot",
		Agents: map[string]AgentEntry{
			"MyRoot": {Name: "MyRoot", Type: agentType, Config: &SequentialAgentConfig{}},
		},
	}
	reg := New(cfg)
	RegisterAgentType(agentType, func(ctx context.Context, name string, cfg *SequentialAgentConfig, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
		return newMockAgent(name), nil
	})

	a, err := reg.GetRoot(context.Background())
	if err != nil {
		t.Fatalf("GetRoot() error: %v", err)
	}
	if a.Name() != "MyRoot" {
		t.Errorf("expected MyRoot, got %q", a.Name())
	}
}

func TestGetRoot_SingleAgentFallback(t *testing.T) {
	agentType := uniqueName(t, "agent-type")
	cfg := &Config{
		RootAgent: "",
		Agents: map[string]AgentEntry{
			"OnlyAgent": {Name: "OnlyAgent", Type: agentType, Config: &SequentialAgentConfig{}},
		},
	}
	reg := New(cfg)
	RegisterAgentType(agentType, func(ctx context.Context, name string, cfg *SequentialAgentConfig, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
		return newMockAgent(name), nil
	})

	a, err := reg.GetRoot(context.Background())
	if err != nil {
		t.Fatalf("GetRoot() error: %v", err)
	}
	if a.Name() != "OnlyAgent" {
		t.Errorf("expected OnlyAgent, got %q", a.Name())
	}
}

func TestGetRoot_MultipleAgentsError(t *testing.T) {
	agentType := uniqueName(t, "agent-type")
	cfg := &Config{
		RootAgent: "",
		Agents: map[string]AgentEntry{
			"Agent1": {Name: "Agent1", Type: agentType, Config: &SequentialAgentConfig{}},
			"Agent2": {Name: "Agent2", Type: agentType, Config: &SequentialAgentConfig{}},
		},
	}
	reg := New(cfg)
	RegisterAgentType(agentType, func(ctx context.Context, name string, cfg *SequentialAgentConfig, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
		return newMockAgent(name), nil
	})

	_, err := reg.GetRoot(context.Background())
	if err == nil {
		t.Fatal("expected error when multiple agents are present and no RootAgent is specified")
	}
	if !strings.Contains(err.Error(), "multiple agents are available") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetRoot_Missing(t *testing.T) {
	cfg := &Config{RootAgent: "Missing", Agents: map[string]AgentEntry{}}
	reg := New(cfg)
	_, err := reg.GetRoot(context.Background())
	if err == nil {
		t.Fatal("expected error for missing root agent")
	}
}

func TestLoadAgent_CircularDependency(t *testing.T) {
	agentType := uniqueName(t, "agent-type")
	cfg := &Config{
		Agents: map[string]AgentEntry{
			"A": {Name: "A", Type: agentType, SubAgents: []string{"B"}, Config: &SequentialAgentConfig{}},
			"B": {Name: "B", Type: agentType, SubAgents: []string{"A"}, Config: &SequentialAgentConfig{}},
		},
	}
	reg := New(cfg)
	RegisterAgentType(agentType, func(ctx context.Context, name string, cfg *SequentialAgentConfig, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
		return newMockAgent(name), nil
	})

	_, err := Get[agent.Agent](context.Background(), reg, "A")
	if err == nil {
		t.Fatal("expected circular dependency error")
	}
	if !strings.Contains(err.Error(), "circular dependency") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadAgent_SelfCycle(t *testing.T) {
	agentType := uniqueName(t, "agent-type")
	cfg := &Config{
		Agents: map[string]AgentEntry{
			"Self": {Name: "Self", Type: agentType, SubAgents: []string{"Self"}, Config: &SequentialAgentConfig{}},
		},
	}
	reg := New(cfg)
	RegisterAgentType(agentType, func(ctx context.Context, name string, cfg *SequentialAgentConfig, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
		return newMockAgent(name), nil
	})

	_, err := Get[agent.Agent](context.Background(), reg, "Self")
	if err == nil {
		t.Fatal("expected circular dependency error")
	}
	if !strings.Contains(err.Error(), "circular dependency") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadAgent_SubAgentBuildFailure(t *testing.T) {
	agentType := uniqueName(t, "agent-type")
	cfg := &Config{
		Agents: map[string]AgentEntry{
			"A": {Name: "A", Type: agentType, SubAgents: []string{"Missing"}, Config: &SequentialAgentConfig{}},
		},
	}
	reg := New(cfg)
	RegisterAgentType(agentType, func(ctx context.Context, name string, cfg *SequentialAgentConfig, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
		return newMockAgent(name), nil
	})

	_, err := Get[agent.Agent](context.Background(), reg, "A")
	if err == nil {
		t.Fatal("expected error for missing sub-agent")
	}
	if !strings.Contains(err.Error(), `failed to build sub-agent "Missing" for "A"`) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadAgent_WithSubAgents(t *testing.T) {
	agentType := uniqueName(t, "agent-type")
	cfg := &Config{
		Agents: map[string]AgentEntry{
			"Parent": {Name: "Parent", Type: agentType, SubAgents: []string{"Child"}, Config: &SequentialAgentConfig{}},
			"Child":  {Name: "Child", Type: agentType, Config: &SequentialAgentConfig{}},
		},
	}
	reg := New(cfg)
	RegisterAgentType(agentType, func(ctx context.Context, name string, cfg *SequentialAgentConfig, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
		return newMockAgent(name), nil
	})

	a, err := Get[agent.Agent](context.Background(), reg, "Parent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Name() != "Parent" {
		t.Errorf("expected Parent, got %q", a.Name())
	}
}

func TestLoadAgent_Workflow(t *testing.T) {
	agentType := uniqueName(t, "agent-type")
	cfg := &Config{
		Agents: map[string]AgentEntry{
			"wf": {
				Name:      "wf",
				Type:      "workflow",
				SubAgents: []string{"Child"},
				Config: &WorkflowAgentConfig{
					Nodes: []WorkflowNodeEntry{
						{Name: "classify", Agent: "Child"},
					},
					Edges: []WorkflowEdgeEntry{
						{From: "START", To: "classify"},
					},
				},
			},
			"Child": {Name: "Child", Type: agentType, Config: &SequentialAgentConfig{}},
		},
	}
	reg := New(cfg)
	RegisterAgentType(agentType, func(ctx context.Context, name string, cfg *SequentialAgentConfig, models ModelRegistry, tools ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
		return newMockAgent(name), nil
	})

	a, err := Get[agent.Agent](context.Background(), reg, "wf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Name() != "wf" {
		t.Errorf("expected wf, got %q", a.Name())
	}
	if len(a.SubAgents()) != 1 || a.SubAgents()[0].Name() != "Child" {
		t.Errorf("expected Child sub-agent, got %v", a.SubAgents())
	}
}

func TestLoadAgent_UnknownType(t *testing.T) {
	cfg := &Config{
		Agents: map[string]AgentEntry{
			"A": {Name: "A", Type: "does-not-exist-type", Config: &SequentialAgentConfig{}},
		},
	}
	reg := New(cfg)
	_, err := Get[agent.Agent](context.Background(), reg, "A")
	if err == nil {
		t.Fatal("expected error for unknown agent type")
	}
	if !strings.Contains(err.Error(), "unknown agent type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateModel_UnknownProvider(t *testing.T) {
	_, err := createModel(context.Background(), "nonexistent-provider", nil)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateTool_UnknownType(t *testing.T) {
	_, err := createTool(context.Background(), "nonexistent-type", "t", nil, nil)
	if err == nil {
		t.Fatal("expected error for unknown tool type")
	}
	if !strings.Contains(err.Error(), "unknown tool type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateAgent_UnknownType(t *testing.T) {
	_, err := createAgent(context.Background(), "nonexistent-type", "a", nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for unknown agent type")
	}
	if !strings.Contains(err.Error(), "unknown agent type") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Config tests ---

func TestConfig_GetModel_NotFound(t *testing.T) {
	cfg := &Config{Models: map[string]ModelEntry{}}
	_, err := cfg.GetModel("missing")
	if err == nil || !strings.Contains(err.Error(), `model "missing" not found`) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfig_GetAgent_NotFound(t *testing.T) {
	cfg := &Config{Agents: map[string]AgentEntry{}}
	_, err := cfg.GetAgent("missing")
	if err == nil || !strings.Contains(err.Error(), `agent "missing" not found`) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfig_GetTool_NotFound(t *testing.T) {
	cfg := &Config{Tools: map[string]ToolEntry{}}
	_, err := cfg.GetTool("missing")
	if err == nil || !strings.Contains(err.Error(), `tool "missing" not found`) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfig_GetDefaultModel_NoModels(t *testing.T) {
	cfg := &Config{Models: map[string]ModelEntry{}}
	_, _, err := cfg.GetDefaultModel()
	if err == nil || !strings.Contains(err.Error(), "no models configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfig_GetDefaultModel_DefaultFlag(t *testing.T) {
	cfg := &Config{
		Models: map[string]ModelEntry{
			"a": {Name: "a", Config: &mockModelConfig{Default: false}},
			"b": {Name: "b", Config: &mockModelConfig{Default: true}},
		},
	}
	name, entry, err := cfg.GetDefaultModel()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "b" || entry.Name != "b" {
		t.Errorf("expected default model 'b', got %q", name)
	}
}

func TestConfig_GetDefaultModel_NoDefaultFlag(t *testing.T) {
	cfg := &Config{
		Models: map[string]ModelEntry{
			"only": {Name: "only", Config: &mockModelConfig{Default: false}},
		},
	}
	name, _, err := cfg.GetDefaultModel()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "only" {
		t.Errorf("expected fallback model 'only', got %q", name)
	}
}

// --- RawConfig UnmarshalYAML tests ---

func TestRawConfig_UnmarshalYAML(t *testing.T) {
	input := `
root_agent: MyRoot
models:
  m1:
    provider: test
agents:
  a1:
    type: llm
tools:
  t1:
    type: builtin
session:
  provider: inmemory
memory:
  provider: inmemory
auth:
  jwt:
    issuer: test
`
	var raw RawConfig
	if err := yaml.Unmarshal([]byte(input), &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if raw.RootAgent != "MyRoot" {
		t.Errorf("expected root_agent MyRoot, got %q", raw.RootAgent)
	}
	if _, ok := raw.Models["m1"]; !ok {
		t.Error("expected model m1")
	}
	if _, ok := raw.Agents["a1"]; !ok {
		t.Error("expected agent a1")
	}
	if _, ok := raw.Tools["t1"]; !ok {
		t.Error("expected tool t1")
	}
	if raw.Session == nil {
		t.Error("expected session config")
	}
	if raw.Memory == nil {
		t.Error("expected memory config")
	}
	if raw.Auth == nil {
		t.Error("expected auth config")
	}
}

func TestRawConfig_UnmarshalYAML_NotMapping(t *testing.T) {
	var raw RawConfig
	err := yaml.Unmarshal([]byte(`"just a string"`), &raw)
	if err == nil {
		t.Fatal("expected error for non-mapping node")
	}
	if !strings.Contains(err.Error(), "expected mapping node") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRawConfig_UnmarshalYAML_UnknownKeys(t *testing.T) {
	input := `
unknown_key: some_value
models:
  m1:
    provider: test
`
	var raw RawConfig
	if err := yaml.Unmarshal([]byte(input), &raw); err != nil {
		t.Fatalf("unknown keys should be ignored: %v", err)
	}
	if _, ok := raw.Models["m1"]; !ok {
		t.Error("expected model m1")
	}
}

func TestRawConfig_UnmarshalYAML_NonMappingModels(t *testing.T) {
	input := `
models: "not a mapping"
`
	var raw RawConfig
	if err := yaml.Unmarshal([]byte(input), &raw); err != nil {
		t.Fatalf("non-mapping models should not error: %v", err)
	}
	if len(raw.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(raw.Models))
	}
}

// --- DecodeModelConfig tests ---

func TestDecodeModelConfig_MissingProvider(t *testing.T) {
	node := yamlNode(t, `model_id: test`)
	_, _, err := DecodeModelConfig("m1", node)
	if err == nil || !strings.Contains(err.Error(), "missing provider") {
		t.Errorf("expected missing provider error, got: %v", err)
	}
}

func TestDecodeModelConfig_UnknownProvider(t *testing.T) {
	node := yamlNode(t, `provider: totally-unknown-provider-xyz`)
	_, _, err := DecodeModelConfig("m1", node)
	if err == nil || !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("expected unknown provider error, got: %v", err)
	}
}

func TestDecodeModelConfig_Happy(t *testing.T) {
	provName := uniqueName(t, "provider")
	RegisterModelProvider(provName, func(ctx context.Context, cfg *mockModelConfig) (model.LLM, error) {
		return &mockLLM{}, nil
	})
	node := yamlNode(t, fmt.Sprintf("provider: %s\ndefault: true", provName))
	prov, cfg, err := DecodeModelConfig("m1", node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prov != provName {
		t.Errorf("expected provider %q, got %q", provName, prov)
	}
	if cfg == nil {
		t.Error("expected non-nil config")
	}
}

// --- DecodeAgentConfig tests ---

func TestDecodeAgentConfig_DefaultType(t *testing.T) {
	// "llm" is registered in init(), but will fail validation (no model)
	node := yamlNode(t, `description: test`)
	typeName, _, err := DecodeAgentConfig("a1", node)
	if err == nil {
		// If it succeeds, type should be llm; if it fails, it's validation
		t.Logf("type: %s", typeName)
	}
	// Either way, the type discriminator should default to "llm"
	if err != nil && !strings.Contains(err.Error(), "model is required") {
		t.Errorf("expected validation error or success, got: %v", err)
	}
}

func TestDecodeAgentConfig_UnknownType(t *testing.T) {
	node := yamlNode(t, `type: totally-unknown-agent-xyz`)
	_, _, err := DecodeAgentConfig("a1", node)
	if err == nil || !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("expected unknown type error, got: %v", err)
	}
}

// --- DecodeToolConfig tests ---

func TestDecodeToolConfig_DefaultType(t *testing.T) {
	// "builtin" is registered in init()
	node := yamlNode(t, `description: test`)
	typeName, _, err := DecodeToolConfig("t1", node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typeName != "builtin" {
		t.Errorf("expected default type 'builtin', got %q", typeName)
	}
}

func TestDecodeToolConfig_UnknownType(t *testing.T) {
	node := yamlNode(t, `type: totally-unknown-tool-xyz`)
	_, _, err := DecodeToolConfig("t1", node)
	if err == nil || !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("expected unknown type error, got: %v", err)
	}
}

// --- Tool handler tests ---

func TestToolHandler_RegisterAndGet(t *testing.T) {
	handlerName := uniqueName(t, "handler")
	handler := func(ctx context.Context, args map[string]any) (any, error) {
		return "result", nil
	}
	RegisterToolHandler(handlerName, handler)

	got, ok := GetToolHandler(handlerName)
	if !ok {
		t.Fatal("expected handler to be found")
	}
	res, err := got(context.Background(), nil)
	if err != nil || res != "result" {
		t.Errorf("unexpected result: %v, %v", res, err)
	}
}

func TestToolHandler_Missing(t *testing.T) {
	_, ok := GetToolHandler("nonexistent-handler-xyz")
	if ok {
		t.Error("expected handler to not be found")
	}
}

// --- geminiToolCreator tests ---

func TestGeminiToolCreator_UnknownBuiltin(t *testing.T) {
	cfg := &GeminiToolConfig{Tool: "nonexistent-gemini-tool"}
	_, err := geminiToolCreator(context.Background(), "t", cfg, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown gemini built-in tool") {
		t.Errorf("expected unknown gemini tool error, got: %v", err)
	}
}

func TestGeminiToolCreator_GoogleSearch(t *testing.T) {
	cfg := &GeminiToolConfig{Tool: "google_search"}
	tool, err := geminiToolCreator(context.Background(), "gs", cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool == nil {
		t.Error("expected non-nil tool")
	}
}

func TestGeminiToolCreator_NameFallback(t *testing.T) {
	cfg := &GeminiToolConfig{Tool: ""}
	_, err := geminiToolCreator(context.Background(), "google_search", cfg, nil)
	if err != nil {
		t.Fatalf("should fall back to name 'google_search': %v", err)
	}
}

// --- strictDecode tests ---

func TestStrictDecode_UnknownField(t *testing.T) {
	type simple struct {
		Name string `yaml:"name"`
	}
	node := yamlNode(t, "name: test\nextra_field: bad")
	var s simple
	err := strictDecode(node, &s)
	if err == nil {
		t.Fatal("expected error for unknown field with strict decode")
	}
}

func TestStrictDecode_ValidField(t *testing.T) {
	type simple struct {
		Name string `yaml:"name"`
	}
	node := yamlNode(t, "name: hello")
	var s simple
	err := strictDecode(node, &s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "hello" {
		t.Errorf("expected 'hello', got %q", s.Name)
	}
}

// --- decodeCfg with Validatable ---

type validatableConfig struct {
	Name string `yaml:"name"`
}

func (v *validatableConfig) Validate() error {
	if v.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

func TestDecodeCfg_ValidationFails(t *testing.T) {
	node := yamlNode(t, `name: ""`)
	_, err := decodeCfg[validatableConfig](node)
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestDecodeCfg_ValidationPasses(t *testing.T) {
	node := yamlNode(t, `name: hello`)
	cfg, err := decodeCfg[validatableConfig](node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vc := cfg.(*validatableConfig)
	if vc.Name != "hello" {
		t.Errorf("expected 'hello', got %q", vc.Name)
	}
}

// --- ModelBase / OllamaConfig validation ---

func TestModelBase_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ModelBase
		wantErr string
	}{
		{"missing provider", ModelBase{ModelID: "x"}, "provider is required"},
		{"missing model_id", ModelBase{Provider: "x"}, "model_id is required"},
		{"valid", ModelBase{Provider: "x", ModelID: "y"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestOllamaConfig_Validate(t *testing.T) {
	cfg := &OllamaConfig{
		ModelBase: ModelBase{Provider: "ollama", ModelID: "llama3"},
		BaseURL:   "",
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "base_url is required") {
		t.Errorf("expected base_url error, got: %v", err)
	}
}

func TestLLMAgentConfig_Validate(t *testing.T) {
	cfg := &LLMAgentConfig{}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "model is required") {
		t.Errorf("expected model required error, got: %v", err)
	}
}

// --- ParseRaw tests ---

func TestParseRaw_SessionParsing(t *testing.T) {
	sessionNode := yamlNode(t, `provider: inmemory`)
	raw := &RawConfig{
		Models:  map[string]*yaml.Node{},
		Agents:  map[string]*yaml.Node{},
		Tools:   map[string]*yaml.Node{},
		Session: sessionNode,
	}
	cfg, err := ParseRaw(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Session == nil {
		t.Fatal("expected session config")
	}
	if cfg.Session.Provider != "inmemory" {
		t.Errorf("expected provider 'inmemory', got %q", cfg.Session.Provider)
	}
}

func TestParseRaw_MemoryParsing(t *testing.T) {
	memNode := yamlNode(t, `provider: inmemory`)
	raw := &RawConfig{
		Models: map[string]*yaml.Node{},
		Agents: map[string]*yaml.Node{},
		Tools:  map[string]*yaml.Node{},
		Memory: memNode,
	}
	cfg, err := ParseRaw(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Memory == nil {
		t.Fatal("expected memory config")
	}
}

func TestParseRaw_AuthParsing(t *testing.T) {
	authNode := yamlNode(t, "jwt:\n  issuer: test-issuer\n  audience: test-aud")
	raw := &RawConfig{
		Models: map[string]*yaml.Node{},
		Agents: map[string]*yaml.Node{},
		Tools:  map[string]*yaml.Node{},
		Auth:   authNode,
	}
	cfg, err := ParseRaw(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Auth == nil || cfg.Auth.JWT == nil {
		t.Fatal("expected auth.jwt config")
	}
	if cfg.Auth.JWT.Issuer != "test-issuer" {
		t.Errorf("expected issuer 'test-issuer', got %q", cfg.Auth.JWT.Issuer)
	}
}

func TestParseRaw_RootAgent(t *testing.T) {
	raw := &RawConfig{
		Models:    map[string]*yaml.Node{},
		Agents:    map[string]*yaml.Node{},
		Tools:     map[string]*yaml.Node{},
		RootAgent: "CustomRoot",
	}
	cfg, err := ParseRaw(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RootAgent != "CustomRoot" {
		t.Errorf("expected 'CustomRoot', got %q", cfg.RootAgent)
	}
}

func TestParseRaw_ToolDecoding(t *testing.T) {
	// "builtin" tool type is registered in init()
	toolNode := yamlNode(t, `description: a test tool`)
	raw := &RawConfig{
		Models: map[string]*yaml.Node{},
		Agents: map[string]*yaml.Node{},
		Tools:  map[string]*yaml.Node{"mytool": toolNode},
	}
	cfg, err := ParseRaw(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cfg.Tools["mytool"]; !ok {
		t.Error("expected tool 'mytool'")
	}
}

// --- Provider tests ---

func TestCreateProvider_NotFound(t *testing.T) {
	type dummyCfg struct{}
	type dummySvc interface{}
	_, err := CreateProvider[dummyCfg, dummySvc](context.Background(), "session", "nonexistent-provider-xyz", &dummyCfg{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got: %v", err)
	}
}

func TestCreateProvider_InvalidCreatorType(t *testing.T) {
	key := "testservice:badtype-" + t.Name()
	compreg.Set(key, "not a function") // store wrong type

	type dummyCfg struct{}
	type dummySvc interface{}
	_, err := CreateProvider[dummyCfg, dummySvc](context.Background(), "testservice", "badtype-"+t.Name(), &dummyCfg{})
	if err == nil || !strings.Contains(err.Error(), "internal error") {
		t.Errorf("expected internal error, got: %v", err)
	}
}

func TestRegisterAndCreateProvider_Happy(t *testing.T) {
	type testCfg struct {
		Value string
	}
	type testSvc struct {
		Created bool
	}

	provName := uniqueName(t, "prov")
	RegisterProvider("testsvc", provName, func(ctx context.Context, cfg *testCfg) (*testSvc, error) {
		return &testSvc{Created: true}, nil
	})

	svc, err := CreateProvider[testCfg, *testSvc](context.Background(), "testsvc", provName, &testCfg{Value: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.Created {
		t.Error("expected Created to be true")
	}
}

// --- Adapter interface compliance tests ---

func TestModelAdapter(t *testing.T) {
	provName := uniqueName(t, "provider")
	cfg := &Config{
		Models: map[string]ModelEntry{
			"m": {Name: "m", Provider: provName, Config: &mockModelConfig{}},
		},
	}
	reg := New(cfg)
	RegisterModelProvider(provName, func(ctx context.Context, cfg *mockModelConfig) (model.LLM, error) {
		return &mockLLM{}, nil
	})

	adapter := &modelAdapter{r: reg}
	m, err := adapter.Get(context.Background(), "m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name() != "mock" {
		t.Errorf("expected 'mock', got %q", m.Name())
	}
}

func TestToolAdapter(t *testing.T) {
	typeName := uniqueName(t, "tool-type")
	cfg := &Config{
		Tools: map[string]ToolEntry{
			"t1": {Name: "t1", Type: typeName, Config: &testToolConfig{}},
		},
	}
	reg := New(cfg)
	RegisterToolType(typeName, func(ctx context.Context, name string, cfg *testToolConfig, _ SandboxRegistry) (tool.Tool, error) {
		return &mockTool{name: name}, nil
	})

	adapter := &toolAdapter{r: reg}
	tools, err := adapter.GetMultiple(context.Background(), []string{"t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 || tools[0].Name() != "t1" {
		t.Errorf("unexpected tools: %v", tools)
	}
}

// --- Registry.Config() test ---

func TestRegistryConfig(t *testing.T) {
	cfg := &Config{RootAgent: "test"}
	reg := New(cfg)
	if reg.Config() != cfg {
		t.Error("Config() should return the config passed to New()")
	}
}
