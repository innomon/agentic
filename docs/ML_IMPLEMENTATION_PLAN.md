# ML Embedded Model Provider — Implementation Plan

## Prerequisites

Before starting, read `docs/ML_SPECS.md` for the full specification.

---

## Phase 0: Scaffolding & Registration

**Goal:** Wire up the `ml` provider in the registry so it's recognized in YAML configs, even before inference works.

### Tasks

1. **Create `internal/ml/` package directory**

2. **Define config struct** — `internal/ml/config.go`
   ```go
   type MLConfig struct {
       registry.ModelBase `yaml:",inline"`
       ModelPath      string `yaml:"model_path"`
       Backend        string `yaml:"backend"`
       BackendConfig  string `yaml:"backend_config"`
       ContextLength  int    `yaml:"context_length"`
       Threads        int    `yaml:"threads"`
       TokenizerPath  string `yaml:"tokenizer_path"`
       MemoryBudgetMB int    `yaml:"memory_budget_mb"`
   }

   func (c *MLConfig) Validate() error {
       if err := c.ModelBase.Validate(); err != nil { return err }
       if c.ModelPath == "" { return fmt.Errorf("model_path is required for ml provider") }
       return nil
   }
   ```

3. **Register provider** — `internal/ml/provider.go`
   ```go
   func init() {
       registry.RegisterModelProvider("ml", mlCreator)
   }

   func mlCreator(ctx context.Context, cfg *MLConfig) (model.LLM, error) {
       return NewMLModel(ctx, cfg)
   }
   ```

4. **Import side-effect** — Add `_ "github.com/innomon/agentic/pkg/ml"` to `main.go`

5. **Stub `MLModel`** — Implement `model.LLM` interface with `Name()` returning the model ID and `GenerateContent()` returning a "not yet implemented" error.

### Verification
```bash
go build -o agentic .
# Should compile without errors
```

### Deliverables
- `internal/ml/config.go`
- `internal/ml/provider.go`
- `internal/ml/model.go` (stub)
- Updated `main.go` import

---

## Phase 1: GGUF Parsing & Metadata Extraction

**Goal:** Load a GGUF file, extract metadata (architecture, hyperparameters, tokenizer info), and display it.

### Tasks

1. **Add `gpustack/gguf-parser-go` dependency**
   ```bash
   go get github.com/gpustack/gguf-parser-go
   ```

2. **Create `internal/ml/gguf.go`** — GGUF loading module
   - Parse GGUF file header and metadata.
   - Extract `general.architecture`, `general.name`.
   - Extract model hyperparameters: `*.embedding_length`, `*.block_count`, `*.attention.head_count`, `*.attention.head_count_kv`, `*.feed_forward_length`, `*.vocab_size`, `*.context_length`.
   - Extract quantization type (`general.file_type`).
   - Estimate memory usage.

3. **Create `internal/ml/gguf_test.go`** — Unit tests
   - Test with a small GGUF file (e.g., SmolLM2-135M Q4_K_M, ~80MB).
   - Verify metadata extraction matches expected values.

### Verification
```bash
go test ./pkg/ml/ -run TestGGUFParse -v
```

### Deliverables
- `internal/ml/gguf.go`
- `internal/ml/gguf_test.go`

---

## Phase 2: Tokenizer

**Goal:** Tokenize and detokenize text using vocabulary extracted from GGUF.

### Tasks

1. **Add `go-sentencepiece` dependency**
   ```bash
   go get github.com/eliben/go-sentencepiece
   ```

2. **Create `internal/ml/tokenizer.go`** — Tokenizer abstraction
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

5. **Create `internal/ml/tokenizer_test.go`**
   - Round-trip test: `Decode(Encode(text)) == text` for various strings.
   - Test special tokens (BOS, EOS, padding).

### Verification
```bash
go test ./pkg/ml/ -run TestTokenizer -v
```

### Deliverables
- `internal/ml/tokenizer.go`
- `internal/ml/tokenizer_sp.go` (SentencePiece impl)
- `internal/ml/tokenizer_bpe.go` (BPE impl)
- `internal/ml/tokenizer_test.go`

---

## Phase 3: Dequantization & Weight Loading

**Goal:** Read quantized GGUF tensors, dequantize to float32, and load into ML context.

### Tasks

1. **Add `ml` dependency**
   ```bash
   go get github.com/ml/ml
   ```

2. **Create `internal/ml/dequant.go`** — Dequantization routines
   - Implement dequantization for each supported quantization type:
     - `F32` — Direct copy.
     - `F16` — float16 → float32 conversion.
     - `Q8_0` — 8-bit block dequantization (block_size=32).
     - `Q4_0`, `Q4_1` — 4-bit block dequantization.
     - `Q4_K_M`, `Q5_K_M`, `Q6_K` — k-quant dequantization.
   - Each routine: `func dequantize(raw []byte, quantType, shape) ([]float32, error)`

3. **Create `internal/ml/weights.go`** — Weight loading into ML
   - Map GGUF tensor names to ML context variable paths.
   - Create ML tensors from dequantized weights.
   - Load into a `context.Context`.

4. **Create tests** — Verify dequantization correctness
   - Compare dequantized values against known reference outputs (e.g., from `gguf-parser-go` or Python's `gguf` library).
   - Verify tensor shapes match expected dimensions.

### Verification
```bash
go test ./pkg/ml/ -run TestDequant -v
go test ./pkg/ml/ -run TestWeightLoading -v
```

### Deliverables
- `internal/ml/dequant.go`
- `internal/ml/dequant_test.go`
- `internal/ml/weights.go`
- `internal/ml/weights_test.go`

---

## Phase 4: Transformer Graph — LLaMA Architecture

**Goal:** Implement the LLaMA transformer architecture in ML and run a single forward pass.

### Tasks

1. **Create `internal/ml/arch/` package**

2. **Create `internal/ml/arch/arch.go`** — Architecture interface
   ```go
   type Architecture interface {
       Name() string
       BuildGraph(ctx *context.Context, inputIDs *graph.Node, kvCache *KVCache) *graph.Node
       ChatTemplate() ChatTemplate
   }
   ```

3. **Create `internal/ml/arch/llama.go`** — LLaMA transformer
   - RMSNorm layer.
   - Rotary Position Embeddings (RoPE).
   - Grouped-Query Attention (GQA) with KV cache.
   - SwiGLU feed-forward network.
   - Token embedding + output projection (weight tying).
   - Full transformer stack with residual connections.

4. **Create `internal/ml/kvcache.go`** — KV cache management
   - Pre-allocate KV cache buffers based on context length and model dims.
   - Support cache update (append new KV entries) and reset.

5. **Create `internal/ml/arch/llama_test.go`**
   - Load a small LLaMA GGUF (e.g., SmolLM2-135M).
   - Run a single forward pass.
   - Verify output logits shape: `[1, vocab_size]`.

### Verification
```bash
go test ./pkg/ml/arch/ -run TestLlamaForward -v
```

### Deliverables
- `internal/ml/arch/arch.go`
- `internal/ml/arch/llama.go`
- `internal/ml/arch/llama_test.go`
- `internal/ml/kvcache.go`

---

## Phase 5: Sampler & Text Generation

**Goal:** Generate text autoregressively using the transformer and sampler.

### Tasks

1. **Create `internal/ml/sampler.go`** — Token sampling
   - Temperature scaling.
   - Top-K filtering.
   - Top-P (nucleus) sampling.
   - Repetition penalty.
   - Random sampling with `math/rand/v2`.

2. **Create `internal/ml/generate.go`** — Autoregressive generation loop
   ```go
   func (m *MLModel) Generate(ctx context.Context, prompt string, params GenerateParams) iter.Seq2[string, error]
   ```
   - Tokenize prompt.
   - Prefill: forward pass on all prompt tokens.
   - Decode loop: sample, check stop conditions, yield tokens.
   - Detokenize and yield text chunks.

3. **Create `internal/ml/generate_test.go`**
   - Generate a short completion from a small model.
   - Verify output is non-empty and coherent.
   - Test stop sequence detection.
   - Test max token limit.

### Verification
```bash
go test ./pkg/ml/ -run TestGenerate -v -timeout 120s
```

### Deliverables
- `internal/ml/sampler.go`
- `internal/ml/generate.go`
- `internal/ml/generate_test.go`

---

## Phase 6: ADK model.LLM Integration

**Goal:** Wire the generation engine into the ADK `model.LLM` interface so agents can use it.

### Tasks

1. **Complete `internal/ml/model.go`** — Full `model.LLM` implementation
   ```go
   type MLModel struct {
       name       string
       arch       arch.Architecture
       tokenizer  Tokenizer
       ctx        *context.Context   // ML context (weights)
       backend    backends.Backend
       cfg        *MLConfig
   }

   func (m *MLModel) Name() string { return m.name }

   func (m *MLModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
       // 1. Build prompt from SystemInstruction + Contents
       // 2. Format tool declarations if present
       // 3. Tokenize
       // 4. Generate (streaming or non-streaming)
       // 5. Parse output for tool calls if tools were provided
       // 6. Build LLMResponse with Content, UsageMetadata, FinishReason
   }
   ```

2. **Create `internal/ml/prompt.go`** — Prompt formatting
   - Convert `[]*genai.Content` history to a single prompt string using the architecture's chat template.
   - Handle system instructions.
   - Handle tool declarations (format as system prompt section).

3. **Create `internal/ml/toolparse.go`** — Tool call parsing
   - Detect tool call patterns in model output (architecture-specific).
   - Extract function name and JSON arguments.
   - Emit `genai.FunctionCall` parts.

4. **End-to-end test** — `internal/ml/model_test.go`
   - Create a `MLModel` from a small GGUF.
   - Call `GenerateContent` with a simple prompt.
   - Verify the response has valid Content, FinishReason, and UsageMetadata.
   - Test streaming mode.

### Verification
```bash
go test ./pkg/ml/ -run TestModelLLM -v -timeout 120s
```

### Deliverables
- `internal/ml/model.go` (completed)
- `internal/ml/prompt.go`
- `internal/ml/toolparse.go`
- `internal/ml/model_test.go`

---

## Phase 7: Additional Architectures

**Goal:** Extend support beyond LLaMA.

### Tasks (per architecture)

1. **Qwen2** — `internal/ml/arch/qwen2.go`
   - Similar to LLaMA but with different norm placement, attention bias, and embedding dimensions.
   - BPE tokenizer (not SentencePiece).

2. **Gemma2** — `internal/ml/arch/gemma2.go`
   - Can reference the existing [ml/gemma](https://github.com/ml/gemma) implementation.
   - Different normalization (pre+post norm), logit soft-capping.

3. **Phi-3** — `internal/ml/arch/phi3.go`
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
- `internal/ml/arch/qwen2.go`
- `internal/ml/arch/gemma2.go`
- `internal/ml/arch/phi3.go`

---

## Phase 8: Integration Testing & Example

**Goal:** Full integration with Agentic's config-driven agent system.

### Tasks

1. **Create example config** — `examples/ml/config.yaml`
   ```yaml
   models:
     local-llm:
       provider: ml
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

2. **Create example README** — `examples/ml/README.md`
   - Download instructions for a small GGUF model.
   - Run command: `./agentic examples/ml/config.yaml console`

3. **Integration test** — `internal/ml/integration_test.go`
   - Full round-trip: YAML config → registry → model creation → generate content.
   - Requires a test GGUF file (downloaded in CI or skipped).

4. **Update AGENTS.md** — Document the `ml` provider.

### Verification
```bash
# Download a small test model
wget -P examples/ml/models/ \
  https://huggingface.co/bartowski/SmolLM2-135M-Instruct-GGUF/resolve/main/SmolLM2-135M-Instruct-Q4_K_M.gguf

# Run
./agentic examples/ml/config.yaml console
```

### Deliverables
- `examples/ml/config.yaml`
- `examples/ml/README.md`
- `internal/ml/integration_test.go`
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
| ML pure-Go backend too slow for practical use | Default to XLA backend. Pure Go is a fallback for portability. Small models (135M–3B) are usable on pure Go. |
| GGUF dequantization bugs | Compare outputs against `llama.cpp` reference implementation. Use known-good test vectors. |
| ML API instability | Pin ML version in `go.mod`. ML has regular releases (latest v0.26.0). |
| Large GGUF files in CI | Use SmolLM2-135M (~80MB) for CI tests. Larger model tests run manually or in nightly CI. |
| Missing ML ops for some architectures | ML has comprehensive op coverage. File issues upstream for any missing ops. |
| Memory pressure from dequantized weights | 4-bit → float32 expands ~8x. A 4GB Q4 GGUF becomes ~32GB in memory. Document memory requirements. Consider future quantized-in-memory execution path. |

---

## Future Enhancements (Post-v1)

- **Quantized inference** — Keep weights in quantized form and use quantized matmul kernels (ML quantization support is planned).
- **Multi-modal** — Vision-language models (e.g., LLaVA, Qwen2-VL).
- **Split GGUF files** — Support multi-file GGUF models.
- **Model download** — Integrate `go-huggingface` for automatic GGUF download from HuggingFace.
- **Speculative decoding** — Use a small draft model for faster generation.
- **ONNX fallback** — For architectures not yet implemented in ML, load the ONNX version via `onnx-ml`.
- **Tensor parallelism** — Multi-GPU inference via XLA's distributed execution.
