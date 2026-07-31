package gnogent

import (
	"context"
	"fmt"
	"iter"
	"os"

	"github.com/innomon/agentic/pkg/gnovm"
	"github.com/innomon/agentic/pkg/gnogent/storage"
	"github.com/innomon/agentic/pkg/registry"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/genai"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type GnoAgentConfig struct {
	registry.AgentBase `yaml:",inline"`
	Database           struct {
		DSN         string `yaml:"dsn"`
		AutoMigrate bool   `yaml:"auto_migrate"`
	} `yaml:"database"`
	GnoVM struct {
		SourceFile string `yaml:"source_file"`
		PkgPath    string `yaml:"pkg_path"`
	} `yaml:"gnovm"`
	Tools []string `yaml:"tools"`
}

func (c *GnoAgentConfig) Validate() error {
	if c.Database.DSN == "" {
		return fmt.Errorf("database.dsn is required for gnoagent")
	}
	if c.GnoVM.SourceFile == "" {
		return fmt.Errorf("gnovm.source_file is required for gnoagent")
	}
	return nil
}

func gnoAgentCreator(ctx context.Context, name string, cfg *GnoAgentConfig, _ registry.ModelRegistry, tools registry.ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("gnoagent %q: postgres connection failed: %w", name, err)
	}
	if cfg.Database.AutoMigrate {
		if err := db.AutoMigrate(&storage.AgentSession{}); err != nil {
			return nil, fmt.Errorf("gnoagent %q: migration failed: %w", name, err)
		}
	}

	pkgPath := cfg.GnoVM.PkgPath
	if pkgPath == "" {
		pkgPath = "gno.land/p/agent"
	}

	gnoSource, err := os.ReadFile(cfg.GnoVM.SourceFile)
	if err != nil {
		return nil, fmt.Errorf("gnoagent %q: gno source file not found: %w", name, err)
	}

	vmWrapper, err := gnovm.NewAgentWrapper(pkgPath, string(gnoSource), nil)
	if err != nil {
		return nil, fmt.Errorf("gnoagent %q: GnoVM failed to boot: %w", name, err)
	}

	var toolList []tool.Tool
	if len(cfg.Tools) > 0 && tools != nil {
		toolList, err = tools.GetMultiple(ctx, cfg.Tools)
		if err != nil {
			return nil, fmt.Errorf("gnoagent %q: failed to get tools: %w", name, err)
		}
	}

	return agent.New(agent.Config{
		Name:        name,
		Description: cfg.Description,
		SubAgents:   sub,
		Run:         newGnoRun(db, vmWrapper, sub, toolList),
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

func newGnoRun(db *gorm.DB, vm *gnovm.AgentWrapper, sub []agent.Agent, tools []tool.Tool) func(agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(invCtx agent.InvocationContext) iter.Seq2[*session.Event, error] {
		return func(yield func(*session.Event, error) bool) {
			userInput := extractUserText(invCtx.UserContent())
			if userInput == "" {
				yield(nil, fmt.Errorf("gnoagent: empty user input"))
				return
			}

			userID := invCtx.Session().UserID()
			vm.Machine.Context = &gnovm.AgentContext{
				InvCtx:    invCtx,
				SubAgents: sub,
				Tools:     tools,
			}

			// 1. Thaw: Restore GnoVM state from DB
			var sess storage.AgentSession
			if err := db.Where("user_id = ?", userID).First(&sess).Error; err == nil {
				if err := vm.RestoreState(sess.VMState); err != nil {
					yield(nil, fmt.Errorf("gnoagent: thaw failure: %w", err))
					return
				}
			}

			// 2. Pulse: Set input variable and run main()
			if err := vm.SetInput(userInput); err != nil {
				yield(nil, fmt.Errorf("gnoagent: failed to set input: %w", err))
				return
			}

			if err := vm.Run(); err != nil {
				yield(nil, fmt.Errorf("gnoagent: execution error: %w", err))
				return
			}

			// 3. Prompt: Get output variable from GnoVM
			response, err := vm.GetOutput()
			if err != nil {
				yield(nil, fmt.Errorf("gnoagent: failed to get output: %w", err))
				return
			}

			// 4. Freeze: Persist GnoVM state to DB
			blob, err := vm.ExportState()
			if err != nil {
				yield(nil, fmt.Errorf("gnoagent: freeze failure: %w", err))
				return
			}

			var existing storage.AgentSession
			if err := db.Where("user_id = ?", userID).First(&existing).Error; err == nil {
				existing.VMState = blob
				db.Save(&existing)
			} else {
				db.Create(&storage.AgentSession{
					UserID:  userID,
					VMState: blob,
				})
			}

			event := session.NewEvent(invCtx, invCtx.InvocationID())
			event.LLMResponse = model.LLMResponse{
				Content: genai.NewContentFromText(response, genai.RoleModel),
			}
			yield(event, nil)
		}
	}
}

func init() {
	registry.RegisterAgentType("gnogent", gnoAgentCreator)
}
