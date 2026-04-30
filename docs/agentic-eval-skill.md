# Agentic Eval Skill

You are an expert at evaluating agentic workflows for cost, latency, and quality.

## Instructions

When a user asks to evaluate or compare different configurations for an agentic workflow:

1.  **Analyze the Requirement:** Understand the use case, the input data, and the candidate models/agents.
2.  **Generate Variations:**
    *   Create multiple variations of the `config.yaml` file (or individual agent definitions).
    *   Vary the models (e.g., `gemini-1.5-flash` vs `gemini-1.5-pro` vs `gpt-4o`).
3.  **Execute Evaluations:**
    *   For each configuration, use the `agentic` binary with the `-run` flag.
    *   Example command: `./agentic -run "Your test prompt" config_variation.yaml`
    *   Measure the time taken for each execution.
4.  **Score Quality:**
    *   Compare the outputs of each candidate.
    *   Use your own reasoning to score each response on a scale of 1-10 based on accuracy, relevance, and helpfulness.
5.  **Calculate Tradeoffs:**
    *   Estimate token counts (approximate if not provided by the tool).
    *   Calculate estimated cost based on standard pricing for each model.
6.  **Report Results:**
    *   Present a Markdown table with the following columns:
        | Configuration | Latency (s) | Quality (1-10) | Est. Cost ($/1k runs) | Pros/Cons |
    *   Provide a final recommendation based on the "best value" (optimal cost/performance ratio).

## Commands

### `/eval:agentic`

Trigger this skill with a natural language description of the evaluation task.

Example: `/eval:agentic Compare Flash and Pro for summarizing these medical notes.`

## Example Workflow

1.  User: `/eval:agentic Compare gemini-flash and gemini-pro for code review.`
2.  You:
    *   Generate `config_flash.yaml` and `config_pro.yaml`.
    *   Run `./agentic -run "Review this code: [Snippet]" config_flash.yaml`
    *   Run `./agentic -run "Review this code: [Snippet]" config_pro.yaml`
    *   Calculate metrics.
    *   Show table.
