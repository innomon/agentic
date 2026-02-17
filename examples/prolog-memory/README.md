# Prolog Memory Example

A knowledge management agent that uses **logic-based memory** powered by an embedded Prolog interpreter ([ichiban/prolog](https://github.com/ichiban/prolog)).

## Features

- **Assert facts** — store structured knowledge as Prolog terms
- **Query with inference** — find information using logical unification
- **Triple-store relations** — `mem_rel(subject, predicate, object)` for graph-like data
- **Persistence** — save/load knowledge base to `.pl` files
- **Sandboxed** — dangerous predicates (`consult`, `halt`, etc.) are blocked

## Running

```bash
# From the project root
./agentic examples/prolog-memory/config.yaml console
```

## Example Session

```
User -> Remember that Alice likes pizza and Bob likes sushi
Agent -> (asserts mem_fact facts via logic_query tool)

User -> What does Alice like?
Agent -> (queries mem_fact(alice, likes, Food).) -> pizza

User -> Alice and Bob are friends
Agent -> (asserts mem_rel(alice, friends_with, bob))

User -> Who is friends with Bob?
Agent -> (queries mem_rel(Who, friends_with, bob).) -> alice

User -> Save the knowledge base
Agent -> (calls save action) -> saved to knowledge.pl
```

## Standard Predicates

| Predicate | Description |
|-----------|-------------|
| `mem_fact(AgentID, Key, Value)` | Basic key-value storage |
| `mem_rel(Subject, Predicate, Object)` | Triple-store relations |
| `mem_context(SessionID, Timestamp, Data)` | Temporal context |
| `agent_rule(RuleName, Head, Body)` | Dynamic rule injection |
