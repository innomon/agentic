# ADK Go SDK v1.0.0 Upgrade

This document outlines the upgrade from `google.golang.org/adk` v0.6.0 to v1.0.0.

## Breaking Changes in v1.0.0

### `memory.Service` Interface

The `memory.Service` interface has been updated to use more descriptive method names.

**Old (v0.6.0):**
```go
type Service interface {
    AddSession(ctx context.Context, s session.Session) error
    Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error)
}
```

**New (v1.0.0):**
```go
type Service interface {
    AddSessionToMemory(ctx context.Context, s session.Session) error
    SearchMemory(ctx context.Context, req *SearchRequest) (*SearchResponse, error)
}
```

### `agent.Memory` Interface

The `agent.Memory` interface also reflects these changes but with a simplified search signature.

```go
type Memory interface {
    AddSessionToMemory(context.Context, session.Session) error
    SearchMemory(ctx context.Context, query string) (*memory.SearchResponse, error)
}
```

## Migration Steps

1.  **Upgrade Dependency:**
    ```bash
    go get google.golang.org/adk@v1.0.0
    go mod tidy
    ```

2.  **Update Memory Implementations:**
    All implementations of `memory.Service` must rename `AddSession` to `AddSessionToMemory` and `Search` to `SearchMemory`.

    Affected files in this project:
    - `pkg/memory/mem2db.go`
    - `pkg/gnogent/storage/memory_service.go`
    - `pkg/prologmem/service.go`

3.  **Verify Build and Tests:**
    ```bash
    go build ./...
    go test ./...
    ```

## Summary of Changes

- Updated `go.mod` to `google.golang.org/adk v1.0.0`.
- Renamed `AddSession` -> `AddSessionToMemory` in all memory service implementations.
- Renamed `Search` -> `SearchMemory` in all memory service implementations.
- Verified compatibility with other ADK components (`agent.Agent`, `model.LLM`, `runner.Runner`, `launcher.Launcher`).
