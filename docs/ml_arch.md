# ML Package Architecture

The `pkg/ml` package provides a pure-Go implementation of transformer inference, designed to run local Large Language Models (LLMs) using GGUF weights. It is optimized for portability and ease of integration within the Agentic framework.

## Overview

The package implements the core components of modern LLM inference:
- **GGUF Parsing:** Loading model metadata and weights from GGUF files.
- **Quantization Support:** Dequantizing various GGUF formats (Q4_0, Q4_K, Q8_0, etc.) into `float32`.
- **Architectures:** Implementations of popular transformer architectures like LLaMA and Granite Hybrid (Mamba2 + Attention).
- **Tokenizers:** Byte Pair Encoding (BPE) and SentencePiece (Unigram) tokenizers.
- **Generation Loop:** Autoregressive generation with KV caching and sampling.

## Core Components

### 1. MLModel (`model.go`)
`MLModel` is the primary entry point. It implements the `google.golang.org/adk/model.Model` interface, allowing it to be used anywhere an LLM is required in the ADK.

- **Initialization:** Uses lazy loading via `Init()`. Weights and tokenizer are loaded from the GGUF file on the first generation request.
- **GenerateContent:** Orchestrates the full generation pipeline:
    - Formats the prompt using architecture-specific templates.
    - Encodes text into tokens.
    - Runs the `Generate` loop.
    - Parses tool calls from the output (if requested).

### 2. Architecture Interface (`arch.go`)
Architectures implement the `Arch` interface:
```go
type Arch interface {
    Name() string
    NewCacheForModel(maxSeqLen int) *KVCache
    Forward(inputIDs []int32, pos int, kv *KVCache) []float32
}
```
- **LlamaArch (`arch_llama.go`):** Implements standard LLaMA-style models (GQA, RoPE, SwiGLU).
- **GraniteHybridArch (`arch_granite_hybrid.go`):** Implements the Granite Hybrid architecture, which mixes Transformer Attention layers with Mamba2 SSM (State Space Model) layers for efficient long-context processing.

### 3. Generation Engine (`generate.go`)
The `Generate` function implements the autoregressive loop:
1. **Prefill:** Processes the initial prompt tokens to populate the KV cache.
2. **Decode Loop:** 
    - Samples the next token from logits.
    - Appends the token to the sequence.
    - Updates the KV cache.
    - Yields the generated text chunk.
    - Checks for stop sequences or max tokens limit.

### 4. KV Cache (`kvcache.go`)
Manages the Key-Value cache used to avoid redundant computations during autoregressive decoding. It also supports Mamba-specific states (Convolution state and SSM state) for hybrid models.

### 5. Tokenizers (`tokenizer.go`, `tokenizer_bpe.go`, `tokenizer_sp.go`)
Supports multiple tokenization schemes:
- **SentencePiece:** Used by LLaMA and similar models.
- **BPE:** Used by GPT-style models.
The appropriate tokenizer is automatically selected based on GGUF metadata.

### 6. Operations (`arch_ops.go`)
Contains highly optimized (yet pure-Go) implementations of common neural network operations:
- `MatVecMul`: Matrix-Vector multiplication (the bottleneck of single-token inference).
- `RMSNorm`: Root Mean Square Layer Normalization.
- `Softmax`, `SiLU`, `RoPE`.
- `Conv1DDepthwiseDecode`: Mamba-specific convolution step.

## Usage

### Configuration
`MLConfig` defines the parameters for the model:
```yaml
model_path: "models/granite-3.0-8b-instruct.Q4_K_M.gguf"
context_length: 4096
threads: 8
```

### Integration
To use the `ml` package in a registry:
```go
cfg := &ml.MLConfig{
    ModelPath: "path/to/model.gguf",
}
model, _ := ml.NewMLModel(cfg)
```

## Performance & Enhancements

The current implementation is pure Go, which ensures maximum compatibility across platforms (including WASM/WASI). 

### Recent Enhancements:
- **Parallel Operations:** `MatVecMul` and `MatMul` in `arch_ops.go` utilize Go concurrency with a configurable worker pool to leverage multi-core CPUs.
- **Configurable Parallelism:** Users can control the number of CPU threads used for inference via the `threads` parameter in the model configuration, which is applied during model initialization.
- **GQA Support:** Efficient Grouped-Query Attention implementation.
- **Hybrid Architecture:** Support for Mamba2 SSM layers alongside Attention.

### Future Roadmap:
- **SIMD Optimization:** Utilizing architecture-specific instructions (AVX, NEON) via Go assembly or intrinsics.
- **Batching:** Support for batched inference to improve throughput.
- **WASM Acceleration:** Optimizing for WebAssembly SIMD.
