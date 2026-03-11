package prolog

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/innomon/agentic/pkg/sandbox"
	"github.com/ichiban/prolog"
	"github.com/ichiban/prolog/engine"
)

type PrologVM struct {
	mu          sync.Mutex
	interpreter *prolog.Interpreter
	cfg         sandbox.VMConfig
	host        *sandbox.HostContext
}

func NewPrologVM() sandbox.SandboxVM {
	return &PrologVM{}
}

func (v *PrologVM) Init(cfg sandbox.VMConfig, host *sandbox.HostContext) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.cfg = cfg
	v.host = host

	v.interpreter = prolog.New(nil, nil)

	// Register log predicate: log(Msg)
	v.interpreter.Register1(engine.NewAtom("log"), func(vm *engine.VM, term engine.Term, cont engine.Cont, env *engine.Env) *engine.Promise {
		if v.host != nil && v.host.Logger != nil {
			fmt.Fprintln(v.host.Logger, vm.Brief(term, env))
		}
		return cont(env)
	})

	return nil
}

func (v *PrologVM) Run(ctx context.Context, code string) (any, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.interpreter == nil {
		return nil, fmt.Errorf("Prolog VM not initialized")
	}

	// For Prolog, we might want to consult the code or query it directly.
	// If it's a query, we return the first solution's bindings.
	
	// Check if it's a fact/rule (consult) or a query
	// Simple heuristic: if it ends with a dot but doesn't look like a query, consult it.
	// For now, let's assume we can execute it.
	
	// Try to execute as a query
	sols, err := v.interpreter.QueryContext(ctx, code)
	if err != nil {
		// If query fails, maybe it's code to be consulted
		if err := v.interpreter.ExecContext(ctx, code); err != nil {
			return nil, err
		}
		return "consulted", nil
	}
	defer sols.Close()

	if sols.Next() {
		var solution any
		// For now, return a placeholder or simple representation of bindings
		// We'd need to iterate and convert engine.Term to Go types
		return "solution found", nil
	}

	return "no solution", nil
}

func (v *PrologVM) Reset() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.interpreter = prolog.New(nil, nil)
	return nil
}

func (v *PrologVM) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.interpreter = nil
	return nil
}

func init() {
	sandbox.RegisterVMEngine("prolog", NewPrologVM)
}
