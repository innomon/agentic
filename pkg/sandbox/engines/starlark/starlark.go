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
	mu      sync.Mutex
	globals starlark.StringDict
	thread  *starlark.Thread
	cfg     sandbox.VMConfig
	host    *sandbox.HostContext
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

func (v *StarlarkVM) injectTools(ctx context.Context) error {
	if v.host == nil || v.host.Tools == nil || len(v.cfg.AllowTools) == 0 {
		return nil
	}

	for _, toolName := range v.cfg.AllowTools {
		name := toolName
		v.globals[name] = starlark.NewBuiltin(name, func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var toolArgs map[string]any
			if len(args) > 0 {
				if dict, ok := args[0].(*starlark.Dict); ok {
					if converted := v.fromStarlarkValue(dict); converted != nil {
						if m, ok := converted.(map[string]any); ok {
							toolArgs = m
						}
					}
				}
			}
			if toolArgs == nil {
				toolArgs = make(map[string]any)
			}

			res, err := v.host.Tools.CallTool(ctx, name, toolArgs)
			if err != nil {
				return starlark.None, err
			}

			return v.toStarlarkValue(res), nil
		})
	}

	return nil
}

func (v *StarlarkVM) Run(ctx context.Context, code string) (any, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.thread == nil {
		return nil, fmt.Errorf("Starlark VM not initialized")
	}

	if err := v.injectTools(ctx); err != nil {
		return nil, err
	}

	dict, err := starlark.ExecFile(v.thread, "sandbox.star", code, v.globals)
	if err != nil {
		return nil, err
	}

	if res, ok := dict["result"]; ok {
		return v.fromStarlarkValue(res), nil
	}

	return nil, nil
}

func (v *StarlarkVM) fromStarlarkValue(val starlark.Value) any {
	switch val := val.(type) {
	case starlark.String:
		return string(val)
	case starlark.Int:
		i, _ := val.Int64()
		return i
	case starlark.Float:
		return float64(val)
	case starlark.Bool:
		return bool(val)
	case *starlark.List:
		res := make([]any, val.Len())
		for i := 0; i < val.Len(); i++ {
			res[i] = v.fromStarlarkValue(val.Index(i))
		}
		return res
	case *starlark.Dict:
		res := make(map[string]any)
		for _, item := range val.Items() {
			keyVal := item.Index(0)
			var key string
			if s, ok := keyVal.(starlark.String); ok {
				key = string(s)
			} else {
				key = keyVal.String()
			}
			res[key] = v.fromStarlarkValue(item.Index(1))
		}
		return res
	case starlark.NoneType:
		return nil
	default:
		return val.String()
	}
}

func (v *StarlarkVM) toStarlarkValue(val any) starlark.Value {
	switch val := val.(type) {
	case string:
		return starlark.String(val)
	case int:
		return starlark.MakeInt(val)
	case int64:
		return starlark.MakeInt64(val)
	case float64:
		return starlark.Float(val)
	case bool:
		return starlark.Bool(val)
	case []any:
		res := make([]starlark.Value, len(val))
		for i, item := range val {
			res[i] = v.toStarlarkValue(item)
		}
		return starlark.NewList(res)
	case map[string]any:
		res := &starlark.Dict{}
		for k, item := range val {
			res.SetKey(starlark.String(k), v.toStarlarkValue(item))
		}
		return res
	case nil:
		return starlark.None
	default:
		return starlark.String(fmt.Sprint(val))
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
