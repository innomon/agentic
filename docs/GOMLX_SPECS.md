# GoMLX Embedded Model Provider — Specification

## 1. Overview

The `gomlx` model provider adds a fully **embedded, zero-network LLM inference** capability to Agentic. Unlike the `gemini`, `openai`, and `ollama` providers—which call external APIs or servers—the `gomlx` provider loads a GGUF model file directly into the Go process and runs inference locally using the [GoMLX](https://github.com/gomlx/gomlx) ML framework with hardware acceleration (XLA/CPU/GPU) or its portable pure-Go backend.

### Why GoMLX + GGUF?

| Concern | Approach |
|---------|----------|
| **No external dependencies** | No Python, no Ollama server, no Docker container. The model runs inside the same `agentic` binary. |
| **GGUF ubiquity** | GGUF is the dominant format for quantized open-weight LLMs. HuggingFace hosts 160k+ GGUF models. |
| **GoMLX maturity** | GoMLX already has a working [Gemma2 implementation](https://github.com/gomlx/gemma) with sampler, tokenizer, and checkpoint loading. It also supports [ONNX model conversion](https://github.com/gomlx/onnx-gomlx). |
| **Hardware acceleration** | GoMLX's XLA backend gives near-Jax performance on CPUs and NVIDIA GPUs (via PJRT). The pure-Go `simplego` backend provides maximum portability (WASM, ARM, embedded). |
| **Single-binary deployment** | Fits Agentic's philosophy of config-driven, self-contained agents. |

### Key Distinction: GoMLX vs llama.cpp Wrappers

Projects like [Yzma](https://github.com/hybridgroup/yzma) wrap `llama.cpp` (a C++ library) via FFI. GoMLX is a **native Go ML framework** — no CGo, no shared `.so` files. The tradeoff is that GoMLX requires model architecture code in Go (not just weight loading), but it provides a cleaner Go-native integration with full control over the computation graph.

---

## 2. YAML Configuration

```yaml
models:
  local-llama:
    provider: gomlx
    model_id: llama3.2-3b
    model_path: ./models/llama-3.2-3b-instruct-Q4_K_M.gguf
    default: true

    # Backend selection (optional)
    backend: xla          # "xla" (default, fastest) | "go" (pure Go, most portable)
    backend_config: cpu   # XLA: "cpu", "cuda", "tpu". Ignored for "go" backend.

    # Inference parameters (optional, overridable per-request via GenerateContentConfig)
    context_length: 4096  # Max context window. Defaults to model's trained max.
    threads: 0            # CPU threads. 0 = auto-detect.

    # Tokenizer (optional, auto-detected from GGUF metadata)
    tokenizer_path: ""    # Override: path to SentencePiece .model file

    # Resource limits (optional)
    memory_budget_mb: 0   # 0 = no limit. Soft cap on model memory footprint.
```

### Minimal Configuration

```yaml
models:
  local:
    provider: gomlx
    model_id: smollm2
    model_path: ./models/SmolLM2-135M-Instruct-Q4_K_M.gguf
```

---

## 3. ADK Interface Mapping

The provider implements `model.LLM` from `google.golang.org/adk/model`:

```go
type LLM interface {
    Name() string
    GenerateContent(ctx context.Context, req *LLMRequest, stream bool) iter.Seq2[*LLMResponse, error]
}
```

### Request Flow

```
LLMRequest
  ├── Config.SystemInstruction → Prepended to prompt
  ├── Contents[]               → Chat history → tokenized
  ├── Config.Tools[]           → Tool declarations → formatted as system prompt / special tokens
  ├── Config.Temperature       → Sampler temperature
  ├── Config.TopP              → Sampler nucleus sampling
  ├── Config.MaxOutputTokens   → Generation length limit
  └── Config.StopSequences     → Stop token detection
```

### Response Mapping

| LLMResponse field | Source |
|-------------------|--------|
| `Content` | Decoded token sequence → `genai.Content{Role: "model", Parts: [TextPart]}` |
| `FinishReason` | `genai.FinishReasonStop` (EOS token or stop sequence), `genai.FinishReasonMaxTokens` (length limit) |
| `UsageMetadata` | Token counts from tokenizer (prompt + completion) |
| `Partial` | `true` for intermediate streaming chunks, `false` for final |

### Tool Calling

GGUF models that support tool calling (e.g., Llama 3.x instruct, Qwen2.5) encode tool calls as structured text in their output. The provider will:

1. Format `Config.Tools` function declarations into the model's expected tool-call prompt format (e.g., Llama-style `<|python_tag|>` or JSON-in-text).
2. Parse model output for tool-call patterns and emit `genai.FunctionCall` parts.
3. Accept `FunctionResponse` parts in subsequent turns.

Architecture-specific tool-call templates will be configurable and extensible.

---

## 4. Supported Model Architectures

Initial support targets the most popular GGUF architectures:

| Architecture | GGUF `general.architecture` | Examples |
|---|---|---|
| **LLaMA** | `llama` | Llama 3.x, CodeLlama, Mistral, Mixtral |
| **Qwen2** | `qwen2` | Qwen2.5, QwQ |
| **Gemma** | `gemma`, `gemma2` | Gemma 2, Gemma 3 |
| **Phi** | `phi3` | Phi-3, Phi-3.5 |

Each architecture requires a Go implementation of:
- **Transformer graph** — Attention, FFN, RMSNorm, RoPE, etc. via GoMLX ops.
- **Tokenizer adapter** — SentencePiece or BPE (extracted from GGUF metadata or external file).
- **Chat template** — Prompt formatting for instruct/chat models.

### Architecture Detection

The GGUF file's `general.architecture` metadata field is read at load time to select the correct transformer implementation. Unknown architectures produce a clear error with the detected value.

---

## 5. GGUF Loading Pipeline

```
┌─────────────┐     ┌──────────────┐     ┌───────────────┐     ┌──────────────┐
│  GGUF File  │────▶│  Parse GGUF  │────▶│  Dequantize   │────▶│  GoMLX       │
│  (on disk)  │     │  (metadata + │     │  Tensors to   │     │  Context     │
│             │     │   tensors)   │     │  float16/32   │     │  (variables) │
└─────────────┘     └──────────────┘     └───────────────┘     └──────────────┘
                          │                                          │
                          ▼                                          ▼
                    ┌──────────────┐                          ┌──────────────┐
                    │  Architecture│                          │  Build       │
                    │  Detection   │─────────────────────────▶│  Computation │
                    │  + Tokenizer │                          │  Graph       │
                    └──────────────┘                          └──────────────┘
```

### GGUF Parsing

Use the [`gpustack/gguf-parser-go`](https://github.com/gpustack/gguf-parser-go) library for:
- Reading GGUF file headers and metadata (architecture, tokenizer vocab, hyperparameters).
- Extracting tensor names, shapes, and quantization types.
- Memory estimation before loading.

### Quantization Handling

GGUF tensors are stored in quantized formats (Q4_0, Q4_K_M, Q8_0, etc.). GoMLX operates on float tensors. The provider will:

1. Read raw quantized bytes from the GGUF file.
2. Dequantize to `float32` (or `float16` on GPU) using Go dequantization routines.
3. Load dequantized weights into GoMLX `context.Context` variables.

Supported quantization types (initial):
- `F32`, `F16` — Direct copy
- `Q8_0` — 8-bit block quantization
- `Q4_0`, `Q4_1` — 4-bit block quantization
- `Q4_K_M`, `Q5_K_M`, `Q6_K` — k-quant formats (most popular on HuggingFace)

---

## 6. Tokenizer

GGUF files embed tokenizer data in their metadata:
- `tokenizer.ggml.model` — Tokenizer type (`llama` = SentencePiece, `gpt2` = BPE)
- `tokenizer.ggml.tokens` — Vocabulary tokens
- `tokenizer.ggml.scores` — Token scores/probabilities
- `tokenizer.ggml.merges` — BPE merge rules (for GPT-2-style tokenizers)

The provider will:
1. Extract tokenizer metadata from the GGUF file.
2. Use [`go-sentencepiece`](https://github.com/eliben/go-sentencepiece) for SentencePiece models.
3. Implement a lightweight BPE tokenizer for GPT-2-style models.
4. Allow override via `tokenizer_path` config field.

---

## 7. Inference Engine

### Text Generation

The inference loop follows the standard autoregressive LLM pattern:

```
1. Tokenize prompt → input_ids
2. Prefill: Forward pass on all input tokens (KV cache populated)
3. Decode loop:
   a. Forward pass on last token → logits
   b. Sample next token from logits (temperature, top-p, top-k)
   c. Check stop conditions (EOS, stop sequences, max tokens)
   d. If streaming: yield partial response
   e. Append token, repeat from (a)
4. Detokenize output tokens → text
5. Yield final response
```

### KV Cache

A per-session KV cache stores attention key/value states to avoid recomputation. The cache is allocated based on `context_length` and the model's hidden dimensions.

### Sampling

The sampler supports:
- **Temperature** — Scales logits before softmax. Default: 1.0.
- **Top-P (nucleus)** — Samples from the smallest set of tokens whose cumulative probability ≥ p.
- **Top-K** — Samples from the top K most probable tokens.
- **Repetition penalty** — Penalizes recently generated tokens.
- **Stop sequences** — Detects and stops on configured text sequences.

---

## 8. Streaming

When `stream=true`, the provider yields `LLMResponse` with `Partial=true` after each generated token (or small batch of tokens). The final response has `Partial=false` and includes complete `UsageMetadata`.

Streaming granularity is per-token by default. A future optimization may batch tokens for reduced overhead.

---

## 9. Thread Safety & Concurrency

- **Model weights** are immutable after loading and shared across goroutines.
- **KV cache** is per-inference and not shared.
- **GoMLX backend** handles its own concurrency (XLA uses internal thread pools).
- Multiple concurrent `GenerateContent` calls are safe but will compete for compute resources.

---

## 10. Error Handling

| Condition | Behavior |
|-----------|----------|
| GGUF file not found | Return error at model creation time |
| Unsupported architecture | Return error with detected `general.architecture` value |
| Unsupported quantization type | Return error listing the unsupported type |
| Out of memory | Return error with memory estimate vs available |
| Context length exceeded | Truncate oldest messages (or return error, configurable) |
| Backend not available | Fall back to `go` backend with warning log |

---

## 11. Limitations & Non-Goals (v1)

- **No training / fine-tuning** — Inference only.
- **No multi-modal** — Text-in, text-out. Vision/audio models are out of scope for v1.
- **No speculative decoding** — Standard autoregressive decoding only.
- **No tensor parallelism** — Single-device inference only.
- **No GGUF split files** — Single-file GGUF only (no `*-00001-of-00003.gguf`).
- **Architecture coverage** — Only the architectures listed in §4. New architectures require Go code.

---

## 12. Dependencies

| Dependency | Purpose | License |
|------------|---------|---------|
| `github.com/gomlx/gomlx` | ML framework (computation graph, backends) | Apache-2.0 |
| `github.com/gomlx/gomlx/backends/simplego` | Pure Go backend (fallback) | Apache-2.0 |
| `github.com/gpustack/gguf-parser-go` | GGUF file parsing | Apache-2.0 |
| `github.com/eliben/go-sentencepiece` | SentencePiece tokenizer | Apache-2.0 |
| `github.com/gomlx/go-huggingface` | Model download from HuggingFace (optional) | Apache-2.0 |

All dependencies are pure Go (no CGo) when using the `simplego` backend. The XLA backend requires the PJRT plugin (auto-installed by GoMLX).

---

## 13. Environment Variables

| Variable | Description |
|----------|-------------|
| `GOMLX_BACKEND` | Override backend selection (e.g., `go`, `xla:cpu`, `xla:cuda`) |
| `GOMLX_NO_AUTO_INSTALL` | Set to `1` to prevent auto-installation of XLA PJRT plugins |

---

## 14. Example: Full Agent Configuration

```yaml
models:
  local-llama:
    provider: gomlx
    model_id: llama3.2-3b-instruct
    model_path: ./models/llama-3.2-3b-instruct-Q4_K_M.gguf
    backend: xla
    backend_config: cpu
    context_length: 4096
    default: true

agents:
  LocalAssistant:
    model: local-llama
    description: A helpful assistant powered by a local Llama model
    instruction: |
      You are a helpful assistant running entirely on the user's machine.
      Be concise and accurate.
    tools: [search_files]

tools:
  search_files:
    description: Search for files by name
    parameters:
      pattern: {type: string, required: true}
```
