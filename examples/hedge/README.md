# Hedge Trading Committee Example

This example demonstrates a multi-agent committee designed to perform comprehensive asset analysis and trading recommendations. It uses the `hedge-mcp` server for real-time (or simulated) financial data.

[TradingAgents: Multi-Agents LLM Financial Trading Framework](https://arxiv.org/pdf/2412.20138)

## Committee Architecture

The example utilizes a hierarchical agent structure:

1.  **Master Trader** (`root_agent`): Orchestrates the analysis process by delegating tasks to specialists and consolidating their findings into a final "TRADE" or "NO TRADE" recommendation.
2.  **Technical Analyst**: Focuses on price action, trends, and momentum using indicators like RSI and MACD.
3.  **Fundamental Analyst**: Evaluates company health, valuation ratios (P/E, Debt-to-Equity), and growth prospects.
4.  **Sentiment Analyst**: Monitors news flow and market mood to assess short-term impact.
5.  **Risk Manager**: Performs volatility analysis (ATR) and enforces position sizing and stop-loss rules. Has VETO power over risky trades.

## Prerequisites

1.  **Hedge MCP Server**: You must have the `hedge-mcp` server running.
    ```bash
    cd /home/innomon/orez/mcp/mcp-collection/hedge-mcp
    export HEDGE_MCP_TRANSPORT=sse
    ./hedge-mcp -simulate
    ```

2.  **API Keys**: Ensure your `GOOGLE_API_KEY` is set in your environment.

## Running the Example

From the root of the `agentic` repository:

```bash
go run main.go -console examples/hedge/config.yaml console
```

Once in the console, you can issue commands like:
- `Analyze AAPL`
- `What is the risk profile for BTC/USD?`

## Configuration (`config.yaml`)

The configuration defines:
- **Models**: Uses Gemini models (expandable via environment variables).
- **Agents**: Each agent is configured with a specific role, instruction, and connection to the MCP toolset.
- **MCP Toolsets**: Remote endpoints defined using `${HEDGE_MCP_URL:-http://localhost:8082/mcp}`.

## Debugging

If you encounter connection issues with the MCP server, you can use the built-in debug command in the console:
```
/mcp http://localhost:8082/mcp
```
This will attempt a manual handshake and list all tools exposed by the server.
