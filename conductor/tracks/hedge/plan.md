# Hedge Implementation Plan (MCP Edition)

## Track: `hedge`
**Status:** `in-progress`

### Phase 1: MCP Server Development (`mcp-collection/hedge-mcp`)
- [x] Initialize `hedge-mcp` Go module.
- [x] Implement `market_data` tool (FinnHub integration).
- [x] Implement `quant_analysis` tool (RSI, MACD, etc. in Go).
- [x] Implement `fundamental_analysis` tool (AlphaVantage integration).
- [x] Implement `sentiment_analysis` tool.
- [x] **Checkpoint:** Verify MCP server tools via `mcp-inspector` or similar.

### Phase 2: Agentic Configuration (`agentic/examples/hedge`)
- [x] Create `examples/hedge/config.yaml`.
- [x] Define the 5 committee agents.
- [x] Configure MCP server connection in the `agentic` config.

### Phase 3: Validation
- [ ] Run the committee using `agentic` CLI.
- [ ] Verify the Master Trader's decision process uses data from the MCP server.
