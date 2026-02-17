package gomlx

import (
	"fmt"

	"github.com/innomon/agentic/internal/registry"
)

type GoMLXConfig struct {
	registry.ModelBase `yaml:",inline"`
	ModelPath          string `yaml:"model_path"`
	Backend            string `yaml:"backend"`
	BackendConfig      string `yaml:"backend_config"`
	ContextLength      int    `yaml:"context_length"`
	Threads            int    `yaml:"threads"`
	TokenizerPath      string `yaml:"tokenizer_path"`
	MemoryBudgetMB     int    `yaml:"memory_budget_mb"`
}

func (c *GoMLXConfig) Validate() error {
	if err := c.ModelBase.Validate(); err != nil {
		return err
	}
	if c.ModelPath == "" {
		return fmt.Errorf("model_path is required for gomlx provider")
	}
	return nil
}
