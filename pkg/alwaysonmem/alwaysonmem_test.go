package alwaysonmem

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAlwaysOnMemoryStoreAndRead(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alwaysonmem_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_memory.db")
	SetDBPath(dbPath)

	ctx := context.Background()

	// 1. Test store_memory tool handler
	storeRes, err := storeMemoryHandler(ctx, map[string]any{
		"raw_text":   "Anthropic reports 62% of Claude usage is code-related.",
		"summary":    "62% of Claude usage is code-related.",
		"entities":   []string{"Anthropic", "Claude"},
		"topics":     []string{"AI", "Code"},
		"importance": 0.8,
		"source":     "test_note.txt",
	})
	if err != nil {
		t.Fatalf("storeMemoryHandler failed: %v", err)
	}

	mMap, ok := storeRes.(map[string]any)
	if !ok || mMap["status"] != "stored" {
		t.Fatalf("Unexpected store result: %v", storeRes)
	}

	// 2. Test get_memory_stats tool handler
	statsRes, err := getMemoryStatsHandler(ctx, nil)
	if err != nil {
		t.Fatalf("getMemoryStatsHandler failed: %v", err)
	}
	stats, ok := statsRes.(map[string]any)
	if !ok || stats["total_memories"].(int64) != 1 {
		t.Fatalf("Unexpected stats result: %v", statsRes)
	}

	// 3. Test read_unconsolidated_memories tool handler
	unconsRes, err := readUnconsolidatedMemoriesHandler(ctx, nil)
	if err != nil {
		t.Fatalf("readUnconsolidatedMemoriesHandler failed: %v", err)
	}
	unconsMap, ok := unconsRes.(map[string]any)
	if !ok || unconsMap["count"].(int) != 1 {
		t.Fatalf("Unexpected unconsolidated result: %v", unconsRes)
	}

	// 4. Test store_consolidation tool handler
	consRes, err := storeConsolidationHandler(ctx, map[string]any{
		"source_ids": []uint{1},
		"summary":    "Code generation is a major use case for AI assistants.",
		"insight":    "Developers are heavily adopting LLMs for coding tasks.",
		"connections": []map[string]any{
			{"from_id": 1, "to_id": 1, "relationship": "self-reflection"},
		},
	})
	if err != nil {
		t.Fatalf("storeConsolidationHandler failed: %v", err)
	}
	cMap, ok := consRes.(map[string]any)
	if !ok || cMap["status"] != "consolidated" {
		t.Fatalf("Unexpected consolidation result: %v", consRes)
	}

	// 5. Test read_all_memories tool handler
	readAllRes, err := readAllMemoriesHandler(ctx, nil)
	if err != nil {
		t.Fatalf("readAllMemoriesHandler failed: %v", err)
	}
	allMap, ok := readAllRes.(map[string]any)
	if !ok || allMap["count"].(int) != 1 {
		t.Fatalf("Unexpected read_all result: %v", readAllRes)
	}

	// 6. Test clear_all_memories tool handler
	clearRes, err := clearAllMemoriesHandler(ctx, nil)
	if err != nil {
		t.Fatalf("clearAllMemoriesHandler failed: %v", err)
	}
	clrMap, ok := clearRes.(map[string]any)
	if !ok || clrMap["status"] != "cleared" {
		t.Fatalf("Unexpected clear result: %v", clearRes)
	}
}
