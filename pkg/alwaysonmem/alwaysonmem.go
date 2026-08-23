package alwaysonmem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/innomon/agentic/pkg/registry"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Memory struct {
	ID           uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	Source       string  `gorm:"default:''" json:"source"`
	RawText      string  `gorm:"not null" json:"raw_text"`
	Summary      string  `gorm:"not null" json:"summary"`
	Entities     string  `gorm:"default:'[]'" json:"entities"`
	Topics       string  `gorm:"default:'[]'" json:"topics"`
	Connections  string  `gorm:"default:'[]'" json:"connections"`
	Importance   float64 `gorm:"default:0.5" json:"importance"`
	CreatedAt    string  `gorm:"not null" json:"created_at"`
	Consolidated int     `gorm:"default:0" json:"consolidated"`
}

type Consolidation struct {
	ID        uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	SourceIDs string `gorm:"not null" json:"source_ids"`
	Summary   string `gorm:"not null" json:"summary"`
	Insight   string `gorm:"not null" json:"insight"`
	CreatedAt string `gorm:"not null" json:"created_at"`
}

type ProcessedFile struct {
	Path        string `gorm:"primaryKey" json:"path"`
	ProcessedAt string `gorm:"not null" json:"processed_at"`
}

var (
	dbInstance *gorm.DB
	dbOnce     sync.Once
	dbErr      error
)

// GetDB returns the singleton GORM database instance for memory storage.
func GetDB() (*gorm.DB, error) {
	dbOnce.Do(func() {
		dbPath := os.Getenv("MEMORY_DB")
		if dbPath == "" {
			dbPath = "memory.db"
		}
		if registry.BasePath != "" && !filepath.IsAbs(dbPath) {
			dbPath = filepath.Join(registry.BasePath, dbPath)
		}

		db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			dbErr = fmt.Errorf("failed to open sqlite db at %s: %w", dbPath, err)
			return
		}

		if err := db.AutoMigrate(&Memory{}, &Consolidation{}, &ProcessedFile{}); err != nil {
			dbErr = fmt.Errorf("failed to auto-migrate tables: %w", err)
			return
		}

		dbInstance = db
	})
	return dbInstance, dbErr
}

// ResetDBForTest resets the once guard if needed.
func SetDBPath(path string) {
	os.Setenv("MEMORY_DB", path)
}

func storeMemoryHandler(_ context.Context, args map[string]any) (any, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	rawText, _ := args["raw_text"].(string)
	summary, _ := args["summary"].(string)
	source, _ := args["source"].(string)

	var importance float64 = 0.5
	if v, ok := args["importance"].(float64); ok {
		importance = v
	} else if v, ok := args["importance"].(int); ok {
		importance = float64(v)
	}

	entitiesJSON := marshalSlice(args["entities"])
	topicsJSON := marshalSlice(args["topics"])

	now := time.Now().UTC().Format(time.RFC3339)
	mem := Memory{
		Source:       source,
		RawText:      rawText,
		Summary:      summary,
		Entities:     entitiesJSON,
		Topics:       topicsJSON,
		Connections:  "[]",
		Importance:   importance,
		CreatedAt:    now,
		Consolidated: 0,
	}

	if err := db.Create(&mem).Error; err != nil {
		return nil, fmt.Errorf("failed to store memory: %w", err)
	}

	return map[string]any{
		"memory_id": mem.ID,
		"status":    "stored",
		"summary":   summary,
	}, nil
}

func readAllMemoriesHandler(_ context.Context, _ map[string]any) (any, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var rows []Memory
	if err := db.Order("created_at desc").Limit(50).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query memories: %w", err)
	}

	var memories []map[string]any
	for _, r := range rows {
		memories = append(memories, map[string]any{
			"id":           r.ID,
			"source":       r.Source,
			"summary":      r.Summary,
			"entities":     unmarshalJSON(r.Entities),
			"topics":       unmarshalJSON(r.Topics),
			"importance":   r.Importance,
			"connections":  unmarshalJSON(r.Connections),
			"created_at":   r.CreatedAt,
			"consolidated": r.Consolidated == 1,
		})
	}

	return map[string]any{
		"memories": memories,
		"count":    len(memories),
	}, nil
}

func readUnconsolidatedMemoriesHandler(_ context.Context, _ map[string]any) (any, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var rows []Memory
	if err := db.Where("consolidated = ?", 0).Order("created_at desc").Limit(10).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query unconsolidated memories: %w", err)
	}

	var memories []map[string]any
	for _, r := range rows {
		memories = append(memories, map[string]any{
			"id":         r.ID,
			"summary":    r.Summary,
			"entities":   unmarshalJSON(r.Entities),
			"topics":     unmarshalJSON(r.Topics),
			"importance": r.Importance,
			"created_at": r.CreatedAt,
		})
	}

	return map[string]any{
		"memories": memories,
		"count":    len(memories),
	}, nil
}

func storeConsolidationHandler(_ context.Context, args map[string]any) (any, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	sourceIDs := parseIDList(args["source_ids"])
	summary, _ := args["summary"].(string)
	insight, _ := args["insight"].(string)

	sourceIDsJSON, _ := json.Marshal(sourceIDs)
	now := time.Now().UTC().Format(time.RFC3339)

	cons := Consolidation{
		SourceIDs: string(sourceIDsJSON),
		Summary:   summary,
		Insight:   insight,
		CreatedAt: now,
	}

	if err := db.Create(&cons).Error; err != nil {
		return nil, fmt.Errorf("failed to store consolidation: %w", err)
	}

	// Update connections if provided
	if rawConns, ok := args["connections"].([]any); ok {
		for _, c := range rawConns {
			if connMap, ok := c.(map[string]any); ok {
				fromID := getIntFromMap(connMap, "from_id")
				toID := getIntFromMap(connMap, "to_id")
				rel, _ := connMap["relationship"].(string)
				if fromID > 0 && toID > 0 {
					addConnection(db, uint(fromID), uint(toID), rel)
					addConnection(db, uint(toID), uint(fromID), rel)
				}
			}
		}
	}

	if len(sourceIDs) > 0 {
		db.Model(&Memory{}).Where("id IN ?", sourceIDs).Update("consolidated", 1)
	}

	return map[string]any{
		"status":             "consolidated",
		"memories_processed": len(sourceIDs),
		"insight":            insight,
	}, nil
}

func readConsolidationHistoryHandler(_ context.Context, _ map[string]any) (any, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var rows []Consolidation
	if err := db.Order("created_at desc").Limit(10).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to read consolidations: %w", err)
	}

	var consolidations []map[string]any
	for _, r := range rows {
		consolidations = append(consolidations, map[string]any{
			"summary":    r.Summary,
			"insight":    r.Insight,
			"source_ids": unmarshalJSON(r.SourceIDs),
			"created_at": r.CreatedAt,
		})
	}

	return map[string]any{
		"consolidations": consolidations,
		"count":          len(consolidations),
	}, nil
}

func getMemoryStatsHandler(_ context.Context, _ map[string]any) (any, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var total int64
	var unconsolidated int64
	var consolidations int64

	db.Model(&Memory{}).Count(&total)
	db.Model(&Memory{}).Where("consolidated = ?", 0).Count(&unconsolidated)
	db.Model(&Consolidation{}).Count(&consolidations)

	return map[string]any{
		"total_memories": total,
		"unconsolidated": unconsolidated,
		"consolidations": consolidations,
	}, nil
}

func deleteMemoryHandler(_ context.Context, args map[string]any) (any, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	mid := getIntFromMap(args, "memory_id")
	if mid <= 0 {
		return nil, fmt.Errorf("missing or invalid 'memory_id'")
	}

	var count int64
	db.Model(&Memory{}).Where("id = ?", mid).Count(&count)
	if count == 0 {
		return map[string]any{"status": "not_found", "memory_id": mid}, nil
	}

	db.Where("id = ?", mid).Delete(&Memory{})
	return map[string]any{"status": "deleted", "memory_id": mid}, nil
}

func clearAllMemoriesHandler(_ context.Context, args map[string]any) (any, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var memCount int64
	db.Model(&Memory{}).Count(&memCount)

	db.Exec("DELETE FROM memories")
	db.Exec("DELETE FROM consolidations")
	db.Exec("DELETE FROM processed_files")

	filesDeleted := 0
	if inboxPath, ok := args["inbox_path"].(string); ok && inboxPath != "" {
		if registry.BasePath != "" && !filepath.IsAbs(inboxPath) {
			inboxPath = filepath.Join(registry.BasePath, inboxPath)
		}
		entries, err := os.ReadDir(inboxPath)
		if err == nil {
			for _, e := range entries {
				if !strings.HasPrefix(e.Name(), ".") {
					_ = os.RemoveAll(filepath.Join(inboxPath, e.Name()))
					filesDeleted++
				}
			}
		}
	}

	return map[string]any{
		"status":           "cleared",
		"memories_deleted": memCount,
		"files_deleted":    filesDeleted,
	}, nil
}

// Helper utilities

func marshalSlice(v any) string {
	if v == nil {
		return "[]"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func unmarshalJSON(s string) any {
	if s == "" {
		return []any{}
	}
	var res any
	if err := json.Unmarshal([]byte(s), &res); err != nil {
		return []any{}
	}
	return res
}

func parseIDList(v any) []uint {
	var ids []uint
	if v == nil {
		return ids
	}
	switch val := v.(type) {
	case []any:
		for _, item := range val {
			if id := toUint(item); id > 0 {
				ids = append(ids, id)
			}
		}
	case []int:
		for _, item := range val {
			if item > 0 {
				ids = append(ids, uint(item))
			}
		}
	case []float64:
		for _, item := range val {
			if item > 0 {
				ids = append(ids, uint(item))
			}
		}
	}
	return ids
}

func getIntFromMap(m map[string]any, key string) int {
	val, ok := m[key]
	if !ok || val == nil {
		return 0
	}
	return int(toUint(val))
}

func toUint(v any) uint {
	switch val := v.(type) {
	case float64:
		return uint(val)
	case int:
		return uint(val)
	case int64:
		return uint(val)
	case uint:
		return val
	case string:
		if u, err := strconv.ParseUint(val, 10, 64); err == nil {
			return uint(u)
		}
	}
	return 0
}

func addConnection(db *gorm.DB, fromID, toID uint, relationship string) {
	var mem Memory
	if err := db.First(&mem, fromID).Error; err != nil {
		return
	}
	var existing []map[string]any
	if mem.Connections != "" {
		_ = json.Unmarshal([]byte(mem.Connections), &existing)
	}
	existing = append(existing, map[string]any{
		"linked_to":    toID,
		"relationship": relationship,
	})
	b, _ := json.Marshal(existing)
	db.Model(&mem).Update("connections", string(b))
}

func init() {
	registry.RegisterToolHandler("store_memory", storeMemoryHandler)
	registry.RegisterToolHandler("read_all_memories", readAllMemoriesHandler)
	registry.RegisterToolHandler("read_unconsolidated_memories", readUnconsolidatedMemoriesHandler)
	registry.RegisterToolHandler("store_consolidation", storeConsolidationHandler)
	registry.RegisterToolHandler("read_consolidation_history", readConsolidationHistoryHandler)
	registry.RegisterToolHandler("get_memory_stats", getMemoryStatsHandler)
	registry.RegisterToolHandler("delete_memory", deleteMemoryHandler)
	registry.RegisterToolHandler("clear_all_memories", clearAllMemoriesHandler)
}
