# ADK-Go GitHub Issues

Submit these issues to https://github.com/google/adk-go/issues

---

## Issue 1: REST API EventActions missing TransferToAgent and Escalate fields

**Title:** `[bug] REST API EventActions doesn't include TransferToAgent, Escalate, SkipSummarization fields`

**Labels:** `bug`

**Body:**

### Description

The REST API model `EventActions` in `server/adkrest/internal/models/event.go` is missing the `TransferToAgent`, `Escalate`, and `SkipSummarization` fields that exist in `session.EventActions`.

This prevents the web UI from knowing when an agent is transferring to another agent or escalating.

### Expected Behavior

The `EventActions` struct in the REST models should include all fields from `session.EventActions`:
- `TransferToAgent string`
- `Escalate bool`
- `SkipSummarization bool`

### Actual Behavior

Currently only `StateDelta` and `ArtifactDelta` are serialized:

```go
// server/adkrest/internal/models/event.go (lines 25-28)
type EventActions struct {
    StateDelta    map[string]any   `json:"stateDelta"`
    ArtifactDelta map[string]int64 `json:"artifactDelta"`
    // Missing: TransferToAgent, Escalate, SkipSummarization
}
```

Compare with `session.EventActions` (session/session.go lines 138-152):
```go
type EventActions struct {
    StateDelta        map[string]any
    ArtifactDelta     map[string]int64
    SkipSummarization bool
    TransferToAgent   string  // NOT serialized in REST API
    Escalate          bool    // NOT serialized in REST API
}
```

### Suggested Fix

Update `EventActions` in `server/adkrest/internal/models/event.go`:

```go
type EventActions struct {
    StateDelta        map[string]any   `json:"stateDelta"`
    ArtifactDelta     map[string]int64 `json:"artifactDelta"`
    TransferToAgent   string           `json:"transferToAgent,omitempty"`
    Escalate          bool             `json:"escalate,omitempty"`
    SkipSummarization bool             `json:"skipSummarization,omitempty"`
}
```

Also update `FromSessionEvent()` and `ToSessionEvent()` to include these fields.

### Environment

- ADK version: v0.2.0
- Go version: 1.24

---

## Issue 2: SSE handler causes "superfluous response.WriteHeader" when errors occur

**Title:** `[bug] RunSSEHandler causes "superfluous response.WriteHeader" on errors`

**Labels:** `bug`

**Body:**

### Description

The `RunSSEHandler` in `server/adkrest/controllers/runtime.go` explicitly calls `rw.WriteHeader(http.StatusOK)` before streaming events. When an error occurs later and the handler returns an error, the `NewErrorHandler` wrapper attempts to call `http.Error()` which internally calls `WriteHeader()` again, causing:

```
http: superfluous response.WriteHeader call from google.golang.org/adk/server/adkrest/internal/routers.(*RuntimeAPIRouter).Routes.NewErrorHandler.func2 (handlers.go:48)
```

### Root Cause

In `runtime.go` line 108:
```go
rw.WriteHeader(http.StatusOK)  // Called here first
for event, err := range resp {
    if err != nil {
        // Error written, handler returns...
    }
}
```

Then in `handlers.go` line 46-48:
```go
if statusErr, ok := err.(statusError); ok {
    http.Error(w, statusErr.Error(), statusErr.Status())  // Tries to WriteHeader again
}
```

### Suggested Fix

Remove the explicit `rw.WriteHeader(http.StatusOK)` call. Go's HTTP server will automatically send 200 OK on the first write.

```go
// BEFORE (runtime.go line 108):
rw.WriteHeader(http.StatusOK)

// AFTER:
// (remove this line - first fmt.Fprintf will implicitly set 200 OK)
```

### Environment

- ADK version: v0.2.0
- Go version: 1.24

---

## Issue 3: SSE handler doesn't send errors as proper SSE events

**Title:** `[bug] RunSSEHandler writes errors as plain text instead of SSE JSON events`

**Labels:** `bug`

**Related:** https://github.com/google/adk-python/issues/4244

**Body:**

### Description

When an error occurs during SSE streaming in `RunSSEHandler`, it writes the error as plain text instead of a properly formatted SSE event with JSON payload.

Current behavior (runtime.go lines 109-115):
```go
if err != nil {
    _, err := fmt.Fprintf(rw, "Error while running agent: %v\n", err)  // Plain text!
    // ...
}
```

This makes it difficult for the web UI to parse and display errors properly.

### Expected Behavior

Errors should be sent as proper SSE events with JSON payload:
```
data: {"id":"...","errorCode":"EXECUTION_ERROR","errorMessage":"actual error message","author":"system"}
```

### Suggested Fix

Create an error event and send it using `flashEvent()`:

```go
if err != nil {
    errorEvent := models.Event{
        ID:           uuid.NewString(),
        Time:         time.Now().Unix(),
        Author:       "system",
        ErrorCode:    "EXECUTION_ERROR",
        ErrorMessage: err.Error(),
    }
    flashEvent(flusher, rw, errorEvent)
    continue
}
```

### Environment

- ADK version: v0.2.0
- Go version: 1.24
