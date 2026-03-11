package prologmem

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestAssertAndQuery(t *testing.T) {
	pm, err := New("")
	if err != nil {
		t.Fatal(err)
	}

	if err := pm.Assert("mem_fact(agent1, name, alice)"); err != nil {
		t.Fatal(err)
	}
	if err := pm.Assert("mem_fact(agent1, likes, pizza)"); err != nil {
		t.Fatal(err)
	}

	results, err := pm.Query("mem_fact(agent1, Key, Value).")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestCheck(t *testing.T) {
	pm, err := New("")
	if err != nil {
		t.Fatal(err)
	}

	if err := pm.Assert("mem_fact(a, b, c)"); err != nil {
		t.Fatal(err)
	}

	ok, err := pm.Check("mem_fact(a, b, c).")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected true")
	}

	ok, err = pm.Check("mem_fact(a, b, d).")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected false")
	}
}

func TestRetract(t *testing.T) {
	pm, err := New("")
	if err != nil {
		t.Fatal(err)
	}

	if err := pm.Assert("mem_fact(a, k, v)"); err != nil {
		t.Fatal(err)
	}

	ok, err := pm.Check("mem_fact(a, k, v).")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("fact should exist before retract")
	}

	if err := pm.Retract("mem_fact(a, k, v)"); err != nil {
		t.Fatal(err)
	}

	ok, err = pm.Check("mem_fact(a, k, v).")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("fact should not exist after retract")
	}
}

func TestSanitization(t *testing.T) {
	pm, err := New("")
	if err != nil {
		t.Fatal(err)
	}

	cases := []string{
		"consult('/etc/passwd')",
		"use_module(library(system))",
		"load_files(evil)",
		"halt",
	}
	for _, c := range cases {
		if err := pm.Assert(c); err == nil {
			t.Errorf("expected error for %q, got nil", c)
		}
		if _, err := pm.Query(c + "."); err == nil {
			t.Errorf("expected error for query %q, got nil", c)
		}
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kb.pl")

	pm, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := pm.Assert("mem_fact(a1, color, red)"); err != nil {
		t.Fatal(err)
	}
	if err := pm.Assert("mem_rel(alice, likes, bob)"); err != nil {
		t.Fatal(err)
	}
	if err := pm.Save(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if content == "" {
		t.Fatal("KB file should not be empty")
	}

	// Reload into new interpreter.
	pm2, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := pm2.Check("mem_fact(a1, color, red).")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("fact should survive persistence round-trip")
	}
	ok, err = pm2.Check("mem_rel(alice, likes, bob).")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("relation should survive persistence round-trip")
	}
}

func TestConcurrency(t *testing.T) {
	pm, err := New("")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = pm.Assert("mem_fact(agent, key, val)")
			_, _ = pm.Query("mem_fact(agent, key, Value).")
			_, _ = pm.Check("mem_fact(agent, key, val).")
			_ = i
		}(i)
	}
	wg.Wait()
}

func TestTimeout(t *testing.T) {
	pm, err := New("", WithTimeout(50*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	// A terminating query should succeed within the timeout.
	if err := pm.Assert("mem_fact(a, b, c)"); err != nil {
		t.Fatal(err)
	}
	ok, err := pm.Check("mem_fact(a, b, c).")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	pm, err := New("/nonexistent/path/kb.pl")
	if err != nil {
		t.Fatal("should not error on non-existent KB file")
	}
	if pm == nil {
		t.Fatal("should return valid PrologMemory")
	}
}

func TestRelations(t *testing.T) {
	pm, err := New("")
	if err != nil {
		t.Fatal(err)
	}

	if err := pm.Assert("mem_rel(alice, knows, bob)"); err != nil {
		t.Fatal(err)
	}
	if err := pm.Assert("mem_rel(bob, knows, charlie)"); err != nil {
		t.Fatal(err)
	}

	results, err := pm.Query("mem_rel(alice, knows, Who).")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}
