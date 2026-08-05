package okf

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGetTaxonomyHandler(t *testing.T) {
	tmpDir := t.TempDir()
	taxFile := filepath.Join(tmpDir, "taxonomy.md")
	content := "# OKF Taxonomy\n\n- `ai_agentic`: AI Agents"
	if err := os.WriteFile(taxFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	res, err := getTaxonomyHandler(context.Background(), map[string]any{
		"path": taxFile,
	})
	if err != nil {
		t.Fatalf("getTaxonomyHandler returned error: %v", err)
	}

	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("Expected map[string]any response, got %T", res)
	}

	if m["content"] != content {
		t.Errorf("Expected content %q, got %q", content, m["content"])
	}
}

func TestFileOpsHandler(t *testing.T) {
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "doc1.txt")
	os.WriteFile(f1, []byte("Hello OKF"), 0644)

	res, err := fileOpsHandler(context.Background(), map[string]any{
		"op":   "list",
		"path": tmpDir,
	})
	if err != nil {
		t.Fatalf("fileOpsHandler list error: %v", err)
	}

	m := res.(map[string]any)
	entries := m["entries"].([]map[string]any)
	if len(entries) != 1 || entries[0]["name"] != "doc1.txt" {
		t.Errorf("Unexpected entries output: %v", entries)
	}

	readRes, err := fileOpsHandler(context.Background(), map[string]any{
		"op":   "read",
		"path": f1,
	})
	if err != nil {
		t.Fatalf("fileOpsHandler read error: %v", err)
	}

	readMap := readRes.(map[string]any)
	if readMap["content"] != "Hello OKF" {
		t.Errorf("Expected content 'Hello OKF', got %q", readMap["content"])
	}
}

func TestRagChunkerAndFullTextIndex(t *testing.T) {
	text := "Paragraph 1 with #ai_agentic tag.\n\nParagraph 2 with `engineering` category."
	
	chunkRes, err := ragChunkerHandler(context.Background(), map[string]any{
		"text": text,
	})
	if err != nil {
		t.Fatalf("ragChunkerHandler error: %v", err)
	}

	cm := chunkRes.(map[string]any)
	if cm["total_chunks"] != 2 {
		t.Errorf("Expected 2 chunks, got %v", cm["total_chunks"])
	}

	indexRes, err := fulltextIndexHandler(context.Background(), map[string]any{
		"query": "engineering",
		"text":  text,
	})
	if err != nil {
		t.Fatalf("fulltextIndexHandler error: %v", err)
	}

	im := indexRes.(map[string]any)
	if im["match_count"] != 1 {
		t.Errorf("Expected 1 match, got %v", im["match_count"])
	}
}
