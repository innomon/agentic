# SPECIFICATION: Agentic Prolog Memory Provider

## 1. Overview

The goal is to implement a logic-based memory module for the [Agentic](https://github.com/innomon/agentic) framework. Unlike standard vector or key-value stores, **Agentic Prolog Memory** allows agents to store facts, define rules, and perform logical inference using the [ichiban/prolog](https://github.com/ichiban/prolog) interpreter.

## 2. Technical Stack

* **Language:** Go (Golang)
* **Base Framework:** `github.com/innomon/agentic` (ADK/Go-SDK)
* **Logic Engine:** `github.com/ichiban/prolog`
* **Data Format:** Prolog (.pl) files or in-memory fact bases.

## 3. Architecture

### 3.1 Memory Provider Interface

The Prolog Memory must implement the Agentic `Memory` interface (or a specialized `LogicMemory` extension).

```go
type PrologMemory struct {
    interpreter *prolog.Interpreter
    kbPath      string // Path to the .pl knowledge base file
    mutex       sync.RWMutex
}

```

### 3.2 Component Interaction

1. **Ingestion:** The agent receives information and "asserts" it as a Prolog fact.
2. **Reasoning:** The agent runs a query against the interpreter to check consistency or derive new information.
3. **Persistence:** The knowledge base (KB) is synced to a `.pl` file to maintain state across agent restarts.

## 4. Logical Schema (Predicates)

To ensure interoperability, the following standard predicates are reserved:

* `mem_fact(AgentID, Key, Value)`: Basic key-value storage.
* `mem_rel(Subject, Predicate, Object)`: Triple-store style relations.
* `mem_context(SessionID, Timestamp, Data)`: Temporal context.
* `agent_rule(RuleName, Head, Body)`: Dynamic rule injection.

## 5. Implementation Requirements

### 5.1 Interpreter Initialization

The memory provider must initialize the `ichiban/prolog` interpreter with the standard library and any custom Go-defined predicates (e.g., logging or external API calls).

### 5.2 Thread Safety

Since `ichiban/prolog` is not inherently thread-safe for concurrent assertions/retractions, the `PrologMemory` struct must wrap calls in a `sync.RWMutex`.

### 5.3 Methods to Implement

#### `Assert(fact string) error`

Converts input into a Prolog fact and executes `assertz(fact).`.
*Example:* `memory.Assert("user_likes(user_1, pizza)")`

#### `Query(goal string) ([]Map, error)`

Executes a query and returns a list of variable bindings.
*Example:* `memory.Query("user_likes(user_1, Food)")` -> `[{"Food": "pizza"}]`

#### `Retract(fact string) error`

Removes a fact from the knowledge base using `retract(fact).`.

#### `Check(goal string) bool`

A helper that returns true/false if a logic goal is met (e.g., checking permissions or constraints).

## 6. Agentic Integration (ADK)

The memory provider should be registered within the Agentic lifecycle:

```go
// Example Registration
func NewPrologMemoryProvider(path string) *PrologMemory {
    i := prolog.New(nil, nil) // Initialize ichiban interpreter
    return &PrologMemory{interpreter: i, kbPath: path}
}

```

### Skill/Tool Integration

Agents should have a "Reasoning Tool" that exposes the Prolog engine:

1. **Tool Name:** `logic_query`
2. **Input:** A Prolog query string.
3. **Output:** The logical solution or "No" if the proof fails.

## 7. Development Tasks (Roadmap)

1. **Phase 1: Embedding.** Integrate `ichiban/prolog` into a standalone Go struct that satisfies the `agentic` memory patterns.
2. **Phase 2: Persistence.** Implement logic to load/save the `.pl` file on disk.
3. **Phase 3: ADK Wrapper.** Create the Agentic Tool/Skill wrapper so LLMs can generate Prolog queries to search their own memory.
4. **Phase 4: Optimization.** Implement a "working memory" (volatile) vs "long-term memory" (persistent) distinction using Prolog modules.

## 8. Safety & Constraints

* **Timeout:** All Prolog queries must be wrapped in a context with a timeout to prevent infinite loops (e.g., recursive rules).
* **Sanitization:** Input from the LLM must be sanitized to prevent malicious code injection via `use_module` or `consult`.

---

**Reference Example (Embedding Pattern):**

```go
// Based on ichiban/prolog embed example
i := prolog.New(nil, nil)
err := i.Exec(`assertz(likes(alice, prolog)).`)
sols, err := i.Query(`likes(alice, X).`)
// Iterate solutions...

```

