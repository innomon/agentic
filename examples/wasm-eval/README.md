# WASM Evaluation Meta-Agent

A WASM module that orchestrates multiple "Candidate" agents, records their performance metrics (latency and token usage), and uses a "Judge" agent to score their responses.

## Workflow

1.  **Read Input:** The module reads the test scenario using `get_input`.
2.  **Evaluate Candidates:**
    *   Iterates through all sub-agents except the last one.
    *   **Execution:** Runs each candidate sub-agent with the original input.
    *   **Metrics:** Captures execution time (ms) and token counts (input/output) using the new WASM host ABI.
3.  **Judge Scoring:**
    *   Takes the output of the candidate.
    *   Formats a prompt for the **Judge Agent** (the last sub-agent).
    *   The Judge provides a score and justification.
4.  **Reporting:** Compiles all metrics and scores into a Markdown table.

## New ABI Functions Used

| Function | Description |
|----------|-------------|
| `subagent_duration_ms` | Returns the execution latency in milliseconds |
| `subagent_token_input` | Returns the number of input tokens processed |
| `subagent_token_output` | Returns the number of output tokens generated |

## Build

Requires [TinyGo](https://tinygo.org/).

```bash
make build
```

## Usage

```bash
./agentic -run "Your test task" examples/wasm-eval/config.yaml
```
