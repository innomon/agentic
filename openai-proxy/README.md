# OpenAI Proxy for ADK

The OpenAI Proxy allows you to use OpenAI-compatible clients (like `curl`, Python `openai` library, or LangChain) to interact with ADK (Agent Development Kit) agents. It supports both remote ADK servers and local in-process agent execution.

## Features

- **Multiple Endpoints**: Route requests to different ADK servers or local configurations based on the `model` name.
- **Local Execution**: Run ADK agents in-process by pointing to an agentic `config.yaml`.
- **OpenAI Compatible**: Implements `/v1/chat/completions` and `/v1/models`.
- **Streaming Support**: Supports Server-Sent Events (SSE) for real-time response streaming.

## Configuration

The proxy is configured via a `config.yaml` file.

### Example Configuration

```yaml
proxy:
  # Port to listen on
  listen: ":9080"

  # Define multiple apps/endpoints
  apps:
    # Remote ADK server
    remote-app:
      endpoint: "http://localhost:8080"
      app_name: "Agentic"
    
    # Local in-process agents
    local-app:
      endpoint: "local"
      config_path: "../examples/farmer/config.yaml"

  # Default values
  defaults:
    user_id: "openai-proxy-user"
```

### Configuration Fields

- `proxy.listen`: The address and port the proxy server binds to.
- `proxy.apps`: A map where the key is the OpenAI `model` ID and the value is the ADK configuration.
  - `endpoint`: The base URL of a remote ADK server, or `"local"` to run agents in-process.
  - `app_name`: (Remote only) The ADK application name to route to.
  - `config_path`: (Local only) Path to the agentic configuration file.
- `proxy.adk`: (Legacy) A single ADK configuration block for backward compatibility.

## Testing with Curl

You can test the proxy using standard `curl` commands.

### 1. Check Health
```bash
curl -s http://localhost:9080/health
```

### 2. List Available Models
This will list all configured remote apps and all agents defined in local configuration files.
```bash
curl -s http://localhost:9080/v1/models
```

### 3. Chat Completion (Non-Streaming)
Replace `ModelName` with a name returned by the models endpoint.
```bash
curl -s http://localhost:9080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "ModelName",
    "messages": [{"role": "user", "content": "Hello, how are you?"}],
    "stream": false
  }'
```

### 4. Chat Completion (Streaming)
```bash
curl -s http://localhost:9080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "ModelName",
    "messages": [{"role": "user", "content": "Tell me a long story."}],
    "stream": true
  }'
```

## Internal Architecture

When a request is received:
1. The proxy looks up the `model` field in the request.
2. If it matches a **local agent**:
   - It creates a local session using the internal `session.Service`.
   - It executes the agent using `runner.Runner`.
   - It converts the ADK `session.Event` stream into OpenAI `chat.completion.chunk` events.
3. If it matches a **remote app**:
   - It proxies the request to the specified ADK `endpoint`.
   - It handles session creation and event conversion via HTTP calls.
4. If no match is found, it returns a 404 error.
