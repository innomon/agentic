// Package okf provides deterministic tools and workflows for OKF (Open Knowledge Format) operations.
package okf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/innomon/agentic/pkg/registry"
)

// getTaxonomyHandler retrieves the contents of canonical taxonomy.md using workspace-root relative paths.
func getTaxonomyHandler(_ context.Context, args map[string]any) (any, error) {
	relPath := "taxonomy.md"
	if p, ok := args["path"].(string); ok && p != "" {
		relPath = p
	}

	targetPath := relPath
	if !filepath.IsAbs(targetPath) && registry.BasePath != "" {
		targetPath = filepath.Join(registry.BasePath, targetPath)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read taxonomy file %q: %w", targetPath, err)
	}

	return map[string]any{
		"path":    relPath,
		"content": string(data),
	}, nil
}

func init() {
	registry.RegisterToolHandler("okf_get_taxonomy", getTaxonomyHandler)
}
