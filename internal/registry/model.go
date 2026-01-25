package registry

import (
	"context"
	"fmt"
	"sync"

	"github.com/innomon/med-agent/internal/config"
	"google.golang.org/adk/model"
)

type ModelRegistry struct {
	cfg    *config.Config
	models map[string]model.LLM
	mu     sync.RWMutex
}

func NewModelRegistry(cfg *config.Config) *ModelRegistry {
	return &ModelRegistry{
		cfg:    cfg,
		models: make(map[string]model.LLM),
	}
}

func (r *ModelRegistry) Get(ctx context.Context, name string) (model.LLM, error) {
	r.mu.RLock()
	if m, ok := r.models[name]; ok {
		r.mu.RUnlock()
		return m, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if m, ok := r.models[name]; ok {
		return m, nil
	}

	modelCfg, err := r.cfg.GetModel(name)
	if err != nil {
		return nil, err
	}

	m, err := r.createModel(ctx, modelCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create model %q: %w", name, err)
	}

	r.models[name] = m
	return m, nil
}

func (r *ModelRegistry) GetDefault(ctx context.Context) (model.LLM, error) {
	name, _, err := r.cfg.GetDefaultModel()
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, name)
}

func (r *ModelRegistry) createModel(ctx context.Context, cfg config.ModelConfig) (model.LLM, error) {
	creator, ok := GetModelCreator(cfg.Provider)
	if !ok {
		return nil, fmt.Errorf("unsupported model provider: %s", cfg.Provider)
	}
	return creator(ctx, cfg)
}
