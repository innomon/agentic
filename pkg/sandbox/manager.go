package sandbox

import (
	"context"
	"fmt"
	"sync"
)

// VMFactory is a function that creates a new SandboxVM.
type VMFactory func() SandboxVM

var (
	factories   = make(map[string]VMFactory)
	factoriesMu sync.RWMutex
)

// RegisterVMEngine registers a factory function for a VM engine type.
func RegisterVMEngine(typeName string, factory VMFactory) {
	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	factories[typeName] = factory
}

// SandboxManager manages multiple sandboxes and their lifecycles.
type SandboxManager struct {
	mu        sync.Mutex
	sandboxes map[string]SandboxVM
	hostCtx   *HostContext
}

// NewManager creates a new SandboxManager.
func NewManager(host *HostContext) *SandboxManager {
	return &SandboxManager{
		sandboxes: make(map[string]SandboxVM),
		hostCtx:   host,
	}
}

// GetOrCreateVM retrieves an existing VM or creates a new one of the specified type.
func (m *SandboxManager) GetOrCreateVM(name string, cfg VMConfig) (SandboxVM, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if vm, ok := m.sandboxes[name]; ok {
		return vm, nil
	}

	factoriesMu.RLock()
	factory, ok := factories[cfg.Type]
	factoriesMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unsupported sandbox type: %q", cfg.Type)
	}

	vm := factory()
	if err := vm.Init(cfg, m.hostCtx); err != nil {
		return nil, fmt.Errorf("failed to initialize sandbox %q: %w", name, err)
	}

	m.sandboxes[name] = vm
	return vm, nil
}

// CloseAll closes all managed sandboxes.
func (m *SandboxManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, vm := range m.sandboxes {
		_ = vm.Close()
		delete(m.sandboxes, name)
	}
}
