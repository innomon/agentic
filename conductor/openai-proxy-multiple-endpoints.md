# Objective
Update `openai-proxy` to support a "local" ADK endpoint type that loads an agentic `config.yaml` file, instantiates the defined agents in-process, and routes requests to them using `runner.Runner`.

# Background & Motivation
Currently, `openai-proxy` proxies all requests to a single remote ADK server over HTTP. Users want the ability to run agents locally within the proxy process by specifying an agentic `config.yaml`. By setting the endpoint to "local", the proxy will act as a gateway that executes local agents without needing a separate ADK server, while still providing an OpenAI-compatible API.

# Proposed Solution

1. **Configuration Update**:
   - Add `config_path` to the `ADKConfig` struct in `openai-proxy/config.yaml`.
   - When `endpoint` is set to `"local"`, the proxy will use `config_path` to load the agentic configuration.

2. **Local Initialization**:
   - On startup, if the endpoint is `"local"`, parse the agentic config using `github.com/innomon/agentic/pkg/config`.
   - Instantiate a `registry.Registry` and build a `launcher.Config` to obtain a `session.Service`.
   - Iterate over all agents defined in the config. For each agent, use `registry.Get[agent.Agent]` to load it and wrap it in a `runner.Runner` (`google.golang.org/adk/runner`).
   - Store these runners in an in-memory map keyed by the agent name.

3. **Models API (`/v1/models`)**:
   - If local runners are configured, list all local agent names as available models.
   - If a remote endpoint is configured, list the configured `app_name`.

4. **Chat Completions API (`/v1/chat/completions`)**:
   - Check if `req.Model` matches a local runner.
   - **If Local**:
     - Convert the OpenAI messages to `*genai.Content`.
     - Use the local `session.Service` to create a session.
     - Call `runner.Run` in a goroutine and iterate over the returned channel of `*agent.RunEvent`.
     - Stream or buffer the output similar to how remote SSE events are handled.
   - **If Remote**:
     - Retain the existing HTTP proxy logic.

# Implementation Steps

1. Add `ConfigPath string \`yaml:"config_path"\`` to `ProxyConfig.ADK` in `openai-proxy/main.go`.
2. Add dependencies to `openai-proxy/go.mod` (e.g., `google.golang.org/genai`, `google.golang.org/adk`, `github.com/innomon/agentic/...`).
3. Update `Server` struct to hold `localRunners map[string]*runner.Runner`, `localRegistry *registry.Registry`, and `sessionService session.Service`.
4. In `main.go`, add an initialization block that checks for `Endpoint == "local"`. If true, load config, create registry, and populate `localRunners`.
5. Update `handleModels` to return keys from `localRunners` if they exist.
6. Create `convertToGenaiContent(messages []Message)` to map OpenAI messages to `*genai.Content`.
7. Update `handleChatCompletions` to conditionally execute the local runner loop if `req.Model` is found in `localRunners`.
8. Add logic to translate `agent.RunEvent` into OpenAI `ChatCompletionChunk` and `ChatCompletion` formats.

# Verification & Testing
- Configure `openai-proxy/config.yaml` with `endpoint: "local"` and point it to `../examples/farmer/config.yaml`.
- Query `/v1/models` and verify `FarmerAgent` and `ProductRecommendationAgent` are listed.
- Query `/v1/chat/completions` using `"model": "FarmerAgent"` and verify the local agent generates a response.
