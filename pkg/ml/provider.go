package ml

import (
	"context"

	"github.com/innomon/agentic/pkg/registry"
	"google.golang.org/adk/v2/model"
)

func init() {
	registry.RegisterModelProvider("ml", mlCreator)
}

func mlCreator(_ context.Context, cfg *MLConfig) (model.LLM, error) {
	return NewMLModel(cfg)
}
