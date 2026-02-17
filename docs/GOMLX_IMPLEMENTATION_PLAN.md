# GoMLX Embedded Model Provider — Implementation Plan

## Prerequisites

Before starting, read `docs/GOMLX_SPECS.md` for the full specification.

---

## Phase 0: Scaffolding & Registration

**Goal:** Wire up the `gomlx` provider in the registry so it's recognized in YAML configs, even before inference works.

### Tasks

1. **Create `internal/gomlx/` package directory**

2. **Define config struct** — `internal/gomlx/config.go`
   ```go
   type GoMLXConfig struct {
       registry.ModelBase `yaml:",inline"`
       ModelPath      string `yaml:"model_path"`
       Backend        string `yaml:"backend"`
       BackendConfig  string `yaml:"backend_config"`
       ContextLength  int    `yaml:"context_length"`
       Threads        int    `yaml:"threads"`
       TokenizerPath  string `yaml:"tokenizer_path"`
       MemoryBudgetMB int    `yaml:"memory_budget_mb"`
   }

   func (c *GoMLXConfig) Validate() error {
       if err := c.ModelBase.Validate(); err != nil { return err }
       if c.ModelPath == "" { return fmt.Errorf("model_path is required for gomlx provider") }
       return nil
   }
   ```

3. **Register provider** — `internal/gomlx/provider.go`
   ```go
   func init() {
       registry.RegisterModelProvider("gomlx", gomlxCreator)
   }

   func gomlxCreator(ctx context.Context, cfg *GoMLXConfig) (model.LLM, error) {
       return NewGoMLXModel(ctx, cfg)
   }
   ```

4. **Import side-effect** — Add `_ "github.com/innomon/agentic/internal/gomlx"` to `main.go`

5. **Stub `GoMLXModel`** — Implement `model.LLM` interface with `Name()` returning the model ID and `GenerateContent()` returning a "not yet implemented" error.

### Verification
```bash
go build -o agentic .
# Should compile without errors
```

### Deliverables
- `internal/gomlx/config.go`
- `internal/gomlx/provider.go`
- `internal/gomlx/model.go` (stub)
- Updated `main.go` import

---

## Phase 1: GGUF Parsing & Metadata Extraction

**Goal:** Load a GGUF file, extract metadata (architecture, hyperparameters, tokenizer info), and display it.

### Tasks

1. **Add `gpustack/gguf-parser-go` dependency**
   ```bash
   go get github.com/gpustack/gguf-parser-go
   ```

2. **Create `internal/gomlx/gguf.go`** — GGUF loading module
   - Parse GGUF file header and metadata.
   - Extract `general.architecture`, `general.name`.
   - Extract model hyperparameters: `*.embedding_length`, `*.block_count`, `*.attention.head_count`, `*.attention.head_count_kv`, `*.feed_forward_length`, `*.vocab_size`, `*.context_length`.
   - Extract quantization type (`general.file_type`).
   - Estimate memory usage.

3. **Create `internal/gomlx/gguf_test.go`** — Unit tests
   - Test with a small GGUF file (e.g., SmolLM2-135M Q4_K_M, ~80MB).
   - Verify metadata extraction matches expected values.

### Verification
```bash
go test ./internal/gomlx/ -run TestGGUFParse -v
```

### Deliverables
- `internal/gomlx/gguf.go`
- `internal/gomlx/gguf_test.go`

---

## Phase 2: Tokenizer

**Goal:** Tokenize and detokenize text using vocabulary extracted from GGUF.

### Tasks

1. **Add `go-sentencepiece` dependency**
   ```bash
   go get github.com/eliben/go-sentencepiece
   ```

2. **Create `internal/gomlx/tokenizer.go`** — Tokenizer abstraction
   ```go
   type Tokenizer interface {
       Encode(text string) ([]int32, error)
       Decode(tokens []int32) (string, error)
       VocabSize() int
       EOSToken() int32
       BOSToken() int32
   }
   ```

3. **Implement SentencePiece tokenizer** — For LLaMA/Gemma-family models
   - Extract tokenizer data from GGUF metadata (`tokenizer.ggml.tokens`, `tokenizer.ggml.scores`).
   - Reconstruct a SentencePiece model from GGUF vocab data.
   - Alternatively, load from external `.model` file if `tokenizer_path` is set.

4. **Implement BPE tokenizer** — For GPT-2/Qwen-family models
   - Extract BPE merges from `tokenizer.ggml.merges`.
   - Implement byte-pair encoding/decoding.

5. **Create `internal/gomlx/tokenizer_test.go`**
   - Round-trip test: `Decode(Encode(text)) == text` for various strings.
   - Test special tokens (BOS, EOS, padding).

### Verification
```bash
go test ./internal/gomlx/ -run TestTokenizer -v
```

### Deliverables
- `internal/gomlx/tokenizer.go`
- `internal/gomlx/tokenizer_sp.go` (SentencePiece impl)
- `internal/gomlx/tokenizer_bpe.go` (BPE impl)
- `internal/gomlx/tokenizer_test.go`

---

## Phase 3: Dequantization & Weight Loading

**Goal:** Read quantized GGUF tensors, dequantize to float32, and load into GoMLX context.

### Tasks

1. **Add `gomlx` dependency**
   ```bash
   go get github.com/gomlx/gomlx
   ```

2. **Create `internal/gomlx/dequant.go`** — Dequantization routines
   - Implement dequantization for each supported quantization type:
     - `F32` — Direct copy.
     - `F16` — float16 → float32 conversion.
     - `Q8_0` — 8-bit block dequantization (block_size=32).
     - `Q4_0`, `Q4_1` — 4-bit block dequantization.
     - `Q4_K_M`, `Q5_K_M`, `Q6_K` — k-quant dequantization.
   - Each routine: `func dequantize(raw []byte, quantType, shape) ([]float32, error)`

3. **Create `internal/gomlx/weights.go`** — Weight loading into GoMLX
   - Map GGUF tensor names to GoMLX context variable paths.
   - Create GoMLX tensors from dequantized weights.
   - Load into a `context.Context`.

4. **Create tests** — Verify dequantization correctness
   - Compare dequantized values against known reference outputs (e.g., from `gguf-parser-go` or Python's `gguf` library).
   - Verify tensor shapes match expected dimensions.

### Verification
```bash
go test ./internal/gomlx/ -run TestDequant -v
go test ./internal/gomlx/ -run TestWeightLoading -v
```

### Deliverables
- `internal/gomlx/dequant.go`
- `internal/gomlx/dequant_test.go`
- `internal/gomlx/weights.go`
- `internal/gomlx/weights_test.go`

---

## Phase 4: Transformer Graph — LLaMA Architecture

**Goal:** Implement the LLaMA transformer architecture in GoMLX and run a single forward pass.

### Tasks

1. **Create `internal/gomlx/arch/` package**

2. **Create `internal/gomlx/arch/arch.go`** — Architecture interface
   ```go
   type Architecture interface {
       Name() string
       BuildGraph(ctx *context.Context, inputIDs *graph.Node, kvCache *KVCache) *graph.Node
       ChatTemplate() ChatTemplate
   }
   ```

3. **Create `internal/gomlx/arch/llama.go`** — LLaMA transformer
   - RMSNorm layer.
   - Rotary Position Embeddings (RoPE).
   - Grouped-Query Attention (GQA) with KV cache.
   - SwiGLU feed-forward network.
   - Token embedding + output projection (weight tying).
   - Full transformer stack with residual connections.

4. **Create `internal/gomlx/kvcache.go`** — KV cache management
   - Pre-allocate KV cache buffers based on context length and model dims.
   - Support cache update (append new KV entries) and reset.

5. **Create `internal/gomlx/arch/llama_test.go`**
   - Load a small LLaMA GGUF (e.g., SmolLM2-135M).
   - Run a single forward pass.
   - Verify output logits shape: `[1, vocab_size]`.

### Verification
```bash
go test ./internal/gomlx/arch/ -run TestLlamaForward -v
```

### Deliverables
- `internal/gomlx/arch/arch.go`
- `internal/gomlx/arch/llama.go`
- `internal/gomlx/arch/llama_test.go`
- `internal/gomlx/kvcache.go`

---

## Phase 5: Sampler & Text Generation

**Goal:** Generate text autoregressively using the transformer and sampler.

### Tasks

1. **Create `internal/gomlx/sampler.go`** — Token sampling
   - Temperature scaling.
   - Top-K filtering.
   - Top-P (nucleus) sampling.
   - Repetition penalty.
   - Random sampling with `math/rand/v2`.

2. **Create `internal/gomlx/generate.go`** — Autoregressive generation loop
   ```go
   func (m *GoMLXModel) Generate(ctx context.Context, prompt string, params GenerateParams) iter.Seq2[string, error]
   ```
   - Tokenize prompt.
   - Prefill: forward pass on all prompt tokens.
   - Decode loop: sample, check stop conditions, yield tokens.
   - Detokenize and yield text chunks.

3. **Create `internal/gomlx/generate_test.go`**
   - Generate a short completion from a small model.
   - Verify output is non-empty and coherent.
   - Test stop sequence detection.
   - Test max token limit.

### Verification
```bash
go test ./internal/gomlx/ -run TestGenerate -v -timeout 120s
```

### Deliverables
- `internal/gomlx/sampler.go`
- `internal/gomlx/generate.go`
- `internal/gomlx/generate_test.go`

---

## Phase 6: ADK model.LLM Integration

**Goal:** Wire the generation engine into the ADK `model.LLM` interface so agents can use it.

### Tasks

1. **Complete `internal/gomlx/model.go`** — Full `model.LLM` implementation
   ```go
   type GoMLXModel struct {
       name       string
       arch       arch.Architecture
       tokenizer  Tokenizer
       ctx        *context.Context   // GoMLX context (weights)
       backend    backends.Backend
       cfg        *GoMLXConfig
   }

   func (m *GoMLXModel) Name() string { return m.name }

   func (m *GoMLXModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
       // 1. Build prompt from SystemInstruction + Contents
       // 2. Format tool declarations if present
       // 3. Tokenize
       // 4. Generate (streaming or non-streaming)
       // 5. Parse output for tool calls if tools were provided
       // 6. Build LLMResponse with Content, UsageMetadata, FinishReason
   }
   ```

2. **Create `internal/gomlx/prompt.go`** — Prompt formatting
   - Convert `[]*genai.Content` history to a single prompt string using the architecture's chat template.
   - Handle system instructions.
   - Handle tool declarations (format as system prompt section).

3. **Create `internal/gomlx/toolparse.go`** — Tool call parsing
   - Detect tool call patterns in model output (architecture-specific).
   - Extract function name and JSON arguments.
   - Emit `genai.FunctionCall` parts.

4. **End-to-end test** — `internal/gomlx/model_test.go`
   - Create a `GoMLXModel` from a small GGUF.
   - Call `GenerateContent` with a simple prompt.
   - Verify the response has valid Content, FinishReason, and UsageMetadata.
   - Test streaming mode.

### Verification
```bash
go test ./internal/gomlx/ -run TestModelLLM -v -timeout 120s
```

### Deliverables
- `internal/gomlx/model.go` (completed)
- `internal/gomlx/prompt.go`
- `internal/gomlx/toolparse.go`
- `internal/gomlx/model_test.go`

---

## Phase 7: Additional Architectures

**Goal:** Extend support beyond LLaMA.

### Tasks (per architecture)

1. **Qwen2** — `internal/gomlx/arch/qwen2.go`
   - Similar to LLaMA but with different norm placement, attention bias, and embedding dimensions.
   - BPE tokenizer (not SentencePiece).

2. **Gemma2** — `internal/gomlx/arch/gemma2.go`
   - Can reference the existing [gomlx/gemma](https://github.com/gomlx/gemma) implementation.
   - Different normalization (pre+post norm), logit soft-capping.

3. **Phi-3** — `internal/gomlx/arch/phi3.go`
   - BlockSparse attention variant, RoPE differences.

4. **Architecture registry** — Auto-detect from GGUF and dispatch
   ```go
   var archRegistry = map[string]func(params ArchParams) Architecture{
       "llama":  NewLlamaArch,
       "qwen2":  NewQwen2Arch,
       "gemma":  NewGemmaArch,
       "gemma2": NewGemma2Arch,
       "phi3":   NewPhi3Arch,
   }
   ```

### Deliverables
- `internal/gomlx/arch/qwen2.go`
- `internal/gomlx/arch/gemma2.go`
- `internal/gomlx/arch/phi3.go`

---

## Phase 8: Integration Testing & Example

**Goal:** Full integration with Agentic's config-driven agent system.

### Tasks

1. **Create example config** — `examples/gomlx/config.yaml`
   ```yaml
   models:
     local-llm:
       provider: gomlx
       model_id: smollm2-135m
       model_path: ./models/SmolLM2-135M-Instruct-Q4_K_M.gguf
       backend: go
       default: true

   agents:
     LocalAgent:
       model: local-llm
       description: A local assistant
       instruction: You are a helpful assistant running locally.

   root_agent: LocalAgent
   ```

2. **Create example README** — `examples/gomlx/README.md`
   - Download instructions for a small GGUF model.
   - Run command: `./agentic examples/gomlx/config.yaml console`

3. **Integration test** — `internal/gomlx/integration_test.go`
   - Full round-trip: YAML config → registry → model creation → generate content.
   - Requires a test GGUF file (downloaded in CI or skipped).

4. **Update AGENTS.md** — Document the `gomlx` provider.

### Verification
```bash
# Download a small test model
wget -P examples/gomlx/models/ \
  https://huggingface.co/bartowski/SmolLM2-135M-Instruct-GGUF/resolve/main/SmolLM2-135M-Instruct-Q4_K_M.gguf

# Run
./agentic examples/gomlx/config.yaml console
```

### Deliverables
- `examples/gomlx/config.yaml`
- `examples/gomlx/README.md`
- `internal/gomlx/integration_test.go`
- Updated `AGENTS.md`

---

## Phase Summary & Dependencies

```
Phase 0: Scaffolding          ─────┐
Phase 1: GGUF Parsing         ─────┤
Phase 2: Tokenizer            ─────┤──▶ Phase 4: Transformer ──▶ Phase 5: Generation ──▶ Phase 6: ADK Integration
Phase 3: Dequant & Weights    ─────┘                                                          │
                                                                                               ▼
                                                                              Phase 7: More Architectures
                                                                                               │
                                                                                               ▼
                                                                              Phase 8: Integration & Example
```

Phases 0–3 can proceed in parallel. Phase 4 depends on all of 1–3. Phases 5–8 are sequential.

---

## Estimated Effort

| Phase | Effort | Complexity |
|-------|--------|------------|
| 0 — Scaffolding | 1 day | Low |
| 1 — GGUF Parsing | 2 days | Low |
| 2 — Tokenizer | 3 days | Medium |
| 3 — Dequantization | 3 days | Medium–High |
| 4 — LLaMA Transformer | 5 days | High |
| 5 — Sampler & Generation | 3 days | Medium |
| 6 — ADK Integration | 3 days | Medium |
| 7 — Additional Architectures | 3 days each | Medium |
| 8 — Integration & Example | 2 days | Low |
| **Total (through Phase 8, 4 archs)** | **~31 days** | |

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| GoMLX pure-Go backend too slow for practical use | Default to XLA backend. Pure Go is a fallback for portability. Small models (135M–3B) are usable on pure Go. |
| GGUF dequantization bugs | Compare outputs against `llama.cpp` reference implementation. Use known-good test vectors. |
| GoMLX API instability | Pin GoMLX version in `go.mod`. GoMLX has regular releases (latest v0.26.0). |
| Large GGUF files in CI | Use SmolLM2-135M (~80MB) for CI tests. Larger model tests run manually or in nightly CI. |
| Missing GoMLX ops for some architectures | GoMLX has comprehensive op coverage. File issues upstream for any missing ops. |
| Memory pressure from dequantized weights | 4-bit → float32 expands ~8x. A 4GB Q4 GGUF becomes ~32GB in memory. Document memory requirements. Consider future quantized-in-memory execution path. |

---

## Future Enhancements (Post-v1)

- **Quantized inference** — Keep weights in quantized form and use quantized matmul kernels (GoMLX quantization support is planned).
- **Multi-modal** — Vision-language models (e.g., LLaVA, Qwen2-VL).
- **Split GGUF files** — Support multi-file GGUF models.
- **Model download** — Integrate `go-huggingface` for automatic GGUF download from HuggingFace.
- **Speculative decoding** — Use a small draft model for faster generation.
- **ONNX fallback** — For architectures not yet implemented in GoMLX, load the ONNX version via `onnx-gomlx`.
- **Tensor parallelism** — Multi-GPU inference via XLA's distributed execution.
