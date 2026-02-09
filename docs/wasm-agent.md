# AgenticWasm : WASM Agent

Create and register a go Agent, that will act as a wasm wrapper, use the project's framework and design pattern, refer [README](../README.md) and [AGENTS](../AGENETS.md). 

The idea is to add new agents in runtime, without the need to recompile the project.

The wasm agent can be configured using a config file, create a sample `agentic-wasm.yaml` for demonstrating the usage. The config will add a wasm file path, along with the existing parameters. 

The wasm container agent will use [wazero](https://github.com/wazero/wazero), this agent will manage the wasm lifecycle of the wasm agent plugin.

Also create a sample wasm agent, in go, compiled using [tinygo](https://tinygo.org/) that will have the same functionality as [sequential agent](https://google.golang.org/adk/agent/workflowagents/sequentialagent)