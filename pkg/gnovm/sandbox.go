package gnovm

import (
	"context"
	"fmt"

	"github.com/innomon/agentic/pkg/sandbox"
)

type SandboxVM struct {
	wrapper *MachineWrapper
	cfg     sandbox.VMConfig
	host    *sandbox.HostContext
}

func NewSandboxVM() sandbox.SandboxVM {
	return &SandboxVM{}
}

func (v *SandboxVM) Init(cfg sandbox.VMConfig, host *sandbox.HostContext) error {
	v.cfg = cfg
	v.host = host

	wrapper, err := NewMachineWrapper(MachineOptions{
		PkgPath:    "agentic/e/sandbox",
		NativePkgs: sandboxNativePkgs,
		Context:    host,
	})
	if err != nil {
		return err
	}
	v.wrapper = wrapper
	return nil
}

func (v *SandboxVM) Run(ctx context.Context, code string) (any, error) {
	if v.wrapper == nil {
		return nil, fmt.Errorf("Gno sandbox not initialized")
	}

	// Try to evaluate as an expression
	res, err := v.wrapper.Eval(code)
	if err != nil {
		return nil, fmt.Errorf("Gno evaluation error: %w", err)
	}

	if len(res) == 0 {
		return nil, nil
	}

	// For now, return the string representation of the first result
	return res[0].String(), nil
}

func (v *SandboxVM) Reset() error {
	if v.wrapper == nil {
		return nil
	}
	return v.Init(v.cfg, v.host)
}

func (v *SandboxVM) Close() error {
	v.wrapper = nil
	return nil
}
