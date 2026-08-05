package okf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/innomon/agentic/pkg/registry"
)

// Chunk represents an OKF text chunk with extracted metadata.
type Chunk struct {
	ID         string   `json:"id"`
	Content    string   `json:"content"`
	Source     string   `json:"source"`
	Tags       []string `json:"tags"`
	Taxonomies []string `json:"taxonomies"`
}

// ragChunkerHandler splits input document text into RAG chunks retaining metadata.
func ragChunkerHandler(_ context.Context, args map[string]any) (any, error) {
	path, _ := args["path"].(string)
	text, _ := args["text"].(string)

	if text == "" && path != "" {
		targetPath := path
		if !filepath.IsAbs(targetPath) && registry.BasePath != "" {
			targetPath = filepath.Join(registry.BasePath, targetPath)
		}
		b, err := os.ReadFile(targetPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %q: %w", targetPath, err)
		}
		text = string(b)
	}

	if text == "" {
		return nil, fmt.Errorf("either 'path' or 'text' parameter is required")
	}

	chunks := parseMetadataChunks(text, path)
	return map[string]any{
		"total_chunks": len(chunks),
		"chunks":       chunks,
	}, nil
}

// fulltextIndexHandler indexes and performs keyword/tag search across document chunks.
func fulltextIndexHandler(_ context.Context, args map[string]any) (any, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("missing required parameter 'query'")
	}

	text, _ := args["text"].(string)
	path, _ := args["path"].(string)

	if text == "" && path != "" {
		targetPath := path
		if !filepath.IsAbs(targetPath) && registry.BasePath != "" {
			targetPath = filepath.Join(registry.BasePath, targetPath)
		}
		b, err := os.ReadFile(targetPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %q: %w", targetPath, err)
		}
		text = string(b)
	}

	if text == "" {
		// Fallback to taxonomy.md if no text/path provided
		targetPath := "taxonomy.md"
		if registry.BasePath != "" {
			targetPath = filepath.Join(registry.BasePath, targetPath)
		}
		b, err := os.ReadFile(targetPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read default taxonomy.md: %w", err)
		}
		text = string(b)
		path = "taxonomy.md"
	}

	chunks := parseMetadataChunks(text, path)
	var matches []Chunk
	queryTerms := strings.Fields(strings.ToLower(query))

	for _, c := range chunks {
		cText := strings.ToLower(c.Content)
		matchCount := 0
		for _, q := range queryTerms {
			if strings.Contains(cText, q) {
				matchCount++
			}
		}
		if matchCount > 0 {
			matches = append(matches, c)
		}
	}

	return map[string]any{
		"query":         query,
		"match_count":   len(matches),
		"matched_chunks": matches,
	}, nil
}

func parseMetadataChunks(text, source string) []Chunk {
	paragraphs := strings.Split(text, "\n\n")
	var chunks []Chunk
	tagRegex := regexp.MustCompile(`(?:#|` + "`" + `)([a-zA-Z0-9_-]+)(?:` + "`" + `)?`)

	for idx, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}

		rawMatches := tagRegex.FindAllStringSubmatch(trimmed, -1)
		var tags []string
		tagMap := make(map[string]bool)
		for _, m := range rawMatches {
			if len(m) > 1 {
				t := strings.ToLower(m[1])
				if !tagMap[t] {
					tagMap[t] = true
					tags = append(tags, t)
				}
			}
		}

		chunks = append(chunks, Chunk{
			ID:      fmt.Sprintf("chunk-%d", idx+1),
			Content: trimmed,
			Source:  source,
			Tags:    tags,
		})
	}
	return chunks
}

func init() {
	registry.RegisterToolHandler("okf_rag_chunker", ragChunkerHandler)
	registry.RegisterToolHandler("okf_fulltext_index", fulltextIndexHandler)
}
