# ADK-Go Patches and Upstream Contributions

This document describes the patches applied to Google's ADK-Go library and how to contribute fixes upstream.

## Overview

We discovered three bugs in ADK-Go v0.2.0 that affect the web UI experience:

1. **TransferToAgent not visible**: The REST API doesn't expose agent transfer information
2. **Superfluous WriteHeader error**: SSE handler causes server errors when exceptions occur
3. **Errors not as SSE events**: Runtime errors are sent as plain text instead of JSON events

## Current Status

- **Local patches applied**: `patches/adk-fork/` contains a patched ADK v0.2.0
- **Activated via go.mod**: `replace google.golang.org/adk => ./patches/adk-fork`
- **Upstream issues**: Not yet submitted

## Detailed Changes

### Issue 1: TransferToAgent Missing from REST API

**Problem**: When an agent calls `transfer_to_agent`, the web UI cannot display which agent is being transferred to because the REST API model doesn't include this field.

**Root Cause**: The `EventActions` struct in `server/adkrest/internal/models/event.go` only has:
```go
type EventActions struct {
    StateDelta    map[string]any   `json:"stateDelta"`
    ArtifactDelta map[string]int64 `json:"artifactDelta"`
    // Missing: TransferToAgent, Escalate, SkipSummarization
}
```

But `session.EventActions` has these additional fields:
```go
type EventActions struct {
    StateDelta        map[string]any
    ArtifactDelta     map[string]int64
    SkipSummarization bool
    TransferToAgent   string   // NOT serialized!
    Escalate          bool     // NOT serialized!
}
```

**Fix Applied**: Added the missing fields to the REST model and updated `FromSessionEvent()`/`ToSessionEvent()` functions.

---

### Issue 2: Superfluous WriteHeader Error

**Problem**: Server logs show:
```
http: superfluous response.WriteHeader call from google.golang.org/adk/server/adkrest/internal/routers.(*RuntimeAPIRouter).Routes.NewErrorHandler.func2 (handlers.go:48)
```

**Root Cause**: In `server/adkrest/controllers/runtime.go`, the SSE handler does:
```go
rw.WriteHeader(http.StatusOK)  // Line 108 - called first
for event, err := range resp {
    if err != nil {
        // ... handler returns error
    }
}
```

Then `NewErrorHandler` in `handlers.go` tries to call `http.Error()` which internally calls `WriteHeader()` again - violating Go's HTTP contract.

**Fix Applied**: Removed the explicit `rw.WriteHeader(http.StatusOK)` call. Go's HTTP server automatically sends 200 OK on the first write.

---

### Issue 3: Errors Sent as Plain Text

**Problem**: When an error occurs during SSE streaming, the client receives:
```
Error while running agent: some error message
```

This is plain text, not valid SSE format, so the web UI can't parse or display it properly.

**Root Cause**: In `runtime.go`:
```go
if err != nil {
    fmt.Fprintf(rw, "Error while running agent: %v\n", err)  // Plain text!
}
```

**Fix Applied**: Create a synthetic error event and send it as proper SSE JSON:
```go
errorEvent := models.Event{
    ID:           uuid.NewString(),
    Time:         time.Now().Unix(),
    Author:       "system",
    ErrorCode:    "EXECUTION_ERROR",
    ErrorMessage: err.Error(),
}
flashEvent(flusher, rw, errorEvent)
```

---

## How to Submit Upstream Issues

### Step 1: Create GitHub Account (if needed)

Go to https://github.com and sign up.

### Step 2: Navigate to ADK-Go Issues

Open https://github.com/google/adk-go/issues

### Step 3: Submit Each Issue

Click "New issue" and use the templates below. Submit them as **3 separate issues**.

---

### Issue Template 1: TransferToAgent Missing

**Title**: `[bug] REST API EventActions doesn't include TransferToAgent, Escalate, SkipSummarization fields`

**Labels**: `bug`

**Body**:
```markdown
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
```

---

### Issue Template 2: Superfluous WriteHeader

**Title**: `[bug] RunSSEHandler causes "superfluous response.WriteHeader" on errors`

**Labels**: `bug`

**Body**:
```markdown
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
```

---

### Issue Template 3: SSE Error Events

**Title**: `[bug] RunSSEHandler writes errors as plain text instead of SSE JSON events`

**Labels**: `bug`

**Body**:
```markdown
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

### Related Issue

https://github.com/google/adk-python/issues/4244 (same issue in Python SDK)

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
```

---

## After Upstream Fixes

Once Google merges fixes for these issues:

1. Update ADK version in `go.mod`:
   ```go
   google.golang.org/adk v0.X.X  // New version with fixes
   ```

2. Remove the replace directive:
   ```go
   // DELETE THIS LINE:
   replace google.golang.org/adk => ./patches/adk-fork
   ```

3. Run `go mod tidy`

4. Optionally delete `patches/adk-fork/` directory

## Files Reference

| File | Purpose |
|------|---------|
| `patches/adk-fork/` | Complete patched ADK v0.2.0 fork |
| `patches/adk-fork/PATCHES.md` | Technical patch documentation |
| `patches/adk-fork/GITHUB_ISSUES.md` | Raw issue templates |
| `go.mod` | Contains `replace` directive |
