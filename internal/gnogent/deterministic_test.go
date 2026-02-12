package gnogent

import (
	"context"
	"iter"
	"testing"
	"time"

	"github.com/innomon/agentic/internal/gnogent/gnovm"
	"github.com/innomon/agentic/internal/gnogent/storage"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type mockSession struct {
	id     string
	userID string
}

func (m *mockSession) ID() string             { return m.id }
func (m *mockSession) AppName() string         { return "test" }
func (m *mockSession) UserID() string          { return m.userID }
func (m *mockSession) State() session.State    { return &mockState{} }
func (m *mockSession) Events() session.Events  { return nil }
func (m *mockSession) LastUpdateTime() time.Time { return time.Now() }

type mockState struct{}

func (m *mockState) Get(string) (any, error)      { return nil, nil }
func (m *mockState) Set(string, any) error         { return nil }
func (m *mockState) All() iter.Seq2[string, any]   { return func(yield func(string, any) bool) {} }

type mockInvocationContext struct {
	context.Context
	userContent *genai.Content
	sess        session.Session
	ag          agent.Agent
}

func (m *mockInvocationContext) Agent() agent.Agent             { return m.ag }
func (m *mockInvocationContext) Artifacts() agent.Artifacts     { return nil }
func (m *mockInvocationContext) Memory() agent.Memory           { return nil }
func (m *mockInvocationContext) Session() session.Session       { return m.sess }
func (m *mockInvocationContext) InvocationID() string           { return "test-inv-1" }
func (m *mockInvocationContext) Branch() string                 { return "" }
func (m *mockInvocationContext) UserContent() *genai.Content    { return m.userContent }
func (m *mockInvocationContext) RunConfig() *agent.RunConfig    { return nil }
func (m *mockInvocationContext) EndInvocation()                 {}
func (m *mockInvocationContext) Ended() bool                    { return false }


func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&storage.AgentSession{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func setupTestVM(t *testing.T) *gnovm.GnoMachineWrapper {
	t.Helper()
	vm, err := gnovm.NewGnoMachineWrapper("gno/agent", "package agent")
	if err != nil {
		t.Fatalf("failed to create GnoVM: %v", err)
	}
	return vm
}

func TestDeterministicRun_YieldsOneEvent(t *testing.T) {
	db := setupTestDB(t)
	vm := setupTestVM(t)
	run := newDeterministicRun(db, vm)

	invCtx := &mockInvocationContext{
		Context:     context.Background(),
		userContent: genai.NewContentFromText("hello", genai.RoleUser),
		sess:        &mockSession{id: "s1", userID: "user1"},
	}

	var events []*session.Event
	var errs []error
	for ev, err := range run(invCtx) {
		events = append(events, ev)
		errs = append(errs, err)
	}

	for _, e := range errs {
		if e != nil {
			t.Fatalf("unexpected error: %v", e)
		}
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	ev := events[0]
	if ev.LLMResponse.Content == nil {
		t.Fatal("expected non-nil content")
	}
	if ev.LLMResponse.Content.Role != genai.RoleModel {
		t.Errorf("expected role %q, got %q", genai.RoleModel, ev.LLMResponse.Content.Role)
	}
	if len(ev.LLMResponse.Content.Parts) == 0 || ev.LLMResponse.Content.Parts[0].Text == "" {
		t.Error("expected non-empty text in response")
	}
}

func TestDeterministicRun_EmptyInput(t *testing.T) {
	db := setupTestDB(t)
	vm := setupTestVM(t)
	run := newDeterministicRun(db, vm)

	invCtx := &mockInvocationContext{
		Context:     context.Background(),
		userContent: genai.NewContentFromText("", genai.RoleUser),
		sess:        &mockSession{id: "s1", userID: "user1"},
	}

	var gotErr error
	for _, err := range run(invCtx) {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestDeterministicRun_PersistsState(t *testing.T) {
	db := setupTestDB(t)
	vm := setupTestVM(t)
	run := newDeterministicRun(db, vm)

	invCtx := &mockInvocationContext{
		Context:     context.Background(),
		userContent: genai.NewContentFromText("hello", genai.RoleUser),
		sess:        &mockSession{id: "s1", userID: "user1"},
	}

	for ev, err := range run(invCtx) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = ev
	}

	var sess storage.AgentSession
	if err := db.Where("user_id = ?", "user1").First(&sess).Error; err != nil {
		t.Fatalf("expected session to be persisted: %v", err)
	}
	if sess.UserID != "user1" {
		t.Errorf("expected user_id %q, got %q", "user1", sess.UserID)
	}
}

func TestDeterministicRun_RestoresState(t *testing.T) {
	db := setupTestDB(t)
	vm := setupTestVM(t)

	db.Create(&storage.AgentSession{
		UserID:          "user1",
		VMState:         []byte("snapshot"),
		FriendshipScore: 50,
		MoodTag:         "Happy",
	})

	run := newDeterministicRun(db, vm)

	invCtx := &mockInvocationContext{
		Context:     context.Background(),
		userContent: genai.NewContentFromText("thanks", genai.RoleUser),
		sess:        &mockSession{id: "s1", userID: "user1"},
	}

	var events []*session.Event
	for ev, err := range run(invCtx) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, ev)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestExtractUserText(t *testing.T) {
	tests := []struct {
		name    string
		content *genai.Content
		want    string
	}{
		{"nil content", nil, ""},
		{"empty parts", &genai.Content{Parts: []*genai.Part{}}, ""},
		{"text part", genai.NewContentFromText("hello", genai.RoleUser), "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractUserText(tt.content)
			if got != tt.want {
				t.Errorf("extractUserText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeterministicGnogentConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     DeterministicGnogentConfig
		wantErr bool
	}{
		{
			name:    "missing dsn",
			cfg:     DeterministicGnogentConfig{},
			wantErr: true,
		},
		{
			name: "missing source_file",
			cfg: func() DeterministicGnogentConfig {
				c := DeterministicGnogentConfig{}
				c.Database.DSN = "postgres://localhost/test"
				return c
			}(),
			wantErr: true,
		},
		{
			name: "valid",
			cfg: func() DeterministicGnogentConfig {
				c := DeterministicGnogentConfig{}
				c.Database.DSN = "postgres://localhost/test"
				c.GnoVM.SourceFile = "./agent.gno"
				return c
			}(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
