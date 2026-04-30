package wasm

import (
	"context"
	"fmt"
	"iter"
	"log"
	"os"
	"time"

	"github.com/innomon/agentic/pkg/registry"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/session"
)

type WasmAgentConfig struct {
	registry.AgentBase `yaml:",inline"`
	ModulePath         string            `yaml:"module_path"`
	Params             map[string]string `yaml:"params,omitempty"`
}

func (c *WasmAgentConfig) Validate() error {
	if c.ModulePath == "" {
		return fmt.Errorf("module_path is required for wasm agent")
	}
	return nil
}

type subagentMetrics struct {
	durationMs   int64
	inputTokens  int32
	outputTokens int32
}

type wasmEnv struct {
	invCtx    agent.InvocationContext
	subs      []agent.Agent
	yield     func(*session.Event, error) bool
	lastErr   string
	outputs   map[int32]string
	nextInput string
	metrics   map[int32]subagentMetrics
	params    map[string]string
}

type wrappedCtx struct {
	agent.InvocationContext
	input string
}

func (c *wrappedCtx) Input() string { return c.input }

type hasInput interface {
	Input() string
}

func getInput(ctx agent.InvocationContext) string {
	if hi, ok := ctx.(hasInput); ok {
		return hi.Input()
	}
	// Fallback to UserContent if available
	content := ctx.UserContent()
	if content == nil {
		return ""
	}
	var text string
	for _, p := range content.Parts {
		text += p.Text
	}
	return text
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
		Run:         newWasmRunFunc(wasmBytes, sub, cfg),
	})
}

func newWasmRunFunc(wasmBytes []byte, subs []agent.Agent, cfg *WasmAgentConfig) func(agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(invCtx agent.InvocationContext) iter.Seq2[*session.Event, error] {
		return func(yield func(*session.Event, error) bool) {
			env := &wasmEnv{
				invCtx:  invCtx,
				subs:    subs,
				yield:   yield,
				outputs: make(map[int32]string),
				metrics: make(map[int32]subagentMetrics),
				params:  cfg.Params,
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
				WithFunc(func(_ context.Context, mod api.Module, keyPtr, keyLen int32) int32 {
					keyBytes, ok := mod.Memory().Read(uint32(keyPtr), uint32(keyLen))
					if !ok {
						return -1
					}
					val, ok := env.params[string(keyBytes)]
					if !ok {
						return -1
					}
					return int32(len(val))
				}).
				Export("get_config_param_len")

			hostBuilder.NewFunctionBuilder().
				WithFunc(func(_ context.Context, mod api.Module, keyPtr, keyLen, valPtr, valCap int32) int32 {
					keyBytes, ok := mod.Memory().Read(uint32(keyPtr), uint32(keyLen))
					if !ok {
						return -1
					}
					val, ok := env.params[string(keyBytes)]
					if !ok {
						return -1
					}
					valBytes := []byte(val)
					if int32(len(valBytes)) > valCap {
						valBytes = valBytes[:valCap]
					}
					if !mod.Memory().Write(uint32(valPtr), valBytes) {
						return -1
					}
					return int32(len(valBytes))
				}).
				Export("get_config_param")

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

					var runCtx agent.InvocationContext = env.invCtx
					if env.nextInput != "" {
						runCtx = &wrappedCtx{InvocationContext: env.invCtx, input: env.nextInput}
						env.nextInput = ""
					}

					sub := env.subs[index]
					var output string
					var inputTokens, outputTokens int32
					start := time.Now()
					for ev, err := range sub.Run(runCtx) {
						if err != nil {
							env.lastErr = err.Error()
							return 1
						}
						if ev.Content != nil {
							for _, p := range ev.Content.Parts {
								output += p.Text
							}
						}
						if ev.LLMResponse.UsageMetadata != nil {
							inputTokens += ev.LLMResponse.UsageMetadata.PromptTokenCount
							outputTokens += ev.LLMResponse.UsageMetadata.CandidatesTokenCount
						}
						if !env.yield(ev, nil) {
							return 0
						}
					}
					duration := time.Since(start)
					env.outputs[index] = output
					env.metrics[index] = subagentMetrics{
						durationMs:   duration.Milliseconds(),
						inputTokens:  inputTokens,
						outputTokens: outputTokens,
					}
					return 0
				}).
				Export("run_subagent")

			hostBuilder.NewFunctionBuilder().
				WithFunc(func(_ context.Context, _ api.Module, index int32) int64 {
					return env.metrics[index].durationMs
				}).
				Export("subagent_duration_ms")

			hostBuilder.NewFunctionBuilder().
				WithFunc(func(_ context.Context, _ api.Module, index int32) int32 {
					return env.metrics[index].inputTokens
				}).
				Export("subagent_token_input")

			hostBuilder.NewFunctionBuilder().
				WithFunc(func(_ context.Context, _ api.Module, index int32) int32 {
					return env.metrics[index].outputTokens
				}).
				Export("subagent_token_output")

			hostBuilder.NewFunctionBuilder().
				WithFunc(func(_ context.Context, _ api.Module, index int32) int32 {
					return int32(len(env.outputs[index]))
				}).
				Export("subagent_output_len")

			hostBuilder.NewFunctionBuilder().
				WithFunc(func(_ context.Context, mod api.Module, index, bufPtr, bufCap int32) int32 {
					out, ok := env.outputs[index]
					if !ok {
						return -1
					}
					outBytes := []byte(out)
					if int32(len(outBytes)) > bufCap {
						outBytes = outBytes[:bufCap]
					}
					if !mod.Memory().Write(uint32(bufPtr), outBytes) {
						return -1
					}
					return int32(len(outBytes))
				}).
				Export("subagent_output_get")

			hostBuilder.NewFunctionBuilder().
				WithFunc(func(_ context.Context, mod api.Module, ptr, length int32) {
					buf, ok := mod.Memory().Read(uint32(ptr), uint32(length))
					if ok {
						env.nextInput = string(buf)
					}
				}).
				Export("set_input")

			hostBuilder.NewFunctionBuilder().
				WithFunc(func(_ context.Context, _ api.Module) int32 {
					return int32(len(getInput(env.invCtx)))
				}).
				Export("get_input_len")

			hostBuilder.NewFunctionBuilder().
				WithFunc(func(_ context.Context, mod api.Module, bufPtr, bufCap int32) int32 {
					input := []byte(getInput(env.invCtx))
					if int32(len(input)) > bufCap {
						input = input[:bufCap]
					}
					if !mod.Memory().Write(uint32(bufPtr), input) {
						return -1
					}
					return int32(len(input))
				}).
				Export("get_input")

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
					errMsg = fmt.Sprintf("exit code %d", results[0])
				}
				yield(nil, fmt.Errorf("wasm: %s", errMsg))
			}
		}
	}
}

func init() {
	registry.RegisterAgentType("wasm", wasmCreator)
}
