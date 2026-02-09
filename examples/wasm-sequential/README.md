# WASM Sequential Agent Plugin

A sample TinyGo WASM module that implements sequential agent behavior: it runs all configured sub-agents in order, stopping on the first error.

## ABI

The module imports host functions from the `env` module:

| Import | Signature | Description |
|--------|-----------|-------------|
| `subagent_count` | `() → i32` | Returns the number of sub-agents |
| `run_subagent` | `(index i32) → i32` | Runs sub-agent at index; returns 0 on success |
| `subagent_name` | `(index i32, buf_ptr i32, buf_cap i32) → i32` | Writes sub-agent name into buffer, returns length |
| `log_msg` | `(ptr i32, len i32)` | Logs a message to the host |

The module exports:

| Export | Signature | Description |
|--------|-----------|-------------|
| `execute` | `() → i32` | Entry point; returns 0 on success |
| `malloc` | `(size u32) → *byte` | Memory allocator for host use |
| `free` | `(ptr *byte)` | Free memory (no-op) |

## Build

Requires [TinyGo](https://tinygo.org/getting-started/install/).

```bash
make build
```

Or manually:

```bash
tinygo build -o sequential.wasm -scheduler=none --no-debug -target wasi ./main.go
```

## Usage in config.yaml

Reference the compiled `.wasm` file from your agent configuration:

```yaml
agents:
  MySequentialWasm:
    type: wasm
    module_path: examples/wasm-sequential/sequential.wasm
    sub_agents:
      - StepOne
      - StepTwo
      - StepThree
```
