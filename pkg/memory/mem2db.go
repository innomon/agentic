package memory

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	"google.golang.org/adk/memory"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

func OpenDatabaseMemoryService(driverName, dsn string, autoMigrate bool) (*DatabaseMemoryService, error) {
	var dialector gorm.Dialector
	switch driverName {
	case "postgres":
		dialector = postgres.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported driver %q (supported: postgres, sqlite)", driverName)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	svc := NewDatabaseMemoryService(db)
	if autoMigrate {
		if err := svc.AutoMigrate(); err != nil {
			return nil, fmt.Errorf("failed to auto-migrate: %w", err)
		}
	}
	return svc, nil
}

// storageMemoryEntry corresponds to the 'memory_entries' table.
type storageMemoryEntry struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	AppName   string `gorm:"index:idx_mem_app_user;not null"`
	UserID    string `gorm:"index:idx_mem_app_user;not null"`
	SessionID string `gorm:"not null"`
	Author    string
	Timestamp time.Time
	Content   contentJSON `gorm:"type:jsonb"`
	Words     wordsJSON   `gorm:"type:jsonb"`
}

func (storageMemoryEntry) TableName() string {
	return "memory_entries"
}

// DatabaseMemoryService implements memory.Service backed by a database via GORM.
type DatabaseMemoryService struct {
	db *gorm.DB
}

// NewDatabaseMemoryService creates a memory.Service from an existing *gorm.DB.
func NewDatabaseMemoryService(db *gorm.DB) *DatabaseMemoryService {
	return &DatabaseMemoryService{db: db}
}

// AutoMigrate ensures the memory_entries table schema is up to date.
func (s *DatabaseMemoryService) AutoMigrate() error {
	return s.db.AutoMigrate(&storageMemoryEntry{})
}

// AddSessionToMemory ingests a session's LLM responses into the database.
// Re-adding the same session replaces its previous entries.
func (s *DatabaseMemoryService) AddSessionToMemory(ctx context.Context, curSession session.Session) error {
	appName := curSession.AppName()
	userID := curSession.UserID()
	sid := curSession.ID()

	var entries []storageMemoryEntry

	for event := range curSession.Events().All() {
		if event.LLMResponse.Content == nil {
			continue
		}

		words := make(map[string]struct{})
		for _, part := range event.LLMResponse.Content.Parts {
			if part.Text == "" {
				continue
			}
			maps.Copy(words, extractWords(part.Text))
		}

		if len(words) == 0 {
			continue
		}

		entries = append(entries, storageMemoryEntry{
			AppName:   appName,
			UserID:    userID,
			SessionID: sid,
			Author:    event.Author,
			Timestamp: event.Timestamp,
			Content:   contentJSON{Content: event.LLMResponse.Content},
			Words:     wordsJSON(words),
		})
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("app_name = ? AND user_id = ? AND session_id = ?", appName, userID, sid).
			Delete(&storageMemoryEntry{}).Error; err != nil {
			return fmt.Errorf("failed to clear old memory entries: %w", err)
		}

		if len(entries) > 0 {
			if err := tx.Create(&entries).Error; err != nil {
				return fmt.Errorf("failed to insert memory entries: %w", err)
			}
		}

		return nil
	})
}

// SearchMemory returns memory entries matching the query by keyword intersection,
// scoped to the given AppName and UserID.
func (s *DatabaseMemoryService) SearchMemory(ctx context.Context, req *memory.SearchRequest) (*memory.SearchResponse, error) {
	var entries []storageMemoryEntry

	if err := s.db.WithContext(ctx).
		Where("app_name = ? AND user_id = ?", req.AppName, req.UserID).
		Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("failed to search memory: %w", err)
	}

	if len(entries) == 0 {
		return &memory.SearchResponse{}, nil
	}

	queryWords := extractWords(req.Query)

	res := &memory.SearchResponse{}
	for _, e := range entries {
		if checkMapsIntersect(map[string]struct{}(e.Words), queryWords) {
			res.Memories = append(res.Memories, memory.Entry{
				Content:   e.Content.Content,
				Author:    e.Author,
				Timestamp: e.Timestamp,
			})
		}
	}

	return res, nil
}

func checkMapsIntersect(m1, m2 map[string]struct{}) bool {
	if len(m1) == 0 || len(m2) == 0 {
		return false
	}
	if len(m1) > len(m2) {
		m1, m2 = m2, m1
	}
	for k := range m1 {
		if _, ok := m2[k]; ok {
			return true
		}
	}
	return false
}

func extractWords(text string) map[string]struct{} {
	res := make(map[string]struct{})
	for s := range strings.SplitSeq(text, " ") {
		if s == "" {
			continue
		}
		res[strings.ToLower(s)] = struct{}{}
	}
	return res
}

// contentJSON handles JSON serialization of *genai.Content for GORM.
type contentJSON struct {
	Content *genai.Content
}

func (c contentJSON) Value() (driver.Value, error) {
	if c.Content == nil {
		return nil, nil
	}
	b, err := json.Marshal(c.Content)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (c *contentJSON) Scan(value any) error {
	if value == nil {
		c.Content = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("contentJSON.Scan: unsupported type %T", value)
	}
	return json.Unmarshal(bytes, &c.Content)
}

func (contentJSON) GormDataType() string { return "jsonb" }

func (contentJSON) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "JSONB"
	default:
		return "TEXT"
	}
}

func (c contentJSON) GormValue(_ context.Context, db *gorm.DB) clause.Expr {
	if c.Content == nil {
		return gorm.Expr("NULL")
	}
	data, _ := json.Marshal(c.Content)
	return gorm.Expr("?", string(data))
}

// wordsJSON handles JSON serialization of the word set for GORM.
// Stored as a JSON array of strings.
type wordsJSON map[string]struct{}

func (w wordsJSON) Value() (driver.Value, error) {
	if w == nil {
		return "[]", nil
	}
	keys := make([]string, 0, len(w))
	for k := range w {
		keys = append(keys, k)
	}
	b, err := json.Marshal(keys)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (w *wordsJSON) Scan(value any) error {
	if value == nil {
		*w = make(wordsJSON)
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("wordsJSON.Scan: unsupported type %T", value)
	}
	var keys []string
	if err := json.Unmarshal(bytes, &keys); err != nil {
		return err
	}
	m := make(wordsJSON, len(keys))
	for _, k := range keys {
		m[k] = struct{}{}
	}
	*w = m
	return nil
}

func (wordsJSON) GormDataType() string { return "jsonb" }

func (wordsJSON) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "JSONB"
	default:
		return "TEXT"
	}
}

func (w wordsJSON) GormValue(_ context.Context, db *gorm.DB) clause.Expr {
	if w == nil {
		return gorm.Expr("'[]'")
	}
	keys := make([]string, 0, len(w))
	for k := range w {
		keys = append(keys, k)
	}
	data, _ := json.Marshal(keys)
	return gorm.Expr("?", string(data))
}
