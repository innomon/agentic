package gnogent

import (
	"context"
	"fmt"
	"os"

	"github.com/innomon/agentic/internal/gnogent/gnovm"
	"github.com/innomon/agentic/internal/gnogent/storage"
	"github.com/innomon/agentic/internal/registry"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type GnogentAgentConfig struct {
	registry.AgentBase `yaml:",inline"`
	Model              string `yaml:"model"`
	Instruction        string `yaml:"instruction"`
	Tools              []string `yaml:"tools"`
	Database           struct {
		DSN         string `yaml:"dsn"`
		AutoMigrate bool   `yaml:"auto_migrate"`
	} `yaml:"database"`
	GnoVM struct {
		SourceFile string `yaml:"source_file"`
		PkgPath    string `yaml:"pkg_path"`
	} `yaml:"gnovm"`
}

func (c *GnogentAgentConfig) Validate() error {
	if c.Model == "" {
		return fmt.Errorf("model is required for gnogent agent")
	}
	if c.Database.DSN == "" {
		return fmt.Errorf("database.dsn is required for gnogent agent")
	}
	if c.GnoVM.SourceFile == "" {
		return fmt.Errorf("gnovm.source_file is required for gnogent agent")
	}
	return nil
}

type ThawArgs struct {
	UserID string `json:"user_id" jsonschema:"description=The user ID to restore state for"`
}

type ThawResult struct {
	Status string `json:"status"`
}

type FreezeArgs struct {
	UserID string `json:"user_id" jsonschema:"description=The user ID to persist state for"`
}

type FreezeResult struct {
	Status string `json:"status"`
}

type BrainQueryArgs struct {
	Query string `json:"query" jsonschema:"description=Expression to evaluate in the GnoVM brain"`
}

type BrainQueryResult struct {
	Mood       string `json:"mood"`
	Friendship int    `json:"friendship"`
	Context    string `json:"context"`
}

func gnogentCreator(ctx context.Context, name string, cfg *GnogentAgentConfig, models registry.ModelRegistry, tools registry.ToolRegistry, sub []agent.Agent) (agent.Agent, error) {
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("gnogent %q: postgres connection failed: %w", name, err)
	}
	if cfg.Database.AutoMigrate {
		if err := db.AutoMigrate(&storage.AgentSession{}); err != nil {
			return nil, fmt.Errorf("gnogent %q: migration failed: %w", name, err)
		}
	}

	pkgPath := cfg.GnoVM.PkgPath
	if pkgPath == "" {
		pkgPath = "gno/agent"
	}

	gnoSource, err := os.ReadFile(cfg.GnoVM.SourceFile)
	if err != nil {
		return nil, fmt.Errorf("gnogent %q: gno source file not found: %w", name, err)
	}

	vmWrapper, err := gnovm.NewGnoMachineWrapper(pkgPath, string(gnoSource))
	if err != nil {
		return nil, fmt.Errorf("gnogent %q: GnoVM failed to boot: %w", name, err)
	}

	thawTool, err := functiontool.New(
		functiontool.Config{
			Name:        "thaw_state",
			Description: "Restore the GnoVM brain state for a user from Postgres.",
		},
		func(ctx tool.Context, args ThawArgs) (ThawResult, error) {
			var session storage.AgentSession
			if err := db.Where("user_id = ?", args.UserID).First(&session).Error; err != nil {
				return ThawResult{Status: "new_session"}, nil
			}
			if err := vmWrapper.RestoreState(session.VMState); err != nil {
				return ThawResult{}, fmt.Errorf("thaw failure: %v", err)
			}
			return ThawResult{Status: "restored"}, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("gnogent %q: failed to create thaw tool: %w", name, err)
	}

	freezeTool, err := functiontool.New(
		functiontool.Config{
			Name:        "freeze_state",
			Description: "Persist the current GnoVM brain state to Postgres for a user.",
		},
		func(ctx tool.Context, args FreezeArgs) (FreezeResult, error) {
			blob, err := vmWrapper.ExportState()
			if err != nil {
				return FreezeResult{}, fmt.Errorf("freeze failure: %v", err)
			}
			friendship, _ := vmWrapper.Friendship()
			mood, _ := vmWrapper.Mood()
			session := storage.AgentSession{
				UserID:          args.UserID,
				VMState:         blob,
				FriendshipScore: friendship,
				MoodTag:         mood,
			}
			db.Save(&session)
			return FreezeResult{Status: "saved"}, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("gnogent %q: failed to create freeze tool: %w", name, err)
	}

	brainTool, err := functiontool.New(
		functiontool.Config{
			Name:        "query_brain",
			Description: "Query the GnoVM brain for the agent's current mood, friendship level, and system context.",
		},
		func(ctx tool.Context, args BrainQueryArgs) (BrainQueryResult, error) {
			if err := vmWrapper.SyncState(args.Query, 0); err != nil {
				return BrainQueryResult{}, err
			}
			sysCtx, _ := vmWrapper.GetSystemContext()
			friendship, _ := vmWrapper.Friendship()
			mood, _ := vmWrapper.Mood()
			return BrainQueryResult{
				Mood:       mood,
				Friendship: friendship,
				Context:    sysCtx,
			}, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("gnogent %q: failed to create brain tool: %w", name, err)
	}

	m, err := models.Get(ctx, cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("gnogent %q: failed to get model: %w", name, err)
	}

	gnogentTools := []tool.Tool{thawTool, freezeTool, brainTool}

	if len(cfg.Tools) > 0 && tools != nil {
		extra, err := tools.GetMultiple(ctx, cfg.Tools)
		if err != nil {
			return nil, fmt.Errorf("gnogent %q: failed to get tools: %w", name, err)
		}
		gnogentTools = append(gnogentTools, extra...)
	}

	return llmagent.New(llmagent.Config{
		Name:        name,
		Model:       m,
		Description: cfg.Description,
		Instruction: cfg.Instruction,
		Tools:       gnogentTools,
		SubAgents:   sub,
	})
}

func init() {
	registry.RegisterAgentType("gnogent", gnogentCreator)
}
