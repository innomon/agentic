package wasm

import (
	"context"
	"fmt"
	"iter"
	"log"
	"os"

	"github.com/innomon/med-agent/internal/registry"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/session"
)

type WasmAgentConfig struct {
	registry.AgentBase `yaml:",inline"`
	ModulePath         string `yaml:"module_path"`
}

func (c *WasmAgentConfig) Validate() error {
	if c.ModulePath == "" {
		return fmt.Errorf("module_path is required for wasm agent")
	}
	return nil
}

type wasmEnv struct {
	invCtx  agent.InvocationContext
	subs    []agent.Agent
	yield   func(*session.Event, error) bool
	lastErr string
}

func wasmCreator(ctx context.Context, name string, cfg *WasmAgentConfig, _ registry.ModelRegistry, _ registry.ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
	wasmBytes, err := os.ReadFile(cfg.ModulePath)
	if err != nil {
		return nil, fmt.Errorf("wasm agent %q: failed to read module %q: %w", name, cfg.ModulePath, err)
	}

	return agent.New(agent.Config{
		Name:        name,
		Description: cfg.Description,
		SubAgents:   sub,
		Run:         newWasmRunFunc(wasmBytes, sub),
	})
}

func newWasmRunFunc(wasmBytes []byte, subs []agent.Agent) func(agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(invCtx agent.InvocationContext) iter.Seq2[*session.Event, error] {
		return func(yield func(*session.Event, error) bool) {
			env := &wasmEnv{
				invCtx: invCtx,
				subs:   subs,
				yield:  yield,
			}

			rtConfig := wazero.NewRuntimeConfig().
				WithCompilationCache(getCompilationCache())
			rt := wazero.NewRuntimeWithConfig(invCtx, rtConfig)
			defer rt.Close(invCtx)

			wasi_snapshot_preview1.MustInstantiate(invCtx, rt)

			hostBuilder := rt.NewHostModuleBuilder("env")
			hostBuilder.NewFunctionBuilder().
				WithFunc(func(_ context.Context, _ api.Module) int32 {
					return int32(len(env.subs))
				}).
				Export("subagent_count")

			hostBuilder.NewFunctionBuilder().
				WithFunc(func(_ context.Context, mod api.Module, index, bufPtr, bufCap int32) int32 {
					if int(index) < 0 || int(index) >= len(env.subs) {
						return -1
					}
					nameBytes := []byte(env.subs[index].Name())
					if int32(len(nameBytes)) > bufCap {
						return -1
					}
					if !mod.Memory().Write(uint32(bufPtr), nameBytes) {
						return -1
					}
					return int32(len(nameBytes))
				}).
				Export("subagent_name")

			hostBuilder.NewFunctionBuilder().
				WithFunc(func(_ context.Context, _ api.Module, index int32) int32 {
					if int(index) < 0 || int(index) >= len(env.subs) {
						env.lastErr = fmt.Sprintf("sub-agent index %d out of range", index)
						return 1
					}
					sub := env.subs[index]
					for ev, err := range sub.Run(env.invCtx) {
						if err != nil {
							env.lastErr = err.Error()
							return 1
						}
						if !env.yield(ev, nil) {
							return 0
						}
					}
					return 0
				}).
				Export("run_subagent")

			hostBuilder.NewFunctionBuilder().
				WithFunc(func(_ context.Context, mod api.Module, ptr, length int32) {
					buf, ok := mod.Memory().Read(uint32(ptr), uint32(length))
					if ok {
						log.Printf("[wasm] %s", string(buf))
					}
				}).
				Export("log_msg")

			if _, err := hostBuilder.Instantiate(invCtx); err != nil {
				yield(nil, fmt.Errorf("wasm: failed to instantiate host module: %w", err))
				return
			}

			compiled, err := rt.CompileModule(invCtx, wasmBytes)
			if err != nil {
				yield(nil, fmt.Errorf("wasm: failed to compile module: %w", err))
				return
			}

			mod, err := rt.InstantiateModule(invCtx, compiled, wazero.NewModuleConfig())
			if err != nil {
				yield(nil, fmt.Errorf("wasm: failed to instantiate module: %w", err))
				return
			}
			defer mod.Close(invCtx)

			execute := mod.ExportedFunction("execute")
			if execute == nil {
				yield(nil, fmt.Errorf("wasm: module does not export 'execute' function"))
				return
			}

			results, err := execute.Call(invCtx)
			if err != nil {
				yield(nil, fmt.Errorf("wasm: execute failed: %w", err))
				return
			}

			if len(results) > 0 && results[0] != 0 {
				errMsg := env.lastErr
				if errMsg == "" {
					errMsg = fmt.Sprintf("execute returned non-zero: %d", results[0])
				}
				yield(nil, fmt.Errorf("wasm: %s", errMsg))
			}
		}
	}
}

func init() {
	registry.RegisterAgentType("wasm", wasmCreator)
}
