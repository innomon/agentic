# ADK-Go Patches

This directory contains a patched fork of `google.golang.org/adk` v0.2.0.

## Applied Patches

### 1. EventActions: Added missing fields (event.go)

**File:** `server/adkrest/internal/models/event.go`

The REST API `EventActions` struct was missing `TransferToAgent`, `Escalate`, and `SkipSummarization` fields that exist in `session.EventActions`.

**Changes:**
- Added `TransferToAgent string` field with JSON tag `transferToAgent,omitempty`
- Added `Escalate bool` field with JSON tag `escalate,omitempty`
- Added `SkipSummarization bool` field with JSON tag `skipSummarization,omitempty`
- Updated `FromSessionEvent()` and `ToSessionEvent()` to map these fields

**Impact:** The web UI can now see which agent is being called during `transfer_to_agent` operations.

### 2. SSE Handler: Fixed WriteHeader and error handling (runtime.go)

**File:** `server/adkrest/controllers/runtime.go`

Two issues fixed:

#### a) "superfluous response.WriteHeader" error

The original code called `rw.WriteHeader(http.StatusOK)` explicitly before streaming. When an error occurred later and the handler returned, `NewErrorHandler` tried to call `http.Error()` which internally calls `WriteHeader()` again.

**Fix:** Removed explicit `rw.WriteHeader(http.StatusOK)` call. Go's HTTP server automatically sends 200 OK on first write.

#### b) Errors written as plain text instead of SSE events

Errors were written as plain text: `"Error while running agent: %v\n"`, making them unparseable by the web UI.

**Fix:** 
- Create a synthetic `models.Event` with `ErrorCode` and `ErrorMessage`
- Send it via `flashEvent()` as proper SSE JSON
- Track `streamStarted` to know whether we can still return errors to `NewErrorHandler`

## Usage

This fork is activated via `replace` directive in `go.mod`:

```go
replace google.golang.org/adk => ./patches/adk-fork
```

## Upstream Issues

See `GITHUB_ISSUES.md` in this directory for issue templates to submit upstream.

Once these issues are fixed in the upstream ADK, remove the `replace` directive from `go.mod`.
