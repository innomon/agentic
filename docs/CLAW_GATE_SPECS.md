# OpenClaw Gateway — Go Reimplementation Specification

Purpose
- Implement an Open Claw compatible ADK go agent as per the framework of this project.
- use go-sdk for ADK as per this project's framework 
- Reuse of existing registered agent is preferred. Create a new custom agent only if the registered agents in this project do not fit the need.
- finally create a new dir under the [example](../examples/) with `config.yaml` and `README.md` 
 
Scope
- Gateway protocol (JSON-over-WebSocket): frames, handshake, request/response/event semantics.
- Client behaviors: connection/backoff, connect handshake, auth (token/password/device identity), pending requests, expectFinal semantics, tick/keepalive, sequence/gap tracking.
- Server behaviors: connection lifecycle, method dispatch, channel & subscription management, idempotency, de-duplication, maintenance timers, restart/update hooks.
- Security: TLS fingerprint verification, device signature scheme, token storage and rotation, permissions and scopes.
- Operational details: payload limits, timeouts, max payload, error/close codes.
- Tests and e2e harness design.
- Suggested Go package layout, libraries, and milestones.

High-level overview
- Transport: WebSocket with JSON text frames.
- First client action: call the `connect` method (request frame). Server may first send an event `connect.challenge` to request a device nonce.
- Frame types:
  - Request frame (client -> server): { "type": "req", "id": string, "method": string, "params": object? }
  - Response frame (server -> client): { "type": "res", "id": string, "ok": boolean, "payload"?: object, "error"?: { message: string, code?: string } }
  - Event frame (server -> client): { "type": "event", "event": string, "payload": object?, "seq"?: number, "stateVersion"?: string }
- Validation: all frames must be validated against a JSON schema. Invalid frames should be handled gracefully (reject request or ignore event) and optionally close connection for protocol violations.

Frame schema (canonical examples)
- Request:
  {
    "type": "req",
    "id": "uuid-v4",
    "method": "connect",
    "params": { ... }
  }
- Response:
  {
    "type": "res",
    "id": "uuid-v4",
    "ok": true,
    "payload": { ... }
  }
  or
  {
    "type": "res",
    "id": "uuid-v4",
    "ok": false,
    "error": { "message": "reason", "code": "ERR_CODE" }
  }
- Event:
  {
    "type": "event",
    "event": "connect.challenge",
    "payload": { "nonce": "..." },
    "seq": 123,
    "stateVersion": "..."
  }

Protocol details
- Handshake:
  - Client must send a `connect` request as the first request; server responds with Hello/HelloOk (the `res` for connect) or may first emit `connect.challenge` event to request client-signed device auth.
  - `connect` params contain protocol version min/max, client metadata, role, scopes, auth (token/password), device payload (publicKey/id/signature/signedAt/nonce).
  - Server validates protocol version, auth, device signature, and returns HelloOk which may include `auth.deviceToken` for subsequent connections. Server also returns a `policy` object (e.g., tickIntervalMs).
- Auth modes:
  - Token auth: client provides token string (OPENCLAW_GATEWAY_TOKEN or CLI `--token`). Server verifies token.
  - Password auth: server checks provided password if gateway configured to allow password mode.
  - Device identity: client signs a device auth payload with a private key (Ed25519 recommended); server verifies signature against provided public key. If server accepts device identity, it may issue a per-device deviceToken to be persisted on the client device for future use.
  - Device token storage: server must return deviceToken and scopes; client stores per-device-id+role token persistently.
- Device signature:
  - Use Ed25519 signatures.
  - Canonical payload: includes deviceId, clientId, clientMode, role, scopes, signedAtMs, token (nullable), nonce (optional).
  - Signatures expressed as base64url strings.
- TLS fingerprint:
  - Client may supply expected TLS fingerprint to validate the gateway's certificate (fingerprint256, normalized).
  - Server must present certificate that clients can validate; clients will optionally verify fingerprint; mismatch triggers policy error and close code 1008 or application-defined close code.
- Tick/keepalive:
  - Server periodically emits `tick` events. Clients track `lastTick`; if gap exceeds tickIntervalMs * 2, client closes with tick timeout.
  - Server supplies tickIntervalMs via HelloOk.policy.tickIntervalMs (can default to 30_000 ms).
- Sequence & gap detection:
  - Server events include a numeric `seq`. Client tracks lastSeq; if a received seq > lastSeq + 1, client raises onGap.
- Request lifecycle on client:
  - Client keeps a pending map keyed by id with resolve/reject and expectFinal boolean.
  - Server may respond with interim ACKs using res frames with payload.status = "accepted". If client set expectFinal, it must wait for final result (status != "accepted").
  - On connection close or error, pending requests should be rejected with an error.
- Idempotency & de-dup:
  - For side-effecting operations (e.g., `send`, `agent`), clients must include an idempotency key (server expects it in params). Server keeps short-lived dedupe cache to ensure safe retries.
- Limits:
  - Max message payload default: 25 * 1024 * 1024 bytes (25 MB).
  - Request timeouts: clients should allow configurable default (e.g., 30s).
- Error & close codes:
  - Map standard WebSocket codes and give hints:
    - 1000: normal closure
    - 1006: abnormal closure (no close frame)
    - 1008: policy violation (used to signal e.g., bad token or TLS fingerprint mismatch)
    - Application-defined codes: e.g., 4008 to indicate connect failed (browser-side); server may choose custom app codes > 4000 to avoid conflict with standard codes.
  - When closing due to device token mismatch, server may close with 1008 and text "device token mismatch"; client should clear stored token.

Server responsibilities and components
- WebSocket acceptor: listen on configured bind (loopback/0.0.0.0/Tailscale, etc.), accept WS upgrade and negotiate.
- Protocol validator: robust JSON input validation against schemas for request/response/event frames and method params.
- Auth subsystem:
  - Validate incoming token/password/device identity.
  - Issue device tokens (signed opaque token or just random string tied to server-side mapping).
  - Persist device auth tokens with deviceId, role and scopes.
- Channel manager:
  - Manage per-connection channel state and subscriptions.
  - Keep mapping of nodes/agents and their subscriptions to route events.
- Method dispatch:
  - Implement core RPC methods (see "Core methods" below). Each method should run under defined permission checks and may schedule asynchronous work (responses, events).
- De-dup cache:
  - For idempotent/side-effecting method calls, keep a time-bounded cache to deduplicate repeated requests by idempotency key.
- Tick/maintenance/crons:
  - Emit ticks to clients.
  - Cron jobs for channel/agent housekeeping.
  - Maintenance timers for periodic tasks (e.g., model catalog refresh).
- Restart/update support:
  - Methods to allow gateway control for restart (SIGUSR1) and in-place update.run. Support writing restart sentinel and scheduled restarts when config changes require restart.
- Config management:
  - Methods for `config.get`, `config.schema`, `config.apply`, `config.patch`. `config.apply` replaces whole configuration; `config.patch` merges partial configuration safely.
- Mobile nodes and discovery:
  - Keep track of mobile node connections and allow pairing/push behavior for mobile nodes.
- Logging & metrics:
  - Structured logs, error levels (debug/info/warn/error), and hooks for telemetry.
- Tests: unit tests for each component and e2e tests that spin up server and client (see `gateway.e2e.test.ts`).

Core RPC methods (non-exhaustive)
- connect
- send
- agent.*
- config.get, config.schema, config.apply, config.patch
- update.run
- restart
- methods to manage channels, subscriptions, model catalog, exec approvals, node subscriptions

Client library behaviors (Go package)
- Provide a client package that mirrors GatewayClient/GatewayBrowserClient:
  - Connect(url string, opts...) -> establish WS, handle TLS fingerprint verification, device identity signing, persistent device token storage, queue connect with optional nonce handling.
  - Request(method string, params interface{}, expectFinal bool) -> send req frame, return result or error.
  - OnEvent callbacks: allow registration of event handlers and onClose.
  - Reconnect/backoff: backoff strategy (start 1000ms doubling to 30s for node client; browser used different constants); jitter optional.
  - Max payload: 25MB.
  - Tick watch: close on tick timeout.
  - Gap detection: call user-provided callback when gap detected.
  - Properly detect and clear stored device token on mismatch.
- Persistence:
  - Provide helper types for storing device identity and device auth tokens using local disk (config dir) or supplied persistent store abstraction. Provide default implementation using BoltDB or SQLite.

Security & cryptography
- Signature scheme: Ed25519 for device signatures (server uses crypto/ed25519).
- Key formats:
  - Keep PEM or raw keys. Provide conversion helpers.
  - publicKey in payload: base64url-encoded raw public key string.
  - signature: base64url of Ed25519 signature of canonical payload bytes (utf-8).
- TLS fingerprint:
  - Use SHA-256 fingerprint of certificate (normalized string). Client should inspect TLS connection state and compute fingerprint256; compare to expected.
- Input sanitization & rate limiting:
  - Rate-limit heavy operations and large uploads to avoid DoS.

JSON schema & validation
- Provide JSON Schemas for:
  - request frame
  - response frame
  - event frame
  - connect.params and connect.hello responses
  - core methods' params/payloads
- Implement JSON schema validation on the server using a robust validator (see suggested libraries).

Operational & runtime configuration
- Config options (via file/env):
  - bind address, port
  - allowed origins/loopback binding
  - gateway.mode: local/remote
  - gateway.auth: tokens/passwords, allowed roles
  - tls settings (cert, key, optional fingerprint for clients)
  - maxPayload
  - maintenance/cron settings
  - commands.restart boolean (controls ability for gateway-tool to request restart)
- Dev defaults:
  - Default WS URL: ws://127.0.0.1:18789
  - For dev profile, provide easily resettable workspace.

Message size & transports
- Accept text frames only containing JSON.
- If binary frames are necessary, define encoding — but current behavior is JSON text.

Error handling & diagnostics
- Errors returned in response frames should have structured message and optionally code.
- Server should log parse/validation errors and optionally send minimal error response before closing on protocol violations.

Suggested Go package layout
- cmd/gateway/ — main binary and CLI bootstrap
- internal/gateway/
  - protocol/ — JSON schema definitions, frame types, canonical payload builders, validators
  - server/ — WebSocket acceptor, connection lifecycle, handler registry, channel manager, subscriptions
  - auth/ — device signature verification, token issuance, token store
  - methods/ — core RPC method implementations (config, update, restart, send, agent)
  - storage/ — persistence abstractions (device tokens, config snapshots), default BoltDB/SQLite implementation
  - net/ — low-level websocket wrappers and TLS fingerprint utilities
  - maintenance/ — cron, tick emitter
  - util/ — logging, metrics, backoff helpers
- pkg/gatewayclient/ — public client library for other Go programs to call the gateway
- internal/tests/ — test helpers and e2e harness

## Web Sublauncher Integration

The OpenClaw gateway can run as a web sublauncher within the universal launcher, sharing the HTTP server with the REST API and Web UI. This is the recommended deployment for most use cases.

### Running as a Sublauncher

```bash
# API + OpenClaw gateway on shared port 8080
./agentic web api openclaw

# All sublaunchers
./agentic web api webui openclaw

# Custom WebSocket path
./agentic web api openclaw -ws-path /openclaw/ws
```

### Architecture

When running as a sublauncher, the OpenClaw server mounts its WebSocket handler onto the shared gorilla/mux router via `SetupSubrouters`. The `AgentBridge` is created from the same `launcher.Config` used by the REST API, ensuring shared session and memory services.

```
Universal Launcher
├── console (SubLauncher)
└── web (SubLauncher)
    ├── api (web.Sublauncher)        → /api/*
    ├── webui (web.Sublauncher)      → /ui/*
    └── openclaw (web.Sublauncher)   → /ws
```

### Package Location

- `internal/openclaw/launcher/launcher.go` — implements `web.Sublauncher` interface

### Standalone Binary

The standalone `cmd/clawgate` binary remains available for dedicated gateway deployments where the OpenClaw gateway runs on its own port and process.

Recommended libraries
- WebSocket: github.com/gorilla/websocket
- JSON schema validation: github.com/santhosh-tekuri/jsonschema or github.com/xeipuuv/gojsonschema
- UUID: github.com/google/uuid
- Ed25519: crypto/ed25519 (stdlib)
- Persistent store: go.etcd.io/bbolt or modernc.org/sqlite (or github.com/mattn/go-sqlite3) depending on target environments
- Logger: go.uber.org/zap or github.com/sirupsen/logrus
- Config: github.com/spf13/viper or simple file-based config loader

Data models & storage
- DeviceIdentity: { deviceId, publicKey, privateKey (secure), createdAt }
- DeviceAuthToken: { deviceId, role, token, scopes, issuedAt }
- Session/config snapshots: persist raw and hash for config.patch and config.apply logic.
- De-dup cache: in-memory LRU with TTL; optionally persist small window to disk for restarts.

Testing & e2e
- Unit tests:
  - Protocol validation tests for every frame and method schema.
  - Auth tests: verify device signature flow and token issuance.
  - De-dup tests: ensure server deduplicates repeated idempotent requests.
  - Tick tests: simulate missing ticks and ensure clients close connection.
- Integration / e2e:
  - Start gateway server (in-process) and a gateway client; validate connect, challenge -> signed connect, HelloOk, device token issuance, event delivery, request/response flows.
  - Reconnect scenarios: server closes, client reconnects with backoff and replays/waits for sequence handling.
  - Gateway-tool actions: config.apply, config.patch, update.run, restart flows should be validated by end-to-end tests.
- Test harness:
  - Provide helper to start a server on ephemeral port bound to loopback and return a client instance.
  - Implement an automated test similar to `gateway.e2e.test.ts` behavior (create temporary workspace, write dev files, run client/server pair).

Backwards compatibility & interop
- Keep the WebSocket wire protocol identical so existing clients (browser and Node) can interoperate with new Go server.
- Ensure JSON field names, event names, and expected behavior (connect challenge, device token in hello.auth.deviceToken, tick, seq) match existing behavior.

Priority implementation plan (milestones)
1. Protocol & types
   - Implement frame structs & JSON schemas.
   - Implement validator utilities and canonical payload builders for device signature.
2. Simple server + client handshake
   - WS acceptor, connect request handling, secret/token check, HelloOk response.
   - Basic event emission (tick) and seq management.
3. Auth & device tokens
   - Device signature verification, issuance of deviceToken, persistent token store API.
   - API for clearing tokens on mismatch.
4. Request handling & pending responses
   - request/res frames, pending map, expectFinal support, ack handling.
5. Channel manager & subscription system
   - Manage channels, route events to connected clients.
6. Methods & server subsystems
   - Implement core methods (`config.*`, `restart`, `update.run`, `send`, `agent.*`).
7. Maintenance & cron tasks
   - Implement tick emission, maintenance timers, model catalog loader.
8. Idempotency & de-dup cache
9. Tests & e2e harness
10. Performance, metrics, TLS fingerprint checks, production hardening

Operational notes for implementers
- Keep the server modular: separate protocol/parsing, auth, method handlers, and runtime maintenance.
- Document wire protocol with JSON schema and examples in repository (add generated Go types from schemas if desired).
- Make default behavior safe for local dev (bind to loopback by default).
- Provide clear errors when expecting build artifacts are missing and when the client uses invalid token or signature.

Example behaviors to match exactly
- Client ignores ERR_MODULE_NOT_FOUND equivalent (for Go, the server should not crash on missing optional artifact; ensure robust error handling).
- When server closes with 1008 and message contains "device token mismatch", clients must clear stored tokens for that device id and role.
- Server must support both `.js` and `.mjs` style of produced artifacts? (Not applicable for Go, but keep flexibility for accepting both token formats and device auth variants).
- Max payload handling (25MB by default).

## Agent Bridge — Routing Conversations to ADK Agents

The OpenClaw gateway routes user conversations to ADK agents via the `AgentBridge`, which lives in `internal/openclaw/server/agentbridge.go`. The bridge is wired up in `cmd/clawgate/main.go`.

### Architecture Flow

```
Client (WebSocket)
  │
  ├─ connect ──────────► Server (auth, handshake, tick loop)
  │
  ├─ agent.session.create ──► AgentBridge
  │                              │
  │                              ├─ SessionService.Create()
  │                              └─ returns { sessionId }
  │
  └─ agent.send { text } ──► AgentBridge
                                │
                                ├─ Returns immediate ACK: { status: "accepted" }
                                │
                                └─ (goroutine) runner.Run(ctx, userID, sessionID, msg)
                                     │
                                     ├─ Streams agent.text events back to client
                                     │   { requestId, text, author, partial, final }
                                     │
                                     ├─ On error: agent.error event
                                     │   { requestId, error }
                                     │
                                     └─ On completion: final res frame
                                         { text: "<final response>" }
```

### How It Works

1. **Startup**: `cmd/clawgate/main.go` loads the YAML config, builds a `launcher.Config` via the registry (which resolves the root agent, session service, memory service, etc.), then creates an `AgentBridge` from it.

2. **Wiring**: The bridge's `Handler()` method is passed to `srv.SetAgentHandler()`, which registers it as the handler for all `agent.*` method prefixes in the server's dispatch table.

3. **Session creation** (`agent.session.create`): The client creates an ADK session. The bridge stores a mapping of `clientID → sessionID` so subsequent `agent.send` calls can omit the session ID.

4. **Message dispatch** (`agent.send`): The bridge:
   - Validates the request and resolves the session.
   - Returns an immediate `{ status: "accepted" }` ACK response.
   - Spawns a goroutine that calls `runner.Run()` with SSE streaming mode.
   - Each agent event is sent to the client as an `agent.text` WebSocket event frame.
   - When the agent finishes, a final `res` frame is sent with the complete response text.

5. **Client handling**: The client uses `expectFinal: true` semantics — it receives the initial ACK, then listens for `agent.text` events for streaming output, and finally receives the closing response frame.

### Agent Methods

| Method | Direction | Description |
|--------|-----------|-------------|
| `agent.session.create` | req/res | Create a new ADK session for conversation |
| `agent.send` | req/res + events | Send user message, receive streaming agent response |

### agent.session.create

**Request params:**
```json
{
  "userId": "optional-user-id"
}
```

**Response payload:**
```json
{
  "sessionId": "uuid"
}
```

### agent.send

**Request params:**
```json
{
  "sessionId": "optional-if-previously-created",
  "userId": "optional-user-id",
  "text": "user message"
}
```

**Immediate ACK response:**
```json
{
  "status": "accepted"
}
```

**Streaming event (agent.text):**
```json
{
  "type": "event",
  "event": "agent.text",
  "payload": {
    "requestId": "original-req-id",
    "text": "partial or full text",
    "author": "AgentName",
    "partial": true,
    "final": false
  },
  "seq": 5
}
```

**Error event (agent.error):**
```json
{
  "type": "event",
  "event": "agent.error",
  "payload": {
    "requestId": "original-req-id",
    "error": "error description"
  },
  "seq": 6
}
```

**Final response:**
```json
{
  "type": "res",
  "id": "original-req-id",
  "ok": true,
  "payload": {
    "text": "complete agent response"
  }
}
```

## Appendix A — Event & Response examples

- connect (client -> server req)
  {
    "type":"req",
    "id":"<uuid>",
    "method":"connect",
    "params":{
      "minProtocol":3,
      "maxProtocol":3,
      "client": { "id":"controlui", "version":"dev", "platform":"web", "mode":"webchat" },
      "role":"operator",
      "scopes":["operator.admin"],
      "device": { "id":"dev-123", "publicKey":"<b64url>", "signature":"<b64url>", "signedAt":167..., "nonce":"..." },
      "auth": { "token":"...", "password":"..." }
    }
  }
- connect response (server -> client res, success)
  {
    "type":"res",
    "id":"<same-uuid>",
    "ok": true,
    "payload": {
      "auth": { "deviceToken":"<token>", "role":"operator", "scopes":["operator.admin"] },
      "policy": { "tickIntervalMs": 30000 }
    }
  }
- connect challenge (server -> client event)
  {
    "type":"event",
    "event":"connect.challenge",
    "payload": { "nonce": "<string>" },
    "seq": 1
  }

Acceptance criteria
- A Go gateway binary that accepts WebSocket clients built against the above wire protocol and passes a minimal e2e test:
  - Successful connect with device signature flow and device token issuance.
  - Request/response flows with ack + final semantics (expectFinal).
  - Tick emission and client tick timeout behavior.
  - Sequence numbers and gap detection.
  - Proper handling of device token mismatch (server closes with 1008 and message; client clears token).
- A Go client library supporting connect, request, onEvent, backoff/reconnect, and persistent device token storage.
- Unit and e2e tests covering the major behaviors.

## Appendix B - JSON schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "https://openclaw.github.io/schemas/gateway-protocol.schema.json",
  "title": "OpenClaw Gateway Protocol",
  "description": "JSON Schema for OpenClaw gateway frames (request/response/event) and connect payloads.",
  "type": "object",
  "oneOf": [
    { "$ref": "#/definitions/RequestFrame" },
    { "$ref": "#/definitions/ResponseFrame" },
    { "$ref": "#/definitions/EventFrame" }
  ],
  "definitions": {
    "uuid": {
      "type": "string",
      "format": "uuid",
      "description": "RFC 4122 UUID (v4 preferred)."
    },
    "base64url": {
      "type": "string",
      "pattern": "^[A-Za-z0-9_-]+$",
      "description": "Base64url-encoded data (without padding)."
    },
    "ErrorObject": {
      "type": "object",
      "properties": {
        "message": { "type": "string" },
        "code": { "type": "string" }
      },
      "required": ["message"],
      "additionalProperties": false
    },
    "ClientInfo": {
      "type": "object",
      "properties": {
        "id": { "type": "string" },
        "displayName": { "type": "string" },
        "version": { "type": "string" },
        "platform": { "type": "string" },
        "mode": { "type": "string" },
        "instanceId": { "type": "string" }
      },
      "required": ["id"],
      "additionalProperties": false
    },
    "DevicePayload": {
      "type": ["object", "null"],
      "properties": {
        "id": { "type": "string" },
        "publicKey": { "$ref": "#/definitions/base64url" },
        "signature": { "$ref": "#/definitions/base64url" },
        "signedAt": { "type": "integer", "minimum": 0 },
        "nonce": { "type": "string" }
      },
      "required": ["id", "publicKey", "signature", "signedAt"],
      "additionalProperties": false
    },
    "AuthObject": {
      "type": ["object", "null"],
      "properties": {
        "token": { "type": "string" },
        "password": { "type": "string" }
      },
      "additionalProperties": false
    },
    "ConnectParams": {
      "type": "object",
      "properties": {
        "minProtocol": { "type": "integer", "minimum": 1 },
        "maxProtocol": { "type": "integer", "minimum": 1 },
        "client": { "$ref": "#/definitions/ClientInfo" },
        "caps": {
          "type": "array",
          "items": { "type": "string" },
          "default": []
        },
        "commands": {
          "type": "array",
          "items": { "type": "string" }
        },
        "permissions": {
          "type": "object",
          "additionalProperties": true
        },
        "pathEnv": {
          "type": "object",
          "additionalProperties": { "type": "string" }
        },
        "auth": { "$ref": "#/definitions/AuthObject" },
        "role": { "type": "string" },
        "scopes": {
          "type": "array",
          "items": { "type": "string" }
        },
        "device": { "$ref": "#/definitions/DevicePayload" }
      },
      "required": ["minProtocol", "maxProtocol", "client", "role", "scopes"],
      "additionalProperties": false
    },
    "HelloAuthPayload": {
      "type": "object",
      "properties": {
        "deviceToken": { "type": "string" },
        "role": { "type": "string" },
        "scopes": {
          "type": "array",
          "items": { "type": "string" }
        },
        "issuedAtMs": { "type": "integer", "minimum": 0 }
      },
      "additionalProperties": false
    },
    "HelloPayload": {
      "type": "object",
      "properties": {
        "auth": { "$ref": "#/definitions/HelloAuthPayload" },
        "policy": {
          "type": "object",
          "properties": {
            "tickIntervalMs": { "type": "integer", "minimum": 0 }
          },
          "additionalProperties": false
        }
      },
      "additionalProperties": false
    },
    "RequestFrame": {
      "type": "object",
      "properties": {
        "type": { "const": "req" },
        "id": { "$ref": "#/definitions/uuid" },
        "method": { "type": "string" },
        "params": { "type": ["object", "null"] }
      },
      "required": ["type", "id", "method"],
      "additionalProperties": false
    },
    "ResponseFrame": {
      "type": "object",
      "properties": {
        "type": { "const": "res" },
        "id": { "$ref": "#/definitions/uuid" },
        "ok": { "type": "boolean" },
        "payload": { "type": ["object", "array", "string", "number", "boolean", "null"] },
        "error": { "$ref": "#/definitions/ErrorObject" }
      },
      "required": ["type", "id", "ok"],
      "additionalProperties": false,
      "allOf": [
        {
          "if": {
            "properties": { "ok": { "const": true } }
          },
          "then": { "required": ["payload"], "properties": { "error": { "not": {} } } }
        },
        {
          "if": {
            "properties": { "ok": { "const": false } }
          },
          "then": { "required": ["error"], "properties": { "payload": { "not": {} } } }
        }
      ]
    },
    "EventFrame": {
      "type": "object",
      "properties": {
        "type": { "const": "event" },
        "event": { "type": "string" },
        "payload": { "type": ["object", "array", "string", "number", "boolean", "null"] },
        "seq": { "type": "integer", "minimum": 0 },
        "stateVersion": { "type": "string" }
      },
      "required": ["type", "event"],
      "additionalProperties": false
    }
  },
  "examples": [
    {
      "type": "req",
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "method": "connect",
      "params": {
        "minProtocol": 3,
        "maxProtocol": 3,
        "client": { "id": "control-ui", "version": "dev", "platform": "web", "mode": "webchat" },
        "role": "operator",
        "scopes": ["operator.admin"],
        "device": {
          "id": "device-xyz",
          "publicKey": "BlaBase64UrlPubKey",
          "signature": "SigBase64Url",
          "signedAt": 1680000000000,
          "nonce": "nonce-string"
        },
        "auth": { "token": "sometoken" }
      }
    },
    {
      "type": "res",
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "ok": true,
      "payload": {
        "auth": { "deviceToken": "device-token-value", "role": "operator", "scopes": ["operator.admin"] },
        "policy": { "tickIntervalMs": 30000 }
      }
    },
    {
      "type": "event",
      "event": "connect.challenge",
      "payload": { "nonce": "nonce-string" },
      "seq": 1
    }
  ]
}
```

## Appendix C: Gateway files referred 

[Open Claw](https://github.com/openclaw/openclaw) files used to create this specification

```
src/gateway/client.ts         # Gateway client (GatewayClient) used by Node-side clients
src/gateway/net.ts            # Low-level WebSocket / networking helpers
src/gateway/server.impl.ts    # Server assembly: channel manager, handlers, maintenance, discovery
src/gateway/protocol/schema.ts# Protocol / schema definitions for gateway messages
src/gateway/server-methods.ts # Core RPC/event handlers
src/gateway/server-channels.ts# Channel management (per-connection/channel logic)
src/gateway/server-chat.ts    # Chat / agent event handling
src/gateway/server-close.ts   # Close code handling and shutdown logic
src/gateway/server-*.ts       # (server-cron.ts, server-lanes.ts, server-maintenance.ts, server-discovery-runtime.ts, server-node-subscriptions.ts, server-model-catalog.ts) — supporting server subsystems
src/gateway/gateway.e2e.test.ts # End-to-end tests for the gateway behavior
ui/src/ui/gateway.ts          # Browser-side Gateway client (GatewayBrowserClient)
src/gateway/auth.ts           # Auth helpers used by gateway and clients (resolveGatewayAuth)
src/gateway/call.ts           # Helpers to build gateway connection details / call gateway
```

## Appendix D: Open Claw Gateway Compatibility  

On "is this executable a droping replacement for openclaw gateway?" 

Github Copilot answers:

Short answer: Mostly yes — this binary is implemented to run an "OpenClaw gateway" backed by Agentic, but it may not be a guaranteed drop‑in replacement in every environment without verifying configuration and API compatibility.

Why it looks like an OpenClaw gateway
- main.go imports github.com/innomon/agentic/internal/openclaw/server and calls server.NewAgentBridge(launcherConfig) to create an AgentBridge, then creates server.New(server.Config{}) and sets the bridge as the agent handler. It also logs "Starting OpenClaw gateway server..." — so it is explicitly the OpenClaw gateway entrypoint for this repo.

Caveats / things that can break drop‑in behavior
- server.Config{} here uses the package defaults. If your current OpenClaw gateway was started with non‑default bind address/path/TLS/middleware, you must make the new binary use the same settings (edit the YAML config or server code).
- Authentication: main.go only constructs a JWT verifier if the config provides it (public key path + issuer/audience). Make sure JWT settings match what clients expect (key path, issuer, audience, token locations).
- Registered components: blank imports (_ "…/gnogent", _ "…/routing", _ "…/wasm") register available agent types/tools. If your previous gateway exposed different providers or plugins, ensure the same components are registered.
- API surface: confirm the internal/openclaw/server.Handler() exposes the same endpoints, request/response shapes, streaming/SSE/websocket behavior and headers your clients rely on.
- TLS, proxies, rate limiting, CORS, and other infra differences may change behavior even if HTTP paths match.

How to confirm quickly
1. Build and run the binary:
   - go build -o clawgate ./cmd/clawgate
   - ./clawgate config/path.yaml
2. Compare a couple of representative calls from your current gateway to the new one (use curl or an HTTP proxy):
   - Verify health/ready endpoints if present
   - Send a real request your clients use and compare status codes, headers, and JSON schema
3. Inspect the implementation/details if something differs:
   - Check internal/openclaw/server package for Config defaults, Bind/Path, and Handler routes
   - Check launcher/registry config to ensure sessions/memory/agents are built the same
4. Run integration tests or smoke tests used in your deployment.


I started loading the repository search ability so I could open internal/openclaw/server and show exact endpoints/defaults, but the code-search calls failed (repo inaccessible from my tooling right now). I can retry if you want, or you can paste the contents of internal/openclaw/server (or give me read access) and I’ll extract the precise endpoints and config fields.

While we get that, here are concrete, actionable next steps and ready-to-run checks you can use now — plus sample curl/websocket tests you can copy/paste after substituting your actual bind address and path.

What I attempted and next step
- I tried to open internal/openclaw/server to list exact routes and Config fields but the code search failed. If you want the exact function names, endpoints and default values I can:
  - retry the repo search, or
  - you can paste internal/openclaw/server/*.go here (or grant access).
- Meanwhile, below are practical checks and tests you can run immediately.

Quick checklist to confirm drop-in compatibility
1. Build and run this binary with the same config your existing gateway uses:
   - go build -o clawgate ./cmd/clawgate
   - ./clawgate path/to/your-config.yaml
2. Confirm bind/path and server config:
   - The binary logs bind and path at startup (main.go logs: "Starting OpenClaw gateway server on %s%s" with srv.Cfg().Bind and srv.Cfg().Path). Make sure they match your current gateway.
3. Verify authentication:
   - If using JWT, ensure public key path, issuer, and audience in config match the previous gateway.
   - Check how tokens are expected (Authorization header? cookie?). Match clients.
4. Confirm registered components/plugins:
   - main.go blank-imports gnogent, routing, wasm — if your old gateway had other providers, ensure identical registration.
5. Confirm TLS/CORS/proxy configuration:
   - Match any TLS certs, reverse-proxy headers (X-Forwarded-For/Proto), and CORS rules.
6. Smoke-tests (see below).

Generic smoke-test commands (replace BIND and PATH)
- Replace:
  - BIND_HOST_PORT with e.g. 127.0.0.1:8080
  - BASE_PATH with the server path printed at startup (e.g. /openclaw or /)

1) Health / readiness
- curl (HTTP):
  curl -i "http://BIND_HOST_PORTBASE_PATH/health" 
  or
  curl -i "http://BIND_HOST_PORT/health"
(If you’re not sure path, try both with and without BASE_PATH.)

2) Basic GET endpoint test (list agents / capabilities)
- curl -i "http://BIND_HOST_PORTBASE_PATH/agents"
- Example:
  curl -i "http://127.0.0.1:8080/openclaw/agents"

3) POST a typical agent request (JSON)
- This is an example of a POST run/execute endpoint — replace route, body and headers to match your gateway API:
  curl -i -X POST "http://BIND_HOST_PORTBASE_PATH/agent/run" \
    -H "Content-Type: application/json" \
    -d '{"input":"hello","options":{}}'
- Compare status code, JSON keys, error shape vs your existing gateway.

4) JWT-protected endpoints
- If JWT is enabled, include Authorization header:
  curl -i -H "Authorization: Bearer <TOKEN>" "http://BIND_HOST_PORTBASE_PATH/agents"

5) WebSocket (if gateway supports WS agent streams)
- Using wscat:
  wscat -c "ws://BIND_HOST_PORTBASE_PATH/ws" \
    -H "Authorization: Bearer <TOKEN>"
- Or with JavaScript:
  const ws = new WebSocket("ws://127.0.0.1:8080/openclaw/ws", { headers: { Authorization: "Bearer <TOKEN>" } });

6) Long-poll / SSE streaming
- curl -N "http://BIND_HOST_PORTBASE_PATH/stream?session=..."  (use -N to not buffer)

How to compare responses
- Record a few representative calls against your current gateway (status code, response headers, Content-Type, JSON schema).
- Run the same calls against the new binary and compare:
  - HTTP status codes
  - Response headers (CORS, auth, content-type)
  - JSON field names and nesting
  - Streaming connection behavior (do you get same frames / events?)
- If differences exist, inspect internal/openclaw/server to find route handlers and config defaults.

