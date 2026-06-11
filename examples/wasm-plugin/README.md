# WASM Plugin Example

This directory contains a WebAssembly (WASM) plugin example for the Agentic framework built in TinyGo.

## How it works
The plugin implements the `on_user_message` callback hook. It intercepts every message sent by the user:
1. If the message contains the word "secret" (case-insensitive), it returns a guardrail error that prevents the execution and responds to the user directly with the error.
2. Otherwise, it automatically appends a suffix `\n\n[Secured by WasmPlugin]` to the user's message before forwarding it to the LLM agent.

## Building the plugin
Make sure you have [TinyGo](https://tinygo.org/) installed, then run:

```bash
make build
```

This will produce `plugin.wasm` compiled as a WASI reactor (`-buildmode=c-shared`).

## Running the example
To run the example console with the WASM plugin loaded:

```bash
./agentic -console examples/wasm-plugin/config.yaml
```

Try typing:
- `Hello, tell me a joke.` (should print the response along with the `[Secured by WasmPlugin]` suffix injected).
- `What is the secret formula?` (should block and print the guardrail error message directly).
