# WASM Loop Agent Plugin

A sample TinyGo WASM module that implements an iterative refinement loop between a **Worker** and a **Critic**.

## Workflow

1.  **Read Input:** The module reads the original user prompt using `get_input`.
2.  **Iterative Refinement:**
    *   **Worker Phase:** Runs the first sub-agent (index 0) with the current task.
    *   **Critic Phase:** Captures the worker's output and passes it to the second sub-agent (index 1).
    *   **Decision:**
        *   If the Critic returns `[APPROVED]`, the loop stops.
        *   Otherwise, the Critic's feedback is combined with the original task and fed back into the next Worker iteration.
    *   **Max Iterations:** The loop stops after `max_iterations` (configured in `config.yaml` under `params`, default is 10). If set to 0, it runs only once.
3.  **Completion:** Returns the final state to the host.

## New ABI Functions Used

| Function | Description |
|----------|-------------|
| `get_input_len` | Returns the length of the original user prompt |
| `get_input` | Copies the original user prompt into guest memory |
| `get_config_param_len` | Returns the length of a configuration parameter value |
| `get_config_param` | Copies a configuration parameter value into guest memory |

## Build

```bash
make build
```

## Usage

```bash
./agentic -console examples/wasm-loop/config.yaml
```
