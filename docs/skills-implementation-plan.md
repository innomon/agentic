This plan outlines how to build a production-grade, skill-aware agent system using the **Go ADK**. It follows a modular architecture where skills are stored as data and agents are generated as lightweight workers managed by a central supervisor.

---

# IMPLEMENTATION_PLAN.md: Skill-Based Agent System (Go)

## 1. Project Architecture

The system will follow a "Registry" pattern to decouple the instruction logic (Markdown) from the execution engine (Go).

### Directory Structure

```text
.
├── cmd/
│   └── agent/main.go        # Entry point
├── internal/
│   ├── skills/              # Skill loading & parsing logic
│   ├── registry/            # Agent management & supervisor logic
│   └── mcp/                 # (Optional) MCP tool connectors
├── skills/                  # The "Knowledge Base"
│   ├── architect/
│   │   └── SKILL.md
│   └── security/
│       └── SKILL.md
└── go.mod

```

---

## 2. Phase 1: The Skill Parser

**Goal:** Convert raw `SKILL.md` files into structured Go objects.

* **Action:** Implement a `LoadSkills(dir string)` function using `os.ReadDir`.
* **Action:** Use a YAML parser (e.g., `gopkg.in/yaml.v3`) to extract metadata from frontmatter.
* **Requirement:** Ensure the instructions (everything after the second `---`) are captured as a clean string for the System Prompt.

---

## 3. Phase 2: Agent Registry & Supervisor

**Goal:** Create a central hub to manage worker agents and the routing logic.

* **Action:** Initialize a `map[string]*adk.Agent` to store pre-warmed worker agents.
* **Action:** Create a specialized **Supervisor Agent**.
* **Prompt:** "You are an orchestrator. Given the user prompt, return ONLY the name of the most relevant skill."


* **Action:** Implement the `Route(input string)` method:
1. Call Supervisor with the input.
2. Parse the returned skill name.
3. Retrieve the corresponding worker from the map.



---

## 4. Phase 3: Tools & Protocols (MCP)

**Goal:** Connect the skills to the real world.

* **Action:** In `internal/mcp`, set up clients for any external data sources.
* **Action:** Define which skills have access to which tools.
* *Example:* The `security` skill gets access to the `filesystem_tool`, but the `architect` skill does not.


* **Action:** Use `agent.RegisterTool()` within the registry to bind these capabilities during initialization.

---

## 5. Phase 4: Shared Context (The "State" Bridge)

**Goal:** Allow agents to pass findings to one another.

* **Action:** Implement a `SessionContext` struct to track conversation history.
* **Action:** Pass a pointer to this context into each worker's `Prompt` call.
* **Action:** Ensure the Supervisor summarizes previous worker outputs before handing off to the next worker.

---

## 6. Execution Roadmap

| Step | Task | Deliverable |
| --- | --- | --- |
| **01** | Setup Go Environment | `go.mod` with ADK and YAML dependencies. |
| **02** | File System Crawler | Logic to find all `SKILL.md` files in a directory. |
| **03** | Worker Initialization | Loop creating `adk.Agent` instances for every found skill. |
| **04** | Router Logic | The Supervisor prompt and selection logic. |
| **05** | CLI Interface | A basic `for` loop in `main.go` to accept user input. |

---

### Verification Checklist

* [ ] Does the Supervisor correctly identify the `security` skill vs the `architect` skill?
* [ ] Are the instructions from `SKILL.md` actually influencing the output?
* [ ] Does the system handle "No Skill Found" gracefully (e.g., fallback to a general assistant)?

---

