package okf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/innomon/agentic/pkg/registry"
)

// fileOpsHandler performs directory listing, file reading, and catalog inspection for OKF.
func fileOpsHandler(_ context.Context, args map[string]any) (any, error) {
	op, _ := args["op"].(string)
	if op == "" {
		op = "list"
	}

	targetPath, _ := args["path"].(string)
	if targetPath == "" {
		targetPath = "."
	}

	if !filepath.IsAbs(targetPath) && registry.BasePath != "" {
		targetPath = filepath.Join(registry.BasePath, targetPath)
	}

	switch strings.ToLower(op) {
	case "list":
		entries, err := os.ReadDir(targetPath)
		if err != nil {
			return nil, fmt.Errorf("failed to list directory %q: %w", targetPath, err)
		}
		var files []map[string]any
		for _, e := range entries {
			info, _ := e.Info()
			var size int64
			if info != nil {
				size = info.Size()
			}
			files = append(files, map[string]any{
				"name":   e.Name(),
				"is_dir": e.IsDir(),
				"size":   size,
			})
		}
		return map[string]any{"path": targetPath, "entries": files}, nil

	case "read":
		data, err := os.ReadFile(targetPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %q: %w", targetPath, err)
		}
		return map[string]any{"path": targetPath, "content": string(data)}, nil

	default:
		return nil, fmt.Errorf("unsupported op %q, supported ops are 'list' and 'read'", op)
	}
}

func init() {
	registry.RegisterToolHandler("okf_file_ops", fileOpsHandler)
}
