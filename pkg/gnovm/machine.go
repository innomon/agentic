package gnovm

import (
	"context"
	"fmt"
	"strings"

	"github.com/gnolang/gno/gnovm/pkg/gnolang"
	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/db"
	"github.com/gnolang/gno/tm2/pkg/db/memdb"
	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/gnolang/gno/tm2/pkg/store/dbadapter"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/toolconfirmation"
	"google.golang.org/genai"
)

// AgentContext is passed to GnoVM as Context for agents.
type AgentContext struct {
	InvCtx    agent.InvocationContext
	SubAgents []agent.Agent
	Tools     []tool.Tool
}

// FunctionTool defines the interface we expect from ADK tools to execute them.
type FunctionTool interface {
	tool.Tool
	Declaration() *genai.FunctionDeclaration
	Run(ctx tool.Context, args any) (result map[string]any, err error)
}

// DummyToolContext provides a minimal tool.Context for tools called from GnoVM.
type DummyToolContext struct {
	context.Context
}

func (c DummyToolContext) UserContent() *genai.Content               { return nil }
func (c DummyToolContext) InvocationID() string                      { return "gnovm-call" }
func (c DummyToolContext) AgentName() string                         { return "gnovm-agent" }
func (c DummyToolContext) ReadonlyState() session.ReadonlyState      { return nil }
func (c DummyToolContext) UserID() string                            { return "gnovm-user" }
func (c DummyToolContext) AppName() string                           { return "gnovm-app" }
func (c DummyToolContext) SessionID() string                         { return "gnovm-session" }
func (c DummyToolContext) Branch() string                            { return "root" }
func (c DummyToolContext) Artifacts() agent.Artifacts                { return nil }
func (c DummyToolContext) State() session.State                      { return nil }
func (c DummyToolContext) FunctionCallID() string                    { return "gnovm-call" }
func (c DummyToolContext) Actions() *session.EventActions            { return nil }
func (c DummyToolContext) SearchMemory(ctx context.Context, query string) (*memory.SearchResponse, error) {
	return nil, fmt.Errorf("memory search not available in gnovm tool calls")
}
func (c DummyToolContext) RequestConfirmation(hint string, payload any) error {
	return fmt.Errorf("confirmation not supported in gnovm tool calls")
}
func (c DummyToolContext) ToolConfirmation() *toolconfirmation.ToolConfirmation {
	return nil
}
func (c DummyToolContext) WithContext(ctx context.Context) tool.Context {
	return DummyToolContext{Context: ctx}
}

// NativePkg represents a native package to be injected into the GnoVM.
type NativePkg struct {
	Name  string
	Path  string
	Funcs map[string]func(m *gnolang.Machine)
}

var (
	sandboxNativePkgs []*NativePkg
	agentNativePkgs   []*NativePkg
)

// RegisterSandboxNativePkg registers a native package for all Gno sandboxes.
func RegisterSandboxNativePkg(pkg *NativePkg) {
	sandboxNativePkgs = append(sandboxNativePkgs, pkg)
}

// RegisterAgentNativePkg registers a native package for all Gno agents.
func RegisterAgentNativePkg(pkg *NativePkg) {
	agentNativePkgs = append(agentNativePkgs, pkg)
}

// MachineOptions contains parameters for Gno machine creation.
type MachineOptions struct {
	PkgPath    string
	Store      db.DB
	Source     map[string]string // File name -> content
	NativePkgs []*NativePkg
	Context    any
}

// MachineWrapper wraps a Gno machine and provides high-level operations.
type MachineWrapper struct {
	Machine *gnolang.Machine
	Store   gnolang.Store
	DB      db.DB
	PkgPath string
}

// NewMachineWrapper creates a new Gno machine wrapper.
func NewMachineWrapper(opts MachineOptions) (*MachineWrapper, error) {
	if opts.PkgPath == "" {
		opts.PkgPath = "agentic/r/agent"
	}

	baseDB := opts.Store
	if baseDB == nil {
		baseDB = memdb.NewMemDB()
	}

	baseStore := dbadapter.Store{DB: baseDB}

	alloc := gnolang.NewAllocator(0)
	store := gnolang.NewStore(alloc, baseStore, baseStore)

	// Inject native packages
	if len(opts.NativePkgs) > 0 {
		store.SetNativeResolver(func(pkgPath string, name gnolang.Name) func(m *gnolang.Machine) {
			for _, pkg := range opts.NativePkgs {
				if pkg.Path == pkgPath {
					if fn, ok := pkg.Funcs[string(name)]; ok {
						return fn
					}
				}
			}
			return nil
		})
	}

	m := gnolang.NewMachine(opts.PkgPath, store)
	m.Context = opts.Context

	if len(opts.Source) > 0 {
		pkgName := "main"
		if opts.PkgPath != "" {
			parts := strings.Split(opts.PkgPath, "/")
			pkgName = parts[len(parts)-1]
		}
		mpkg := &std.MemPackage{
			Name:  pkgName,
			Path:  opts.PkgPath,
			Type:  gnolang.MPUserProd,
			Files: make([]*std.MemFile, 0, len(opts.Source)),
		}
		for name, body := range opts.Source {
			mpkg.Files = append(mpkg.Files, &std.MemFile{
				Name: name,
				Body: body,
			})
		}
		_, pv := m.RunMemPackage(mpkg, true)
		m.SetActivePackage(pv)
	}

	return &MachineWrapper{
		Machine: m,
		Store:   m.Store,
		DB:      baseDB,
		PkgPath: opts.PkgPath,
	}, nil
}

type dbEntry struct {
	K []byte
	V []byte
}

// ExportState exports the VM state as a byte slice.
func (w *MachineWrapper) ExportState() ([]byte, error) {
	it, err := w.DB.Iterator(nil, nil)
	if err != nil {
		return nil, err
	}
	defer it.Close()

	var entries []dbEntry
	for ; it.Valid(); it.Next() {
		entries = append(entries, dbEntry{K: it.Key(), V: it.Value()})
	}
	return amino.Marshal(entries)
}

// RestoreState restores the VM state from a byte slice.
func (w *MachineWrapper) RestoreState(b []byte) error {
	var entries []dbEntry
	if err := amino.Unmarshal(b, &entries); err != nil {
		return err
	}

	for _, e := range entries {
		if err := w.DB.Set(e.K, e.V); err != nil {
			return err
		}
	}
	return nil
}

// Eval evaluates a Gno expression.
func (w *MachineWrapper) Eval(expr string) ([]gnolang.TypedValue, error) {
	parsed := w.Machine.MustParseExpr(expr)
	res := w.Machine.Eval(parsed)
	return res, nil
}

// Run executes a package main function.
func (w *MachineWrapper) Run() error {
	// Gno doesn't have a simple 'Run' for the whole machine,
	// but we can evaluate 'main()' if it exists.
	_, err := w.Eval("main()")
	return err
}
