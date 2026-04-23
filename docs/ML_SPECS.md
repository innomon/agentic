# ML Embedded Model Provider — Specification

## 1. Overview

The `ml` model provider adds a fully **embedded, zero-network LLM inference** capability to Agentic. Unlike the `gemini`, `openai`, and `ollama` providers—which call external APIs or servers—the `ml` provider loads a GGUF model file directly into the Go process and runs inference locally using a handcrafted, pure-Go transformer engine.

### Why pure-Go + GGUF?

| Concern | Approach |
|---------|----------|
| **No external dependencies** | No Python, no Ollama server, no Docker container. The model runs inside the same `agentic` binary. |
| **GGUF ubiquity** | GGUF is the dominant format for quantized open-weight LLMs. HuggingFace hosts 160k+ GGUF models. |
| **Pure Go implementation** | Built for maximum portability (WASM, ARM, embedded) without CGo or external C++ libraries. |
| **Hardware concurrency** | Leverages Go's goroutines for parallel matrix operations, providing efficient performance on multi-core CPUs. |
| **Single-binary deployment** | Fits Agentic's philosophy of config-driven, self-contained agents. |

---

## 2. YAML Configuration

```yaml
models:
  local-llama:
    provider: ml
    model_id: llama3.2-3b
    model_path: ./models/llama-3.2-3b-instruct-Q4_K_M.gguf
    default: true

    # Inference parameters (optional, overridable per-request via GenerateContentConfig)
    context_length: 4096  # Max context window.
    threads: 8            # CPU threads for parallel operations. 0 = auto-detect.

    # Resource limits (optional)
    memory_budget_mb: 2048 # Soft cap on model memory footprint.
```

### Minimal Configuration

```yaml
models:
  local:
    provider: ml
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
  ├── Config.Tools[]           → Tool declarations → formatted as system prompt
  ├── Config.Temperature       → Sampler temperature
  ├── Config.TopP              → Sampler nucleus sampling
  ├── Config.MaxOutputTokens   → Generation length limit
  └── Config.StopSequences     → Stop token detection
```

---

## 4. Supported Model Architectures

The provider supports popular GGUF architectures through specialized Go implementations:

| Architecture | GGUF `general.architecture` | Examples |
|---|---|---|
| **LLaMA** | `llama` | Llama 3.x, Mistral, Mixtral |
| **Granite Hybrid** | `granite`, `granitehybrid` | Granite 3.0 Hybrid (Mamba2 + Attention) |

Each architecture implements the `Arch` interface:
- **Transformer Forward Pass** — Attention, FFN, RMSNorm, RoPE, etc.
- **KV Cache Management** — Specialized state caching for Attention and SSM (Mamba) layers.

---

## 5. GGUF Loading Pipeline

1. **Metadata Parsing:** Uses `gpustack/gguf-parser-go` to read architecture, hyperparameters, and tokenizer data.
2. **Weight Loading:** Reads tensor data and dequantizes it from formats like `Q4_0`, `Q4_K`, `Q8_0` into `float32`.
3. **Lazy Initialization:** Weights are loaded on the first generation request to save memory during startup.

---

## 6. Tokenizer

Supports multiple schemes extracted directly from GGUF metadata:
- **SentencePiece (Unigram):** Used by LLaMA and similar models.
- **BPE:** Used by GPT-style models.
- **Byte Fallback:** Handles out-of-vocabulary characters via `<0xHH>` tokens.

---

## 7. Inference Engine

### Text Generation

The inference loop follows the standard autoregressive LLM pattern:
1. **Tokenize prompt** → input_ids.
2. **Prefill:** Forward pass on all input tokens to populate the KV cache.
3. **Decode loop:**
   - Forward pass on last token → logits.
   - Sample next token from logits (temperature, top-p, top-k).
   - Check stop conditions (EOS, stop sequences, max tokens).
   - Yield partial response (if streaming).
   - Append token and repeat.

### Parallel Operations

Core matrix operations (`MatVecMul`, `MatMul`) are parallelized using goroutines to utilize all available CPU cores, as configured by the `threads` parameter.

---

## 8. Tool Calling

The provider supports tool calling by:
1. Formatting available `Config.Tools` into the system prompt with a structured JSON template.
2. Parsing model output for JSON blocks matching the tool call schema.
3. Emitting `genai.FunctionCall` parts in the response.

---

## 9. Dependencies

| Dependency | Purpose | License |
|------------|---------|---------|
| `github.com/gpustack/gguf-parser-go` | GGUF file parsing | Apache-2.0 |
| `google.golang.org/adk` | Agentic Development Kit interfaces | Apache-2.0 |
| `google.golang.org/genai` | Generative AI types and schemas | Apache-2.0 |

---

## 10. Implementation Details

Refer to [ML Package Architecture](ml_arch.md) for a deep dive into the code structure and performance optimizations.
