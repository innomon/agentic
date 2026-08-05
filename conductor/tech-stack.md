# Technology Stack

## Core Technologies
- **Language**: Go 1.24+
- **Framework**: ADK-Go (Google Agent Development Kit for Go)
- **Configuration**: YAML (`pkg/config`, `config/config.yaml`)
- **LLM Providers**: Gemini (default: `gemini-2.0-flash`), OpenAI, Ollama, local GGUF
- **Storage/DB**: GORM (PostgreSQL / SQLite) for persistent user/session memory
- **WASM Runtime**: wazero
- **Logic Engine**: Prolog (`ichiban/prolog`)
