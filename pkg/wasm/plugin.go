package wasm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/innomon/agentic/pkg/registry"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/plugin"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/genai"
)

// JSON helper structs to pass context over WASM boundary

type InvocationContextJSON struct {
	InvocationID string `json:"invocation_id"`
	SessionID    string `json:"session_id"`
	UserID       string `json:"user_id"`
	AppName      string `json:"app_name"`
	Branch       string `json:"branch"`
	Input        string `json:"input,omitempty"`
}

func toInvocationContextJSON(ctx agent.InvocationContext) InvocationContextJSON {
	return InvocationContextJSON{
		InvocationID: ctx.InvocationID(),
		SessionID:    ctx.Session().ID(),
		UserID:       ctx.Session().UserID(),
		AppName:      ctx.Session().AppName(),
		Branch:       ctx.Branch(),
		Input:        getInput(ctx),
	}
}

type CallbackContextJSON struct {
	InvocationID string `json:"invocation_id"`
	SessionID    string `json:"session_id"`
	UserID       string `json:"user_id"`
	AppName      string `json:"app_name"`
	Branch       string `json:"branch"`
	AgentName    string `json:"agent_name"`
}

func toCallbackContextJSON(ctx agent.Context) CallbackContextJSON {
	return CallbackContextJSON{
		InvocationID: ctx.InvocationID(),
		SessionID:    ctx.SessionID(),
		UserID:       ctx.UserID(),
		AppName:      ctx.AppName(),
		Branch:       ctx.Branch(),
		AgentName:    ctx.AgentName(),
	}
}

type ToolContextJSON struct {
	InvocationID   string `json:"invocation_id"`
	SessionID      string `json:"session_id"`
	UserID         string `json:"user_id"`
	AppName        string `json:"app_name"`
	Branch         string `json:"branch"`
	AgentName      string `json:"agent_name"`
	FunctionCallID string `json:"function_call_id"`
}

func toToolContextJSON(ctx agent.Context) ToolContextJSON {
	return ToolContextJSON{
		InvocationID:   ctx.InvocationID(),
		SessionID:      ctx.SessionID(),
		UserID:         ctx.UserID(),
		AppName:        ctx.AppName(),
		Branch:         ctx.Branch(),
		AgentName:      ctx.AgentName(),
		FunctionCallID: ctx.FunctionCallID(),
	}
}

// Callback Input/Output Wrapper Structs

type OnUserMessageInput struct {
	Context     InvocationContextJSON `json:"context"`
	UserMessage *genai.Content        `json:"user_message"`
}

type ContentOutput struct {
	Content *genai.Content `json:"content,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type OnEventInput struct {
	Context InvocationContextJSON `json:"context"`
	Event   *session.Event        `json:"event"`
}

type OnEventOutput struct {
	Event *session.Event `json:"event,omitempty"`
	Error string         `json:"error,omitempty"`
}

type BeforeRunInput struct {
	Context InvocationContextJSON `json:"context"`
}

type AfterRunInput struct {
	Context InvocationContextJSON `json:"context"`
}

type BeforeAgentInput struct {
	Context CallbackContextJSON `json:"context"`
}

type AfterAgentInput struct {
	Context CallbackContextJSON `json:"context"`
}

type BeforeModelInput struct {
	Context CallbackContextJSON `json:"context"`
	Request *model.LLMRequest   `json:"request"`
}

type ModelOutput struct {
	Response *model.LLMResponse `json:"response,omitempty"`
	Error    string             `json:"error,omitempty"`
}

type AfterModelInput struct {
	Context  CallbackContextJSON `json:"context"`
	Response *model.LLMResponse  `json:"response"`
	Error    string              `json:"error,omitempty"`
}

type OnModelErrorInput struct {
	Context CallbackContextJSON `json:"context"`
	Request *model.LLMRequest   `json:"request"`
	Error   string              `json:"error,omitempty"`
}

type BeforeToolInput struct {
	Context  ToolContextJSON `json:"context"`
	ToolName string          `json:"tool_name"`
	Args     map[string]any  `json:"args"`
}

type ToolArgsOutput struct {
	Args  map[string]any `json:"args,omitempty"`
	Error string         `json:"error,omitempty"`
}

type AfterToolInput struct {
	Context  ToolContextJSON `json:"context"`
	ToolName string          `json:"tool_name"`
	Args     map[string]any  `json:"args"`
	Result   map[string]any  `json:"result"`
	Error    string          `json:"error,omitempty"`
}

type ToolResultOutput struct {
	Result map[string]any `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}

type OnToolErrorInput struct {
	Context  ToolContextJSON `json:"context"`
	ToolName string          `json:"tool_name"`
	Args     map[string]any  `json:"args"`
	Error    string          `json:"error,omitempty"`
}

func invokeWasmCallback[I any, O any](ctx context.Context, mod api.Module, mu *sync.Mutex, funcName string, input I) (*O, error) {
	fn := mod.ExportedFunction(funcName)
	if fn == nil {
		return nil, nil // If the guest doesn't export the function, it's a no-op
	}

	mu.Lock()
	defer mu.Unlock()

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input for %s: %w", funcName, err)
	}

	alloc := mod.ExportedFunction("alloc")
	if alloc == nil {
		return nil, fmt.Errorf("wasm module does not export 'alloc' function")
	}

	allocResults, err := alloc.Call(ctx, uint64(len(inputJSON)))
	if err != nil {
		return nil, fmt.Errorf("alloc failed for %s: %w", funcName, err)
	}
	if len(allocResults) == 0 {
		return nil, fmt.Errorf("alloc returned no results for %s", funcName)
	}
	inPtr := uint32(allocResults[0])
	if inPtr == 0 {
		return nil, fmt.Errorf("alloc returned null pointer for %s", funcName)
	}

	if !mod.Memory().Write(inPtr, inputJSON) {
		return nil, fmt.Errorf("failed to write input to wasm memory for %s at offset %d (len %d)", funcName, inPtr, len(inputJSON))
	}

	results, err := fn.Call(ctx, uint64(inPtr), uint64(len(inputJSON)))

	// Always free the input buffer
	freeFn := mod.ExportedFunction("free")
	if freeFn != nil {
		_, _ = freeFn.Call(ctx, uint64(inPtr), uint64(len(inputJSON)))
	}

	if err != nil {
		return nil, fmt.Errorf("guest function %s execution failed: %w", funcName, err)
	}

	if len(results) == 0 {
		return nil, nil
	}

	packed := results[0]
	outPtr := uint32(packed >> 32)
	outLen := uint32(packed & 0xFFFFFFFF)

	if outLen == 0 {
		return nil, nil
	}

	if outLen > maxOutputSize {
		return nil, fmt.Errorf("output size %d exceeds maximum %d", outLen, maxOutputSize)
	}

	outBytes, ok := mod.Memory().Read(outPtr, outLen)
	if !ok {
		return nil, fmt.Errorf("failed to read output from wasm memory at offset %d (len %d)", outPtr, outLen)
	}

	if freeFn != nil {
		_, _ = freeFn.Call(ctx, uint64(outPtr), uint64(outLen))
	}

	var output O
	if err := json.Unmarshal(outBytes, &output); err != nil {
		return nil, fmt.Errorf("failed to unmarshal guest output for %s: %w", funcName, err)
	}

	return &output, nil
}

func NewWasmPlugin(ctx context.Context, name string, modulePath string, pluginConfig map[string]any) (*plugin.Plugin, error) {
	wasmBytes, err := os.ReadFile(modulePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read wasm plugin module %q: %w", modulePath, err)
	}

	rtConfig := wazero.NewRuntimeConfig().
		WithCompilationCache(getCompilationCache())
	rt := wazero.NewRuntimeWithConfig(ctx, rtConfig)

	// In case of error before we finish building the plugin, close runtime
	success := false
	defer func() {
		if !success {
			_ = rt.Close(ctx)
		}
	}()

	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	hostBuilder := rt.NewHostModuleBuilder("env")
	hostBuilder.NewFunctionBuilder().
		WithFunc(func(_ context.Context, mod api.Module, ptr, length int32) {
			buf, ok := mod.Memory().Read(uint32(ptr), uint32(length))
			if ok {
				log.Printf("[%s] %s", name, string(buf))
			}
		}).
		Export("log_msg")

	if _, err := hostBuilder.Instantiate(ctx); err != nil {
		return nil, fmt.Errorf("failed to instantiate env module for wasm plugin: %w", err)
	}

	compiled, err := rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile wasm plugin module: %w", err)
	}

	mod, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().
		WithStdout(os.Stdout).
		WithStderr(os.Stderr))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate wasm plugin module: %w", err)
	}

	// Call any standard initialize/start function if exported by guest, passing pluginConfig as JSON
	if startFn := mod.ExportedFunction("initialize"); startFn != nil {
		cfgJSON, err := json.Marshal(pluginConfig)
		if err != nil {
			mod.Close(ctx)
			return nil, fmt.Errorf("failed to marshal pluginConfig: %w", err)
		}
		alloc := mod.ExportedFunction("alloc")
		if alloc == nil {
			mod.Close(ctx)
			return nil, fmt.Errorf("wasm module does not export 'alloc' function")
		}
		allocResults, err := alloc.Call(ctx, uint64(len(cfgJSON)))
		if err != nil || len(allocResults) == 0 {
			mod.Close(ctx)
			return nil, fmt.Errorf("failed to alloc for initialize: %w", err)
		}
		inPtr := uint32(allocResults[0])
		if !mod.Memory().Write(inPtr, cfgJSON) {
			mod.Close(ctx)
			return nil, fmt.Errorf("failed to write initialize config to wasm memory")
		}
		if _, err := startFn.Call(ctx, uint64(inPtr), uint64(len(cfgJSON))); err != nil {
			mod.Close(ctx)
			return nil, fmt.Errorf("failed to call initialize: %w", err)
		}
		if freeFn := mod.ExportedFunction("free"); freeFn != nil {
			_, _ = freeFn.Call(ctx, uint64(inPtr), uint64(len(cfgJSON)))
		}
	}

	var mu sync.Mutex

	pConfig := plugin.Config{
		Name: name,
		CloseFunc: func() error {
			return rt.Close(context.Background())
		},
	}

	// Conditionally bind callbacks only if the guest exports the corresponding functions
	if mod.ExportedFunction("on_user_message") != nil {
		pConfig.OnUserMessageCallback = func(ic agent.InvocationContext, msg *genai.Content) (*genai.Content, error) {
			input := OnUserMessageInput{
				Context:     toInvocationContextJSON(ic),
				UserMessage: msg,
			}
			out, err := invokeWasmCallback[OnUserMessageInput, ContentOutput](ic, mod, &mu, "on_user_message", input)
			if err != nil {
				return nil, err
			}
			if out == nil {
				return nil, nil
			}
			if out.Error != "" {
				return nil, fmt.Errorf("%s", out.Error)
			}
			return out.Content, nil
		}
	}

	if mod.ExportedFunction("on_event") != nil {
		pConfig.OnEventCallback = func(ic agent.InvocationContext, event *session.Event) (*session.Event, error) {
			input := OnEventInput{
				Context: toInvocationContextJSON(ic),
				Event:   event,
			}
			out, err := invokeWasmCallback[OnEventInput, OnEventOutput](ic, mod, &mu, "on_event", input)
			if err != nil {
				return nil, err
			}
			if out == nil {
				return nil, nil
			}
			if out.Error != "" {
				return nil, fmt.Errorf("%s", out.Error)
			}
			return out.Event, nil
		}
	}

	if mod.ExportedFunction("before_run") != nil {
		pConfig.BeforeRunCallback = func(ic agent.InvocationContext) (*genai.Content, error) {
			input := BeforeRunInput{
				Context: toInvocationContextJSON(ic),
			}
			out, err := invokeWasmCallback[BeforeRunInput, ContentOutput](ic, mod, &mu, "before_run", input)
			if err != nil {
				return nil, err
			}
			if out == nil {
				return nil, nil
			}
			if out.Error != "" {
				return nil, fmt.Errorf("%s", out.Error)
			}
			return out.Content, nil
		}
	}

	if mod.ExportedFunction("after_run") != nil {
		pConfig.AfterRunCallback = func(ic agent.InvocationContext) {
			input := AfterRunInput{
				Context: toInvocationContextJSON(ic),
			}
			_, _ = invokeWasmCallback[AfterRunInput, any](ic, mod, &mu, "after_run", input)
		}
	}

	if mod.ExportedFunction("before_agent") != nil {
		pConfig.BeforeAgentCallback = func(cc agent.Context) (*genai.Content, error) {
			input := BeforeAgentInput{
				Context: toCallbackContextJSON(cc),
			}
			out, err := invokeWasmCallback[BeforeAgentInput, ContentOutput](context.Background(), mod, &mu, "before_agent", input)
			if err != nil {
				return nil, err
			}
			if out == nil {
				return nil, nil
			}
			if out.Error != "" {
				return nil, fmt.Errorf("%s", out.Error)
			}
			return out.Content, nil
		}
	}

	if mod.ExportedFunction("after_agent") != nil {
		pConfig.AfterAgentCallback = func(cc agent.Context) (*genai.Content, error) {
			input := AfterAgentInput{
				Context: toCallbackContextJSON(cc),
			}
			out, err := invokeWasmCallback[AfterAgentInput, ContentOutput](context.Background(), mod, &mu, "after_agent", input)
			if err != nil {
				return nil, err
			}
			if out == nil {
				return nil, nil
			}
			if out.Error != "" {
				return nil, fmt.Errorf("%s", out.Error)
			}
			return out.Content, nil
		}
	}

	if mod.ExportedFunction("before_model") != nil {
		pConfig.BeforeModelCallback = func(cc agent.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
			input := BeforeModelInput{
				Context: toCallbackContextJSON(cc),
				Request: req,
			}
			out, err := invokeWasmCallback[BeforeModelInput, ModelOutput](context.Background(), mod, &mu, "before_model", input)
			if err != nil {
				return nil, err
			}
			if out == nil {
				return nil, nil
			}
			if out.Error != "" {
				return nil, fmt.Errorf("%s", out.Error)
			}
			return out.Response, nil
		}
	}

	if mod.ExportedFunction("after_model") != nil {
		pConfig.AfterModelCallback = func(cc agent.Context, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			input := AfterModelInput{
				Context:  toCallbackContextJSON(cc),
				Response: resp,
				Error:    errStr,
			}
			out, errVal := invokeWasmCallback[AfterModelInput, ModelOutput](context.Background(), mod, &mu, "after_model", input)
			if errVal != nil {
				return nil, errVal
			}
			if out == nil {
				return nil, nil
			}
			if out.Error != "" {
				return nil, fmt.Errorf("%s", out.Error)
			}
			return out.Response, nil
		}
	}

	if mod.ExportedFunction("on_model_error") != nil {
		pConfig.OnModelErrorCallback = func(cc agent.Context, req *model.LLMRequest, err error) (*model.LLMResponse, error) {
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			input := OnModelErrorInput{
				Context: toCallbackContextJSON(cc),
				Request: req,
				Error:   errStr,
			}
			out, errVal := invokeWasmCallback[OnModelErrorInput, ModelOutput](context.Background(), mod, &mu, "on_model_error", input)
			if errVal != nil {
				return nil, errVal
			}
			if out == nil {
				return nil, nil
			}
			if out.Error != "" {
				return nil, fmt.Errorf("%s", out.Error)
			}
			return out.Response, nil
		}
	}

	if mod.ExportedFunction("before_tool") != nil {
		pConfig.BeforeToolCallback = func(tc agent.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
			input := BeforeToolInput{
				Context:  toToolContextJSON(tc),
				ToolName: t.Name(),
				Args:     args,
			}
			out, err := invokeWasmCallback[BeforeToolInput, ToolArgsOutput](context.Background(), mod, &mu, "before_tool", input)
			if err != nil {
				return nil, err
			}
			if out == nil {
				return nil, nil
			}
			if out.Error != "" {
				return nil, fmt.Errorf("%s", out.Error)
			}
			return out.Args, nil
		}
	}

	if mod.ExportedFunction("after_tool") != nil {
		pConfig.AfterToolCallback = func(tc agent.Context, t tool.Tool, args, result map[string]any, err error) (map[string]any, error) {
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			input := AfterToolInput{
				Context:  toToolContextJSON(tc),
				ToolName: t.Name(),
				Args:     args,
				Result:   result,
				Error:    errStr,
			}
			out, errVal := invokeWasmCallback[AfterToolInput, ToolResultOutput](context.Background(), mod, &mu, "after_tool", input)
			if errVal != nil {
				return nil, errVal
			}
			if out == nil {
				return nil, nil
			}
			if out.Error != "" {
				return nil, fmt.Errorf("%s", out.Error)
			}
			return out.Result, nil
		}
	}

	if mod.ExportedFunction("on_tool_error") != nil {
		pConfig.OnToolErrorCallback = func(tc agent.Context, t tool.Tool, args map[string]any, err error) (map[string]any, error) {
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			input := OnToolErrorInput{
				Context:  toToolContextJSON(tc),
				ToolName: t.Name(),
				Args:     args,
				Error:    errStr,
			}
			out, errVal := invokeWasmCallback[OnToolErrorInput, ToolResultOutput](context.Background(), mod, &mu, "on_tool_error", input)
			if errVal != nil {
				return nil, errVal
			}
			if out == nil {
				return nil, nil
			}
			if out.Error != "" {
				return nil, fmt.Errorf("%s", out.Error)
			}
			return out.Result, nil
		}
	}

	pl, err := plugin.New(pConfig)
	if err != nil {
		mod.Close(ctx)
		return nil, fmt.Errorf("failed to create plugin struct: %w", err)
	}

	success = true
	return pl, nil
}

func init() {
	registry.RegisterPluginCreator("wasm", func(ctx context.Context, name string, entry registry.PluginEntry) (*plugin.Plugin, error) {
		modulePath, ok := entry.Config["module_path"].(string)
		if !ok || modulePath == "" {
			return nil, fmt.Errorf("module_path is required for wasm plugin")
		}

		var pluginParams map[string]any
		if params, ok := entry.Config["config"].(map[string]any); ok {
			pluginParams = params
		} else if paramsNode, ok := entry.Config["config"].(map[any]any); ok {
			pluginParams = make(map[string]any)
			for k, v := range paramsNode {
				pluginParams[fmt.Sprintf("%v", k)] = v
			}
		}

		return NewWasmPlugin(ctx, name, modulePath, pluginParams)
	})

	registry.RegisterPluginCreator("wasm_plugin", func(ctx context.Context, name string, entry registry.PluginEntry) (*plugin.Plugin, error) {
		modulePath, ok := entry.Config["module_path"].(string)
		if !ok || modulePath == "" {
			return nil, fmt.Errorf("module_path is required for wasm plugin")
		}

		var pluginParams map[string]any
		if params, ok := entry.Config["config"].(map[string]any); ok {
			pluginParams = params
		} else if paramsNode, ok := entry.Config["config"].(map[any]any); ok {
			pluginParams = make(map[string]any)
			for k, v := range paramsNode {
				pluginParams[fmt.Sprintf("%v", k)] = v
			}
		}

		return NewWasmPlugin(ctx, name, modulePath, pluginParams)
	})
}
