package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"
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
	configs   map[string]VMConfig
	hostCtx   *HostContext
	logBuffer *bytes.Buffer
}

// NewManager creates a new SandboxManager.
func NewManager(host *HostContext) *SandboxManager {
	_ = time.Second // Force usage of time package
	var buf bytes.Buffer
	// Wrap the original logger to also capture in our buffer
	originalLogger := host.Logger
	host.Logger = io.MultiWriter(originalLogger, &buf)

	return &SandboxManager{
		sandboxes: make(map[string]SandboxVM),
		configs:   make(map[string]VMConfig),
		hostCtx:   host,
		logBuffer: &buf,
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
	m.configs[name] = cfg
	return vm, nil
}

// Run executes code in the named sandbox.
func (m *SandboxManager) Run(ctx context.Context, name string, code string) (*SandboxResult, error) {
	m.mu.Lock()
	vm, ok := m.sandboxes[name]
	cfg := m.configs[name]
	m.logBuffer.Reset()
	m.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("sandbox %q not found", name)
	}

	runCtx := ctx
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	val, err := vm.Run(runCtx, code)
	
	m.mu.Lock()
	logs := m.logBuffer.String()
	m.mu.Unlock()

	if err != nil {
		return &SandboxResult{Logs: logs}, err
	}

	return &SandboxResult{
		Value: val,
		Logs:  logs,
	}, nil
}

// CloseAll closes all managed sandboxes.
func (m *SandboxManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, vm := range m.sandboxes {
		_ = vm.Close()
		delete(m.sandboxes, name)
		delete(m.configs, name)
	}
}
