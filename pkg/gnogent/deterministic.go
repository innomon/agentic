package gnogent

import (
	"context"
	"fmt"
	"iter"
	"os"
	"time"

	"github.com/innomon/agentic/pkg/gnogent/gnovm"
	"github.com/innomon/agentic/pkg/gnogent/storage"
	"github.com/innomon/agentic/pkg/registry"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DeterministicGnogentConfig struct {
	registry.AgentBase `yaml:",inline"`
	Database           struct {
		DSN         string `yaml:"dsn"`
		AutoMigrate bool   `yaml:"auto_migrate"`
	} `yaml:"database"`
	GnoVM struct {
		SourceFile string `yaml:"source_file"`
		PkgPath    string `yaml:"pkg_path"`
	} `yaml:"gnovm"`
}

func (c *DeterministicGnogentConfig) Validate() error {
	if c.Database.DSN == "" {
		return fmt.Errorf("database.dsn is required for deterministic gnogent agent")
	}
	if c.GnoVM.SourceFile == "" {
		return fmt.Errorf("gnovm.source_file is required for deterministic gnogent agent")
	}
	return nil
}

func deterministicGnogentCreator(ctx context.Context, name string, cfg *DeterministicGnogentConfig, _ registry.ModelRegistry, _ registry.ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("deterministic-gnogent %q: postgres connection failed: %w", name, err)
	}
	if cfg.Database.AutoMigrate {
		if err := db.AutoMigrate(&storage.AgentSession{}); err != nil {
			return nil, fmt.Errorf("deterministic-gnogent %q: migration failed: %w", name, err)
		}
	}

	pkgPath := cfg.GnoVM.PkgPath
	if pkgPath == "" {
		pkgPath = "gno.land/p/agent"
	}

	gnoSource, err := os.ReadFile(cfg.GnoVM.SourceFile)
	if err != nil {
		return nil, fmt.Errorf("deterministic-gnogent %q: gno source file not found: %w", name, err)
	}

	vmWrapper, err := gnovm.NewGnoMachineWrapper(pkgPath, string(gnoSource))
	if err != nil {
		return nil, fmt.Errorf("deterministic-gnogent %q: GnoVM failed to boot: %w", name, err)
	}

	return agent.New(agent.Config{
		Name:        name,
		Description: cfg.Description,
		SubAgents:   sub,
		Run:         newDeterministicRun(db, vmWrapper),
	})
}

func extractUserText(content *genai.Content) string {
	if content == nil {
		return ""
	}
	for _, part := range content.Parts {
		if part.Text != "" {
			return part.Text
		}
	}
	return ""
}

func newDeterministicRun(db *gorm.DB, vm *gnovm.GnoMachineWrapper) func(agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(invCtx agent.InvocationContext) iter.Seq2[*session.Event, error] {
		return func(yield func(*session.Event, error) bool) {
			userInput := extractUserText(invCtx.UserContent())
			if userInput == "" {
				yield(nil, fmt.Errorf("deterministic-gnogent: empty user input"))
				return
			}

			userID := invCtx.Session().UserID()

			var sess storage.AgentSession
			if err := db.Where("user_id = ?", userID).First(&sess).Error; err == nil {
				if err := vm.RestoreState(sess.VMState); err != nil {
					yield(nil, fmt.Errorf("deterministic-gnogent: thaw failure: %w", err))
					return
				}
			}

			now := time.Now().Unix()
			if err := vm.SyncState(userInput, now); err != nil {
				yield(nil, fmt.Errorf("deterministic-gnogent: sync error: %w", err))
				return
			}

			response, err := vm.GetSystemContext()
			if err != nil {
				yield(nil, fmt.Errorf("deterministic-gnogent: context error: %w", err))
				return
			}

			_ = vm.AddTurn(userInput, response)

			blob, err := vm.ExportState()
			if err != nil {
				yield(nil, fmt.Errorf("deterministic-gnogent: freeze failure: %w", err))
				return
			}
			friendship, _ := vm.Friendship()
			mood, _ := vm.Mood()

			var existing storage.AgentSession
			if err := db.Where("user_id = ?", userID).First(&existing).Error; err == nil {
				existing.VMState = blob
				existing.FriendshipScore = friendship
				existing.MoodTag = mood
				db.Save(&existing)
			} else {
				db.Create(&storage.AgentSession{
					UserID:          userID,
					VMState:         blob,
					FriendshipScore: friendship,
					MoodTag:         mood,
				})
			}

			event := session.NewEvent(invCtx.InvocationID())
			event.LLMResponse = model.LLMResponse{
				Content: genai.NewContentFromText(response, genai.RoleModel),
			}
			yield(event, nil)
		}
	}
}

func init() {
	registry.RegisterAgentType("gnogent-deterministic", deterministicGnogentCreator)
}
