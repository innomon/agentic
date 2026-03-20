package prologmem

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/innomon/agentic/pkg/registry"
	adkmemory "google.golang.org/adk/memory"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// LogicQueryToolConfig is the YAML config for a logic_query tool.
type LogicQueryToolConfig struct {
	registry.ToolBase `yaml:",inline"`
	KBPath            string `yaml:"kb_path"`
	TimeoutSeconds    int    `yaml:"timeout_seconds"`
}

var (
	sharedMu      sync.Mutex
	sharedEngines = map[string]*PrologMemory{}
)

// getOrCreateEngine returns a singleton PrologMemory per kb_path.
func getOrCreateEngine(kbPath string, timeout time.Duration) (*PrologMemory, error) {
	sharedMu.Lock()
	defer sharedMu.Unlock()

	if pm, ok := sharedEngines[kbPath]; ok {
		return pm, nil
	}

	pm, err := New(kbPath, WithTimeout(timeout))
	if err != nil {
		return nil, err
	}
	sharedEngines[kbPath] = pm
	return pm, nil
}

func logicQueryToolCreator(_ context.Context, name string, cfg *LogicQueryToolConfig, _ registry.SandboxRegistry) (tool.Tool, error) {
	timeout := DefaultTimeout
	if cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}

	pm, err := getOrCreateEngine(cfg.KBPath, timeout)
	if err != nil {
		return nil, fmt.Errorf("logic_query tool %q: %w", name, err)
	}

	return functiontool.New(functiontool.Config{
		Name:        name,
		Description: cfg.Description,
	}, func(ctx tool.Context, args map[string]any) (any, error) {
		action, _ := args["action"].(string)
		query, _ := args["query"].(string)
		if query == "" {
			return nil, fmt.Errorf("missing required parameter 'query'")
		}

		switch action {
		case "assert":
			if err := pm.Assert(query); err != nil {
				return map[string]any{"success": false, "error": err.Error()}, nil
			}
			return map[string]any{"success": true}, nil

		case "retract":
			if err := pm.Retract(query); err != nil {
				return map[string]any{"success": false, "error": err.Error()}, nil
			}
			return map[string]any{"success": true}, nil

		case "check":
			ok, err := pm.Check(query)
			if err != nil {
				return map[string]any{"result": false, "error": err.Error()}, nil
			}
			return map[string]any{"result": ok}, nil

		case "query", "":
			results, err := pm.Query(query)
			if err != nil {
				return map[string]any{"solutions": nil, "error": err.Error()}, nil
			}
			if len(results) == 0 {
				return map[string]any{"solutions": "no"}, nil
			}
			b, _ := json.Marshal(results)
			return map[string]any{"solutions": json.RawMessage(b)}, nil

		case "save":
			if err := pm.Save(); err != nil {
				return map[string]any{"success": false, "error": err.Error()}, nil
			}
			return map[string]any{"success": true}, nil

		default:
			return nil, fmt.Errorf("unknown action %q (use: query, assert, retract, check, save)", action)
		}
	})
}

func prologMemoryCreator(_ context.Context, cfg *registry.MemoryConfig) (adkmemory.Service, error) {
	if cfg.KBPath == "" {
		return nil, fmt.Errorf("prolog memory provider requires kb_path")
	}
	pm, err := New(cfg.KBPath, WithTimeout(10*time.Second))
	if err != nil {
		return nil, fmt.Errorf("failed to create prolog memory: %w", err)
	}
	return NewService(pm), nil
}

func init() {
	registry.RegisterToolType("logic_query", logicQueryToolCreator)
	registry.RegisterProvider("memory", "prolog",
		registry.ProviderCreator[registry.MemoryConfig, adkmemory.Service](prologMemoryCreator))
}
