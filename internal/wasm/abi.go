package wasm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

const maxOutputSize = 16 * 1024 * 1024 // 16MB

func callTool(ctx context.Context, mod api.Module, args map[string]any) (any, error) {
	inputJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	alloc := mod.ExportedFunction("alloc")
	if alloc == nil {
		return nil, fmt.Errorf("wasm module does not export 'alloc' function")
	}

	runTool := mod.ExportedFunction("run_tool")
	if runTool == nil {
		return nil, fmt.Errorf("wasm module does not export 'run_tool' function")
	}

	allocResults, err := alloc.Call(ctx, uint64(len(inputJSON)))
	if err != nil {
		return nil, fmt.Errorf("alloc failed: %w", err)
	}
	if len(allocResults) == 0 {
		return nil, fmt.Errorf("alloc returned no results")
	}
	inPtr := uint32(allocResults[0])
	if inPtr == 0 {
		return nil, fmt.Errorf("alloc returned null pointer")
	}

	if !mod.Memory().Write(inPtr, inputJSON) {
		return nil, fmt.Errorf("failed to write input to wasm memory at offset %d (len %d)", inPtr, len(inputJSON))
	}

	results, err := runTool.Call(ctx, uint64(inPtr), uint64(len(inputJSON)))
	if err != nil {
		return nil, fmt.Errorf("run_tool failed: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("run_tool returned no results")
	}

	packed := results[0]
	outPtr := uint32(packed >> 32)
	outLen := uint32(packed & 0xFFFFFFFF)

	if outLen == 0 {
		return map[string]any{"status": "ok"}, nil
	}

	if outLen > maxOutputSize {
		return nil, fmt.Errorf("output size %d exceeds maximum %d", outLen, maxOutputSize)
	}

	outBytes, ok := mod.Memory().Read(outPtr, outLen)
	if !ok {
		return nil, fmt.Errorf("failed to read output from wasm memory at offset %d (len %d)", outPtr, outLen)
	}

	freeFn := mod.ExportedFunction("free")
	if freeFn != nil {
		_, _ = freeFn.Call(ctx, uint64(inPtr), uint64(len(inputJSON)))
		_, _ = freeFn.Call(ctx, uint64(outPtr), uint64(outLen))
	}

	var result any
	if err := json.Unmarshal(outBytes, &result); err != nil {
		return string(outBytes), nil
	}

	return result, nil
}
