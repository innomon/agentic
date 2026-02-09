# 🤖 AGENTS.md: Contributor & Coding Agent Guide

Welcome, Agent. **Gnogent** is a high-integrity, stateful AI framework. To maintain the deterministic nature of the GnoVM while extending functionality, you must follow these architectural constraints.

## 🎯 High-Level Mission

Enhance Gnogent's capabilities (tools, memory, and security) without breaking the **Freeze/Thaw** cycle or violating the **Asymmetric Security** model.

---

## 🏗 Architectural Invariants

### 1. The GnoVM Sandbox (The "Brain")

* **Determinism is King:** No side effects (HTTP calls, time.Now, random numbers) inside `gno/agent.gno`. All external data must be passed in via arguments.
* **State Persistence:** Only package-level global variables are snapshotted. Do not use local function variables for data that must survive turn-to-turn.
* **Logic Isolation:** Gno logic should strictly handle *reasoning and history management*. Use Go-ADK for *execution and I/O*.

### 2. The Go Wrapper (The "Body")

* **Session Context:** Always extract `GnogentClaims` from the context before interacting with the database.
* **Snapshot Integrity:** Ensure `VMState` is never modified outside the `BeforeSave` and `AfterFind` hooks.

---

## 🛠 Extension Points (Where to Code)

### Adding New Tools (Capabilities)

If you need the agent to perform actions (e.g., "Check Weather", "Query Jira"):

1. **Define the Tool in Go:** Create a function in `internal/tools/`.
2. **Register in ADK:** Add the tool to the `llmagent.Config` in `cmd/agent/main.go`.
3. **Update Gno History:** Ensure the tool output is passed back to `agent.AddTurn()` to maintain the chain of thought.

### Enhancing Security

To add new custom claims (e.g., `subscription_level` or `org_id`):

1. Update `internal/auth/claims.go`.
2. Update the JWT generator script in `scripts/gen_test_token.go`.
3. Update the `SyncAuth` function in `gno/agent.gno`.

### Memory Pruning (Optimization)

To prevent the GnoVM heap from growing too large:

1. Modify `gno/agent.gno` to implement a sliding window for the `history` slice.
2. Use the `AddToPermanentMemory` hook to send pruned turns to the Postgres vector store.

---

## 🧪 Testing Protocol

Before submitting an enhancement, verify the following:

1. **Serialization Check:** Does the agent remember Turn 1 after the server process is killed and restarted?
2. **Security Check:** Does a JWT with a mismatched `userId` or `channel` correctly fail to "Thaw" the state?
3. **Deterministic Check:** Does the GnoVM produce the same output for the same input state?

---

## 📜 Coding Standards

* **Error Handling:** Always return wrapped errors in Go. In Gno, use `panic()` for illegal states that should prevent a snapshot save.
* **Naming:** Follow the `Gnogent` prefix for all project-specific modules.
* **Documentation:** Every new tool must be documented in the `README.md` under a "Capabilities" section.

---

## 🚀 Future Roadmap (High-Priority Tasks)

* [ ] **Vector Search Integration:** Connect the `SearchLongTermMemory` tool to the Postgres `pgvector` store.
* [ ] **Multi-Agent Orchestration:** Allow one GnoVM to "call" another GnoVM for specialized tasks.
* [ ] **Encrypted Snapshots:** Integrate the AES-GCM layer into the GORM hooks to encrypt the `VMState` blob.

---

