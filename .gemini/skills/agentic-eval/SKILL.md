---
name: agentic-eval
description: Evaluate and compare agentic workflows for cost, latency, and quality. Use when a user needs to optimize LLM selection or configuration for a specific task using the agentic framework.
---

# Agentic Eval

You are an expert at evaluating agentic workflows for cost, latency, and quality. This skill enables you to automate the process of testing different LLM configurations and reporting on their performance tradeoffs.

## Workflow

When triggered, follow this process to perform a comprehensive evaluation:

### 1. Analyze and Plan
Understand the user's task, the target agents/models, and the test data. Define a grading rubric for the "Judge" phase.

### 2. Generate Candidate Configs
Create temporary `config.yaml` variations. For example, vary the `model` field between `gemini-2.0-flash` and `gemini-2.0-pro-exp-02-05`.

### 3. Execute Benchmarks
Run each configuration using the `agentic` binary with the `-run` flag.
```bash
./agentic -run "Your test prompt" config_variation.yaml
```
Measure the wall-clock time for each execution to calculate latency.

### 4. LLM-as-a-Judge Scoring
Compare the outputs. Use your own reasoning (as a high-capability model) to score each response (1-10) based on accuracy, relevance, and your predefined rubric.

### 5. Report Tradeoffs
Present a Markdown table comparing metrics.
- **Latency**: Actual execution time.
- **Quality**: Your judge score.
- **Cost**: Estimated based on token counts (Flash is ~$0.10/1M, Pro is ~$1.25/1M).

## Example Commands

### Compare Flash and Pro
User: `/eval:agentic Compare Flash and Pro for code review.`

1. Generate `config_flash.yaml` and `config_pro.yaml`.
2. Run benchmark: `./agentic -run "Review this code..." config_flash.yaml`
3. Score results.
4. Output comparison table.
