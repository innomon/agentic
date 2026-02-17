package prologmem

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ichiban/prolog"
)

// DefaultTimeout is the default timeout for Prolog queries.
const DefaultTimeout = 5 * time.Second

// dangerousPredicates lists Prolog predicates that must be rejected to
// prevent code injection from LLM-generated input.
var dangerousPredicates = []string{
	"use_module", "consult", "load_files", "ensure_loaded",
	"open", "close", "read", "write", "put_char", "get_char",
	"halt", "shell", "system",
}

// PrologMemory provides a logic-based memory backed by an embedded
// Prolog interpreter (ichiban/prolog). All operations are thread-safe.
type PrologMemory struct {
	interp  *prolog.Interpreter
	kbPath  string
	timeout time.Duration
	mu      sync.RWMutex
}

// Option configures a PrologMemory instance.
type Option func(*PrologMemory)

// WithTimeout sets the query timeout (default 5s).
func WithTimeout(d time.Duration) Option {
	return func(pm *PrologMemory) { pm.timeout = d }
}

// New creates a PrologMemory. If kbPath is non-empty and the file exists,
// its contents are loaded into the interpreter at startup.
func New(kbPath string, opts ...Option) (*PrologMemory, error) {
	pm := &PrologMemory{
		interp:  prolog.New(nil, nil),
		kbPath:  kbPath,
		timeout: DefaultTimeout,
	}
	for _, o := range opts {
		o(pm)
	}

	// Bootstrap standard predicates.
	if err := pm.interp.Exec(`:- dynamic(mem_fact/3).`); err != nil {
		return nil, fmt.Errorf("bootstrap mem_fact: %w", err)
	}
	if err := pm.interp.Exec(`:- dynamic(mem_rel/3).`); err != nil {
		return nil, fmt.Errorf("bootstrap mem_rel: %w", err)
	}
	if err := pm.interp.Exec(`:- dynamic(mem_context/3).`); err != nil {
		return nil, fmt.Errorf("bootstrap mem_context: %w", err)
	}
	if err := pm.interp.Exec(`:- dynamic(agent_rule/3).`); err != nil {
		return nil, fmt.Errorf("bootstrap agent_rule: %w", err)
	}

	if kbPath != "" {
		if err := pm.loadFile(kbPath); err != nil {
			return nil, err
		}
	}

	return pm, nil
}

// Assert adds a fact to the knowledge base using assertz.
func (pm *PrologMemory) Assert(fact string) error {
	if err := sanitize(fact); err != nil {
		return err
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return pm.execGoal(fmt.Sprintf("assertz(%s).", fact))
}

// Retract removes the first matching fact from the knowledge base.
func (pm *PrologMemory) Retract(fact string) error {
	if err := sanitize(fact); err != nil {
		return err
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return pm.execGoal(fmt.Sprintf("retract(%s).", fact))
}

// execGoal runs a Prolog goal via Query+Next (ichiban/prolog requires
// this for side-effecting goals like assertz/retract).
func (pm *PrologMemory) execGoal(goal string) error {
	ctx, cancel := context.WithTimeout(context.Background(), pm.timeout)
	defer cancel()

	sols, err := pm.interp.QueryContext(ctx, goal)
	if err != nil {
		return err
	}
	defer sols.Close()

	sols.Next()
	return sols.Err()
}

// Query executes a Prolog goal and returns all variable bindings.
func (pm *PrologMemory) Query(goal string) ([]map[string]any, error) {
	if err := sanitize(goal); err != nil {
		return nil, err
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), pm.timeout)
	defer cancel()

	sols, err := pm.interp.QueryContext(ctx, goal)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer sols.Close()

	var results []map[string]any
	for sols.Next() {
		m := make(map[string]any)
		if err := sols.Scan(&m); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		results = append(results, m)
	}
	if err := sols.Err(); err != nil {
		return nil, fmt.Errorf("solution error: %w", err)
	}

	return results, nil
}

// Check returns true if the goal succeeds.
func (pm *PrologMemory) Check(goal string) (bool, error) {
	if err := sanitize(goal); err != nil {
		return false, err
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), pm.timeout)
	defer cancel()

	sols, err := pm.interp.QueryContext(ctx, goal)
	if err != nil {
		return false, fmt.Errorf("check error: %w", err)
	}
	defer sols.Close()

	ok := sols.Next()
	if err := sols.Err(); err != nil {
		return false, fmt.Errorf("check error: %w", err)
	}
	return ok, nil
}

// Save writes all facts for the standard predicates to the .pl file.
func (pm *PrologMemory) Save() error {
	if pm.kbPath == "" {
		return nil
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var buf strings.Builder

	predicates := []struct {
		functor string
		arity   int
	}{
		{"mem_fact", 3},
		{"mem_rel", 3},
		{"mem_context", 3},
		{"agent_rule", 3},
	}

	for _, p := range predicates {
		if err := pm.dumpPredicate(&buf, p.functor, p.arity); err != nil {
			return fmt.Errorf("dumping %s/%d: %w", p.functor, p.arity, err)
		}
	}

	return os.WriteFile(pm.kbPath, []byte(buf.String()), 0644)
}

// dumpPredicate writes all clauses of functor/arity to buf.
func (pm *PrologMemory) dumpPredicate(buf *strings.Builder, functor string, arity int) error {
	// Build a query with anonymous-like variables.
	vars := make([]string, arity)
	for i := range arity {
		vars[i] = fmt.Sprintf("V%d", i)
	}
	goal := fmt.Sprintf("%s(%s).", functor, strings.Join(vars, ", "))

	ctx, cancel := context.WithTimeout(context.Background(), pm.timeout)
	defer cancel()

	sols, err := pm.interp.QueryContext(ctx, goal)
	if err != nil {
		return err
	}
	defer sols.Close()

	for sols.Next() {
		m := make(map[string]any)
		if err := sols.Scan(&m); err != nil {
			return err
		}
		vals := make([]string, arity)
		for i := range arity {
			vals[i] = formatTerm(m[vars[i]])
		}
		fmt.Fprintf(buf, "%s(%s).\n", functor, strings.Join(vals, ", "))
	}
	return sols.Err()
}

// loadFile reads a .pl file and executes its contents.
func (pm *PrologMemory) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading KB file: %w", err)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil
	}
	if err := pm.interp.Exec(content); err != nil {
		return fmt.Errorf("loading KB file: %w", err)
	}
	return nil
}

// sanitize rejects input containing dangerous predicates.
func sanitize(input string) error {
	lower := strings.ToLower(input)
	for _, pred := range dangerousPredicates {
		if strings.Contains(lower, pred) {
			return fmt.Errorf("rejected: input contains forbidden predicate %q", pred)
		}
	}
	return nil
}

// formatTerm converts a Go value to a Prolog term string.
func formatTerm(v any) string {
	switch val := v.(type) {
	case string:
		if isAtom(val) {
			return val
		}
		return fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "\\'"))
	case int, int64, float64:
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// isAtom returns true if s looks like a simple Prolog atom (lowercase start, alnum/underscore).
func isAtom(s string) bool {
	if s == "" {
		return false
	}
	if s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for _, r := range s[1:] {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}
