package prolog

import (
	"context"
	"fmt"
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
			fmt.Fprintln(v.host.Logger, fmt.Sprint(env.Resolve(term)))
		}
		return cont(env)
	})

	return nil
}

func (v *PrologVM) injectTools(ctx context.Context) error {
	if v.host == nil || v.host.Tools == nil || len(v.cfg.AllowTools) == 0 {
		return nil
	}

	for _, toolName := range v.cfg.AllowTools {
		name := toolName
		// Register tool as a predicate: tool_name(Args, Result)
		v.interpreter.Register2(engine.NewAtom(name), func(vm *engine.VM, args engine.Term, result engine.Term, cont engine.Cont, env *engine.Env) *engine.Promise {
			// Convert Prolog args to Go map
			goArgs := v.fromPrologTerm(args, env)
			argsMap, ok := goArgs.(map[string]any)
			if !ok {
				argsMap = make(map[string]any)
			}

			res, err := v.host.Tools.CallTool(ctx, name, argsMap)
			if err != nil {
				return engine.Error(fmt.Errorf("%s failed: %w", name, err))
			}

			// Convert Go result back to Prolog term and unify with 'result'
			resTerm := v.toPrologTerm(res)
			return engine.Unify(vm, result, resTerm, cont, env)
		})
	}

	return nil
}

func (v *PrologVM) Run(ctx context.Context, code string) (any, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.interpreter == nil {
		return nil, fmt.Errorf("Prolog VM not initialized")
	}

	if err := v.injectTools(ctx); err != nil {
		return nil, err
	}

	sols, err := v.interpreter.QueryContext(ctx, code)
	if err != nil {
		if err := v.interpreter.ExecContext(ctx, code); err != nil {
			return nil, err
		}
		return "consulted", nil
	}
	defer sols.Close()

	if sols.Next() {
		// Just return success for now
		return "solution found", nil
	}

	return "no solution", nil
}

func (v *PrologVM) fromPrologTerm(term engine.Term, env *engine.Env) any {
	switch t := env.Resolve(term).(type) {
	case engine.Atom:
		return t.String()
	case engine.Integer:
		return int64(t)
	case engine.Float:
		return float64(t)
	case engine.Compound:
		// Basic conversion for lists and simple structures
		if t.Functor() == engine.NewAtom(".") && t.Arity() == 2 {
			// It's a list
			var res []any
			curr := t
			for {
				res = append(res, v.fromPrologTerm(curr.Arg(0), env))
				next := env.Resolve(curr.Arg(1))
				if nextAtom, ok := next.(engine.Atom); ok && nextAtom == engine.NewAtom("[]") {
					break
				}
				if nextCompound, ok := next.(engine.Compound); ok && nextCompound.Functor() == engine.NewAtom(".") && nextCompound.Arity() == 2 {
					curr = nextCompound
				} else {
					break
				}
			}
			return res
		}
		return fmt.Sprint(t)
	default:
		return fmt.Sprint(t)
	}
}

func (v *PrologVM) toPrologTerm(val any) engine.Term {
	switch val := val.(type) {
	case string:
		return engine.NewAtom(val)
	case int:
		return engine.Integer(val)
	case int64:
		return engine.Integer(val)
	case float64:
		return engine.Float(val)
	case bool:
		if val {
			return engine.NewAtom("true")
		}
		return engine.NewAtom("false")
	case nil:
		return engine.NewAtom("nil")
	default:
		return engine.NewAtom(fmt.Sprint(val))
	}
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
