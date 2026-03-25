package storage

import (
	"time"

	"gorm.io/gorm"
)

// FileSys represents a GNO package file stored in Postgres.
type FileSys struct {
	Path      string `gorm:"primaryKey"`
	Metadata  string `gorm:"type:jsonb"` // Store as string for flexibility
	Content   []byte `gorm:"type:bytea"`
	Timestamp time.Time `gorm:"default:current_timestamp"`
}

func (f *FileSys) TableName() string {
	return "filesys"
}

// AgentSession is moved here for unified storage management.
type AgentSession struct {
	gorm.Model
	UserID          string `gorm:"index"`
	SessionID       string `gorm:"uniqueIndex"`
	VMState         []byte `gorm:"type:bytea"`
	FriendshipScore int
	MoodTag         string
}
