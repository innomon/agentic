# ML — Local Embedded LLM Example

Run an LLM agent entirely on your local machine — no API keys, no network calls. This example uses the ML provider to load a GGUF-quantized model and run inference in pure Go.

## Overview

The ML provider embeds a GGUF model loader and transformer inference engine directly into Agentic. Models are loaded from disk and executed on the CPU, making it ideal for offline use, edge deployments, and privacy-sensitive workloads.

## Prerequisites

- Go 1.24+
- ~200 MB free disk space (for the 135M model)

## Setup

### 1. Download a GGUF model

```bash
mkdir -p examples/ml/models
wget -P examples/ml/models/ \
  https://huggingface.co/bartowski/SmolLM2-135M-Instruct-GGUF/resolve/main/SmolLM2-135M-Instruct-Q4_K_M.gguf
```

### 2. Build and run

```bash
go build -o agentic .
./agentic examples/ml/config.yaml console
```

You should see the console prompt. Type a message and the local model will respond:

```
User -> What is the capital of France?
```

## Configuration Reference

See the [ML provider section in AGENTS.md](../../AGENTS.md) and [docs/ML_SPECS.md](../../docs/ML_SPECS.md) for the full configuration reference.

### Key fields

| Field | Description |
|-------|-------------|
| `model_path` | Path to the `.gguf` model file (required) |
| `context_length` | Max context window (default: from GGUF metadata) |
| `threads` | CPU threads for inference (default: runtime.NumCPU) |
| `memory_budget_mb` | Memory limit in MB (default: unlimited) |
| `tokenizer_path` | Override tokenizer file (default: embedded in GGUF) |

## Supported Architectures

The ML provider supports the **LLaMA** architecture family, which covers:

- LLaMA 2 / LLaMA 3
- Mistral / Mixtral
- SmolLM / SmolLM2
- CodeLlama
- TinyLlama
- Other LLaMA-derived models

## Performance Notes

ML runs inference in pure Go on the CPU. It is best suited for small models (up to ~3B parameters). Larger models will work but may be slow. For production workloads with large models, consider the Gemini, OpenAI, or Ollama providers instead.
