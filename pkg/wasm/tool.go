package wasm

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/innomon/agentic/pkg/registry"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type WasmToolConfig struct {
	registry.ToolBase `yaml:",inline"`
	ModulePath        string         `yaml:"module_path,omitempty"`
	OCIRef            string         `yaml:"oci_ref,omitempty"`
	CacheDir          string         `yaml:"cache_dir,omitempty"`
	Security          SecurityPolicy `yaml:"security"`
}

func (c *WasmToolConfig) Validate() error {
	if c.ModulePath == "" && c.OCIRef == "" {
		return fmt.Errorf("either module_path or oci_ref is required for wasm tool")
	}
	if c.ModulePath != "" && c.OCIRef != "" {
		return fmt.Errorf("only one of module_path or oci_ref can be specified")
	}
	return c.Security.Validate()
}

var (
	ociPullerOnce sync.Once
	ociPuller     *OCIPuller
)

func getOCIPuller(cacheDir string) *OCIPuller {
	ociPullerOnce.Do(func() {
		ociPuller = NewOCIPuller(cacheDir)
	})
	return ociPuller
}

func loadWasmBytes(ctx context.Context, cfg *WasmToolConfig) ([]byte, error) {
	if cfg.ModulePath != "" {
		if cached, ok := wasmBytecodeCache.Get(cfg.ModulePath); ok {
			return cached, nil
		}
		data, err := os.ReadFile(cfg.ModulePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read wasm module %q: %w", cfg.ModulePath, err)
		}
		wasmBytecodeCache.Set(cfg.ModulePath, data)
		return data, nil
	}

	if cached, ok := wasmBytecodeCache.Get(cfg.OCIRef); ok {
		return cached, nil
	}
	puller := getOCIPuller(cfg.CacheDir)
	data, err := puller.PullWasm(ctx, cfg.OCIRef)
	if err != nil {
		return nil, fmt.Errorf("failed to pull wasm from OCI %q: %w", cfg.OCIRef, err)
	}
	wasmBytecodeCache.Set(cfg.OCIRef, data)
	return data, nil
}

func wasmToolCreator(ctx context.Context, name string, cfg *WasmToolConfig) (tool.Tool, error) {
	wasmBytes, err := loadWasmBytes(ctx, cfg)
	if err != nil {
		return nil, err
	}

	policy := cfg.Security

	return functiontool.New(functiontool.Config{
		Name:        name,
		Description: cfg.Description,
	}, func(toolCtx tool.Context, args map[string]any) (any, error) {
		return executeWasmTool(toolCtx, wasmBytes, &policy, name, args)
	})
}

func executeWasmTool(ctx context.Context, wasmBytes []byte, policy *SecurityPolicy, name string, args map[string]any) (any, error) {
	rtConfig := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(policy.EffectiveMemoryMaxPages()).
		WithCompilationCache(getCompilationCache())

	rt := wazero.NewRuntimeWithConfig(ctx, rtConfig)
	defer rt.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	hostBuilder := rt.NewHostModuleBuilder("env")
	hostBuilder.NewFunctionBuilder().
		WithFunc(func(_ context.Context, mod api.Module, ptr, length int32) {
			buf, ok := mod.Memory().Read(uint32(ptr), uint32(length))
			if ok {
				log.Printf("[wasm:%s] %s", name, string(buf))
			}
		}).
		Export("log_msg")
	registerNetHostFunctions(hostBuilder, policy)

	if _, err := hostBuilder.Instantiate(ctx); err != nil {
		return nil, fmt.Errorf("failed to instantiate host module: %w", err)
	}

	compiled, err := rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile wasm module: %w", err)
	}

	modConfig := wazero.NewModuleConfig().
		WithFSConfig(policy.FSConfig()).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr)

	mod, err := rt.InstantiateModule(ctx, compiled, modConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate wasm module: %w", err)
	}
	defer mod.Close(ctx)

	return callTool(ctx, mod, args)
}

func init() {
	registry.RegisterToolType("wasm", wasmToolCreator)
}
