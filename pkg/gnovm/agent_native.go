package gnovm

import (
	"fmt"
	"reflect"

	"github.com/gnolang/gno/gnovm/pkg/gnolang"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/tool"
)

func init() {
	RegisterAgentNativePkg(&NativePkg{
		Name: "log",
		Path: "gno.land/p/log",
		Funcs: map[string]func(m *gnolang.Machine){
			"Println": func(m *gnolang.Machine) {
				arg := m.PopValue()
				fmt.Println("AGENT LOG:", arg.String())
			},
		},
	})

	RegisterAgentNativePkg(&NativePkg{
		Name: "agent",
		Path: "gno.land/p/agent",
		Funcs: map[string]func(m *gnolang.Machine){
			"CallSubAgent": func(m *gnolang.Machine) {
				name := m.PopValue().GetString()
				// input := m.PopValue().GetString() // input is not used yet

				ctx, ok := m.Context.(*AgentContext)
				if !ok {
					m.PushValue(gnolang.Go2GnoValue(m.Alloc, m.Store, reflect.ValueOf("error: missing agent context")))
					return
				}

				var target agent.Agent
				for _, sub := range ctx.SubAgents {
					if sub.Name() == name {
						target = sub
						break
					}
				}

				if target == nil {
					m.PushValue(gnolang.Go2GnoValue(m.Alloc, m.Store, reflect.ValueOf(fmt.Sprintf("error: sub-agent %q not found", name))))
					return
				}

				// This is a synchronous call for now.
				var finalResponse string
				for event, err := range target.Run(ctx.InvCtx) {
					if err != nil {
						m.PushValue(gnolang.Go2GnoValue(m.Alloc, m.Store, reflect.ValueOf(fmt.Sprintf("error: %v", err))))
						return
					}
					if event.LLMResponse.Content != nil {
						for _, part := range event.LLMResponse.Content.Parts {
							if part.Text != "" {
								finalResponse = part.Text
							}
						}
					}
				}

				m.PushValue(gnolang.Go2GnoValue(m.Alloc, m.Store, reflect.ValueOf(finalResponse)))
			},
			"CallTool": func(m *gnolang.Machine) {
				name := m.PopValue().GetString()
				// args is not handled yet

				ctx, ok := m.Context.(*AgentContext)
				if !ok {
					m.PushValue(gnolang.Go2GnoValue(m.Alloc, m.Store, reflect.ValueOf("error: missing agent context")))
					return
				}

				var target tool.Tool
				for _, t := range ctx.Tools {
					if t.Name() == name {
						target = t
						break
					}
				}

				if target == nil {
					m.PushValue(gnolang.Go2GnoValue(m.Alloc, m.Store, reflect.ValueOf(fmt.Sprintf("error: tool %q not found", name))))
					return
				}

				ft, ok := target.(FunctionTool)
				if !ok {
					m.PushValue(gnolang.Go2GnoValue(m.Alloc, m.Store, reflect.ValueOf(fmt.Sprintf("error: tool %q is not a FunctionTool", name))))
					return
				}

				tCtx := DummyToolContext{Context: ctx.InvCtx}
				res, err := ft.Run(tCtx, map[string]any{}) // empty args for now
				if err != nil {
					m.PushValue(gnolang.Go2GnoValue(m.Alloc, m.Store, reflect.ValueOf(fmt.Sprintf("error: %v", err))))
					return
				}

				m.PushValue(gnolang.Go2GnoValue(m.Alloc, m.Store, reflect.ValueOf(fmt.Sprintln(res))))
			},
		},
	})
}
