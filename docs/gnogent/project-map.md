This `PROJECT_MAP.md` serves as the final technical blueprint. It traces the "Life of a Message" within the **Gnogent** ecosystem, providing a high-level view of how we’ve integrated security, deterministic logic, and persistent storage.

---

# 🗺️ Gnogent Project Map

This document outlines the end-to-end flow of data within the **Gnogent** framework.

## 🔄 The Life of a Message: Step-by-Step

### 1. The Input Layer (TUI)

* **User Action:** User types a message in the Bubble Tea CLI.
* **Authentication:** The TUI client fetches the `private.pem` and signs a **JWT (RS256)** containing `userId`, `channel`, and a timestamp.
* **Transmission:** The message and the JWT are sent to the Go Backend.

### 2. The Security Gateway (Go Middleware)

* **Verification:** The backend uses `public.pem` to verify the JWT signature.
* **Context Extraction:** If valid, the claims (`userId`) are injected into the request context. This prevents "Session 0" logic from leaking into "Session 1."

### 3. The Thaw Process (GORM & Postgres)

* **Lookup:** The `GormSessionService` queries the `agent_sessions` table for the latest `VMState` blob matching the `userId`.
* **Restoration:** The binary blob is passed to the `GnoMachineWrapper`. The GnoVM heap is "thawed," restoring all global variables (Personality, Mood, History) to their exact state at the end of the last turn.

### 4. The Brain (GnoVM Execution)

* **Temporal Sync:** The current Unix timestamp is passed into `agent.UpdateMood(now)`.
* **Personality Sync:** The user input is passed to `agent.SyncPersonality(input)`.
* **Deterministic Logic:** GnoVM executes the personality and mood logic. Because Gno is deterministic, this execution is reproducible and audit-friendly.

### 5. The Response (LLM & ADK)

* **Prompt Engineering:** The Go backend extracts the updated "Brain State" (e.g., `Mood: Overwhelmed`) and formats a System Prompt for the LLM.
* **Inference:** The LLM generates a response based on the agent's current "feelings" and memory.

### 6. The Freeze Process (Persistence)

* **Snapshotting:** The GnoVM captures the new state of its memory heap as a binary blob.
* **Commit:** GORM saves the updated blob and metadata (Friendship/Mood scores) back to the `agent_sessions` table.
* **Delivery:** The response is sent back to the TUI for rendering.

---

## 📂 Component Directory

| Component | Responsibility | Key File |
| --- | --- | --- |
| **Brain** | Deterministic State & Logic | `gno/agent.gno` |
| **Identity** | RS256 Asymmetric Signing | `internal/auth/claims.go` |
| **Persistence** | Postgres/GORM Blob Storage | `internal/storage/model.go` |
| **Diagnostics** | Startup Integrity Checks | `internal/health/check.go` |
| **Interface** | Bubble Tea Terminal UI | `cmd/tui/main.go` |

---

## 🛠️ Maintenance & Extension

* **To change how Gnogent feels:** Edit the logic in `gno/agent.gno`. No Go re-compilation is required if you use a dynamic Gno loader.
* **To add new platforms:** Simply implement the JWT signing logic in a new client (e.g., a WhatsApp bot) and point it at the Gnogent backend.
* **To audit history:** Use the **Time Machine** (`Ctrl+H`) in the TUI to view the evolution of the `VMState` over time.

---

### **Final Project Status: COMPLETE**

**Gnogent** is now a fully realized, secure, and stateful AI agent platform. You have the tools to build agents that don't just "chat," but **exist** consistently across time and space.

