# Hedge Specification: Trading Committee Port

## 1. Overview
Porting the `TauricResearch/TradingAgents` framework to the **Agentic** Go framework. Hedge is a multi-agent system that simulates a trading committee to analyze financial assets and make trade recommendations.

## 2. Architecture
The system utilizes a hierarchical multi-agent orchestrator where specialized analysts provide data to a Master Trader who makes the final decision, subject to Risk Manager approval.

### 2.1 Core Components
- **Orchestrator:** `agentic.Orchestrator` (Sequential/Parallel)
- **State Management:** `session.Session` (ADK)
- **Tools:** Native Go functions registered as `tool.Tool`

## 3. Tool Specifications

### 3.1 Market Data Tool (`market_data`)
- `GetPrices(symbol, resolution)`: Candle/quote data.
- `GetVolume(symbol)`: Trading volume.
- `GetHistoricalData(symbol, start, end)`: Time-series data.
- **Provider:** FinnHub.

### 3.2 Quantitative Tool (`quant_analysis`)
- Hand-crafted Go implementations for:
  - RSI (Relative Strength Index)
  - MACD (Moving Average Convergence Divergence)
  - Bollinger Bands
  - EMA (50/200)
  - ATR (Average True Range)

### 3.3 Sentiment Tool (`sentiment_analysis`)
- `GetNews(symbol)`: Latest news headlines.
- `AnalyzeSentiment(text)`: Sentiment scoring (handled by Analyst agent).

### 3.4 Fundamental Tool (`fundamental_analysis`)
- `GetFinancials(symbol)`: Key ratios (P/E, Debt-to-Equity).
- **Provider:** Alpha Vantage.

## 4. Agent Definitions

| Agent | Instruction | Tools |
| :--- | :--- | :--- |
| **Technical Analyst** | Analyze price trends and momentum. identify entry/exit points. | `market_data`, `quant_analysis` |
| **Fundamental Analyst** | Examine company health and intrinsic valuation. | `fundamental_analysis` |
| **Sentiment Analyst** | Monitor news flow and market mood. | `sentiment_analysis` |
| **Risk Manager** | Evaluate trade risk (ATR), stop-loss, and sizing. Can veto. | `InternalMathTool` |
| **Master Trader** | Final decision (BUY/SELL/HOLD) based on committee input. | Committee Results |

## 5. Implementation Roadmap
1. **Module Setup:** `examples/hedge` directory.
2. **Tool Implementation:** Implement `pkg/hedge/tools` with FinnHub/AlphaVantage integrations.
3. **Agent Configuration:** `examples/hedge/config.yaml`.
4. **Main Entrypoint:** `examples/hedge/main.go` to wire everything.
