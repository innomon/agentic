# Hedge Specification: Trading Committee (MCP Edition)

## 1. Overview
Porting the `TauricResearch/TradingAgents` framework to the **Agentic** Go framework. Hedge is a multi-agent system that simulates a trading committee to analyze financial assets. 

**Architectural Shift:** To maintain a clean, configuration-driven example, all tools and functions are offloaded to a dedicated **Hedge MCP Server**. The `agentic` example will contain only configuration and will connect to this server.

## 2. Architecture
The system utilizes a hierarchical multi-agent orchestrator.

### 2.1 Core Components
- **Orchestrator:** `agentic.Orchestrator` (Sequential/Parallel)
- **State Management:** `session.Session` (ADK)
- **Tooling:** All tools provided via [Model Context Protocol (MCP)](https://modelcontextprotocol.io).

## 3. MCP Server: `hedge-mcp`
Located at `/home/innomon/orez/mcp/mcp-collection/hedge-mcp`.

### 3.1 Tools Provided
- **Market Data (`market_data`)**:
    - `get_prices(symbol, resolution)`: Candle/quote data.
    - `get_volume(symbol)`: Trading volume.
- **Quantitative Analysis (`quant_analysis`)**:
    - `calculate_indicators(data)`: Returns RSI, MACD, Bollinger Bands, EMA (50/200), ATR.
- **Sentiment & News (`sentiment_analysis`)**:
    - `get_news(symbol)`: Headlines and news text.
- **Fundamental Analysis (`fundamental_analysis`)**:
    - `get_financials(symbol)`: P/E, Debt-to-Equity, etc.

## 4. Agent Definitions (Configuration Only)

| Agent | Instruction | MCP Tools |
| :--- | :--- | :--- |
| **Technical Analyst** | Analyze price trends and momentum. identify entry/exit points. | `market_data`, `quant_analysis` |
| **Fundamental Analyst** | Examine company health and intrinsic valuation. | `fundamental_analysis` |
| **Sentiment Analyst** | Monitor news flow and market mood. | `sentiment_analysis` |
| **Risk Manager** | Evaluate trade risk (ATR), stop-loss, and sizing. | `quant_analysis` (ATR) |
| **Master Trader** | Final decision (BUY/SELL/HOLD) based on committee input. | N/A |

## 5. Implementation Roadmap
1. **MCP Server Development:** Implement `hedge-mcp` in `mcp-collection` using `github.com/modelcontextprotocol/go-sdk/mcp`.
2. **Agentic Configuration:** Create `examples/hedge/config.yaml` defining the agents and connecting to the `hedge-mcp` server.
3. **Execution:** Run via `agentic` CLI or a generic runner that supports MCP.

