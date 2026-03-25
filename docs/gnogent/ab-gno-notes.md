# Agentic Gno VM Redesign Status

GNO VM has two use cases in Agentic:
1.  **Agent Implementation**: `pkg/gnogent` (Stateful Brain)
2.  **Sandbox Scripting**: `pkg/sandbox/engines/gnovm` (Ephemeral Execution)

## Unified Gno VM Package (`pkg/gnovm`)

The implementation has been unified in `pkg/gnovm` to provide a consistent interface for both use cases.

### Core Components

-   `MachineWrapper` (`pkg/gnovm/machine.go`): Handles the core Gno machine lifecycle, state export/restore, and expression evaluation.
-   `AgentWrapper` (`pkg/gnovm/agent_wrapper.go`): Extends `MachineWrapper` with agent-specific methods (`SyncState`, `AddTurn`, `GetSystemContext`, etc.).
-   `SandboxVM` (`pkg/gnovm/sandbox.go`): Implements the `sandbox.SandboxVM` interface for ephemeral code execution.

## Storage Implementation

### Postgres Storage for Packages

Gno package files can be stored in a Postgres table named `filesys`.

```sql
CREATE TABLE IF NOT EXISTS filesys (
    path TEXT PRIMARY KEY,
    metadata JSONB,
    content BYTEA,
    timestamp TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
```

The `pkg/gnovm/storage` package provides:
-   `FileSys` model for GORM.
-   `PostgresDB` implementing Gno's `db.DB` interface to load packages directly from Postgres.

### Agent Session Persistence

Agent states (snapshots) are stored in the `agent_sessions` table.

```go
type AgentSession struct {
	gorm.Model
	UserID          string `gorm:"index"`
	SessionID       string `gorm:"uniqueIndex"`
	VMState         []byte `gorm:"type:bytea"`
	FriendshipScore int
	MoodTag         string
}
```

## Implementation Details

### Machine Initialization

```go
wrapper, err := gnovm.NewMachineWrapper(gnovm.MachineOptions{
    PkgPath: "gno.land/p/myagent",
    Store:   myPostgresDB, // Optional, defaults to memdb
    Source:  map[string]string{"agent.gno": src},
})
```

### Freeze/Thaw Cycle

```go
// Freeze
blob, err := wrapper.ExportState()

// Thaw
err := wrapper.RestoreState(blob)
```
