package storage

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"iter"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/adk/session"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type gnogentSessionRow struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	AppName   string    `gorm:"uniqueIndex:idx_gnogent_sess;not null"`
	UserID    string    `gorm:"uniqueIndex:idx_gnogent_sess;not null"`
	SessionID string    `gorm:"uniqueIndex:idx_gnogent_sess;not null"`
	StateJSON stateJSON `gorm:"type:jsonb"`
	UpdatedAt time.Time
}

func (gnogentSessionRow) TableName() string {
	return "gnogent_sessions"
}

type gnogentEventRow struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	AppName   string    `gorm:"index:idx_gnogent_evt_sess;not null"`
	UserID    string    `gorm:"index:idx_gnogent_evt_sess;not null"`
	SessionID string    `gorm:"index:idx_gnogent_evt_sess;not null"`
	Ordering  int       `gorm:"not null"`
	EventJSON eventJSON `gorm:"type:jsonb"`
	Timestamp time.Time
}

func (gnogentEventRow) TableName() string {
	return "gnogent_events"
}

// GnogentSessionService implements session.Service backed by GORM.
type GnogentSessionService struct {
	db *gorm.DB
}

func NewGnogentSessionService(db *gorm.DB) *GnogentSessionService {
	return &GnogentSessionService{db: db}
}

func (s *GnogentSessionService) AutoMigrate() error {
	return s.db.AutoMigrate(&gnogentSessionRow{}, &gnogentEventRow{})
}

func (s *GnogentSessionService) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	if req.AppName == "" || req.UserID == "" {
		return nil, fmt.Errorf("app_name and user_id are required, got app_name: %q, user_id: %q", req.AppName, req.UserID)
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	state := req.State
	if state == nil {
		state = make(map[string]any)
	}

	now := time.Now()
	persistState := filterTempKeys(state)

	row := gnogentSessionRow{
		AppName:   req.AppName,
		UserID:    req.UserID,
		SessionID: sessionID,
		StateJSON: stateJSON(persistState),
		UpdatedAt: now,
	}

	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	sess := &gnogentSession{
		id:        sessionID,
		appName:   req.AppName,
		userID:    req.UserID,
		state:     maps.Clone(state),
		updatedAt: now,
	}

	return &session.CreateResponse{Session: sess}, nil
}

func (s *GnogentSessionService) Get(ctx context.Context, req *session.GetRequest) (*session.GetResponse, error) {
	if req.AppName == "" || req.UserID == "" || req.SessionID == "" {
		return nil, fmt.Errorf("app_name, user_id, session_id are required, got app_name: %q, user_id: %q, session_id: %q", req.AppName, req.UserID, req.SessionID)
	}

	var row gnogentSessionRow
	err := s.db.WithContext(ctx).
		Where("app_name = ? AND user_id = ? AND session_id = ?", req.AppName, req.UserID, req.SessionID).
		First(&row).Error
	if err != nil {
		return nil, fmt.Errorf("session %q not found: %w", req.SessionID, err)
	}

	events, err := s.loadEvents(ctx, req.AppName, req.UserID, req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load events: %w", err)
	}

	if req.NumRecentEvents > 0 && len(events) > req.NumRecentEvents {
		events = events[len(events)-req.NumRecentEvents:]
	}
	if !req.After.IsZero() && len(events) > 0 {
		idx := sort.Search(len(events), func(i int) bool {
			return !events[i].Timestamp.Before(req.After)
		})
		events = events[idx:]
	}

	sess := &gnogentSession{
		id:        row.SessionID,
		appName:   row.AppName,
		userID:    row.UserID,
		state:     map[string]any(row.StateJSON),
		events:    events,
		updatedAt: row.UpdatedAt,
	}
	if sess.state == nil {
		sess.state = make(map[string]any)
	}

	return &session.GetResponse{Session: sess}, nil
}

func (s *GnogentSessionService) List(ctx context.Context, req *session.ListRequest) (*session.ListResponse, error) {
	if req.AppName == "" {
		return nil, fmt.Errorf("app_name is required, got app_name: %q", req.AppName)
	}

	var rows []gnogentSessionRow
	q := s.db.WithContext(ctx).Where("app_name = ?", req.AppName)
	if req.UserID != "" {
		q = q.Where("user_id = ?", req.UserID)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	sessions := make([]session.Session, 0, len(rows))
	for _, r := range rows {
		st := map[string]any(r.StateJSON)
		if st == nil {
			st = make(map[string]any)
		}
		sessions = append(sessions, &gnogentSession{
			id:        r.SessionID,
			appName:   r.AppName,
			userID:    r.UserID,
			state:     st,
			updatedAt: r.UpdatedAt,
		})
	}

	return &session.ListResponse{Sessions: sessions}, nil
}

func (s *GnogentSessionService) Delete(ctx context.Context, req *session.DeleteRequest) error {
	if req.AppName == "" || req.UserID == "" || req.SessionID == "" {
		return fmt.Errorf("app_name, user_id, session_id are required, got app_name: %q, user_id: %q, session_id: %q", req.AppName, req.UserID, req.SessionID)
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("app_name = ? AND user_id = ? AND session_id = ?", req.AppName, req.UserID, req.SessionID).
			Delete(&gnogentEventRow{}).Error; err != nil {
			return fmt.Errorf("failed to delete events: %w", err)
		}
		if err := tx.Where("app_name = ? AND user_id = ? AND session_id = ?", req.AppName, req.UserID, req.SessionID).
			Delete(&gnogentSessionRow{}).Error; err != nil {
			return fmt.Errorf("failed to delete session: %w", err)
		}
		return nil
	})
}

func (s *GnogentSessionService) AppendEvent(ctx context.Context, curSession session.Session, event *session.Event) error {
	if curSession == nil {
		return fmt.Errorf("session is nil")
	}
	if event == nil {
		return fmt.Errorf("event is nil")
	}
	if event.Partial {
		return nil
	}

	sess, ok := curSession.(*gnogentSession)
	if !ok {
		return fmt.Errorf("unexpected session type %T", curSession)
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	sess.events = append(sess.events, event)
	sess.updatedAt = event.Timestamp

	if len(event.Actions.StateDelta) > 0 {
		for k, v := range event.Actions.StateDelta {
			if !strings.HasPrefix(k, session.KeyPrefixTemp) {
				sess.state[k] = v
			}
		}
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maxOrder int
		tx.Model(&gnogentEventRow{}).
			Where("app_name = ? AND user_id = ? AND session_id = ?", sess.appName, sess.userID, sess.id).
			Select("COALESCE(MAX(ordering), -1)").
			Scan(&maxOrder)

		evtRow := gnogentEventRow{
			AppName:   sess.appName,
			UserID:    sess.userID,
			SessionID: sess.id,
			Ordering:  maxOrder + 1,
			EventJSON: eventJSON{Event: event},
			Timestamp: event.Timestamp,
		}
		if err := tx.Create(&evtRow).Error; err != nil {
			return fmt.Errorf("failed to insert event: %w", err)
		}

		persistState := filterTempKeys(sess.state)
		if err := tx.Model(&gnogentSessionRow{}).
			Where("app_name = ? AND user_id = ? AND session_id = ?", sess.appName, sess.userID, sess.id).
			Updates(map[string]any{
				"state_json": stateJSON(persistState),
				"updated_at": event.Timestamp,
			}).Error; err != nil {
			return fmt.Errorf("failed to update session state: %w", err)
		}

		return nil
	})
}

func (s *GnogentSessionService) loadEvents(ctx context.Context, appName, userID, sessionID string) ([]*session.Event, error) {
	var rows []gnogentEventRow
	err := s.db.WithContext(ctx).
		Where("app_name = ? AND user_id = ? AND session_id = ?", appName, userID, sessionID).
		Order("ordering ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	events := make([]*session.Event, 0, len(rows))
	for _, r := range rows {
		if r.EventJSON.Event != nil {
			events = append(events, r.EventJSON.Event)
		}
	}
	return events, nil
}

func filterTempKeys(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if !strings.HasPrefix(k, session.KeyPrefixTemp) {
			out[k] = v
		}
	}
	return out
}

// gnogentSession implements session.Session.
type gnogentSession struct {
	id        string
	appName   string
	userID    string
	mu        sync.RWMutex
	state     map[string]any
	events    []*session.Event
	updatedAt time.Time
}

func (s *gnogentSession) ID() string      { return s.id }
func (s *gnogentSession) AppName() string  { return s.appName }
func (s *gnogentSession) UserID() string   { return s.userID }
func (s *gnogentSession) State() session.State {
	return &gnogentState{mu: &s.mu, data: s.state}
}
func (s *gnogentSession) Events() session.Events {
	return gnogentEvents(s.events)
}
func (s *gnogentSession) LastUpdateTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updatedAt
}

type gnogentState struct {
	mu   *sync.RWMutex
	data map[string]any
}

func (s *gnogentState) Get(key string) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	if !ok {
		return nil, session.ErrStateKeyNotExist
	}
	return val, nil
}

func (s *gnogentState) Set(key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}

func (s *gnogentState) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		s.mu.RLock()
		for k, v := range s.data {
			s.mu.RUnlock()
			if !yield(k, v) {
				return
			}
			s.mu.RLock()
		}
		s.mu.RUnlock()
	}
}

type gnogentEvents []*session.Event

func (e gnogentEvents) All() iter.Seq[*session.Event] {
	return func(yield func(*session.Event) bool) {
		for _, ev := range e {
			if !yield(ev) {
				return
			}
		}
	}
}

func (e gnogentEvents) Len() int { return len(e) }

func (e gnogentEvents) At(i int) *session.Event {
	if i >= 0 && i < len(e) {
		return e[i]
	}
	return nil
}

// stateJSON handles GORM JSON serialization for map[string]any.
type stateJSON map[string]any

func (s stateJSON) Value() (driver.Value, error) {
	if s == nil {
		return "{}", nil
	}
	b, err := json.Marshal(map[string]any(s))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (s *stateJSON) Scan(value any) error {
	if value == nil {
		*s = make(stateJSON)
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("stateJSON.Scan: unsupported type %T", value)
	}
	m := make(map[string]any)
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	*s = stateJSON(m)
	return nil
}

func (stateJSON) GormDataType() string { return "jsonb" }

func (stateJSON) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "JSONB"
	default:
		return "TEXT"
	}
}

func (s stateJSON) GormValue(_ context.Context, db *gorm.DB) clause.Expr {
	if s == nil {
		return gorm.Expr("'{}'")
	}
	data, _ := json.Marshal(map[string]any(s))
	return gorm.Expr("?", string(data))
}

// eventJSON handles GORM JSON serialization for *session.Event.
type eventJSON struct {
	Event *session.Event
}

func (e eventJSON) Value() (driver.Value, error) {
	if e.Event == nil {
		return nil, nil
	}
	b, err := json.Marshal(e.Event)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (e *eventJSON) Scan(value any) error {
	if value == nil {
		e.Event = nil
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("eventJSON.Scan: unsupported type %T", value)
	}
	return json.Unmarshal(raw, &e.Event)
}

func (eventJSON) GormDataType() string { return "jsonb" }

func (eventJSON) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "JSONB"
	default:
		return "TEXT"
	}
}

func (e eventJSON) GormValue(_ context.Context, db *gorm.DB) clause.Expr {
	if e.Event == nil {
		return gorm.Expr("NULL")
	}
	data, _ := json.Marshal(e.Event)
	return gorm.Expr("?", string(data))
}

var _ session.Service = (*GnogentSessionService)(nil)
