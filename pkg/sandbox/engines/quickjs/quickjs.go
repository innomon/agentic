package quickjs

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/innomon/agentic/pkg/sandbox"
	"modernc.org/quickjs"
)

type QuickJSVM struct {
	mu      sync.Mutex
	runtime *quickjs.Runtime
	ctx     *quickjs.Context
	cfg     sandbox.VMConfig
	host    *sandbox.HostContext
}

func NewQuickJSVM() sandbox.SandboxVM {
	return &QuickJSVM{}
}

func (v *QuickJSVM) Init(cfg sandbox.VMConfig, host *sandbox.HostContext) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.cfg = cfg
	v.host = host

	rt := quickjs.NewRuntime()
	// rt.SetMemoryLimit(uint64(cfg.MemoryLimitMB) * 1024 * 1024) // If supported by modernc wrapper
	v.runtime = &rt

	qctx := rt.NewContext()
	v.ctx = &qctx

	// Register basic log function
	v.registerBuiltins()

	return nil
}

func (v *QuickJSVM) registerBuiltins() {
	globals := v.ctx.Globals()
	defer globals.Free()

	// log(...args)
	logFn := v.ctx.Function(func(qctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		if v.host != nil && v.host.Logger != nil {
			for i, arg := range args {
				if i > 0 {
					io.WriteString(v.host.Logger, " ")
				}
				io.WriteString(v.host.Logger, arg.String())
			}
			io.WriteString(v.host.Logger, "\n")
		}
		return qctx.Undefined()
	})
	defer logFn.Free()
	globals.Set("log", logFn)
	globals.Set("console", v.ctx.Object())
	console := globals.Get("console")
	defer console.Free()
	console.Set("log", logFn)
}

func (v *QuickJSVM) Run(ctx context.Context, code string) (any, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.ctx == nil {
		return nil, fmt.Errorf("QuickJS VM not initialized")
	}

	val, err := v.ctx.Eval(code)
	if err != nil {
		return nil, err
	}
	defer val.Free()

	if val.IsError() {
		return nil, fmt.Errorf("QuickJS execution error: %v", val.String())
	}

	return v.convertValue(val), nil
}

func (v *QuickJSVM) convertValue(val quickjs.Value) any {
	if val.IsString() {
		return val.String()
	}
	if val.IsNumber() {
		// Try int first, then float
		// QuickJS wrapper might not have direct type checks for float vs int easily
		// For now, return string representation or use general approach
		return val.String()
	}
	if val.IsBool() {
		return val.Bool()
	}
	if val.IsObject() {
		// Simplified conversion to string or placeholder
		return val.String()
	}
	if val.IsNull() || val.IsUndefined() {
		return nil
	}
	return val.String()
}

func (v *QuickJSVM) Reset() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.ctx != nil {
		v.ctx.Free()
	}
	if v.runtime != nil {
		v.runtime.Free()
	}

	rt := quickjs.NewRuntime()
	v.runtime = &rt
	qctx := rt.NewContext()
	v.ctx = &qctx
	v.registerBuiltins()

	return nil
}

func (v *QuickJSVM) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.ctx != nil {
		v.ctx.Free()
		v.ctx = nil
	}
	if v.runtime != nil {
		v.runtime.Free()
		v.runtime = nil
	}
	return nil
}

func init() {
	sandbox.RegisterVMEngine("quickjs", NewQuickJSVM)
}
