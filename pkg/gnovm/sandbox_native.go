package gnovm

import (
	"fmt"
	"io"

	"github.com/gnolang/gno/gnovm/pkg/gnolang"
	"github.com/innomon/agentic/pkg/sandbox"
)

func init() {
	RegisterSandboxNativePkg(&NativePkg{
		Name: "log",
		Path: "gno.land/p/log",
		Funcs: map[string]func(m *gnolang.Machine){
			"Println": func(m *gnolang.Machine) {
				arg := m.PopValue()
				var w io.Writer
				if host, ok := m.Context.(*sandbox.HostContext); ok {
					w = host.Logger
				}
				if w == nil {
					w = m.Output
				}
				if w == nil {
					return
				}
				fmt.Fprintln(w, arg.String())
			},
		},
	})
}
