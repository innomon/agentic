package quickjs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/innomon/agentic/pkg/sandbox"
	"modernc.org/quickjs"
)

type QuickJSVM struct {
	mu sync.Mutex
	vm *quickjs.VM
}

func NewQuickJSVM() sandbox.SandboxVM {
	return &QuickJSVM{}
}

func (v *QuickJSVM) Init(cfg sandbox.VMConfig, host *sandbox.HostContext) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	vm, err := quickjs.NewVM()
	if err != nil {
		return err
	}
	v.vm = vm

	if cfg.MemoryLimitMB > 0 {
		v.vm.SetMemoryLimit(uintptr(cfg.MemoryLimitMB * 1024 * 1024))
	}
	if cfg.Timeout > 0 {
		v.vm.SetEvalTimeout(cfg.Timeout)
	}

	// Register log
	err = v.vm.RegisterFunc("log", func(args ...any) any {
		if host != nil && host.Logger != nil {
			for i, arg := range args {
				if i > 0 {
					io.WriteString(host.Logger, " ")
				}
				fmt.Fprint(host.Logger, arg)
			}
			io.WriteString(host.Logger, "\n")
		}
		return nil
	}, false)
	if err != nil {
		return err
	}

	// Inject tools
	for _, toolName := range cfg.AllowTools {
		name := toolName
		err = v.vm.RegisterFunc(name, func(args ...any) any {
			var toolArgs map[string]any
			if len(args) > 0 {
				if m, ok := args[0].(map[string]any); ok {
					toolArgs = m
				}
			}
			if toolArgs == nil {
				toolArgs = make(map[string]any)
			}

			res, err := host.Tools.CallTool(context.Background(), name, toolArgs)
			if err != nil {
				return map[string]any{"error": err.Error()}
			}
			
			val, err := v.toValue(res)
			if err != nil {
				return map[string]any{"error": fmt.Sprintf("failed to convert result: %v", err)}
			}
			return val
		}, false)
		if err != nil {
			return err
		}
	}

	return nil
}

func (v *QuickJSVM) toValue(val any) (any, error) {
	if val == nil {
		return nil, nil
	}

	switch t := val.(type) {
	case map[string]any:
		obj, err := v.vm.NewObjectValue()
		if err != nil {
			return nil, err
		}
		for k, v2 := range t {
			atom, err := v.vm.NewAtom(k)
			if err != nil {
				return nil, err
			}
			converted, err := v.toValue(v2)
			if err != nil {
				return nil, err
			}
			if err := v.vm.SetProperty(obj, atom, converted); err != nil {
				return nil, err
			}
		}
		return obj, nil
	case []any:
		obj, err := v.vm.NewObjectValue() 
		if err != nil {
			return nil, err
		}
		for i, v2 := range t {
			atom, err := v.vm.NewAtom(fmt.Sprint(i))
			if err != nil {
				return nil, err
			}
			converted, err := v.toValue(v2)
			if err != nil {
				return nil, err
			}
			if err := v.vm.SetProperty(obj, atom, converted); err != nil {
				return nil, err
			}
		}
		lengthAtom, _ := v.vm.NewAtom("length")
		v.vm.SetProperty(obj, lengthAtom, len(t))
		return obj, nil
	default:
		return val, nil
	}
}

func (v *QuickJSVM) Run(ctx context.Context, code string) (any, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.vm == nil {
		return nil, fmt.Errorf("QuickJS VM not initialized")
	}

	val, err := v.vm.EvalValue(code, 0)
	if err != nil {
		return nil, err
	}
	defer val.Free()

	// Convert back to Go type
	return v.fromValue(val)
}

func (v *QuickJSVM) fromValue(val quickjs.Value) (any, error) {
	if val.IsUndefined() {
		return nil, nil
	}

	// Try MarshalJSON for complex objects
	data, err := val.MarshalJSON()
	if err == nil && len(data) > 0 && string(data) != "undefined" {
		var res any
		if err := json.Unmarshal(data, &res); err == nil {
			return res, nil
		}
	}

	return val.Any()
}

func (v *QuickJSVM) Reset() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.vm != nil {
		v.vm.Close()
	}
	return nil
}

func (v *QuickJSVM) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.vm != nil {
		err := v.vm.Close()
		v.vm = nil
		return err
	}
	return nil
}

func init() {
	sandbox.RegisterVMEngine("quickjs", NewQuickJSVM)
}
