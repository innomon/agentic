# MCP Tool Discovery Test

This is a minimal example to verify that `agentic` can connect to an MCP server and discover its tools. It uses a simple "Hello" MCP server.

## Prerequisites

1.  **Start the Hello MCP Server** in SSE transport:
    ```bash
    cd /home/innomon/orez/mcp/mcp-collection/hello-mcp
    export HELLO_MCP_TRANSPORT=sse
    export HELLO_MCP_SSE_HOST=127.0.0.1
    ./hello-mcp
    ```

2.  **Ensure your Google API Key** is set:
    ```bash
    export GOOGLE_API_KEY=your_api_key_here
    ```

## Running the Test

1.  **Run the Agentic Console**:
    ```bash
    cd /home/innomon/orez/adk/agentic
    go run main.go -console examples/mcp_test/config.yaml console
    ```

2.  **Verify via `/mcp` Command**:
    Once the console is open, use the debug command to verify the connection:
    ```
    User -> /mcp http://127.0.0.1:8083/mcp
    ```
    This should list: `hello`.

3.  **Verify via Agent**:
    Ask the agent to list its tools and use them:
    ```
    User -> List all the tools you have access to.
    User -> Say hello to "Gemini".
    ```

## Troubleshooting

### `Bad Request` during Initialization
If you see the following error:
```
Error: failed to init MCP session: calling "initialize": sending "initialize": Bad Request
```
This indicates a failure during the MCP protocol handshake over SSE. 

**Current Status:**
- The `hello-mcp` server correctly starts and listens on the SSE endpoint.
- The `agentic` client successfully reaches the server.
- The `initialize` request is rejected by the server or transport layer with a 400 Bad Request.

This is a known issue being investigated in the SSE transport integration. You can verify the server is "alive" by visiting `http://127.0.0.1:8083/mcp` in your browser; you should see an event stream start.
