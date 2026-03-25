package storage

import (
	"time"

	"github.com/innomon/agentic/pkg/gnovm"

	"gorm.io/gorm"
)

type AgentSession struct {
	gorm.Model
	UserID    string `gorm:"index"`
	SessionID string `gorm:"uniqueIndex"`
	VMState   []byte `gorm:"type:bytea"`
}

type SnapshotMetadata struct {
	ID        uint
	CreatedAt time.Time
}

type GormSessionService struct {
	DB        *gorm.DB
	VMWrapper *gnovm.AgentWrapper
}

func (s *GormSessionService) GetSnapshotHistory(userID string) ([]SnapshotMetadata, error) {
	var results []SnapshotMetadata
	err := s.DB.Model(&AgentSession{}).
		Select("id, created_at").
		Where("user_id = ?", userID).
		Order("created_at desc").
		Limit(10).
		Scan(&results).Error
	return results, err
}
