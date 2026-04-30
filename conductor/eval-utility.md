# Agentic Evaluation Utility Plan

## Background & Motivation
For a given agentic workflow, different LLMs (Gemini Flash, Pro, GPT-4o, Local Llama) offer varying capabilities and incur different token costs. To optimize use cases, we need a utility to evaluate multiple workflow configurations against test scenarios, calculating a cost/time tradeoff. The user has requested a dual approach:
1. **Gemini CLI Skill (`agentic-eval`):** A fast, conversational tool for ad-hoc evaluations, driven by Gemini.
2. **Meta-Agent (WASM Orchestrator):** A native, dogfooding approach where an evaluation orchestrator is built entirely as a WASM agent within the framework.

## Scope & Impact
*   **CLI Skill:** Create a `SKILL.md` file outlining the instructions for Gemini CLI to perform automated variations, execution, and LLM-as-a-Judge reporting.
*   **WASM ABI Extensions:** Modify `pkg/wasm/wasm.go` to expose sub-agent execution latency and token usage (if available from the ADK model provider) so the WASM meta-agent can calculate exact costs.
*   **WASM Meta-Agent:** Develop a TinyGo/AssemblyScript WASM module that orchestrates sub-agents, records their metrics, queries a "Judge" agent, and emits a final report.

## Proposed Solution

### Part 1: Gemini CLI Skill (`agentic-eval`)
The skill will instruct the Gemini CLI to:
1. Receive a user's task and a set of candidate LLM configurations.
2. Generate temporary `config.yaml` files for each candidate.
3. Run the `agentic` binary non-interactively (or via a scripted input) for each config.
4. Capture the latency (wall-clock time) and the output text.
5. Use Gemini's LLM reasoning to evaluate the output quality against the prompt.
6. Present a markdown table comparing: LLM Model, Execution Time, Quality Score, and Estimated Cost.

### Part 2: WASM Meta-Agent
A new example workflow (`examples/wasm-eval`) featuring:
1. **Host ABI Updates:** Add `subagent_duration_ms(index) -> i64` and `subagent_token_input(index) -> i32`, `subagent_token_output(index) -> i32` to the `env` module in `pkg/wasm/wasm.go`. The host Go code will measure `time.Since(start)` during the `run_subagent` call and attempt to extract `UsageMetadata` from the generated ADK events.
2. **EvalModule (`eval.wasm`):** A custom module that:
   * Takes the test prompt via `get_input()`.
   * Loops over $N$ sub-agents (the candidates).
   * Records start/end times and fetches token usage via the new ABI.
   * Calls the $(N+1)$th sub-agent (the **Critic/Judge Agent**) with the candidate's output and the grading rubric.
   * Compiles the latency, token cost, and Judge score into a JSON report or structured summary, returning it as its output.

## Implementation Steps

### Phase 1: The `agentic-eval` Skill
1.  Draft the `SKILL.md` file containing the persona, instructions, and prompt templates for the `agentic-eval` skill.
2.  Provide instructions on how the user can install and invoke it globally or project-locally.

### Phase 2: ABI Extensions for WASM
1.  Update `wasmEnv` in `pkg/wasm/wasm.go` to track `durationMs` and `tokenInput/Output` for each sub-agent index.
2.  In the `run_subagent` host function, capture `time.Now()`.
3.  Inspect the yielded `session.Event` objects. If they contain ADK `UsageMetadata`, aggregate the token counts.
4.  Export `subagent_duration_ms`, `subagent_token_input`, and `subagent_token_output` to the WASM host builder.

### Phase 3: The WASM Eval Module
1.  Create `examples/wasm-eval/main.go` (TinyGo source).
2.  Implement the logic:
    *   Read input (the test scenario).
    *   For `i = 0` to `subagent_count - 2`:
        *   `run_subagent(i)`
        *   Read output, duration, and tokens.
        *   Format a judge prompt: "Score this response: [output]".
        *   Set input for Judge: `set_input(judge_prompt)`.
        *   `run_subagent(JudgeIndex)`.
        *   Parse the Judge's score.
    *   Generate a final evaluation summary string.
3.  Build the module `make build`.

### Phase 4: Example Configuration
1.  Create `examples/wasm-eval/config.yaml` defining the `EvalOrchestrator` (WASM), candidate agents (e.g., `FlashCandidate`, `OllamaCandidate`), and the `JudgeAgent` (e.g., `ProJudge`).

## Verification & Testing
*   Verify the Gemini CLI skill works by having it evaluate a simple "summarize this text" prompt using two temporary configs.
*   Verify the WASM module successfully accesses the new timing/token ABI functions and successfully routes outputs to the Judge agent.
*   Check that the final report correctly calculates the time/cost tradeoffs.
