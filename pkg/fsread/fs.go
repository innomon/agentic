package fsread

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/innomon/agentic/pkg/registry"
)

// fsReadHandler implements the 'fs_read' builtin tool.
func fsReadHandler(_ context.Context, args map[string]any) (any, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("missing required parameter 'path'")
	}

	if !filepath.IsAbs(path) && registry.BasePath != "" {
		path = filepath.Join(registry.BasePath, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", path, err)
	}
	return string(data), nil
}

func init() {
	registry.RegisterToolHandler("fs_read", fsReadHandler)
}
