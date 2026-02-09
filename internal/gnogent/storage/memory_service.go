package storage

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

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type gnogentMemoryEntry struct {
	ID        uint              `gorm:"primaryKey;autoIncrement"`
	AppName   string            `gorm:"index:idx_gnogent_mem_app_user;not null"`
	UserID    string            `gorm:"index:idx_gnogent_mem_app_user;not null"`
	SessionID string            `gorm:"not null"`
	Author    string
	Timestamp time.Time
	Content   gnogentContentJSON `gorm:"type:jsonb"`
	Words     gnogentWordsJSON   `gorm:"type:jsonb"`
}

func (gnogentMemoryEntry) TableName() string {
	return "gnogent_memory_entries"
}

type GnogentMemoryService struct {
	db *gorm.DB
}

func NewGnogentMemoryService(db *gorm.DB) *GnogentMemoryService {
	return &GnogentMemoryService{db: db}
}

func (s *GnogentMemoryService) AutoMigrate() error {
	return s.db.AutoMigrate(&gnogentMemoryEntry{})
}

func (s *GnogentMemoryService) AddSession(ctx context.Context, curSession session.Session) error {
	appName := curSession.AppName()
	userID := curSession.UserID()
	sid := curSession.ID()

	var entries []gnogentMemoryEntry

	for event := range curSession.Events().All() {
		if event.LLMResponse.Content == nil {
			continue
		}

		words := make(map[string]struct{})
		for _, part := range event.LLMResponse.Content.Parts {
			if part.Text == "" {
				continue
			}
			maps.Copy(words, gnogentExtractWords(part.Text))
		}

		if len(words) == 0 {
			continue
		}

		entries = append(entries, gnogentMemoryEntry{
			AppName:   appName,
			UserID:    userID,
			SessionID: sid,
			Author:    event.Author,
			Timestamp: event.Timestamp,
			Content:   gnogentContentJSON{Content: event.LLMResponse.Content},
			Words:     gnogentWordsJSON(words),
		})
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("app_name = ? AND user_id = ? AND session_id = ?", appName, userID, sid).
			Delete(&gnogentMemoryEntry{}).Error; err != nil {
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

func (s *GnogentMemoryService) Search(ctx context.Context, req *memory.SearchRequest) (*memory.SearchResponse, error) {
	var entries []gnogentMemoryEntry

	if err := s.db.WithContext(ctx).
		Where("app_name = ? AND user_id = ?", req.AppName, req.UserID).
		Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("failed to search memory: %w", err)
	}

	if len(entries) == 0 {
		return &memory.SearchResponse{}, nil
	}

	queryWords := gnogentExtractWords(req.Query)

	res := &memory.SearchResponse{}
	for _, e := range entries {
		if gnogentCheckMapsIntersect(map[string]struct{}(e.Words), queryWords) {
			res.Memories = append(res.Memories, memory.Entry{
				Content:   e.Content.Content,
				Author:    e.Author,
				Timestamp: e.Timestamp,
			})
		}
	}

	return res, nil
}

func gnogentCheckMapsIntersect(m1, m2 map[string]struct{}) bool {
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

func gnogentExtractWords(text string) map[string]struct{} {
	res := make(map[string]struct{})
	for s := range strings.SplitSeq(text, " ") {
		if s == "" {
			continue
		}
		res[strings.ToLower(s)] = struct{}{}
	}
	return res
}

type gnogentContentJSON struct {
	Content *genai.Content
}

func (c gnogentContentJSON) Value() (driver.Value, error) {
	if c.Content == nil {
		return nil, nil
	}
	b, err := json.Marshal(c.Content)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (c *gnogentContentJSON) Scan(value any) error {
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
		return fmt.Errorf("gnogentContentJSON.Scan: unsupported type %T", value)
	}
	return json.Unmarshal(bytes, &c.Content)
}

func (gnogentContentJSON) GormDataType() string { return "jsonb" }

func (gnogentContentJSON) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "JSONB"
	default:
		return "TEXT"
	}
}

func (c gnogentContentJSON) GormValue(_ context.Context, db *gorm.DB) clause.Expr {
	if c.Content == nil {
		return gorm.Expr("NULL")
	}
	data, _ := json.Marshal(c.Content)
	return gorm.Expr("?", string(data))
}

type gnogentWordsJSON map[string]struct{}

func (w gnogentWordsJSON) Value() (driver.Value, error) {
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

func (w *gnogentWordsJSON) Scan(value any) error {
	if value == nil {
		*w = make(gnogentWordsJSON)
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("gnogentWordsJSON.Scan: unsupported type %T", value)
	}
	var keys []string
	if err := json.Unmarshal(bytes, &keys); err != nil {
		return err
	}
	m := make(gnogentWordsJSON, len(keys))
	for _, k := range keys {
		m[k] = struct{}{}
	}
	*w = m
	return nil
}

func (gnogentWordsJSON) GormDataType() string { return "jsonb" }

func (gnogentWordsJSON) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "JSONB"
	default:
		return "TEXT"
	}
}

func (w gnogentWordsJSON) GormValue(_ context.Context, db *gorm.DB) clause.Expr {
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
