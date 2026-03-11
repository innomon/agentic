package starlark

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/innomon/agentic/pkg/sandbox"
	"go.starlark.net/starlark"
)

type StarlarkVM struct {
	mu       sync.Mutex
	globals  starlark.StringDict
	thread   *starlark.Thread
	cfg      sandbox.VMConfig
	host     *sandbox.HostContext
}

func NewStarlarkVM() sandbox.SandboxVM {
	return &StarlarkVM{}
}

func (v *StarlarkVM) Init(cfg sandbox.VMConfig, host *sandbox.HostContext) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.cfg = cfg
	v.host = host
	v.thread = &starlark.Thread{Name: "sandbox"}
	v.globals = make(starlark.StringDict)

	v.registerBuiltins()

	return nil
}

func (v *StarlarkVM) registerBuiltins() {
	// log(*args)
	v.globals["log"] = starlark.NewBuiltin("log", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if v.host != nil && v.host.Logger != nil {
			for i, arg := range args {
				if i > 0 {
					io.WriteString(v.host.Logger, " ")
				}
				io.WriteString(v.host.Logger, arg.String())
			}
			io.WriteString(v.host.Logger, "\n")
		}
		return starlark.None, nil
	})
}

func (v *StarlarkVM) Run(ctx context.Context, code string) (any, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.thread == nil {
		return nil, fmt.Errorf("Starlark VM not initialized")
	}

	// For Starlark, we usually execute as a script. 
	// To get a return value, we can assume the last expression or a specific variable 'result'.
	dict, err := starlark.ExecFile(v.thread, "sandbox.star", code, v.globals)
	if err != nil {
		return nil, err
	}

	// Update globals for persistence if needed, but for now we just return something
	// Let's look for 'result' variable or return the whole dict?
	// Consistent with other VMs, we might want a single result.
	if res, ok := dict["result"]; ok {
		return v.convertValue(res), nil
	}

	return nil, nil
}

func (v *StarlarkVM) convertValue(val starlark.Value) any {
	switch v := val.(type) {
	case starlark.String:
		return string(v)
	case starlark.Int:
		i, _ := v.Int64()
		return i
	case starlark.Float:
		return float64(v)
	case starlark.Bool:
		return bool(v)
	case *starlark.List:
		var res []any
		for i := 0; i < v.Len(); i++ {
			res = append(res, v.convertValue(v.Index(i)))
		}
		return res
	case *starlark.Dict:
		res := make(map[string]any)
		for _, item := range v.Items() {
			key := item.Index(0).String()
			res[key] = v.convertValue(item.Index(1))
		}
		return res
	case starlark.NoneType:
		return nil
	default:
		return val.String()
	}
}

func (v *StarlarkVM) Reset() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.thread = &starlark.Thread{Name: "sandbox"}
	v.globals = make(starlark.StringDict)
	v.registerBuiltins()
	return nil
}

func (v *StarlarkVM) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.thread = nil
	v.globals = nil
	return nil
}

func init() {
	sandbox.RegisterVMEngine("starlark", NewStarlarkVM)
}
