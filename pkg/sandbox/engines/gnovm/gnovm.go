package gnovm

import (
	"github.com/innomon/agentic/pkg/gnovm"
	"github.com/innomon/agentic/pkg/sandbox"
)

func init() {
	sandbox.RegisterVMEngine("gnovm", func() sandbox.SandboxVM {
		return gnovm.NewSandboxVM()
	})
}
