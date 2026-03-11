package sandbox

import (
	"context"
	"io"
	"time"

	"google.golang.org/adk/tool"
)

// SandboxVM represents a specific Virtual Machine engine (QuickJS, Prolog, etc.)
type SandboxVM interface {
	// Init initializes the VM with the given configuration and host context.
	Init(cfg VMConfig, host *HostContext) error

	// Run executes the provided code and returns the result.
	Run(ctx context.Context, code string) (any, error)

	// Reset clears the VM state.
	Reset() error

	// Close releases resources associated with the VM.
	Close() error
}

// VMConfig contains configuration for a specific VM instance.
type VMConfig struct {
	Type          string            `yaml:"type"`
	MemoryLimitMB int               `yaml:"memory_limit_mb"`
	Timeout       time.Duration     `yaml:"timeout"`
	AllowTools    []string          `yaml:"allow_tools"`
	AllowNet      []string          `yaml:"allow_net"`
	Env           map[string]string `yaml:"env"`
}

// HostContext provides the bridge between the VM and the host environment.
type HostContext struct {
	// Tools are the external tools available to the VM.
	Tools tool.ToolRegistry

	// Logger receives logs from the VM.
	Logger io.Writer

	// Registry is used to look up additional components if needed.
	// (Optional: can be expanded based on requirements)
}

// SandboxResult contains the output of a sandbox execution.
type SandboxResult struct {
	Value any    `json:"value"`
	Logs  string `json:"logs"`
}
