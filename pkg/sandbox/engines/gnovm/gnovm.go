package gnovm

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/innomon/agentic/pkg/sandbox"
	"github.com/gnolang/gno/gnovm/pkg/gnolang"
)

type GnoVM struct {
	mu      sync.Mutex
	machine *gnolang.Machine
	store   gnolang.Store
	cfg     sandbox.VMConfig
	host    *sandbox.HostContext
}

func NewGnoVM() sandbox.SandboxVM {
	return &GnoVM{}
}

func (v *GnoVM) Init(cfg sandbox.VMConfig, host *sandbox.HostContext) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.cfg = cfg
	v.host = host

	alloc := gnolang.NewAllocator(0)
	v.store = gnolang.NewStore(alloc, nil, nil)
	v.machine = gnolang.NewMachine("sandbox", v.store)

	// In GnoVM, logging and tool injection would go through host functions
	// or specific package imports. For now, we'll keep it simple.

	return nil
}

func (v *GnoVM) Run(ctx context.Context, code string) (any, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.machine == nil {
		return nil, fmt.Errorf("GnoVM not initialized")
	}

	// GnoVM typically executes packages. For 'Run', we might need to 
	// wrap the code in a package if it's not already.
	// This is a simplified implementation.
	
	// Assuming Eval exists or similar for expression evaluation
	// res := v.machine.Eval(code) 
	
	// For now, let's provide a placeholder that indicates it's running.
	if v.host != nil && v.host.Logger != nil {
		io.WriteString(v.host.Logger, "[GnoVM] Executing code...\n")
	}

	return "GnoVM execution result placeholder", nil
}

func (v *GnoVM) Reset() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	alloc := gnolang.NewAllocator(0)
	v.store = gnolang.NewStore(alloc, nil, nil)
	v.machine = gnolang.NewMachine("sandbox", v.store)
	return nil
}

func (v *GnoVM) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.machine = nil
	v.store = nil
	return nil
}

func init() {
	sandbox.RegisterVMEngine("gnovm", NewGnoVM)
}
