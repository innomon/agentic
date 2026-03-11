package userdb

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type UserStatus string

const (
	StatusActive    UserStatus = "Active"
	StatusPending   UserStatus = "Pending"
	StatusSuspended UserStatus = "Suspended"
)

type UserProfile struct {
	UserID   string   `json:"user_id"`
	Channels []string `json:"channels"`
	Roles    []string `json:"roles"`
}

type UserMetadata struct {
	UpdateTimestamp time.Time `json:"update_timestamp"`
	UpdatedBy      string    `json:"updated_by"`
	Channel        string    `json:"channel"`
}

type UserRecord struct {
	UserID   string      `gorm:"primaryKey;column:user_id"`
	Status   UserStatus  `gorm:"column:status;default:Pending"`
	Profile  ProfileJSON `gorm:"column:profile;type:jsonb"`
	Metadata MetaJSON    `gorm:"column:metadata;type:jsonb"`
}

func (UserRecord) TableName() string { return "users" }

// ProfileJSON handles JSON serialization of UserProfile for GORM.
type ProfileJSON struct {
	UserProfile
}

func (p ProfileJSON) Value() (driver.Value, error) {
	b, err := json.Marshal(p.UserProfile)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (p *ProfileJSON) Scan(value any) error {
	if value == nil {
		p.UserProfile = UserProfile{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("ProfileJSON.Scan: unsupported type %T", value)
	}
	return json.Unmarshal(bytes, &p.UserProfile)
}

func (ProfileJSON) GormDataType() string { return "jsonb" }

func (ProfileJSON) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "JSONB"
	default:
		return "TEXT"
	}
}

func (p ProfileJSON) GormValue(_ context.Context, db *gorm.DB) clause.Expr {
	data, _ := json.Marshal(p.UserProfile)
	return gorm.Expr("?", string(data))
}

// MetaJSON handles JSON serialization of UserMetadata for GORM.
type MetaJSON struct {
	UserMetadata
}

func (m MetaJSON) Value() (driver.Value, error) {
	b, err := json.Marshal(m.UserMetadata)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (m *MetaJSON) Scan(value any) error {
	if value == nil {
		m.UserMetadata = UserMetadata{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("MetaJSON.Scan: unsupported type %T", value)
	}
	return json.Unmarshal(bytes, &m.UserMetadata)
}

func (MetaJSON) GormDataType() string { return "jsonb" }

func (MetaJSON) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "JSONB"
	default:
		return "TEXT"
	}
}

func (m MetaJSON) GormValue(_ context.Context, db *gorm.DB) clause.Expr {
	data, _ := json.Marshal(m.UserMetadata)
	return gorm.Expr("?", string(data))
}

type UserDB struct {
	db         *gorm.DB
	mu         sync.RWMutex
	adminUsers map[string]bool
}

func Open(driverName, dsn string, autoMigrate bool) (*UserDB, error) {
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

	udb := NewUserDB(db)
	if autoMigrate {
		if err := udb.AutoMigrate(); err != nil {
			return nil, fmt.Errorf("failed to auto-migrate: %w", err)
		}
	}
	return udb, nil
}

func NewUserDB(db *gorm.DB) *UserDB {
	return &UserDB{db: db, adminUsers: make(map[string]bool)}
}

func (u *UserDB) AutoMigrate() error {
	return u.db.AutoMigrate(&UserRecord{})
}

func (u *UserDB) SetAdminUsers(admins []string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.adminUsers = make(map[string]bool, len(admins))
	for _, id := range admins {
		u.adminUsers[id] = true
	}
}

func (u *UserDB) IsAdmin(userID string) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.adminUsers[userID]
}

func (u *UserDB) GetUser(ctx context.Context, userID string) (*UserRecord, error) {
	var rec UserRecord
	if err := u.db.WithContext(ctx).Where("user_id = ?", userID).First(&rec).Error; err != nil {
		return nil, fmt.Errorf("failed to get user %q: %w", userID, err)
	}
	return &rec, nil
}

func (u *UserDB) CreateUser(ctx context.Context, rec *UserRecord) error {
	if err := u.db.WithContext(ctx).Create(rec).Error; err != nil {
		return fmt.Errorf("failed to create user %q: %w", rec.UserID, err)
	}
	return nil
}

func (u *UserDB) UpdateUser(ctx context.Context, rec *UserRecord, callerID string) error {
	if err := rejectAdminRole(rec.Profile.Roles); err != nil {
		return err
	}
	rec.Metadata.UpdateTimestamp = time.Now().UTC()
	rec.Metadata.UpdatedBy = callerID
	if err := u.db.WithContext(ctx).Save(rec).Error; err != nil {
		return fmt.Errorf("failed to update user %q: %w", rec.UserID, err)
	}
	return nil
}

func (u *UserDB) DeleteUser(ctx context.Context, userID string) error {
	if err := u.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&UserRecord{}).Error; err != nil {
		return fmt.Errorf("failed to delete user %q: %w", userID, err)
	}
	return nil
}

func (u *UserDB) SetStatus(ctx context.Context, userID string, status UserStatus, callerID string) error {
	result := u.db.WithContext(ctx).Model(&UserRecord{}).Where("user_id = ?", userID).Updates(map[string]any{
		"status": status,
		"metadata": MetaJSON{UserMetadata{
			UpdateTimestamp: time.Now().UTC(),
			UpdatedBy:      callerID,
		}},
	})
	if result.Error != nil {
		return fmt.Errorf("failed to set status for user %q: %w", userID, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user %q not found", userID)
	}
	return nil
}

func (u *UserDB) SetRoles(ctx context.Context, userID string, roles []string, callerID string) error {
	if err := rejectAdminRole(roles); err != nil {
		return err
	}

	rec, err := u.GetUser(ctx, userID)
	if err != nil {
		return err
	}

	rec.Profile.Roles = roles
	rec.Metadata.UpdateTimestamp = time.Now().UTC()
	rec.Metadata.UpdatedBy = callerID

	if err := u.db.WithContext(ctx).Save(rec).Error; err != nil {
		return fmt.Errorf("failed to set roles for user %q: %w", userID, err)
	}
	return nil
}

func (u *UserDB) SetChannels(ctx context.Context, userID string, channels []string, callerID string) error {
	rec, err := u.GetUser(ctx, userID)
	if err != nil {
		return err
	}

	rec.Profile.Channels = channels
	rec.Metadata.UpdateTimestamp = time.Now().UTC()
	rec.Metadata.UpdatedBy = callerID

	if err := u.db.WithContext(ctx).Save(rec).Error; err != nil {
		return fmt.Errorf("failed to set channels for user %q: %w", userID, err)
	}
	return nil
}

func (u *UserDB) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	rec, err := u.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	roles := make([]string, len(rec.Profile.Roles))
	copy(roles, rec.Profile.Roles)

	if u.IsAdmin(userID) {
		roles = append(roles, "admin")
	}

	return roles, nil
}

func rejectAdminRole(roles []string) error {
	for _, r := range roles {
		if strings.ToLower(r) == "admin" {
			return fmt.Errorf("admin role cannot be set via database; configure admin users in application config")
		}
	}
	return nil
}
