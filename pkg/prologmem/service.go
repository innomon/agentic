package prologmem

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/adk/v2/memory"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

// Service wraps PrologMemory to implement the ADK memory.Service interface.
type Service struct {
	pm *PrologMemory
}

// NewService creates an ADK memory.Service backed by a PrologMemory instance.
func NewService(pm *PrologMemory) *Service {
	return &Service{pm: pm}
}

// AddSessionToMemory ingests a session's LLM responses as mem_context facts.
func (s *Service) AddSessionToMemory(_ context.Context, sess session.Session) error {
	sid := sess.ID()
	for event := range sess.Events().All() {
		if event.LLMResponse.Content == nil {
			continue
		}
		var parts []string
		for _, part := range event.LLMResponse.Content.Parts {
			if part.Text != "" {
				parts = append(parts, part.Text)
			}
		}
		if len(parts) == 0 {
			continue
		}

		text := strings.Join(parts, " ")
		// Escape single quotes for Prolog atom.
		escaped := strings.ReplaceAll(text, "'", "\\'")
		ts := event.Timestamp.Format(time.RFC3339)

		fact := fmt.Sprintf("mem_context('%s', '%s', '%s')", sid, ts, escaped)
		if err := s.pm.Assert(fact); err != nil {
			return fmt.Errorf("asserting session event: %w", err)
		}
	}
	return nil
}

// SearchMemory queries mem_fact and mem_rel predicates for keyword matches.
func (s *Service) SearchMemory(_ context.Context, req *memory.SearchRequest) (*memory.SearchResponse, error) {
	queryWords := extractQueryWords(req.Query)
	if len(queryWords) == 0 {
		return &memory.SearchResponse{}, nil
	}

	// Search mem_context for matching content.
	results, err := s.pm.Query("mem_context(SID, TS, Data).")
	if err != nil {
		return &memory.SearchResponse{}, nil
	}

	resp := &memory.SearchResponse{}
	for _, r := range results {
		data, _ := r["Data"].(string)
		if data == "" {
			continue
		}
		if matchesAnyWord(data, queryWords) {
			resp.Memories = append(resp.Memories, memory.Entry{
				Author:  "model",
				Content: textContent(data),
			})
		}
	}

	return resp, nil
}

func extractQueryWords(query string) []string {
	var words []string
	for _, w := range strings.Fields(query) {
		w = strings.ToLower(w)
		if w != "" {
			words = append(words, w)
		}
	}
	return words
}

func matchesAnyWord(text string, words []string) bool {
	lower := strings.ToLower(text)
	for _, w := range words {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

func textContent(text string) *genai.Content {
	return &genai.Content{
		Parts: []*genai.Part{genai.NewPartFromText(text)},
		Role:  "model",
	}
}
