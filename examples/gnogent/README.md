# Gnogent — Deterministic Stateful Agent

A stateful AI agent that uses GnoVM for deterministic personality logic and delegates to an LLM sub-agent for natural language generation.

## Architecture

```
User Input
    │
    ▼
┌───────────────────┐
│  Gnogent (GnoVM)  │  ← Deterministic: mood, friendship, preferences
│  type: gnogent-   │
│  deterministic    │
└────────┬──────────┘
         │ sub-agent call
         ▼
┌───────────────────┐
│    ChatAgent      │  ← LLM: generates response using GnoVM context
│    (Gemini)       │
└───────────────────┘
```

## Prerequisites

- PostgreSQL running on `localhost:5432`
- Database `gnogent` created
- `GOOGLE_API_KEY` environment variable set

## Usage

```bash
# From the project root
./agentic examples/gnogent/config.yaml console
```

## How It Works

1. **Thaw** — The Go wrapper restores GnoVM state from Postgres for the current user.
2. **Pulse** — `SyncState(userInput, timestamp)` evolves mood and personality deterministically.
3. **Context** — `GetSystemContext()` exports the internal state as a system prompt fragment.
4. **Delegate** — The root agent calls `ChatAgent` (LLM sub-agent) which uses the context to generate a reply.
5. **Freeze** — `AddTurn()` archives the exchange; the VM snapshot is saved back to Postgres.
