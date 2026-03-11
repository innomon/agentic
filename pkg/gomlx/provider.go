package gomlx

import (
	"context"

	"github.com/innomon/agentic/pkg/registry"
	"google.golang.org/adk/model"
)

func init() {
	registry.RegisterModelProvider("gomlx", gomlxCreator)
}

func gomlxCreator(_ context.Context, cfg *GoMLXConfig) (model.LLM, error) {
	return NewGoMLXModel(cfg)
}
