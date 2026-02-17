# OpenClaw Gateway

An OpenClaw-compatible WebSocket gateway server built on the agentic framework.

## Overview

This example implements the [OpenClaw Gateway Protocol](https://github.com/openclaw/openclaw),
providing a JSON-over-WebSocket gateway with:

- **Device authentication** — Ed25519 signature-based device identity
- **Token/password auth** — Static token or password authentication
- **Tick/keepalive** — Server-emitted tick events for connection health
- **Sequence tracking** — Monotonic sequence numbers with gap detection
- **Method dispatch** — RPC-style request/response with `connect`, `send`, `config.*`, and `agent.*` methods
- **Idempotency** — Deduplication cache for side-effecting operations

## Usage

### Console Mode (Agent Only)

```bash
./agentic examples/openclaw/config.yaml console
```

### Gateway Server Mode

The gateway server runs as a WebSocket server (default: `ws://127.0.0.1:18789/ws`).

See the [gateway server documentation](../../docs/CLAW_GATE_SPECS.md) for full protocol details.

## Wire Protocol

### Frame Types

| Frame | Direction | Fields |
|-------|-----------|--------|
| Request | Client → Server | `type:"req"`, `id`, `method`, `params?` |
| Response | Server → Client | `type:"res"`, `id`, `ok`, `payload?`, `error?` |
| Event | Server → Client | `type:"event"`, `event`, `payload?`, `seq?`, `stateVersion?` |

### Connect Handshake

```json
{
  "type": "req",
  "id": "uuid-v4",
  "method": "connect",
  "params": {
    "minProtocol": 3,
    "maxProtocol": 3,
    "client": { "id": "my-client", "version": "1.0" },
    "role": "operator",
    "scopes": ["operator.admin"],
    "auth": { "token": "my-token" }
  }
}
```

### Core Methods

- `connect` — Handshake and authentication
- `send` — Broadcast messages to other clients
- `config.get` — Get server configuration
- `config.schema` — Get configuration schema
- `agent.*` — Forward to ADK agent for processing

## Architecture

```
Client (WebSocket) ──→ Gateway Server ──→ Method Dispatch
                                              ├── connect (auth, device tokens)
                                              ├── send (broadcast, dedup)
                                              ├── config.* (server config)
                                              └── agent.* (ADK agent invocation)
```
