package registry

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"

	"github.com/innomon/agentic/pkg/sandbox"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/toolconfirmation"
	"google.golang.org/genai"
)

var (
	_ ModelRegistry           = (*modelAdapter)(nil)
	_ ToolRegistry            = (*toolAdapter)(nil)
	_ agent.InvocationContext = (*Registry)(nil)
)

type loader func(ctx context.Context, r *Registry, name string) (any, error)

type Registry struct {
	cfg       *Config
	mu        sync.RWMutex
	items     map[string]any
	loaders   map[reflect.Type]loader
	building  map[string]bool
	closers   []io.Closer
	sandboxes *sandbox.SandboxManager
	ctx       context.Context
}

func New(cfg *Config) *Registry {
	r := &Registry{
		cfg:      cfg,
		items:    make(map[string]any),
		building: make(map[string]bool),
		ctx:      context.Background(),
	}
	r.loaders = map[reflect.Type]loader{
		typeOf[model.LLM]():   loadModel,
		typeOf[tool.Tool]():   loadTool,
		typeOf[agent.Agent](): loadAgent,
	}
	r.sandboxes = sandbox.NewManager(&sandbox.HostContext{
		Tools:             &toolAdapter{r},
		InvocationContext: r,
		Logger:            io.Discard,
	})
	r.closers = append(r.closers, closerFn(func() error {
		r.sandboxes.CloseAll()
		return nil
	}))
	return r
}

type closerFn func() error

func (f closerFn) Close() error { return f() }

func (r *Registry) Close() error {
	var errs []error
	for _, c := range r.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors while closing registry: %v", errs)
	}
	return nil
}

// InvocationContext implementation
func (r *Registry) Deadline() (time.Time, bool) { return r.ctx.Deadline() }
func (r *Registry) Done() <-chan struct{}       { return r.ctx.Done() }
func (r *Registry) Err() error                  { return r.ctx.Err() }
func (r *Registry) Value(key any) any           { return r.ctx.Value(key) }

func (r *Registry) Agent() agent.Agent          { return nil }
func (r *Registry) Artifacts() agent.Artifacts  { return nil }
func (r *Registry) Memory() agent.Memory        { return nil }
func (r *Registry) Session() session.Session    { return nil }
func (r *Registry) InvocationID() string        { return "sandbox-root" }
func (r *Registry) Branch() string              { return "root" }
func (r *Registry) UserContent() *genai.Content { return nil }
func (r *Registry) RunConfig() *agent.RunConfig { return nil }
func (r *Registry) EndInvocation()              {}
func (r *Registry) Ended() bool                 { return false }
func (r *Registry) WithContext(ctx context.Context) agent.InvocationContext {
	return &Registry{
		cfg:       r.cfg,
		items:     r.items,
		loaders:   r.loaders,
		building:  r.building,
		closers:   r.closers,
		sandboxes: r.sandboxes,
		ctx:       ctx,
	}
}

func typeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func itemKey[T any](name string) string {
	return typeOf[T]().String() + ":" + name
}

func Get[T any](ctx context.Context, r *Registry, name string) (T, error) {
	k := itemKey[T](name)

	r.mu.RLock()
	if v, ok := r.items[k]; ok {
		r.mu.RUnlock()
		return v.(T), nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	return getOrLoadLocked[T](ctx, r, name)
}

func getOrLoadLocked[T any](ctx context.Context, r *Registry, name string) (T, error) {
	k := itemKey[T](name)
	if v, ok := r.items[k]; ok {
		return v.(T), nil
	}

	ldr, ok := r.loaders[typeOf[T]()]
	if !ok {
		var zero T
		return zero, fmt.Errorf("no loader registered for type %s", typeOf[T]())
	}

	v, err := ldr(ctx, r, name)
	if err != nil {
		var zero T
		return zero, err
	}

	r.items[k] = v
	return v.(T), nil
}

func (r *Registry) Config() *Config {
	return r.cfg
}

func (r *Registry) GetDefaultModel(ctx context.Context) (model.LLM, error) {
	name, _, err := r.cfg.GetDefaultModel()
	if err != nil {
		return nil, err
	}
	return Get[model.LLM](ctx, r, name)
}

func (r *Registry) GetTools(ctx context.Context, names []string) ([]tool.Tool, error) {
	if len(names) == 0 {
		return nil, nil
	}
	var tools []tool.Tool
	for _, name := range names {
		t, err := Get[tool.Tool](ctx, r, name)
		if err != nil {
			return nil, err
		}
		if t != nil {
			tools = append(tools, t)
		}
	}
	return tools, nil
}

func (r *Registry) GetRoot(ctx context.Context) (agent.Agent, error) {
	name := r.cfg.RootAgent
	if name == "" {
		name = "RootAgent"
	}
	return Get[agent.Agent](ctx, r, name)
}

func loadModel(ctx context.Context, r *Registry, name string) (any, error) {
	entry, err := r.cfg.GetModel(name)
	if err != nil {
		return nil, err
	}
	m, err := createModel(ctx, entry.Provider, entry.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create model %q: %w", name, err)
	}
	return m, nil
}

func loadTool(ctx context.Context, r *Registry, name string) (any, error) {
	entry, err := r.cfg.GetTool(name)
	if err != nil {
		return nil, err
	}
	return createTool(ctx, entry.Type, name, entry.Config, &sandboxAdapter{r})
}

type SandboxRegistry interface {
	Run(ctx context.Context, name string, code string) (*sandbox.SandboxResult, error)
	GetOrCreateVM(name string, cfg sandbox.VMConfig) (sandbox.SandboxVM, error)
	CallTool(ctx context.Context, name string, args map[string]any) (map[string]any, error)
}

type sandboxAdapter struct{ r *Registry }

func (a *sandboxAdapter) Run(ctx context.Context, name string, code string) (*sandbox.SandboxResult, error) {
	return a.r.sandboxes.Run(ctx, name, code)
}

func (a *sandboxAdapter) GetOrCreateVM(name string, cfg sandbox.VMConfig) (sandbox.SandboxVM, error) {
	return a.r.sandboxes.GetOrCreateVM(name, cfg)
}

func (a *sandboxAdapter) CallTool(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	t, err := Get[tool.Tool](ctx, a.r, name)
	if err != nil {
		return nil, err
	}
	ft, ok := t.(functionTool)
	if !ok {
		return nil, fmt.Errorf("tool %q is not callable", name)
	}
	tCtx := dummyToolContext{Context: a.r.ctx}
	return ft.Run(tCtx, args)
}

func loadAgent(ctx context.Context, r *Registry, name string) (any, error) {
	if r.building[name] {
		return nil, fmt.Errorf("circular dependency detected for agent %q", name)
	}
	r.building[name] = true
	defer delete(r.building, name)

	entry, err := r.cfg.GetAgent(name)
	if err != nil {
		return nil, err
	}

	var subAgents []agent.Agent
	for _, subName := range entry.SubAgents {
		sub, err := getOrLoadLocked[agent.Agent](ctx, r, subName)
		if err != nil {
			return nil, fmt.Errorf("failed to build sub-agent %q for %q: %w", subName, name, err)
		}
		subAgents = append(subAgents, sub)
	}

	a, err := createAgent(ctx, entry.Type, name, entry.Config, &modelAdapter{r}, &toolAdapter{r}, subAgents)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent %q: %w", name, err)
	}
	return a, nil
}

type modelAdapter struct{ r *Registry }

func (a *modelAdapter) Get(ctx context.Context, name string) (model.LLM, error) {
	return Get[model.LLM](ctx, a.r, name)
}

type toolAdapter struct{ r *Registry }

func (a *toolAdapter) GetMultiple(ctx context.Context, names []string) ([]tool.Tool, error) {
	return a.r.GetTools(ctx, names)
}

func (a *toolAdapter) GetTools(ctx context.Context, names []string) ([]tool.Tool, error) {
	return a.r.GetTools(ctx, names)
}

func (a *toolAdapter) CallTool(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	t, err := Get[tool.Tool](ctx, a.r, name)
	if err != nil {
		return nil, err
	}
	ft, ok := t.(functionTool)
	if !ok {
		return nil, fmt.Errorf("tool %q is not callable", name)
	}
	tCtx := dummyToolContext{Context: a.r.ctx}
	return ft.Run(tCtx, args)
}

// functionTool defines the interface we expect from ADK tools to execute them.
// This is a copy of google.golang.org/adk/internal/toolinternal.FunctionTool
// to avoid importing an internal package.
type functionTool interface {
	tool.Tool
	Declaration() *genai.FunctionDeclaration
	Run(ctx tool.Context, args any) (result map[string]any, err error)
}

// dummyToolContext provides a minimal tool.Context for tools called from sandboxes/registry.
type dummyToolContext struct {
	context.Context
}

func (c dummyToolContext) UserContent() *genai.Content        { return nil }
func (c dummyToolContext) InvocationID() string               { return "sandbox-root" }
func (c dummyToolContext) AgentName() string                  { return "sandbox-agent" }
func (c dummyToolContext) ReadonlyState() session.ReadonlyState { return nil }
func (c dummyToolContext) UserID() string                     { return "sandbox-user" }
func (c dummyToolContext) AppName() string                    { return "sandbox-app" }
func (c dummyToolContext) SessionID() string                  { return "sandbox-session" }
func (c dummyToolContext) Branch() string                     { return "root" }
func (c dummyToolContext) Artifacts() agent.Artifacts         { return nil }
func (c dummyToolContext) State() session.State               { return nil }
func (c dummyToolContext) FunctionCallID() string             { return "sandbox-call" }
func (c dummyToolContext) Actions() *session.EventActions     { return nil }
func (c dummyToolContext) SearchMemory(ctx context.Context, query string) (*memory.SearchResponse, error) {
	return nil, fmt.Errorf("memory search not available in sandbox tool calls")
}

func (c dummyToolContext) RequestConfirmation(prompt string, metadata any) error {
	return fmt.Errorf("confirmation not supported in sandbox tool calls")
}

func (c dummyToolContext) ToolConfirmation() *toolconfirmation.ToolConfirmation {
	return nil
}

func (c dummyToolContext) WithContext(ctx context.Context) tool.Context {
	return dummyToolContext{Context: ctx}
}
